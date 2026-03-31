package enrichments

import (
	"fmt"
	"log"
	"time"

	"github.com/zhalok/integration-gateway/internal/circuitbreaker"
	"github.com/zhalok/integration-gateway/internal/clients"
	"github.com/zhalok/integration-gateway/internal/domains/cases"
	"github.com/zhalok/integration-gateway/internal/worker"
)

// propertyFetcher is the subset of clients.PropertyClient used by the service.
type propertyFetcher interface {
	Fetch(state, county, parcelID string) clients.PropertyResult
}

// courtFetcher is the subset of clients.CourtClient used by the service.
type courtFetcher interface {
	Fetch(caseNumber string) clients.CourtResult
}

// scraFetcher is the subset of clients.SCRAClient used by the service.
type scraFetcher interface {
	Submit(lastName, firstName, ssnLast4, dob string) clients.SCRAResult
	Poll(searchID string) clients.SCRAResult
}

// Service owns the enrichment lifecycle:
// creating enrichments, resetting failed sources, computing overall status,
// and processing enrichment by calling the external clients.
type Service interface {
	GetByCaseID(caseID string) (*Enrichment, error)
	Create(caseID string, courtCaseNumber *string) (*Enrichment, error)
	ResetFailedSources(e *Enrichment) error
	ComputeAndUpdateStatus(enrichmentID int64) error
	// ProcessEnrichment satisfies worker.Processor — called by the worker pool.
	ProcessEnrichment(enrichmentID int64, caseID string) error
}

type service struct {
	repo           Repository
	caseRepo       cases.Repository
	propertyClient propertyFetcher
	courtClient    courtFetcher
	scraClient     scraFetcher
	cbs            *circuitbreaker.Set
	jobs           chan<- worker.Job
}

func NewService(
	repo Repository,
	caseRepo cases.Repository,
	propertyClient *clients.PropertyClient,
	courtClient *clients.CourtClient,
	scraClient *clients.SCRAClient,
	cbs *circuitbreaker.Set,
	jobs chan<- worker.Job,
) Service {
	return &service{
		repo:           repo,
		caseRepo:       caseRepo,
		propertyClient: propertyClient,
		courtClient:    courtClient,
		scraClient:     scraClient,
		cbs:            cbs,
		jobs:           jobs,
	}
}

func (s *service) GetByCaseID(caseID string) (*Enrichment, error) {
	e, err := s.repo.GetByCaseID(caseID)
	if err != nil {
		return nil, fmt.Errorf("get enrichment: %w", err)
	}
	return e, nil
}

func (s *service) Create(caseID string, courtCaseNumber *string) (*Enrichment, error) {
	e, err := s.repo.Create(caseID, courtCaseNumber)
	if err != nil {
		return nil, fmt.Errorf("create enrichment: %w", err)
	}
	return e, nil
}

// ResetFailedSources sets failed per-source statuses back to pending and persists.
func (s *service) ResetFailedSources(e *Enrichment) error {
	if e.PRStatus == SourceFailed {
		e.PRStatus = SourcePending
		e.PRAttempts = 0
	}
	if e.CRStatus == SourceFailed {
		e.CRStatus = SourcePending
		e.CRAttempts = 0
	}
	if e.SCRAStatus == SourceFailed {
		e.SCRAStatus = SourcePending
		e.SCRAAttempts = 0
	}
	e.Status = StatusPending

	if err := s.repo.Update(e); err != nil {
		return fmt.Errorf("reset failed sources: %w", err)
	}
	return nil
}

func (s *service) ComputeAndUpdateStatus(enrichmentID int64) error {
	if err := s.repo.ComputeAndUpdateOverallStatus(enrichmentID); err != nil {
		return fmt.Errorf("compute status: %w", err)
	}
	return nil
}

