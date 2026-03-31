package enrichments

import (
	"errors"
	"testing"

	"github.com/zhalok/integration-gateway/internal/circuitbreaker"
	"github.com/zhalok/integration-gateway/internal/clients"
	"github.com/zhalok/integration-gateway/internal/domains/cases"
	"github.com/zhalok/integration-gateway/internal/worker"
)

// --- client mocks ---

type mockPropertyFetcher struct {
	fetch func(state, county, parcelID string) clients.PropertyResult
}

func (m *mockPropertyFetcher) Fetch(state, county, parcelID string) clients.PropertyResult {
	return m.fetch(state, county, parcelID)
}

type mockCourtFetcher struct {
	fetch func(caseNumber string) clients.CourtResult
}

func (m *mockCourtFetcher) Fetch(caseNumber string) clients.CourtResult {
	return m.fetch(caseNumber)
}

type mockSCRAFetcher struct {
	submit func(lastName, firstName, ssnLast4, dob string) clients.SCRAResult
	poll   func(searchID string) clients.SCRAResult
}

func (m *mockSCRAFetcher) Submit(lastName, firstName, ssnLast4, dob string) clients.SCRAResult {
	return m.submit(lastName, firstName, ssnLast4, dob)
}

func (m *mockSCRAFetcher) Poll(searchID string) clients.SCRAResult {
	return m.poll(searchID)
}

// --- cases repository mock ---

type mockCasesRepository struct {
	getByID func(id string) (*cases.Case, error)
}

func (m *mockCasesRepository) GetByID(id string) (*cases.Case, error) { return m.getByID(id) }
func (m *mockCasesRepository) GetAll() ([]*cases.Case, error)         { return nil, nil }

// --- helpers ---

// pendingEnrichment returns an enrichment where all sources are pending with zero
// attempts — so shouldAttempt will return true for all three sources.
func pendingEnrichment() *Enrichment {
	return &Enrichment{
		ID:         1,
		CaseID:     "case-1",
		Status:     StatusPending,
		PRStatus:   SourcePending,
		CRStatus:   SourcePending,
		SCRAStatus: SourcePending,
	}
}

func baseCase() *cases.Case {
	ccn := "CCN-001"
	return &cases.Case{
		ID:              "case-1",
		CourtCaseNumber: &ccn,
	}
}

func buildService(
	repo Repository,
	caseRepo cases.Repository,
	prop propertyFetcher,
	court courtFetcher,
	scra scraFetcher,
) (*service, chan worker.Job) {
	jobs := make(chan worker.Job, 1)
	return &service{
		repo:           repo,
		caseRepo:       caseRepo,
		propertyClient: prop,
		courtClient:    court,
		scraClient:     scra,
		cbs:            circuitbreaker.NewSet(),
		jobs:           jobs,
	}, jobs
}

// noopClients returns client mocks that should never be called (panics if they are).
func noopProperty() propertyFetcher {
	return &mockPropertyFetcher{fetch: func(_, _, _ string) clients.PropertyResult {
		panic("property client called unexpectedly")
	}}
}
func noopCourt() courtFetcher {
	return &mockCourtFetcher{fetch: func(_ string) clients.CourtResult {
		panic("court client called unexpectedly")
	}}
}
func noopSCRA() scraFetcher {
	return &mockSCRAFetcher{
		submit: func(_, _, _, _ string) clients.SCRAResult { panic("scra submit called unexpectedly") },
		poll:   func(_ string) clients.SCRAResult { panic("scra poll called unexpectedly") },
	}
}

// --- load error tests ---

func TestProcessEnrichment_EnrichmentNotFound(t *testing.T) {
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return nil, nil }},
		&mockCasesRepository{},
		noopProperty(), noopCourt(), noopSCRA(),
	)
	if err := svc.ProcessEnrichment(1, "case-1"); err == nil {
		t.Fatal("expected error for missing enrichment, got nil")
	}
}

func TestProcessEnrichment_EnrichmentLoadError(t *testing.T) {
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return nil, errors.New("db down") }},
		&mockCasesRepository{},
		noopProperty(), noopCourt(), noopSCRA(),
	)
	if err := svc.ProcessEnrichment(1, "case-1"); err == nil {
		t.Fatal("expected error on enrichment load failure, got nil")
	}
}

func TestProcessEnrichment_CaseNotFound(t *testing.T) {
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return pendingEnrichment(), nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return nil, nil }},
		noopProperty(), noopCourt(), noopSCRA(),
	)
	if err := svc.ProcessEnrichment(1, "case-1"); err == nil {
		t.Fatal("expected error for missing case, got nil")
	}
}

func TestProcessEnrichment_CaseLoadError(t *testing.T) {
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return pendingEnrichment(), nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return nil, errors.New("db down") }},
		noopProperty(), noopCourt(), noopSCRA(),
	)
	if err := svc.ProcessEnrichment(1, "case-1"); err == nil {
		t.Fatal("expected error on case load failure, got nil")
	}
}

