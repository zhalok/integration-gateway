package enrichments

import (
	"errors"
	"testing"

	"github.com/zhalok/integration-gateway/internal/domains/cases"
	"github.com/zhalok/integration-gateway/internal/worker"
)

// --- mocks ---

type mockEnrichmentService struct {
	getByCaseID        func(caseID string) (*Enrichment, error)
	create             func(caseID string, courtCaseNumber *string) (*Enrichment, error)
	resetFailedSources func(e *Enrichment) error
}

func (m *mockEnrichmentService) GetByCaseID(caseID string) (*Enrichment, error) {
	return m.getByCaseID(caseID)
}
func (m *mockEnrichmentService) Create(caseID string, courtCaseNumber *string) (*Enrichment, error) {
	return m.create(caseID, courtCaseNumber)
}
func (m *mockEnrichmentService) ResetFailedSources(e *Enrichment) error {
	return m.resetFailedSources(e)
}
func (m *mockEnrichmentService) ComputeAndUpdateStatus(enrichmentID int64) error { return nil }
func (m *mockEnrichmentService) ProcessEnrichment(enrichmentID int64, caseID string) error {
	return nil
}

type mockCaseService struct {
	getCase func(id string) (*cases.Case, error)
}

func (m *mockCaseService) GetCase(id string) (*cases.Case, error) { return m.getCase(id) }
func (m *mockCaseService) GetAllCases() ([]*cases.Case, error)    { return nil, nil }

// newUsecase builds a usecase with a buffered job channel and returns both.
func newTestUsecase(svc Service, caseSvc cases.Service) (Usecase, chan worker.Job) {
	jobs := make(chan worker.Job, 1)
	return NewUsecase(svc, caseSvc, jobs), jobs
}

// --- tests ---

func TestCreateEnrichment_CaseNotFound(t *testing.T) {
	u, _ := newTestUsecase(
		&mockEnrichmentService{},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) { return nil, nil }},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if code != 404 || err == nil {
		t.Fatalf("expected 404, got %d: %v", code, err)
	}
}

func TestCreateEnrichment_CaseLookupError(t *testing.T) {
	u, _ := newTestUsecase(
		&mockEnrichmentService{},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return nil, errors.New("db down")
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if code != 500 || err == nil {
		t.Fatalf("expected 500, got %d: %v", code, err)
	}
}

func TestCreateEnrichment_EnrichmentLookupError(t *testing.T) {
	u, _ := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) {
				return nil, errors.New("db down")
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if code != 500 || err == nil {
		t.Fatalf("expected 500, got %d: %v", code, err)
	}
}

func TestCreateEnrichment_ExistingComplete(t *testing.T) {
	existing := &Enrichment{ID: 1, Status: StatusComplete}
	resetCalled := false
	u, jobs := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return existing, nil },
			resetFailedSources: func(e *Enrichment) error {
				resetCalled = true
				return nil
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	e, code, err := u.CreateEnrichment("case-1")
	if err != nil || code != 200 || e != existing {
		t.Fatalf("expected 200 with existing, got %d: %v", code, err)
	}
	if resetCalled {
		t.Fatal("ResetFailedSources must not be called for complete enrichment")
	}
	if len(jobs) != 0 {
		t.Fatal("job must not be enqueued for complete enrichment")
	}
}

func TestCreateEnrichment_ExistingPending(t *testing.T) {
	existing := &Enrichment{ID: 2, Status: StatusPending}
	u, jobs := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return existing, nil },
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	e, code, err := u.CreateEnrichment("case-1")
	if err != nil || code != 202 || e != existing {
		t.Fatalf("expected 202 with existing, got %d: %v", code, err)
	}
	if len(jobs) != 0 {
		t.Fatal("job must not be enqueued for already-pending enrichment")
	}
}

func TestCreateEnrichment_ExistingPartial_ResetsAndEnqueues(t *testing.T) {
	existing := &Enrichment{ID: 3, Status: StatusPartial}
	resetCalled := false
	u, jobs := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return existing, nil },
			resetFailedSources: func(e *Enrichment) error {
				resetCalled = true
				return nil
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if err != nil || code != 202 {
		t.Fatalf("expected 202, got %d: %v", code, err)
	}
	if !resetCalled {
		t.Fatal("ResetFailedSources must be called for partial enrichment")
	}
	if len(jobs) != 1 {
		t.Fatal("job must be enqueued for partial enrichment")
	}
	job := <-jobs
	if job.EnrichmentID != existing.ID {
		t.Fatalf("expected enrichmentID=%d, got %d", existing.ID, job.EnrichmentID)
	}
}

func TestCreateEnrichment_ExistingFailed_ResetsAndEnqueues(t *testing.T) {
	existing := &Enrichment{ID: 4, Status: StatusFailed}
	resetCalled := false
	u, jobs := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return existing, nil },
			resetFailedSources: func(e *Enrichment) error {
				resetCalled = true
				return nil
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if err != nil || code != 202 {
		t.Fatalf("expected 202, got %d: %v", code, err)
	}
	if !resetCalled {
		t.Fatal("ResetFailedSources must be called for failed enrichment")
	}
	if len(jobs) != 1 {
		t.Fatal("job must be enqueued for failed enrichment")
	}
}

func TestCreateEnrichment_ExistingPartial_ResetError(t *testing.T) {
	existing := &Enrichment{ID: 5, Status: StatusPartial}
	u, _ := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return existing, nil },
			resetFailedSources: func(e *Enrichment) error {
				return errors.New("db down")
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if code != 500 || err == nil {
		t.Fatalf("expected 500 on reset error, got %d: %v", code, err)
	}
}

func TestCreateEnrichment_NoExisting_CreatesAndEnqueues(t *testing.T) {
	created := &Enrichment{ID: 10, Status: StatusPending}
	u, jobs := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return nil, nil },
			create: func(caseID string, courtCaseNumber *string) (*Enrichment, error) {
				return created, nil
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	e, code, err := u.CreateEnrichment("case-1")
	if err != nil || code != 202 || e != created {
		t.Fatalf("expected 202 with new enrichment, got %d: %v", code, err)
	}
	if len(jobs) != 1 {
		t.Fatal("job must be enqueued for new enrichment")
	}
	job := <-jobs
	if job.EnrichmentID != created.ID {
		t.Fatalf("expected enrichmentID=%d, got %d", created.ID, job.EnrichmentID)
	}
}

func TestCreateEnrichment_NoExisting_CreateError(t *testing.T) {
	u, _ := newTestUsecase(
		&mockEnrichmentService{
			getByCaseID: func(caseID string) (*Enrichment, error) { return nil, nil },
			create: func(caseID string, courtCaseNumber *string) (*Enrichment, error) {
				return nil, errors.New("db down")
			},
		},
		&mockCaseService{getCase: func(id string) (*cases.Case, error) {
			return &cases.Case{ID: id}, nil
		}},
	)
	_, code, err := u.CreateEnrichment("case-1")
	if code != 500 || err == nil {
		t.Fatalf("expected 500 on create error, got %d: %v", code, err)
	}
}
