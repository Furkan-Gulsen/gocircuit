package gocircuit

import (
	"sync/atomic"
	"time"
)

// CircuitState represents the state of the Circuit Breaker.
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

// CircuitBreakerConfig holds the configuration options for the Circuit Breaker.
type CircuitBreakerConfig struct {
	FailureThreshold   int           // Failure threshold
	ResetTimeout       time.Duration // Reset timeout duration
	SuccessThreshold   int           // Success threshold
	AutoCloseThreshold int           // Auto close threshold
	AutoCloseDuration  time.Duration // Auto close duration
	OpenDuration       time.Duration // Open duration
}

// CircuitBreaker represents a Circuit Breaker.
type CircuitBreaker struct {
	state              int32
	config             CircuitBreakerConfig // Configuration options
	failureCount       int32
	successCount       int32
	lastAttempt        int64
	autoCloseCount     int32
	autoCloseStartTime int64
	openStartTime      int64
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

	// Handle open state duration.
	if atomic.LoadInt32(&cb.state) == int32(StateOpen) {
		openSince := time.Since(time.Unix(atomic.LoadInt64(&cb.openStartTime), 0))
		if openSince > cb.config.OpenDuration {
			atomic.StoreInt32(&cb.state, int32(StateHalfOpen))
		}
	}

	// Execute the action.
	err := action()

	// Handle success.
	if err == nil {
		atomic.AddInt32(&cb.successCount, 1)
		if atomic.LoadInt32(&cb.successCount) >= int32(cb.config.SuccessThreshold) {
			if cb.autoCloseCount >= int32(cb.config.AutoCloseThreshold) {
				cb.reset()
			} else if lastAttemptSince > cb.config.AutoCloseDuration {
				// Auto close threshold not met, but auto close duration exceeded, close the circuit.
				cb.reset()
			}
		}
		return nil // Başarılı işlem, hata dönüşü yok
	}

	// Handle failure.
	atomic.AddInt32(&cb.failureCount, 1)
	if atomic.LoadInt32(&cb.failureCount) >= int32(cb.config.FailureThreshold) {
		atomic.StoreInt32(&cb.state, int32(StateOpen))
		atomic.StoreInt64(&cb.openStartTime, time.Now().Unix())
		atomic.StoreInt64(&cb.lastAttempt, time.Now().Unix())
	} else {
		// Reset the auto close count on each failure.
		atomic.StoreInt32(&cb.autoCloseCount, 0)
	}

	return err // Başarısız işlem, hata dönüşü var
}

// reset resets the Circuit Breaker to the closed state.
func (cb *CircuitBreaker) reset() {
	atomic.StoreInt32(&cb.failureCount, 0)
	atomic.StoreInt32(&cb.successCount, 0)
	atomic.StoreInt32(&cb.autoCloseCount, 0)
	atomic.StoreInt32(&cb.state, int32(StateClosed))
}

// State returns the current state of the Circuit Breaker.
func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}