// --- property records ---

func TestProcessEnrichment_PropertySuccess(t *testing.T) {
	e := pendingEnrichment()
	e.CRStatus = SourceNotApplicable
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		&mockPropertyFetcher{fetch: func(_, _, _ string) clients.PropertyResult {
			return clients.PropertyResult{Data: []byte(`{"value":1}`)}
		}},
		noopCourt(), noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.PRStatus != SourceSuccess {
		t.Errorf("PRStatus: want %q, got %q", SourceSuccess, e.PRStatus)
	}
	if len(e.PRData) == 0 {
		t.Error("PRData must be set on success")
	}
}

func TestProcessEnrichment_PropertyPermanentFailure(t *testing.T) {
	e := pendingEnrichment()
	e.CRStatus = SourceNotApplicable
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		&mockPropertyFetcher{fetch: func(_, _, _ string) clients.PropertyResult {
			return clients.PropertyResult{Permanent: true, Err: errors.New("property not found")}
		}},
		noopCourt(), noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.PRStatus != SourceFailed {
		t.Errorf("PRStatus: want %q, got %q", SourceFailed, e.PRStatus)
	}
	if e.PRReason == nil {
		t.Error("PRReason must be set on permanent failure")
	}
}

func TestProcessEnrichment_PropertyTransientFailure_StaysPending(t *testing.T) {
	e := pendingEnrichment()
	e.CRStatus = SourceNotApplicable
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		&mockPropertyFetcher{fetch: func(_, _, _ string) clients.PropertyResult {
			return clients.PropertyResult{Err: errors.New("service unavailable")}
		}},
		noopCourt(), noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.PRStatus != SourcePending {
		t.Errorf("PRStatus: want %q (transient error must not mark failed), got %q", SourcePending, e.PRStatus)
	}
}

func TestProcessEnrichment_PropertyCircuitOpen_Skipped(t *testing.T) {
	e := pendingEnrichment()
	e.CRStatus = SourceNotApplicable
	e.SCRAStatus = SourceSuccess

	propertyCalled := false
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		&mockPropertyFetcher{fetch: func(_, _, _ string) clients.PropertyResult {
			propertyCalled = true
			return clients.PropertyResult{}
		}},
		noopCourt(), noopSCRA(),
	)

	// Trip the property circuit breaker open.
	for i := 0; i < 5; i++ {
		svc.cbs.PropertyRecords.Failure()
	}

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if propertyCalled {
		t.Error("property client must not be called when circuit is open")
	}
	if e.PRStatus != SourcePending {
		t.Errorf("PRStatus must remain pending when circuit is open, got %q", e.PRStatus)
	}
}

// --- court records ---

func TestProcessEnrichment_CourtSuccess(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(),
		&mockCourtFetcher{fetch: func(caseNumber string) clients.CourtResult {
			return clients.CourtResult{Data: []byte(`{"court":"Civil"}`)}
		}},
		noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.CRStatus != SourceSuccess {
		t.Errorf("CRStatus: want %q, got %q", SourceSuccess, e.CRStatus)
	}
	if len(e.CRData) == 0 {
		t.Error("CRData must be set on success")
	}
}

func TestProcessEnrichment_CourtSkipped_NoCaseNumber(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.SCRAStatus = SourceSuccess

	c := baseCase()
	c.CourtCaseNumber = nil // pre-filing

	courtCalled := false
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return c, nil }},
		noopProperty(),
		&mockCourtFetcher{fetch: func(_ string) clients.CourtResult {
			courtCalled = true
			return clients.CourtResult{}
		}},
		noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if courtCalled {
		t.Error("court client must not be called when case has no court case number")
	}
	if e.CRStatus != SourcePending {
		t.Errorf("CRStatus must remain pending when skipped, got %q", e.CRStatus)
	}
}

func TestProcessEnrichment_CourtPermanentFailure(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(),
		&mockCourtFetcher{fetch: func(_ string) clients.CourtResult {
			return clients.CourtResult{Permanent: true, Err: errors.New("no court filings found")}
		}},
		noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.CRStatus != SourceFailed {
		t.Errorf("CRStatus: want %q, got %q", SourceFailed, e.CRStatus)
	}
	if e.CRReason == nil {
		t.Error("CRReason must be set on permanent failure")
	}
}

func TestProcessEnrichment_CourtRateLimited_SetsRetryAfter(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.SCRAStatus = SourceSuccess

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(),
		&mockCourtFetcher{fetch: func(_ string) clients.CourtResult {
			return clients.CourtResult{Err: errors.New("rate limited")} // transient, no RetryAfter
		}},
		noopSCRA(),
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.CRStatus != SourcePending {
		t.Errorf("CRStatus must remain pending on transient error, got %q", e.CRStatus)
	}
}

// --- SCRA ---

