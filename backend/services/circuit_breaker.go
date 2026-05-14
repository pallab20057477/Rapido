package services

import (
	"errors"
	"sync"
	"time"
)

// CircuitBreaker implements the Circuit Breaker pattern for:
// - Preventing cascade failures in distributed systems
// - Automatic recovery detection
// - Half-open state for gradual recovery
// - Per-service breaker isolation
type CircuitBreaker struct {
	mu                sync.RWMutex
	name              string
	state             CircuitState
	failureCount      int
	successCount      int
	lastFailureTime   time.Time
	threshold         int           // Number of failures before opening
	timeout           time.Duration // Time before attempting recovery
	halfOpenMaxCalls  int           // Max calls in half-open state
}

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota    // Normal operation
	StateOpen                          // Failing, reject calls
	StateHalfOpen                      // Testing if service recovered
)

// Common circuit breaker errors
var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		state:            StateClosed,
		threshold:        threshold,
		timeout:          timeout,
		halfOpenMaxCalls: 3,
	}
}

// Execute runs the function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	state := cb.state

	switch state {
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.failureCount = 0
			cb.successCount = 0
			state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}

	case StateHalfOpen:
		// Limit concurrent requests in half-open state
		if cb.failureCount+cb.successCount >= cb.halfOpenMaxCalls {
			cb.mu.Unlock()
			return ErrTooManyRequests
		}
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.recordResult(err)

	return err
}

// recordResult updates circuit breaker state based on execution result
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		switch cb.state {
		case StateClosed:
			if cb.failureCount >= cb.threshold {
				cb.state = StateOpen
			}

		case StateHalfOpen:
			// One failure in half-open closes the circuit again
			cb.state = StateOpen
			cb.failureCount = 0
			cb.successCount = 0
		}
	} else {
		cb.successCount++

		switch cb.state {
		case StateClosed:
			// Reset failure count on success
			cb.failureCount = 0

		case StateHalfOpen:
			// Multiple successes in half-open closes the circuit
			if cb.successCount >= cb.halfOpenMaxCalls {
				cb.state = StateClosed
				cb.failureCount = 0
				cb.successCount = 0
			}
		}
	}
}

// State returns current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns current circuit statistics
func (cb *CircuitBreaker) Stats() (state CircuitState, failureCount, successCount int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state, cb.failureCount, cb.successCount
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate returns existing breaker or creates new one
func (r *CircuitBreakerRegistry) GetOrCreate(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists := r.breakers[name]; exists {
		return cb
	}

	cb := NewCircuitBreaker(name, threshold, timeout)
	r.breakers[name] = cb
	return cb
}

// Get returns existing breaker or nil
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.breakers[name]
}

// AllStats returns stats for all breakers
func (r *CircuitBreakerRegistry) AllStats() map[string]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, cb := range r.breakers {
		state, failures, successes := cb.Stats()
		stats[name] = map[string]interface{}{
			"state":          state,
			"failure_count":  failures,
			"success_count":  successes,
		}
	}
	return stats
}

// Global circuit breaker registry
var CircuitBreakerRegistryInstance = NewCircuitBreakerRegistry()

// ExecuteWithBreaker executes function with circuit breaker for named service
func ExecuteWithBreaker(serviceName string, fn func() error) error {
	cb := CircuitBreakerRegistryInstance.GetOrCreate(serviceName, 5, 30*time.Second)
	return cb.Execute(fn)
}
