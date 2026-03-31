package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

func courtBaseURL() string {
	if u := os.Getenv("COURT_RECORDS_URL"); u != "" {
		return u
	}
	return "http://localhost:9002"
}

type CourtClient struct {
	http    *http.Client
	limiter *rate.Limiter
}

func NewCourtClient(limiter *rate.Limiter) *CourtClient {
	return &CourtClient{
		http:    &http.Client{Timeout: 10 * time.Second},
		limiter: limiter,
	}
}

// CourtResult is the outcome of a single court records fetch.
type CourtResult struct {
	Data       json.RawMessage
	Permanent  bool       // NoFilingFound — do not retry
	RetryAfter *time.Time // set on 429
	Err        error
}

// courtXML mirrors the XML response shape.
type courtXML struct {
	CaseNumber string `xml:"CaseNumber"`
	Court      string `xml:"Court"`
	Division   string `xml:"Division"`
	Judge      string `xml:"Judge"`
	FilingDate string `xml:"FilingDate"`
	CaseType   string `xml:"CaseType"`
	Status     string `xml:"Status"`
	Message    string `xml:"Message"`
	Parties    struct {
		Plaintiff string `xml:"Plaintiff"`
		Defendant string `xml:"Defendant"`
	} `xml:"Parties"`
	Filings []struct {
		Type           string `xml:"Type"`
		FiledDate      string `xml:"FiledDate"`
		DocumentNumber string `xml:"DocumentNumber"`
	} `xml:"Filings>Filing"`
	NextHearing struct {
		Date      string `xml:"Date"`
		Time      string `xml:"Time"`
		Type      string `xml:"Type"`
		Courtroom string `xml:"Courtroom"`
	} `xml:"NextHearing"`
}

// Fetch calls POST /api/court-records/search.
// Blocks on the rate limiter before making the request.
func (c *CourtClient) Fetch(caseNumber string) CourtResult {
	if err := c.limiter.Wait(context.Background()); err != nil {
		return CourtResult{Err: fmt.Errorf("rate limiter: %w", err)}
	}

	body, _ := json.Marshal(map[string]string{"caseNumber": caseNumber})
	resp, err := c.http.Post(courtBaseURL()+"/api/court-records/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return CourtResult{Err: fmt.Errorf("court request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		var retryAfter *time.Time
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil {
				t := time.Now().Add(time.Duration(secs) * time.Second)
				retryAfter = &t
			}
		}
		return CourtResult{
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("court rate limited (429)"),
		}
	}

	if resp.StatusCode != http.StatusOK {
		return CourtResult{Err: fmt.Errorf("unexpected court status %d", resp.StatusCode)}
	}

	var parsed courtXML
	if err := xml.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		// Malformed/truncated XML (~3%) — transient, will retry
		return CourtResult{Err: fmt.Errorf("malformed court XML: %w", err)}
	}

	if parsed.Status == "NoFilingFound" {
		return CourtResult{
			Permanent: true,
			Err:       fmt.Errorf("no court filings found"),
		}
	}

	data, err := json.Marshal(parsed)
	if err != nil {
		return CourtResult{Err: fmt.Errorf("marshal court data: %w", err)}
	}
	return CourtResult{Data: data}
}
