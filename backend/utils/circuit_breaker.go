package utils

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name         string
	maxFailures  int
	timeout      time.Duration
	resetTimeout time.Duration

	failures    int
	lastFailure time.Time
	state       State
	mutex       sync.RWMutex
}

// State represents circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing, reject requests
	StateHalfOpen              // Testing if recovered
)

// Common errors
var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrTimeout     = errors.New("operation timed out")
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		timeout:      30 * time.Second,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Execute runs the function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mutex.Lock()

	// Check if we should transition from Open to Half-Open
	if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
		cb.state = StateHalfOpen
		cb.failures = 0
		Info("Circuit breaker entering half-open state", zap.String("name", cb.name))
	}

	// If circuit is open, reject immediately
	if cb.state == StateOpen {
		cb.mutex.Unlock()
		return ErrCircuitOpen
	}

	cb.mutex.Unlock()

	// Execute the function
	err := fn()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.recordFailure()
		return err
	}

	// Success
	if cb.state == StateHalfOpen {
		// Recovery successful, close circuit
		cb.state = StateClosed
		cb.failures = 0
		Info("Circuit breaker closed after recovery", zap.String("name", cb.name))
	}

	return nil
}

// recordFailure increments failure count and may open circuit
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures {
		if cb.state == StateClosed {
			cb.state = StateOpen
			Error("Circuit breaker opened due to failures",
				zap.String("name", cb.name),
				zap.Int("failures", cb.failures))
		} else if cb.state == StateHalfOpen {
			// Back to open
			cb.state = StateOpen
			cb.failures = cb.maxFailures
		}
	}
}

// GetState returns current circuit state
func (cb *CircuitBreaker) GetState() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// IsOpen returns true if circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// ForceClose manually closes the circuit (for admin use)
func (cb *CircuitBreaker) ForceClose() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	Info("Circuit breaker manually closed", zap.String("name", cb.name))
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	stateStr := "closed"
	if cb.state == StateOpen {
		stateStr = "open"
	} else if cb.state == StateHalfOpen {
		stateStr = "half-open"
	}

	return map[string]interface{}{
		"name":          cb.name,
		"state":         stateStr,
		"failures":      cb.failures,
		"max_failures":  cb.maxFailures,
		"last_failure":  cb.lastFailure,
		"reset_timeout": cb.resetTimeout.String(),
	}
}

// Global circuit breakers for external services
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

var CBRegistry = &CircuitBreakerRegistry{
	breakers: make(map[string]*CircuitBreaker),
}

// Register registers a circuit breaker
func (r *CircuitBreakerRegistry) Register(name string, cb *CircuitBreaker) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.breakers[name] = cb
}

// Get retrieves a circuit breaker
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.breakers[name]
}

// GetAll returns all circuit breakers
func (r *CircuitBreakerRegistry) GetAll() map[string]*CircuitBreaker {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for k, v := range r.breakers {
		result[k] = v
	}
	return result
}

// Initialize standard circuit breakers
func InitCircuitBreakers() {
	// Google Maps API
	CBRegistry.Register("google_maps", NewCircuitBreaker(
		"google_maps",
		5,              // Open after 5 failures
		30*time.Second, // Try again after 30 seconds
	))

	// Razorpay API
	CBRegistry.Register("razorpay", NewCircuitBreaker(
		"razorpay",
		3,              // Open after 3 failures (payment critical)
		10*time.Second, // Try again after 10 seconds
	))

	// Twilio/SMS API
	CBRegistry.Register("sms", NewCircuitBreaker(
		"sms",
		5,
		60*time.Second,
	))

	// FCM Push API
	CBRegistry.Register("fcm", NewCircuitBreaker(
		"fcm",
		5,
		30*time.Second,
	))

	Info("Circuit breakers initialized",
		zap.Int("count", len(CBRegistry.GetAll())))
}
