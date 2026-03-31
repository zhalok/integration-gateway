package cases

import "time"

type Case struct {
	ID                 string
	CaseNumber         string
	CurrentStage       string
	CourtCaseNumber    *string
	BorrowerFirstName  string
	BorrowerLastName   string
	BorrowerSSNLast4   string
	BorrowerDOB        string
	PropertyAddress    string
	PropertyCounty     string
	PropertyState      string
	PropertyParcelID   string
	LoanNumber         string
	LoanServicer       string
	LoanOriginalAmount float64
	CreatedAt          time.Time
}
