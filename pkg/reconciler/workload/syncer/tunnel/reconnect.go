/*
Copyright 2022 The KCP Authors.

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

package tunnel

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// Reconnector handles connection retry logic with exponential backoff
type Reconnector struct {
	attempts    int
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	factor      float64
	jitter      float64
	mu          sync.RWMutex
	
	// Circuit breaker state
	state       circuitState
	failures    int
	maxFailures int
	lastFailure time.Time
	resetTime   time.Duration
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// NewReconnector creates a new reconnector with default settings
func NewReconnector() *Reconnector {
	return &Reconnector{
		maxAttempts: 10,
		baseDelay:   1 * time.Second,
		maxDelay:    5 * time.Minute,
		factor:      2.0,
		jitter:      0.1,
		maxFailures: 5,
		resetTime:   1 * time.Minute,
		state:       circuitClosed,
	}
}

// NewReconnectorWithConfig creates a new reconnector with custom configuration
func NewReconnectorWithConfig(maxAttempts int, baseDelay, maxDelay time.Duration, factor, jitter float64) *Reconnector {
	return &Reconnector{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
		factor:      factor,
		jitter:      jitter,
		maxFailures: 5,
		resetTime:   1 * time.Minute,
		state:       circuitClosed,
	}
}

// ShouldRetry returns whether a retry should be attempted
func (r *Reconnector) ShouldRetry() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Check circuit breaker state
	switch r.state {
	case circuitOpen:
		// Check if enough time has passed to attempt reset
		if time.Since(r.lastFailure) > r.resetTime {
			return true // Allow one attempt to test if service recovered
		}
		return false
	case circuitHalfOpen:
		return true // Allow one attempt
	case circuitClosed:
		return r.attempts < r.maxAttempts
	}
	
	return false
}

// NextDelay calculates the next retry delay with exponential backoff and jitter
func (r *Reconnector) NextDelay() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.attempts++
	
	// Calculate exponential backoff
	delay := float64(r.baseDelay) * math.Pow(r.factor, float64(r.attempts-1))
	
	// Apply maximum delay limit
	if delay > float64(r.maxDelay) {
		delay = float64(r.maxDelay)
	}
	
	// Add jitter to prevent thundering herd
	if r.jitter > 0 {
		jitterRange := delay * r.jitter
		jitterValue := (rand.Float64() * 2 - 1) * jitterRange // Random value between -jitterRange and +jitterRange
		delay += jitterValue
	}
	
	// Ensure delay is not negative
	if delay < 0 {
		delay = float64(r.baseDelay)
	}
	
	return time.Duration(delay)
}

// RecordSuccess resets the reconnection state after a successful connection
func (r *Reconnector) RecordSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.attempts = 0
	r.failures = 0
	r.state = circuitClosed
	
	klog.V(2).Info("Reconnector: success recorded, state reset")
}

// RecordFailure records a connection failure and updates circuit breaker state
func (r *Reconnector) RecordFailure() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.failures++
	r.lastFailure = time.Now()
	
	// Update circuit breaker state
	switch r.state {
	case circuitClosed:
		if r.failures >= r.maxFailures {
			r.state = circuitOpen
			klog.V(2).Info("Reconnector: circuit breaker opened", "failures", r.failures)
		}
	case circuitHalfOpen:
		// Failed during half-open state, go back to open
		r.state = circuitOpen
		klog.V(2).Info("Reconnector: circuit breaker re-opened after half-open failure")
	}
	
	klog.V(2).Info("Reconnector: failure recorded", "attempts", r.attempts, "failures", r.failures, "state", r.state)
}

// Reset resets the reconnection state
func (r *Reconnector) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.attempts = 0
	r.failures = 0
	r.state = circuitClosed
	
	klog.V(2).Info("Reconnector: state manually reset")
}

// GetAttempts returns the current number of attempts
func (r *Reconnector) GetAttempts() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.attempts
}

// GetState returns the current circuit breaker state
func (r *Reconnector) GetState() circuitState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// IsCircuitOpen returns whether the circuit breaker is open
func (r *Reconnector) IsCircuitOpen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state == circuitOpen
}

// TryReconnect attempts reconnection with circuit breaker logic
func (r *Reconnector) TryReconnect(ctx context.Context, connectFn func(ctx context.Context) error) error {
	r.mu.Lock()
	
	// Check if we should allow the attempt based on circuit breaker state
	if r.state == circuitOpen && time.Since(r.lastFailure) > r.resetTime {
		r.state = circuitHalfOpen
		klog.V(2).Info("Reconnector: circuit breaker half-opened for testing")
	}
	
	if r.state == circuitOpen {
		r.mu.Unlock()
		return &CircuitBreakerOpenError{LastFailure: r.lastFailure, ResetTime: r.resetTime}
	}
	
	r.mu.Unlock()
	
	// Attempt connection
	err := connectFn(ctx)
	
	if err != nil {
		r.RecordFailure()
		return err
	}
	
	r.RecordSuccess()
	return nil
}

// CircuitBreakerOpenError indicates the circuit breaker is open
type CircuitBreakerOpenError struct {
	LastFailure time.Time
	ResetTime   time.Duration
}

func (e *CircuitBreakerOpenError) Error() string {
	return "circuit breaker is open, too many consecutive failures"
}

// IsCircuitBreakerOpen returns true if the error indicates circuit breaker is open
func IsCircuitBreakerOpen(err error) bool {
	_, ok := err.(*CircuitBreakerOpenError)
	return ok
}