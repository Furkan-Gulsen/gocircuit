package gocircuit

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     1 * time.Minute,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	if cb.State() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreakerOpenAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		ResetTimeout:     1 * time.Minute,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	action := func() error { return errors.New("failure") }

	// The first call should fail and open the Circuit Breaker.
	err := cb.Execute(action)
	if err == nil {
		t.Errorf("Expected circuit breaker to be open after failure, got %v", cb.State())
	}

	// Check if the state is Open.
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be Open after failure, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     100 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	action := func() error { return errors.New("failure") }

	t.Run("InitialFailure", func(t *testing.T) {
		_ = cb.Execute(action)             // Fail the request
		time.Sleep(200 * time.Millisecond) // Exceed the resetTimeout

		// Make the cb.State() call without locking
		state := cb.State()
		if state != StateHalfOpen {
			t.Errorf("Expected state to be HalfOpen after timeout, got %v", state)
		}
	})
}

func TestCircuitBreakerCloseAfterSuccess(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     100 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	failAction := func() error { return errors.New("failure") }
	successAction := func() error { return nil }

	t.Run("InitialFailure", func(t *testing.T) {
		_ = cb.Execute(failAction)         // Fail the request
		time.Sleep(200 * time.Millisecond) // Exceed the resetTimeout
		_ = cb.Execute(successAction)      // Succeed the request

		if cb.State() != StateHalfOpen {
			t.Errorf("Expected state to be StateHalfOpen after success, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerCloseAfterSuccessThreshold(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     100 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	successAction := func() error { return nil }

	t.Run("InitialFailure", func(t *testing.T) {
		_ = cb.Execute(successAction)
		_ = cb.Execute(successAction)

		if cb.State() != StateClosed {
			t.Errorf("Expected state to be StateClosed after success threshold, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerOpenAfterFailureThreshold(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     100 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)
	failAction := func() error { return errors.New("failure") }

	t.Run("InitialFailure", func(t *testing.T) {
		_ = cb.Execute(failAction)
		_ = cb.Execute(failAction)

		if cb.State() != StateOpen {
			t.Errorf("Expected state to be StateOpen after failure threshold, got %v", cb.State())
		}
	})
}

func TestCircuitBreakerWithFailedURL(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 10,
		ResetTimeout:     1 * time.Minute,
		SuccessThreshold: 5,
	}
	cb := NewCircuitBreaker(config)

	failingURLAction := func() error {
		return errors.New("Failed to access the URL")
	}

	for i := 0; i < 10; i++ {
		err := cb.Execute(failingURLAction)
		if err == nil {
			t.Errorf("Expected circuit breaker to be open after failure, got %v", cb.State())
		}
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected circuit breaker to be open after 10 consecutive failures, got %v", cb.State())
	}
}

func TestCircuitBreakerWithSuccessfulURL(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		ResetTimeout:     1 * time.Minute,
		SuccessThreshold: 3,
	}
	cb := NewCircuitBreaker(config)

	realURL := "https://example.com"

	realHTTPRequest := func() error {
		response, err := http.Get(realURL)
		if err != nil {
			fmt.Println("HTTP request failed:", err)
			return err
		}
		defer response.Body.Close()

		return nil
	}

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
		FailureThreshold: 5,
		ResetTimeout:     200 * time.Millisecond,
		SuccessThreshold: 3,
	}
	cb := NewCircuitBreaker(config)

	t.Run("ResetCircuitBreaker", func(t *testing.T) {
		cb.reset()

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
