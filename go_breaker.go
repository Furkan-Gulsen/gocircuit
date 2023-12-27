package GoBreaker

import (
	"sync/atomic"
	"time"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

// CircuitBreakerConfig holds the configuration options for the Circuit Breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Failure threshold
	ResetTimeout     time.Duration // Reset timeout duration
	SuccessThreshold int           // Success threshold
}

// CircuitBreaker represents a Circuit Breaker.
type CircuitBreaker struct {
	state        int32
	config       CircuitBreakerConfig // Configuration options
	failureCount int32
	successCount int32
	lastAttempt  int64
}

// NewCircuitBreaker creates a new Circuit Breaker with the given configuration.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  int32(StateClosed),
		config: config,
	}
}

// Execute attempts to execute an action using the Circuit Breaker.
func (cb *CircuitBreaker) Execute(action func() error) error {
	// Check if the Circuit Breaker is closed and it's time to transition to half-open state.
	lastAttemptSince := time.Since(time.Unix(atomic.LoadInt64(&cb.lastAttempt), 0))
	if atomic.LoadInt32(&cb.state) == int32(StateClosed) && lastAttemptSince > cb.config.ResetTimeout {
		atomic.StoreInt32(&cb.state, int32(StateHalfOpen))
	}

	// Execute the action.
	err := action()

	// Handle success.
	if err == nil {
		atomic.AddInt32(&cb.successCount, 1)
		if atomic.LoadInt32(&cb.successCount) >= int32(cb.config.SuccessThreshold) {
			cb.reset()
		}
		return nil // Başarılı işlem, hata dönüşü yok
	}

	// Handle failure.
	atomic.AddInt32(&cb.failureCount, 1)
	if atomic.LoadInt32(&cb.failureCount) >= int32(cb.config.FailureThreshold) {
		atomic.StoreInt32(&cb.state, int32(StateOpen))
		atomic.StoreInt64(&cb.lastAttempt, time.Now().Unix())
	}

	return err // Başarısız işlem, hata dönüşü var
}

// reset resets the Circuit Breaker to the closed state.
func (cb *CircuitBreaker) reset() {
	atomic.StoreInt32(&cb.failureCount, 0)
	atomic.StoreInt32(&cb.successCount, 0)
	atomic.StoreInt32(&cb.state, int32(StateClosed))
}

// State returns the current state of the Circuit Breaker.
func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}
