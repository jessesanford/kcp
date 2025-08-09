/*
Copyright 2025 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workqueue

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// circuitBreaker implements circuit breaker pattern to prevent cascading failures.
type circuitBreaker struct {
	config CircuitBreakerConfig
	state  CircuitBreakerState
	
	failures     int
	successes    int
	lastFailTime time.Time
	
	mu sync.RWMutex
}

// newCircuitBreaker creates a new circuit breaker with the specified configuration.
func newCircuitBreaker(config CircuitBreakerConfig) (*circuitBreaker, error) {
	if config.MaxFailures <= 0 {
		return nil, fmt.Errorf("max failures must be greater than 0")
	}
	if config.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than 0")
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 1
	}

	return &circuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
	}, nil
}

// Allow determines if a request should be allowed through the circuit breaker.
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		// Circuit is closed, allow all requests
		return true
		
	case CircuitBreakerOpen:
		// Circuit is open, check if we should transition to half-open
		if time.Since(cb.lastFailTime) >= cb.config.Timeout {
			cb.transitionTo(CircuitBreakerHalfOpen)
			return true
		}
		return false
		
	case CircuitBreakerHalfOpen:
		// Circuit is half-open, allow limited requests
		return true
		
	default:
		return false
	}
}

// RecordSuccess records a successful operation.
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0 // Reset failure count on success

	switch cb.state {
	case CircuitBreakerHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitBreakerClosed)
			cb.successes = 0
		}
		
	case CircuitBreakerClosed:
		// Already closed, nothing to do
		
	case CircuitBreakerOpen:
		// Shouldn't happen, but handle gracefully
		klog.V(4).Info("Recorded success while circuit breaker is open")
	}
}

// RecordFailure records a failed operation.
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.transitionTo(CircuitBreakerOpen)
		}
		
	case CircuitBreakerHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.transitionTo(CircuitBreakerOpen)
		cb.successes = 0
		
	case CircuitBreakerOpen:
		// Already open, update failure count and timestamp
	}
}

// GetState returns the current circuit breaker state.
func (cb *circuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count.
func (cb *circuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// GetSuccessCount returns the current success count (used in half-open state).
func (cb *circuitBreaker) GetSuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successes
}

// transitionTo transitions the circuit breaker to a new state.
func (cb *circuitBreaker) transitionTo(newState CircuitBreakerState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	klog.V(4).Infof("Circuit breaker state transition: %v -> %v", oldState, newState)

	// Call state change callback if configured
	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, newState)
	}
}

// Reset resets the circuit breaker to its initial state.
func (cb *circuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = CircuitBreakerClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastFailTime = time.Time{}

	if oldState != CircuitBreakerClosed && cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, CircuitBreakerClosed)
	}

	klog.V(4).Info("Circuit breaker reset to closed state")
}

// String returns a string representation of the circuit breaker state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "Closed"
	case CircuitBreakerOpen:
		return "Open"
	case CircuitBreakerHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}