// ProcessEnrichment is called by the worker pool for each job.
// It loads both the enrichment and its case, attempts each pending source,
// updates the DB, and re-queues if any source is still pending.
func (s *service) ProcessEnrichment(enrichmentID int64, caseID string) error {
	e, err := s.repo.GetByID(enrichmentID)
	if err != nil {
		return fmt.Errorf("load enrichment: %w", err)
	}
	if e == nil {
		return fmt.Errorf("enrichment not found: %d", enrichmentID)
	}

	c, err := s.caseRepo.GetByID(caseID)
	if err != nil {
		return fmt.Errorf("load case: %w", err)
	}
	if c == nil {
		return fmt.Errorf("case not found: %s", caseID)
	}

	s.attemptPropertyRecords(e, c)
	s.attemptCourtRecords(e, c)
	s.attemptSCRA(e, c)

	if err := s.repo.Update(e); err != nil {
		return fmt.Errorf("update enrichment: %w", err)
	}

	if err := s.repo.ComputeAndUpdateOverallStatus(e.ID); err != nil {
		return fmt.Errorf("compute status: %w", err)
	}

	// Re-queue if any applicable source is still pending
	if e.PRStatus == SourcePending || e.CRStatus == SourcePending || e.SCRAStatus == SourcePending {
		log.Printf("service: re-queuing enrichmentID=%d pr=%s cr=%s scra=%s", e.ID, e.PRStatus, e.CRStatus, e.SCRAStatus)
		time.Sleep(worker.PollInterval)
		select {
		case s.jobs <- worker.Job{EnrichmentID: e.ID, CaseID: caseID}:
		default:
			log.Printf("service: job channel full, dropping re-queue for enrichmentID=%d", e.ID)
		}
	}

	return nil
}

// attemptPropertyRecords fetches property data if the source is still actionable.
func (s *service) attemptPropertyRecords(e *Enrichment, c *cases.Case) {
	if !s.shouldAttempt(e.PRStatus, e.PRAttempts, e.PRLastAttempt, e.PRRetryAfter) {
		return
	}
	if !s.cbs.PropertyRecords.Allow() {
		log.Printf("service: property records circuit open, skipping enrichmentID=%d", e.ID)
		return
	}

	now := time.Now()
	e.PRAttempts++
	e.PRLastAttempt = &now

	result := s.propertyClient.Fetch(c.PropertyState, c.PropertyCounty, c.PropertyParcelID)
	if result.Err == nil {
		e.PRStatus = SourceSuccess
		e.PRData = result.Data
		s.cbs.PropertyRecords.Success()
		log.Printf("service: property records success enrichmentID=%d", e.ID)
	} else if result.Permanent {
		e.PRStatus = SourceFailed
		reason := result.Err.Error()
		e.PRReason = &reason
		s.cbs.PropertyRecords.Success() // 404 is not a service failure
		log.Printf("service: property records permanent failure enrichmentID=%d: %v", e.ID, result.Err)
	} else {
		s.cbs.PropertyRecords.Failure()
		log.Printf("service: property records transient error enrichmentID=%d: %v", e.ID, result.Err)
	}
}

// attemptCourtRecords fetches court data if the source is still actionable.
func (s *service) attemptCourtRecords(e *Enrichment, c *cases.Case) {
	if e.CRStatus == SourceNotApplicable {
		return
	}
	if c.CourtCaseNumber == nil {
		return
	}
	if !s.shouldAttempt(e.CRStatus, e.CRAttempts, e.CRLastAttempt, e.CRRetryAfter) {
		return
	}
	if !s.cbs.CourtRecords.Allow() {
		log.Printf("service: court records circuit open, skipping enrichmentID=%d", e.ID)
		return
	}

	now := time.Now()
	e.CRAttempts++
	e.CRLastAttempt = &now

	result := s.courtClient.Fetch(*c.CourtCaseNumber)
	if result.Err == nil {
		e.CRStatus = SourceSuccess
		e.CRData = result.Data
		s.cbs.CourtRecords.Success()
		log.Printf("service: court records success enrichmentID=%d", e.ID)
	} else if result.Permanent {
		e.CRStatus = SourceFailed
		reason := result.Err.Error()
		e.CRReason = &reason
		s.cbs.CourtRecords.Success() // NoFilingFound is not a service failure
		log.Printf("service: court records permanent failure enrichmentID=%d: %v", e.ID, result.Err)
	} else {
		e.CRRetryAfter = result.RetryAfter
		s.cbs.CourtRecords.Failure()
		log.Printf("service: court records transient error enrichmentID=%d: %v", e.ID, result.Err)
	}
}

