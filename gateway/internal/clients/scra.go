package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const scraBaseURL = "http://localhost:9003"

type SCRAClient struct {
	http *http.Client
}

func NewSCRAClient() *SCRAClient {
	return &SCRAClient{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// SCRAResult is the outcome of a submit or poll call.
type SCRAResult struct {
	SearchID  *string         // set after successful submit
	Data      json.RawMessage // set after successful poll
	Pending   bool            // poll returned 202 — still in progress
	Permanent bool            // error status or 404 — do not retry
	Err       error
}

type scraSubmitRequest struct {
	LastName  string `json:"lastName"`
	FirstName string `json:"firstName"`
	SSNLast4  string `json:"ssnLast4"`
	DOB       string `json:"dob"`
}

// Submit posts a new SCRA search and returns the search ID.
func (c *SCRAClient) Submit(lastName, firstName, ssnLast4, dob string) SCRAResult {
	body, _ := json.Marshal(scraSubmitRequest{
		LastName:  lastName,
		FirstName: firstName,
		SSNLast4:  ssnLast4,
		DOB:       dob,
	})

	resp, err := c.http.Post(scraBaseURL+"/api/scra/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return SCRAResult{Err: fmt.Errorf("scra submit: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return SCRAResult{Err: fmt.Errorf("unexpected scra submit status %d", resp.StatusCode)}
	}

	var result struct {
		SearchID string `json:"searchId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SCRAResult{Err: fmt.Errorf("decode scra submit response: %w", err)}
	}

	return SCRAResult{SearchID: &result.SearchID}
}

// Poll checks the result of a previously submitted SCRA search.
func (c *SCRAClient) Poll(searchID string) SCRAResult {
	resp, err := c.http.Get(scraBaseURL + "/api/scra/results/" + searchID)
	if err != nil {
		return SCRAResult{Err: fmt.Errorf("scra poll: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return SCRAResult{
			Permanent: true,
			Err:       fmt.Errorf("invalid search ID"),
		}
	}

	if resp.StatusCode == http.StatusAccepted {
		// Still pending
		return SCRAResult{Pending: true}
	}

	if resp.StatusCode != http.StatusOK {
		return SCRAResult{Err: fmt.Errorf("unexpected scra poll status %d", resp.StatusCode)}
	}

	var result struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SCRAResult{Err: fmt.Errorf("decode scra poll response: %w", err)}
	}

	switch result.Status {
	case "complete":
		return SCRAResult{Data: result.Result}
	case "error":
		return SCRAResult{
			Permanent: true,
			Err:       fmt.Errorf("%s", result.Error),
		}
	default:
		return SCRAResult{Pending: true}
	}
}

// satisfy time import
var _ = time.Second
