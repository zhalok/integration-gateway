package enrichments

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type Controller struct {
	usecase Usecase
}

func NewController(uc Usecase) *Controller {
	return &Controller{usecase: uc}
}

// POST /api/cases/{id}/enrich
func (c *Controller) CreateEnrichment(w http.ResponseWriter, r *http.Request, caseID string) {
	e, status, err := c.usecase.CreateEnrichment(caseID)
	if err != nil {
		if status == 404 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Printf("CreateEnrichment error caseID=%s: %v", caseID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toResponse(e))
}

// GET /api/cases/{id}/enrichment
func (c *Controller) GetEnrichment(w http.ResponseWriter, r *http.Request, caseID string) {
	e, err := c.usecase.GetEnrichment(caseID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if e == nil {
		http.Error(w, "no enrichment found for case", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(e))
}

// --- response shapes ---

type sourceResponse struct {
	Status    string          `json:"status"`
	Attempts  int             `json:"attempts,omitempty"`
	LastAttempt *string       `json:"lastAttempt,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Reason    *string         `json:"reason,omitempty"`
	SearchID  *string         `json:"searchId,omitempty"`
}

type enrichmentResponse struct {
	CaseID      string `json:"caseId"`
	Status      string `json:"status"`
	Sources     struct {
		PropertyRecords sourceResponse `json:"propertyRecords"`
		CourtRecords    sourceResponse `json:"courtRecords"`
		SCRA            sourceResponse `json:"scra"`
	} `json:"sources"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`
}

func toResponse(e *Enrichment) enrichmentResponse {
	resp := enrichmentResponse{
		CaseID:      e.CaseID,
		Status:      e.Status,
		StartedAt:   e.StartedAt,
		CompletedAt: e.CompletedAt,
	}

	resp.Sources.PropertyRecords = sourceResponse{
		Status:      e.PRStatus,
		Attempts:    e.PRAttempts,
		LastAttempt: timePtr(e.PRLastAttempt),
		Data:        derefRaw(e.PRData),
		Reason:      e.PRReason,
	}
	resp.Sources.CourtRecords = sourceResponse{
		Status:      e.CRStatus,
		Attempts:    e.CRAttempts,
		LastAttempt: timePtr(e.CRLastAttempt),
		Data:        derefRaw(e.CRData),
		Reason:      e.CRReason,
	}
	resp.Sources.SCRA = sourceResponse{
		Status:      e.SCRAStatus,
		Attempts:    e.SCRAAttempts,
		LastAttempt: timePtr(e.SCRALastAttempt),
		Data:        derefRaw(e.SCRAData),
		Reason:      e.SCRAReason,
		SearchID:    e.SCRASearchID,
	}

	return resp
}

func derefRaw(r *json.RawMessage) json.RawMessage {
	if r == nil {
		return nil
	}
	return *r
}

func timePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// extractCaseID pulls the {id} segment from paths like /api/cases/{id}/enrich
func extractCaseID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// /api/cases/{id}/enrich  → parts: ["api","cases","{id}","enrich"]
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
