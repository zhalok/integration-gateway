package api

import (
	"encoding/json"
	"net/http"

	"github.com/zhalok/integration-gateway/internal/circuitbreaker"
)

func HealthHandler(cbs *circuitbreaker.Set) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"circuitBreakers": map[string]interface{}{
				"propertyRecords": cbs.PropertyRecords.GetState(),
				"courtRecords":    cbs.CourtRecords.GetState(),
				"scra":            cbs.SCRA.GetState(),
			},
		})
	}
}
