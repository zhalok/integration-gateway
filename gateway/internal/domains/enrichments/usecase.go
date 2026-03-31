package enrichments

import (
	"fmt"
	"log"

	"github.com/zhalok/integration-gateway/internal/domains/cases"
	"github.com/zhalok/integration-gateway/internal/worker"
)

type Usecase interface {
	CreateEnrichment(caseID string) (*Enrichment, int, error)
	GetEnrichment(caseID string) (*Enrichment, error)
}

type usecase struct {
	svc     Service
	caseSvc cases.Service
	jobs    chan<- worker.Job
}

func NewUsecase(svc Service, caseSvc cases.Service, jobs chan<- worker.Job) Usecase {
	return &usecase{
		svc:     svc,
		caseSvc: caseSvc,
		jobs:    jobs,
	}
}

// CreateEnrichment creates or resumes an enrichment for the given case.
// Returns the enrichment, the HTTP status code to respond with, and any error.
//
// Idempotency rules:
//   - complete              → 200, return existing
//   - pending (in progress) → 202, return current state
//   - partial / failed      → reset failed sources, re-queue, 202
//   - not exists            → create, queue, 202
func (u *usecase) CreateEnrichment(caseID string) (*Enrichment, int, error) {
	c, err := u.caseSvc.GetCase(caseID)
	if err != nil {
		return nil, 500, fmt.Errorf("lookup case: %w", err)
	}
	if c == nil {
		return nil, 404, fmt.Errorf("case not found: %s", caseID)
	}

	existing, err := u.svc.GetByCaseID(caseID)
	if err != nil {
		return nil, 500, fmt.Errorf("lookup enrichment: %w", err)
	}

	if existing != nil {
		switch existing.Status {
		case StatusComplete:
			log.Printf("usecase: enrichment already complete caseID=%s enrichmentID=%d", caseID, existing.ID)
			return existing, 200, nil

		case StatusPending:
			log.Printf("usecase: enrichment already in progress caseID=%s enrichmentID=%d", caseID, existing.ID)
			return existing, 202, nil

		case StatusPartial, StatusFailed:
			log.Printf("usecase: re-queuing enrichment caseID=%s enrichmentID=%d status=%s", caseID, existing.ID, existing.Status)
			if err := u.svc.ResetFailedSources(existing); err != nil {
				return nil, 500, fmt.Errorf("reset enrichment: %w", err)
			}
			u.enqueue(existing.ID, caseID)
			return existing, 202, nil
		}
	}

	e, err := u.svc.Create(caseID, c.CourtCaseNumber)
	if err != nil {
		return nil, 500, fmt.Errorf("create enrichment: %w", err)
	}
	log.Printf("usecase: enrichment created caseID=%s enrichmentID=%d", caseID, e.ID)
	u.enqueue(e.ID, caseID)
	return e, 202, nil
}

func (u *usecase) GetEnrichment(caseID string) (*Enrichment, error) {
	e, err := u.svc.GetByCaseID(caseID)
	if err != nil {
		return nil, fmt.Errorf("get enrichment: %w", err)
	}
	return e, nil
}

// enqueue pushes a job onto the channel non-blocking.
// Drops silently if the channel is full — the worker catches up via DB state.
func (u *usecase) enqueue(enrichmentID int64, caseID string) {
	select {
	case u.jobs <- worker.Job{EnrichmentID: enrichmentID, CaseID: caseID}:
		log.Printf("usecase: job enqueued enrichmentID=%d caseID=%s", enrichmentID, caseID)
	default:
		log.Printf("usecase: job channel full, dropping enqueue enrichmentID=%d caseID=%s", enrichmentID, caseID)
	}
}
