package gocircuit

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	// Create a Circuit Breaker with the initial configuration.
	config := CircuitBreakerConfig{
		FailureThreshold:   3,                      // Number of consecutive failures required to trip the circuit.
		ResetTimeout:       1 * time.Minute,        // Duration after which the circuit transitions to half-open.
		SuccessThreshold:   2,                      // Number of consecutive successes required to reset the circuit.
		AutoCloseThreshold: 2,                      // Number of consecutive successful executions required to auto-close the circuit.
		AutoCloseDuration:  500 * time.Millisecond, // Duration after which the circuit automatically closes if the threshold is not met.
		OpenDuration:       1 * time.Second,        // Duration for which the circuit remains open before transitioning to half-open.
	}
	cb := NewCircuitBreaker(config, nil)

	// Verify that the initial state of the Circuit Breaker is Closed.
	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreakerOpenAfterFailures(t *testing.T) {
	// Create a Circuit Breaker with a configuration that triggers an open state after a single failure.
	config := CircuitBreakerConfig{
		FailureThreshold:   1, // Only one failure is required to open the circuit.
		ResetTimeout:       1 * time.Minute,
		SuccessThreshold:   2,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define a function that always returns an error to simulate a failed action.
	action := func() error { return errors.New("failure") }

	// The first call should fail and open the Circuit Breaker.
	err := cb.Execute(action)
	if err == nil {
		t.Errorf("Expected circuit breaker to be open after failure, got %v", cb.State())
	}

	// Verify that the state is Open.
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be Open after failure, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	// Create a Circuit Breaker with a configuration that transitions to half-open after a short reset timeout.
	config := CircuitBreakerConfig{
		FailureThreshold:   2,
		ResetTimeout:       100 * time.Millisecond, // Reset timeout is short for testing.
		SuccessThreshold:   2,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define a function that always returns an error to simulate a failed action.
	action := func() error { return errors.New("failure") }

	t.Run("InitialFailure", func(t *testing.T) {
		// Execute the action to fail it.
		_ = cb.Execute(action)

		// Wait for a duration that exceeds the reset timeout.
		time.Sleep(1200 * time.Millisecond)

		// Make a cb.State() call without locking.
		state := cb.State()
		if state != StateHalfOpen {
			t.Errorf("Expected state to be HalfOpen after timeout, got %v", state)
		}
	})
}

func TestCircuitBreakerCloseAfterSuccess(t *testing.T) {
	// Create a Circuit Breaker with a configuration for testing a successful transition to HalfOpen.
	config := CircuitBreakerConfig{
		FailureThreshold:   2,
		ResetTimeout:       100 * time.Millisecond,
		SuccessThreshold:   2,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define functions for failure and success actions.
	failAction := func() error { return errors.New("failure") }
	successAction := func() error { return nil }

	t.Run("InitialFailure", func(t *testing.T) {
		// Execute the action to fail it.
		_ = cb.Execute(failAction)

		// Wait for a short duration.
		time.Sleep(200 * time.Millisecond)

		// Execute the action again to succeed.
		_ = cb.Execute(successAction)

		if cb.State() != StateHalfOpen {
			t.Errorf("Expected state to be StateHalfOpen after success, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerCloseAfterSuccessThreshold(t *testing.T) {
	// Create a Circuit Breaker with a configuration for testing a successful reset.
	config := CircuitBreakerConfig{
		FailureThreshold:   2,
		ResetTimeout:       100 * time.Millisecond,
		SuccessThreshold:   2, // Set the success threshold to 2 for testing.
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define a function for a successful action.
	successAction := func() error { return nil }

	t.Run("InitialFailure", func(t *testing.T) {
		// Execute the action twice to meet the success threshold.
		_ = cb.Execute(successAction)
		_ = cb.Execute(successAction)

		if cb.State() != StateClosed {
			t.Errorf("Expected state to be StateClosed after success threshold, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerOpenAfterFailureThreshold(t *testing.T) {
	// Create a Circuit Breaker with a configuration for testing an open state after failure.
	config := CircuitBreakerConfig{
		FailureThreshold:   2, // Set the failure threshold to 2 for testing.
		ResetTimeout:       100 * time.Millisecond,
		SuccessThreshold:   2,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define a function for a failure action.
	failAction := func() error { return errors.New("failure") }

	t.Run("InitialFailure", func(t *testing.T) {
		// Execute the action twice to meet the failure threshold.
		_ = cb.Execute(failAction)
		_ = cb.Execute(failAction)

		if cb.State() != StateOpen {
			t.Errorf("Expected state to be StateOpen after failure threshold, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerWithFailedURL(t *testing.T) {
	// Create a Circuit Breaker with a configuration for testing failed URL requests.
	config := CircuitBreakerConfig{
		FailureThreshold:   10,
		ResetTimeout:       1 * time.Minute,
		SuccessThreshold:   5,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	// Define a function representing a failed URL request.
	failingURLAction := func() error {
		return errors.New("Failed to access the URL")
	}

	// Execute the failing URL request function 10 times.
	for i := 0; i < 10; i++ {
		err := cb.Execute(failingURLAction)
		if err == nil {
			t.Errorf("Expected circuit breaker to be open after failure, got %v", cb.State())
		}
	}

	// Verify that the Circuit Breaker is in an open state.
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit breaker to be open after 10 consecutive failures, got %v", cb.State())
	}
}

func TestCircuitBreakerWithSuccessfulURL(t *testing.T) {
	// Create a Circuit Breaker with a configuration for testing successful URL requests.
	config := CircuitBreakerConfig{
		FailureThreshold:   5,
		ResetTimeout:       1 * time.Minute,
		SuccessThreshold:   3,
		AutoCloseThreshold: 2,
		AutoCloseDuration:  500 * time.Millisecond,
		OpenDuration:       1 * time.Second,
	}
	cb := NewCircuitBreaker(config, nil)

	realURL := "https://example.com"

	// Define a function for making a real HTTP request to the URL.
	realHTTPRequest := func() error {
		response, err := http.Get(realURL)
		if err != nil {
			fmt.Println("HTTP request failed:", err)
			return err
		}
		defer response.Body.Close()

		return nil
	}

	// Execute the real HTTP request function using the Circuit Breaker.
	err := cb.Execute(realHTTPRequest)
	if err != nil {
		fmt.Println("Rejected by Circuit Breaker:", err)
	}

	fmt.Println("Circuit Breaker State:", cb.State())
}

func TestCircuitBreakerWithRealAndFakeURLs(t *testing.T) {
	realURL := "https://example.com"
	fakeURL := "https://nonexistenturl.com"

	realHTTPRequest := func(url string) func() error {
		return func() error {
			response, err := http.Get(url)
			if err != nil {
				fmt.Println("Real URL HTTP request failed:", err)
				return err
			}
			defer response.Body.Close()

			return nil
		}
	}

	fakeHTTPRequest := func(url string) func() error {
		return func() error {
			response, err := http.Get(url)
			if err != nil {
				fmt.Println("Fake URL HTTP request failed:", err)
				return err
			}
			defer response.Body.Close()

			return nil
		}
	}

	config := CircuitBreakerConfig{
		FailureThreshold:   5,
		ResetTimeout:       200 * time.Millisecond,
		SuccessThreshold:   3,
		AutoCloseThreshold: 2,                      // Added auto close threshold
		AutoCloseDuration:  500 * time.Millisecond, // Added auto close duration
		OpenDuration:       1 * time.Second,        // Added open duration
	}
	cb := NewCircuitBreaker(config, nil)

	t.Run("ResetCircuitBreaker", func(t *testing.T) {
		cb.Reset()

		if cb.State() != StateClosed {
			t.Errorf("Circuit breaker should be closed, state: %v", cb.State())
		}
	})

	t.Run("RealURLRequest", func(t *testing.T) {
		req := realHTTPRequest(realURL)
		cb.Execute(req)
		cb.Execute(req)
		cb.Execute(req)

		<-time.After(300 * time.Millisecond)

		if cb.State() != StateClosed {
			t.Errorf("Circuit breaker should be closed, state: %v", cb.State())
		}
	})

	t.Run("FakeURLRequest", func(t *testing.T) {
		req := fakeHTTPRequest(fakeURL)
		cb.Execute(req)
		cb.Execute(req)
		cb.Execute(req)
		cb.Execute(req)
		cb.Execute(req)

		if cb.State() != StateOpen {
			t.Errorf("Circuit breaker should be open for fake URL, state: %v", cb.State())
		}
	})
}
