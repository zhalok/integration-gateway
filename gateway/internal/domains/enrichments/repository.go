package enrichments

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Repository interface {
	GetByID(id int64) (*Enrichment, error)
	GetByCaseID(caseID string) (*Enrichment, error)
	Create(caseID string, courtCaseNumber *string) (*Enrichment, error)
	Update(e *Enrichment) error
	ComputeAndUpdateOverallStatus(enrichmentID int64) error
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetByID(id int64) (*Enrichment, error) {
	e := &Enrichment{}
	err := r.db.QueryRow(`
		SELECT id, case_id, status, started_at, completed_at,
		       pr_status, pr_attempts, pr_last_attempt, pr_retry_after, pr_data, pr_reason,
		       cr_status, cr_attempts, cr_last_attempt, cr_retry_after, cr_data, cr_reason,
		       scra_status, scra_attempts, scra_last_attempt, scra_retry_after, scra_search_id, scra_data, scra_reason
		FROM enrichments WHERE id = $1`, id).Scan(
		&e.ID, &e.CaseID, &e.Status, &e.StartedAt, &e.CompletedAt,
		&e.PRStatus, &e.PRAttempts, &e.PRLastAttempt, &e.PRRetryAfter, &e.PRData, &e.PRReason,
		&e.CRStatus, &e.CRAttempts, &e.CRLastAttempt, &e.CRRetryAfter, &e.CRData, &e.CRReason,
		&e.SCRAStatus, &e.SCRAAttempts, &e.SCRALastAttempt, &e.SCRARetryAfter, &e.SCRASearchID, &e.SCRAData, &e.SCRAReason,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get enrichment by id: %w", err)
	}
	return e, nil
}

func (r *postgresRepository) GetByCaseID(caseID string) (*Enrichment, error) {
	e := &Enrichment{}
	err := r.db.QueryRow(`
		SELECT id, case_id, status, started_at, completed_at,
		       pr_status, pr_attempts, pr_last_attempt, pr_retry_after, pr_data, pr_reason,
		       cr_status, cr_attempts, cr_last_attempt, cr_retry_after, cr_data, cr_reason,
		       scra_status, scra_attempts, scra_last_attempt, scra_retry_after, scra_search_id, scra_data, scra_reason
		FROM enrichments WHERE case_id = $1`, caseID).Scan(
		&e.ID, &e.CaseID, &e.Status, &e.StartedAt, &e.CompletedAt,
		&e.PRStatus, &e.PRAttempts, &e.PRLastAttempt, &e.PRRetryAfter, &e.PRData, &e.PRReason,
		&e.CRStatus, &e.CRAttempts, &e.CRLastAttempt, &e.CRRetryAfter, &e.CRData, &e.CRReason,
		&e.SCRAStatus, &e.SCRAAttempts, &e.SCRALastAttempt, &e.SCRARetryAfter, &e.SCRASearchID, &e.SCRAData, &e.SCRAReason,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get enrichment by case id: %w", err)
	}
	return e, nil
}

func (r *postgresRepository) Create(caseID string, courtCaseNumber *string) (*Enrichment, error) {
	crStatus := SourcePending
	var crReason *string
	if courtCaseNumber == nil {
		crStatus = SourceNotApplicable
		reason := "Case is pre-filing — no court case number"
		crReason = &reason
	}

	e := &Enrichment{}
	err := r.db.QueryRow(`
		INSERT INTO enrichments (
			case_id, status,
			pr_status, cr_status, cr_reason, scra_status
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, case_id, status, started_at, completed_at,
		          pr_status, pr_attempts, pr_last_attempt, pr_retry_after, pr_data, pr_reason,
		          cr_status, cr_attempts, cr_last_attempt, cr_retry_after, cr_data, cr_reason,
		          scra_status, scra_attempts, scra_last_attempt, scra_retry_after, scra_search_id, scra_data, scra_reason`,
		caseID, StatusPending,
		SourcePending, crStatus, crReason, SourcePending,
	).Scan(
		&e.ID, &e.CaseID, &e.Status, &e.StartedAt, &e.CompletedAt,
		&e.PRStatus, &e.PRAttempts, &e.PRLastAttempt, &e.PRRetryAfter, &e.PRData, &e.PRReason,
		&e.CRStatus, &e.CRAttempts, &e.CRLastAttempt, &e.CRRetryAfter, &e.CRData, &e.CRReason,
		&e.SCRAStatus, &e.SCRAAttempts, &e.SCRALastAttempt, &e.SCRARetryAfter, &e.SCRASearchID, &e.SCRAData, &e.SCRAReason,
	)
	if err != nil {
		return nil, fmt.Errorf("create enrichment: %w", err)
	}
	return e, nil
}

func (r *postgresRepository) Update(e *Enrichment) error {
	_, err := r.db.Exec(`
		UPDATE enrichments SET
			status = $1, completed_at = $2,
			pr_status = $3, pr_attempts = $4, pr_last_attempt = $5, pr_retry_after = $6, pr_data = $7, pr_reason = $8,
			cr_status = $9, cr_attempts = $10, cr_last_attempt = $11, cr_retry_after = $12, cr_data = $13, cr_reason = $14,
			scra_status = $15, scra_attempts = $16, scra_last_attempt = $17, scra_retry_after = $18,
			scra_search_id = $19, scra_data = $20, scra_reason = $21
		WHERE id = $22`,
		e.Status, e.CompletedAt,
		e.PRStatus, e.PRAttempts, e.PRLastAttempt, e.PRRetryAfter, nullableJSON(e.PRData), e.PRReason,
		e.CRStatus, e.CRAttempts, e.CRLastAttempt, e.CRRetryAfter, nullableJSON(e.CRData), e.CRReason,
		e.SCRAStatus, e.SCRAAttempts, e.SCRALastAttempt, e.SCRARetryAfter,
		e.SCRASearchID, nullableJSON(e.SCRAData), e.SCRAReason,
		e.ID,
	)
	if err != nil {
		return fmt.Errorf("update enrichment: %w", err)
	}
	return nil
}

// ComputeAndUpdateOverallStatus derives overall status from per-source statuses.
// not_applicable sources are excluded from the computation.
// Sets completed_at when status transitions away from pending.
func (r *postgresRepository) ComputeAndUpdateOverallStatus(enrichmentID int64) error {
	e := &Enrichment{}
	err := r.db.QueryRow(`
		SELECT id, status, completed_at, pr_status, cr_status, scra_status
		FROM enrichments WHERE id = $1`, enrichmentID).Scan(
		&e.ID, &e.Status, &e.CompletedAt, &e.PRStatus, &e.CRStatus, &e.SCRAStatus,
	)
	if err != nil {
		return fmt.Errorf("fetch enrichment for status compute: %w", err)
	}

	sources := []string{e.PRStatus, e.CRStatus, e.SCRAStatus}
	var applicable []string
	for _, s := range sources {
		if s != SourceNotApplicable {
			applicable = append(applicable, s)
		}
	}

	newStatus := computeOverallStatus(applicable)

	var completedAt *time.Time
	if newStatus != StatusPending {
		now := time.Now()
		completedAt = &now
	}

	_, err = r.db.Exec(`
		UPDATE enrichments SET status = $1, completed_at = $2 WHERE id = $3`,
		newStatus, completedAt, enrichmentID,
	)
	if err != nil {
		return fmt.Errorf("update overall status: %w", err)
	}
	return nil
}

func computeOverallStatus(applicable []string) string {
	if len(applicable) == 0 {
		return StatusComplete
	}

	successes, failures, pending := 0, 0, 0
	for _, s := range applicable {
		switch s {
		case SourceSuccess:
			successes++
		case SourceFailed:
			failures++
		case SourcePending:
			pending++
		}
	}

	switch {
	case pending > 0:
		return StatusPending
	case failures == 0:
		return StatusComplete
	case successes == 0:
		return StatusFailed
	default:
		return StatusPartial
	}
}

// nullableJSON returns nil if data is nil or empty, otherwise returns the raw bytes.
func nullableJSON(data *json.RawMessage) interface{} {
	if data == nil || len(*data) == 0 {
		return nil
	}
	return []byte(*data)
}
