package enrichments

import (
	"net/http"
	"strings"
)

// RegisterRoutes attaches enrichment endpoints onto the mux.
// Routes handled:
//   POST /api/cases/{id}/enrich
//   GET  /api/cases/{id}/enrichment
func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/cases/", c.caseSubRouter)
}

// caseSubRouter dispatches /api/cases/{id}/enrich and /api/cases/{id}/enrichment.
func (c *Controller) caseSubRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	caseID := extractCaseID(path)
	if caseID == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	switch {
	case strings.HasSuffix(path, "/enrich") && r.Method == http.MethodPost:
		c.CreateEnrichment(w, r, caseID)

	case strings.HasSuffix(path, "/enrichment") && r.Method == http.MethodGet:
		c.GetEnrichment(w, r, caseID)

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}
