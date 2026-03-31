package enrichments

import (
	"errors"
	"testing"
	"time"

	"github.com/zhalok/integration-gateway/internal/worker"
)

// --- mock repository ---

type mockRepository struct {
	getByID                       func(id int64) (*Enrichment, error)
	update                        func(e *Enrichment) error
	computeAndUpdateOverallStatus func(id int64) error
}

func (m *mockRepository) GetByID(id int64) (*Enrichment, error) {
	if m.getByID != nil {
		return m.getByID(id)
	}
	return nil, nil
}
func (m *mockRepository) GetByCaseID(caseID string) (*Enrichment, error)         { return nil, nil }
func (m *mockRepository) Create(caseID string, ccn *string) (*Enrichment, error) { return nil, nil }
func (m *mockRepository) Update(e *Enrichment) error {
	if m.update != nil {
		return m.update(e)
	}
	return nil
}
func (m *mockRepository) ComputeAndUpdateOverallStatus(id int64) error {
	if m.computeAndUpdateOverallStatus != nil {
		return m.computeAndUpdateOverallStatus(id)
	}
	return nil
}

func newTestService(repo Repository) *service {
	return &service{repo: repo}
}

// --- ResetFailedSources tests ---

func TestResetFailedSources_ResetsFailedSourcesOnly(t *testing.T) {
	e := &Enrichment{
		PRStatus:     SourceFailed,
		PRAttempts:   worker.MaxAttempts,
		CRStatus:     SourceSuccess, // must not be touched
		CRAttempts:   3,
		SCRAStatus:   SourceFailed,
		SCRAAttempts: worker.MaxAttempts,
		Status:       StatusPartial,
	}
	svc := newTestService(&mockRepository{update: func(e *Enrichment) error { return nil }})

	if err := svc.ResetFailedSources(e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.PRStatus != SourcePending {
		t.Errorf("PRStatus: want %q, got %q", SourcePending, e.PRStatus)
	}
	if e.PRAttempts != 0 {
		t.Errorf("PRAttempts: want 0, got %d", e.PRAttempts)
	}
	if e.CRStatus != SourceSuccess {
		t.Errorf("CRStatus must remain %q, got %q", SourceSuccess, e.CRStatus)
	}
	if e.CRAttempts != 3 {
		t.Errorf("CRAttempts must remain 3, got %d", e.CRAttempts)
	}
	if e.SCRAStatus != SourcePending {
		t.Errorf("SCRAStatus: want %q, got %q", SourcePending, e.SCRAStatus)
	}
	if e.SCRAAttempts != 0 {
		t.Errorf("SCRAAttempts: want 0, got %d", e.SCRAAttempts)
	}
	if e.Status != StatusPending {
		t.Errorf("overall Status: want %q, got %q", StatusPending, e.Status)
	}
}

func TestResetFailedSources_NotApplicableUntouched(t *testing.T) {
	e := &Enrichment{
		PRStatus:   SourceFailed,
		PRAttempts: worker.MaxAttempts,
		CRStatus:   SourceNotApplicable,
		CRAttempts: 0,
		SCRAStatus: SourceSuccess,
		Status:     StatusPartial,
	}
	svc := newTestService(&mockRepository{update: func(e *Enrichment) error { return nil }})

	if err := svc.ResetFailedSources(e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.CRStatus != SourceNotApplicable {
		t.Errorf("CRStatus must remain not_applicable, got %q", e.CRStatus)
	}
	if e.SCRAStatus != SourceSuccess {
		t.Errorf("SCRAStatus must remain success, got %q", e.SCRAStatus)
	}
}

func TestResetFailedSources_PropagatesRepoError(t *testing.T) {
	e := &Enrichment{PRStatus: SourceFailed, PRAttempts: 1}
	svc := newTestService(&mockRepository{update: func(e *Enrichment) error {
		return errors.New("db down")
	}})

	err := svc.ResetFailedSources(e)
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

// --- shouldAttempt tests ---

func TestShouldAttempt_PendingUnderLimit(t *testing.T) {
	svc := newTestService(nil)
	if !svc.shouldAttempt(SourcePending, 0, nil, nil) {
		t.Error("expected true for fresh pending source")
	}
}

func TestShouldAttempt_SuccessSkipped(t *testing.T) {
	svc := newTestService(nil)
	if svc.shouldAttempt(SourceSuccess, 0, nil, nil) {
		t.Error("expected false for already-succeeded source")
	}
}

func TestShouldAttempt_NotApplicableSkipped(t *testing.T) {
	svc := newTestService(nil)
	if svc.shouldAttempt(SourceNotApplicable, 0, nil, nil) {
		t.Error("expected false for not_applicable source")
	}
}

func TestShouldAttempt_AtMaxAttemptsBlocked(t *testing.T) {
	svc := newTestService(nil)
	if svc.shouldAttempt(SourcePending, worker.MaxAttempts, nil, nil) {
		t.Error("expected false when attempts == MaxAttempts")
	}
}

func TestShouldAttempt_AfterResetAttemptsZeroed(t *testing.T) {
	// This validates that zeroing attempts in ResetFailedSources unblocks shouldAttempt.
	svc := newTestService(nil)
	if !svc.shouldAttempt(SourcePending, 0, nil, nil) {
		t.Error("expected true after attempts reset to 0")
	}
}

func TestShouldAttempt_RetryAfterInFutureBlocked(t *testing.T) {
	svc := newTestService(nil)
	future := time.Now().Add(10 * time.Minute)
	if svc.shouldAttempt(SourcePending, 0, nil, &future) {
		t.Error("expected false when retryAfter is in the future")
	}
}

func TestShouldAttempt_RetryAfterInPastAllowed(t *testing.T) {
	svc := newTestService(nil)
	past := time.Now().Add(-10 * time.Minute)
	if !svc.shouldAttempt(SourcePending, 0, nil, &past) {
		t.Error("expected true when retryAfter is in the past")
	}
}

func TestShouldAttempt_LastAttemptTooRecentBlocked(t *testing.T) {
	svc := newTestService(nil)
	now := time.Now()
	if svc.shouldAttempt(SourcePending, 0, &now, nil) {
		t.Error("expected false when last attempt was just now")
	}
}

func TestShouldAttempt_LastAttemptOldEnoughAllowed(t *testing.T) {
	svc := newTestService(nil)
	old := time.Now().Add(-(worker.DefaultInterval + time.Second))
	if !svc.shouldAttempt(SourcePending, 0, &old, nil) {
		t.Error("expected true when enough time has passed since last attempt")
	}
}
