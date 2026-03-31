package circuitbreaker

import (
	"sync"
	"time"
)

const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"

	failureThreshold = 5
	cooldown         = 30 * time.Second
)

// CircuitBreaker guards a single external service.
type CircuitBreaker struct {
	mu           sync.Mutex
	Name         string
	state        string
	failures     int
	lastFailure  time.Time
	openedAt     time.Time
	cooldownEnds time.Time
}

func New(name string) *CircuitBreaker {
	return &CircuitBreaker{Name: name, state: StateClosed}
}

// Allow returns true if the request should proceed.
// When open, allows through one probe request (half-open) after cooldown expires.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Now().After(cb.cooldownEnds) {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// Success records a successful call. Closes the circuit if it was half-open.
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = StateClosed
}

// Failure records a failed call. Opens the circuit after threshold is reached.
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.state == StateHalfOpen || cb.failures >= failureThreshold {
		cb.state = StateOpen
		cb.openedAt = time.Now()
		cb.cooldownEnds = cb.openedAt.Add(cooldown)
	}
}

// State returns a snapshot of circuit breaker state for the health endpoint.
type State struct {
	Name         string     `json:"state"`
	Failures     int        `json:"failures"`
	LastFailure  *time.Time `json:"lastFailure,omitempty"`
	OpenedAt     *time.Time `json:"openedAt,omitempty"`
	CooldownEnds *time.Time `json:"cooldownEnds,omitempty"`
}

func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	s := State{
		Name:     cb.state,
		Failures: cb.failures,
	}
	if !cb.lastFailure.IsZero() {
		t := cb.lastFailure
		s.LastFailure = &t
	}
	if cb.state == StateOpen || cb.state == StateHalfOpen {
		t := cb.openedAt
		s.OpenedAt = &t
		t2 := cb.cooldownEnds
		s.CooldownEnds = &t2
	}
	return s
}

// Set holds one circuit breaker per external service.
type Set struct {
	PropertyRecords *CircuitBreaker
	CourtRecords    *CircuitBreaker
	SCRA            *CircuitBreaker
}

func NewSet() *Set {
	return &Set{
		PropertyRecords: New("propertyRecords"),
		CourtRecords:    New("courtRecords"),
		SCRA:            New("scra"),
	}
}