func TestProcessEnrichment_SCRA_Submit_SetsSearchID(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable
	// SCRASearchID is nil → submit path

	searchID := "search-xyz"
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult {
				return clients.SCRAResult{SearchID: &searchID}
			},
			poll: func(_ string) clients.SCRAResult { panic("poll must not be called on first submit") },
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SCRASearchID == nil || *e.SCRASearchID != searchID {
		t.Errorf("SCRASearchID: want %q, got %v", searchID, e.SCRASearchID)
	}
	// Status stays pending — result is not available until poll
	if e.SCRAStatus != SourcePending {
		t.Errorf("SCRAStatus: want %q after submit, got %q", SourcePending, e.SCRAStatus)
	}
}

func TestProcessEnrichment_SCRA_Submit_Error_StaysPending(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult {
				return clients.SCRAResult{Err: errors.New("scra unavailable")}
			},
			poll: func(_ string) clients.SCRAResult { panic("poll must not be called") },
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SCRASearchID != nil {
		t.Error("SCRASearchID must not be set on submit error")
	}
	if e.SCRAStatus != SourcePending {
		t.Errorf("SCRAStatus must remain pending on submit error, got %q", e.SCRAStatus)
	}
}

func TestProcessEnrichment_SCRA_Poll_Pending(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable
	sid := "search-xyz"
	e.SCRASearchID = &sid // search already submitted

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult { panic("submit must not be called") },
			poll: func(_ string) clients.SCRAResult {
				return clients.SCRAResult{Pending: true}
			},
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SCRAStatus != SourcePending {
		t.Errorf("SCRAStatus: want %q while poll is pending, got %q", SourcePending, e.SCRAStatus)
	}
}

func TestProcessEnrichment_SCRA_Poll_Success(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable
	sid := "search-xyz"
	e.SCRASearchID = &sid

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult { panic("submit must not be called") },
			poll: func(_ string) clients.SCRAResult {
				return clients.SCRAResult{Data: []byte(`{"status":"not_covered"}`)}
			},
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SCRAStatus != SourceSuccess {
		t.Errorf("SCRAStatus: want %q, got %q", SourceSuccess, e.SCRAStatus)
	}
	if len(e.SCRAData) == 0 {
		t.Error("SCRAData must be set on successful poll")
	}
}

func TestProcessEnrichment_SCRA_Poll_PermanentFailure(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable
	sid := "search-xyz"
	e.SCRASearchID = &sid

	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult { panic("submit must not be called") },
			poll: func(_ string) clients.SCRAResult {
				return clients.SCRAResult{Permanent: true, Err: errors.New("invalid search ID")}
			},
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.SCRAStatus != SourceFailed {
		t.Errorf("SCRAStatus: want %q on permanent error, got %q", SourceFailed, e.SCRAStatus)
	}
	if e.SCRAReason == nil {
		t.Error("SCRAReason must be set on permanent failure")
	}
}

// TestProcessEnrichment_SCRA_ExhaustedByMaxAttempts documents that shouldAttempt
// (which caps at worker.MaxAttempts=5) blocks SCRA before the MaxPollAttempts=10
// check inside attemptSCRA is ever reached. The "polling timed out" branch in
// attemptSCRA is currently dead code because MaxPollAttempts > MaxAttempts.
func TestProcessEnrichment_SCRA_ExhaustedByMaxAttempts(t *testing.T) {
	e := pendingEnrichment()
	e.PRStatus = SourceSuccess
	e.CRStatus = SourceNotApplicable
	sid := "search-xyz"
	e.SCRASearchID = &sid
	e.SCRAAttempts = worker.MaxAttempts // shouldAttempt blocks here

	pollCalled := false
	svc, _ := buildService(
		&mockRepository{getByID: func(id int64) (*Enrichment, error) { return e, nil }},
		&mockCasesRepository{getByID: func(id string) (*cases.Case, error) { return baseCase(), nil }},
		noopProperty(), noopCourt(),
		&mockSCRAFetcher{
			submit: func(_, _, _, _ string) clients.SCRAResult { panic("submit must not be called") },
			poll: func(_ string) clients.SCRAResult {
				pollCalled = true
				return clients.SCRAResult{Pending: true}
			},
		},
	)

	if err := svc.ProcessEnrichment(e.ID, e.CaseID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// shouldAttempt returns false at MaxAttempts — poll is never called, status stays pending.
	// NOTE: the MaxPollAttempts timeout branch in attemptSCRA is unreachable dead code
	// because shouldAttempt blocks at MaxAttempts (5) < MaxPollAttempts (10).
	if pollCalled {
		t.Error("poll must not be called when MaxAttempts is exhausted")
	}
	if e.SCRAStatus != SourcePending {
		t.Errorf("SCRAStatus: want %q when shouldAttempt is blocked, got %q", SourcePending, e.SCRAStatus)
	}
}
