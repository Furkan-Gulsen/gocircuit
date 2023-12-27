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
	FailureThreshold   int           // The threshold for consecutive failures required to trip the circuit
	ResetTimeout       time.Duration // The duration after which the circuit transitions to half-open
	SuccessThreshold   int           // The threshold for consecutive successes required to reset the circuit
	AutoCloseThreshold int           // The threshold for consecutive successful executions required to auto-close the circuit
	AutoCloseDuration  time.Duration // The duration after which the circuit automatically closes if threshold not met
	OpenDuration       time.Duration // The duration for which the circuit remains open before transitioning to half-open
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
	fallbackFunc       func() error // Fallback function to execute on failure
}

// NewCircuitBreaker creates a new Circuit Breaker with the given configuration.
func NewCircuitBreaker(config CircuitBreakerConfig, fallbackFunc func() error) *CircuitBreaker {
	return &CircuitBreaker{
		state:        int32(StateClosed),
		config:       config,
		fallbackFunc: fallbackFunc, // Fallback function to handle failures
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
				cb.Reset()
			} else if lastAttemptSince > cb.config.AutoCloseDuration {
				// Auto close threshold not met, but auto close duration exceeded, close the circuit.
				cb.Reset()
			}
		}
		return nil // Successful execution, no error returned
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

	// Fallback mechanism: Execute the fallback function on failure.
	if cb.fallbackFunc != nil {
		fallbackErr := cb.fallbackFunc()
		if fallbackErr != nil {
			return fallbackErr
		}
	}

	// Handle auto close start time.
	if atomic.LoadInt32(&cb.state) == int32(StateClosed) {
		autoCloseStartTime := time.Now().Unix()
		atomic.StoreInt64(&cb.autoCloseStartTime, autoCloseStartTime)
	}

	return err // Failed execution, return the error
}

// reset resets the Circuit Breaker to the closed state.
func (cb *CircuitBreaker) Reset() {
	atomic.StoreInt32(&cb.failureCount, 0)
	atomic.StoreInt32(&cb.successCount, 0)
	atomic.StoreInt32(&cb.autoCloseCount, 0)
	atomic.StoreInt32(&cb.state, int32(StateClosed))
	atomic.StoreInt64(&cb.autoCloseStartTime, 0) // Reset auto close start time
}

// State returns the current state of the Circuit Breaker.
func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}
