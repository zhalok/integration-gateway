package cases

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	GetByID(id string) (*Case, error)
	GetAll() ([]*Case, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetByID(id string) (*Case, error) {
	c := &Case{}
	err := r.db.QueryRow(`
		SELECT id, case_number, current_stage, court_case_number,
		       borrower_first_name, borrower_last_name, borrower_ssn_last4, borrower_dob,
		       property_address, property_county, property_state, property_parcel_id,
		       loan_number, loan_servicer, loan_original_amount, created_at
		FROM cases WHERE id = $1`, id).Scan(
		&c.ID, &c.CaseNumber, &c.CurrentStage, &c.CourtCaseNumber,
		&c.BorrowerFirstName, &c.BorrowerLastName, &c.BorrowerSSNLast4, &c.BorrowerDOB,
		&c.PropertyAddress, &c.PropertyCounty, &c.PropertyState, &c.PropertyParcelID,
		&c.LoanNumber, &c.LoanServicer, &c.LoanOriginalAmount, &c.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case by id: %w", err)
	}
	return c, nil
}

func (r *postgresRepository) GetAll() ([]*Case, error) {
	rows, err := r.db.Query(`
		SELECT id, case_number, current_stage, court_case_number,
		       borrower_first_name, borrower_last_name, borrower_ssn_last4, borrower_dob,
		       property_address, property_county, property_state, property_parcel_id,
		       loan_number, loan_servicer, loan_original_amount, created_at
		FROM cases ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("get all cases: %w", err)
	}
	defer rows.Close()

	var cases []*Case
	for rows.Next() {
		c := &Case{}
		if err := rows.Scan(
			&c.ID, &c.CaseNumber, &c.CurrentStage, &c.CourtCaseNumber,
			&c.BorrowerFirstName, &c.BorrowerLastName, &c.BorrowerSSNLast4, &c.BorrowerDOB,
			&c.PropertyAddress, &c.PropertyCounty, &c.PropertyState, &c.PropertyParcelID,
			&c.LoanNumber, &c.LoanServicer, &c.LoanOriginalAmount, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		cases = append(cases, c)
	}
	// satisfy compiler — time import used via model
	_ = time.Time{}
	return cases, rows.Err()
}
