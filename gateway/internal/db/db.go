package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://gateway:gateway@localhost:5432/integration_gateway?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}

func Migrate(db *sql.DB, schema string) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Println("schema applied")
	return nil
}

type caseJSON struct {
	ID           string `json:"id"`
	CaseNumber   string `json:"caseNumber"`
	CurrentStage string `json:"currentStage"`
	CourtCaseNumber *string `json:"courtCaseNumber"`
	Borrower struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		SSNLast4  string `json:"ssnLast4"`
		DOB       string `json:"dob"`
	} `json:"borrower"`
	Property struct {
		Address  string `json:"address"`
		County   string `json:"county"`
		State    string `json:"state"`
		ParcelID string `json:"parcelId"`
	} `json:"property"`
	Loan struct {
		Number         string  `json:"number"`
		Servicer       string  `json:"servicer"`
		OriginalAmount float64 `json:"originalAmount"`
	} `json:"loan"`
}

func Seed(db *sql.DB, casesJSON []byte) error {
	var cases []caseJSON
	if err := json.Unmarshal(casesJSON, &cases); err != nil {
		return fmt.Errorf("parse cases.json: %w", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count); err != nil {
		return fmt.Errorf("count cases: %w", err)
	}
	if count > 0 {
		log.Println("cases already seeded, skipping")
		return nil
	}

	for _, c := range cases {
		_, err := db.Exec(`
			INSERT INTO cases (
				id, case_number, current_stage, court_case_number,
				borrower_first_name, borrower_last_name, borrower_ssn_last4, borrower_dob,
				property_address, property_county, property_state, property_parcel_id,
				loan_number, loan_servicer, loan_original_amount
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			c.ID, c.CaseNumber, c.CurrentStage, c.CourtCaseNumber,
			c.Borrower.FirstName, c.Borrower.LastName, c.Borrower.SSNLast4, c.Borrower.DOB,
			c.Property.Address, c.Property.County, c.Property.State, c.Property.ParcelID,
			c.Loan.Number, c.Loan.Servicer, c.Loan.OriginalAmount,
		)
		if err != nil {
			return fmt.Errorf("seed case %s: %w", c.ID, err)
		}
	}

	log.Printf("seeded %d cases", len(cases))
	return nil
}
