package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const propertyBaseURL = "http://localhost:9001"

type PropertyClient struct {
	http *http.Client
}

func NewPropertyClient() *PropertyClient {
	return &PropertyClient{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// PropertyResult is the outcome of a single fetch attempt.
type PropertyResult struct {
	Data      json.RawMessage
	Permanent bool   // true = do not retry (404)
	Err       error
}

// Fetch calls GET /api/properties/{state}/{county}/{parcelId}.
// Returns a PropertyResult indicating success, transient failure, or permanent failure.
func (c *PropertyClient) Fetch(state, county, parcelID string) PropertyResult {
	url := fmt.Sprintf("%s/api/properties/%s/%s/%s", propertyBaseURL, state, county, parcelID)

	resp, err := c.http.Get(url)
	if err != nil {
		return PropertyResult{Err: fmt.Errorf("property request: %w", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var data json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return PropertyResult{Err: fmt.Errorf("decode property response: %w", err)}
		}
		return PropertyResult{Data: data}

	case http.StatusNotFound:
		return PropertyResult{
			Permanent: true,
			Err:       fmt.Errorf("property not found"),
		}

	case http.StatusServiceUnavailable:
		return PropertyResult{Err: fmt.Errorf("property service unavailable (503)")}

	default:
		return PropertyResult{Err: fmt.Errorf("unexpected property status %d", resp.StatusCode)}
	}
}
