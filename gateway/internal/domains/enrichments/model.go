package enrichments

import (
	"encoding/json"
	"time"
)

// Source-level status values
const (
	SourcePending       = "pending"
	SourceSuccess       = "success"
	SourceFailed        = "failed"
	SourceNotApplicable = "not_applicable"
)

// Overall enrichment status values
const (
	StatusPending  = "pending"
	StatusComplete = "complete"
	StatusPartial  = "partial"
	StatusFailed   = "failed"
)

type Enrichment struct {
	ID          int64
	CaseID      string
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time

	// Property Records
	PRStatus      string
	PRAttempts    int
	PRLastAttempt *time.Time
	PRRetryAfter  *time.Time
	PRData        *json.RawMessage
	PRReason      *string

	// Court Records
	CRStatus      string
	CRAttempts    int
	CRLastAttempt *time.Time
	CRRetryAfter  *time.Time
	CRData        *json.RawMessage
	CRReason      *string

	// SCRA
	SCRAStatus      string
	SCRAAttempts    int
	SCRALastAttempt *time.Time
	SCRARetryAfter  *time.Time
	SCRASearchID    *string
	SCRAData        *json.RawMessage
	SCRAReason      *string
}