// attemptSCRA submits or polls SCRA depending on whether a search ID exists.
func (s *service) attemptSCRA(e *Enrichment, c *cases.Case) {
	if !s.shouldAttempt(e.SCRAStatus, e.SCRAAttempts, e.SCRALastAttempt, e.SCRARetryAfter) {
		return
	}
	if !s.cbs.SCRA.Allow() {
		log.Printf("service: scra circuit open, skipping enrichmentID=%d", e.ID)
		return
	}

	now := time.Now()
	e.SCRAAttempts++
	e.SCRALastAttempt = &now

	if e.SCRASearchID == nil {
		// Step 1: submit
		result := s.scraClient.Submit(c.BorrowerLastName, c.BorrowerFirstName, c.BorrowerSSNLast4, c.BorrowerDOB)
		if result.Err != nil {
			s.cbs.SCRA.Failure()
			log.Printf("service: scra submit error enrichmentID=%d: %v", e.ID, result.Err)
			return
		}
		e.SCRASearchID = result.SearchID
		s.cbs.SCRA.Success()
		log.Printf("service: scra submitted enrichmentID=%d searchID=%s", e.ID, *result.SearchID)
	} else {
		// Step 2: poll
		if e.SCRAAttempts >= worker.MaxPollAttempts {
			e.SCRAStatus = SourceFailed
			reason := "polling timed out"
			e.SCRAReason = &reason
			log.Printf("service: scra poll timed out enrichmentID=%d searchID=%s attempts=%d", e.ID, *e.SCRASearchID, e.SCRAAttempts)
			return
		}

		result := s.scraClient.Poll(*e.SCRASearchID)
		if result.Err != nil && result.Permanent {
			e.SCRAStatus = SourceFailed
			reason := result.Err.Error()
			e.SCRAReason = &reason
			s.cbs.SCRA.Success() // permanent error is not a service failure
			log.Printf("service: scra permanent failure enrichmentID=%d searchID=%s: %v", e.ID, *e.SCRASearchID, result.Err)
		} else if result.Err != nil {
			s.cbs.SCRA.Failure()
			log.Printf("service: scra poll error enrichmentID=%d searchID=%s: %v", e.ID, *e.SCRASearchID, result.Err)
		} else if result.Pending {
			// Still waiting — will re-queue
			s.cbs.SCRA.Success()
			log.Printf("service: scra poll still pending enrichmentID=%d searchID=%s attempts=%d", e.ID, *e.SCRASearchID, e.SCRAAttempts)
		} else {
			e.SCRAStatus = SourceSuccess
			e.SCRAData = result.Data
			s.cbs.SCRA.Success()
			log.Printf("service: scra success enrichmentID=%d searchID=%s", e.ID, *e.SCRASearchID)
		}
	}
}

// shouldAttempt returns true if the source is pending/failed, under the attempt cap,
// and enough time has passed since the last attempt.
func (s *service) shouldAttempt(status string, attempts int, lastAttempt *time.Time, retryAfter *time.Time) bool {
	if status == SourceSuccess || status == SourceNotApplicable {
		return false
	}
	if attempts >= worker.MaxAttempts {
		return false
	}
	if retryAfter != nil && time.Now().Before(*retryAfter) {
		return false
	}
	if lastAttempt != nil && time.Since(*lastAttempt) < worker.DefaultInterval {
		return false
	}
	return true
}
