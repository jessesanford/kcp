/*
Copyright 2024 The KCP Authors.

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

package applier

import (
	"context"
	"math"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

// RetryStrategy defines how to retry operations that fail.
type RetryStrategy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Factor is the multiplier for exponential backoff
	Factor float64
	// Jitter is the maximum random jitter to add (as fraction of delay)
	Jitter float64
	// RetryCondition determines whether to retry based on the error
	RetryCondition func(error) bool
}

// NewDefaultRetryStrategy creates a retry strategy with sensible defaults.
func NewDefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:     3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       5 * time.Second,
		Factor:         2.0,
		Jitter:         0.1,
		RetryCondition: DefaultRetryCondition,
	}
}

// Execute runs the given function with retry logic.
func (rs *RetryStrategy) Execute(ctx context.Context, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= rs.MaxRetries; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			// Success on first try or retry
			if attempt > 0 {
				klog.FromContext(ctx).V(4).Info("Operation succeeded after retries", "attempts", attempt+1)
			}
			return nil
		}
		
		lastErr = err
		
		// Don't retry on the last attempt or if retry condition is not met
		if attempt == rs.MaxRetries || !rs.RetryCondition(err) {
			break
		}
		
		// Calculate delay with exponential backoff and jitter
		delay := rs.calculateDelay(attempt)
		
		klog.FromContext(ctx).V(4).Info("Operation failed, retrying", 
			"attempt", attempt+1, 
			"maxRetries", rs.MaxRetries, 
			"delay", delay,
			"error", err)
		
		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}
	
	return lastErr
}

// calculateDelay computes the delay for the given attempt with exponential backoff and jitter.
func (rs *RetryStrategy) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(rs.InitialDelay) * math.Pow(rs.Factor, float64(attempt))
	
	// Cap at max delay
	if delay > float64(rs.MaxDelay) {
		delay = float64(rs.MaxDelay)
	}
	
	// Add jitter to prevent thundering herd
	if rs.Jitter > 0 {
		jitter := delay * rs.Jitter * (rand.Float64()*2 - 1) // Random between -jitter and +jitter
		delay += jitter
	}
	
	// Ensure non-negative delay
	if delay < 0 {
		delay = float64(rs.InitialDelay)
	}
	
	return time.Duration(delay)
}

// DefaultRetryCondition determines whether to retry based on common error patterns.
func DefaultRetryCondition(err error) bool {
	if err == nil {
		return false
	}
	
	// Retry on conflict errors (optimistic concurrency failures)
	if errors.IsConflict(err) {
		return true
	}
	
	// Retry on server errors (5xx)
	if errors.IsInternalError(err) || errors.IsServerTimeout(err) || errors.IsTimeout(err) {
		return true
	}
	
	// Retry on temporary network errors
	if errors.IsServiceUnavailable(err) || errors.IsTooManyRequests(err) {
		return true
	}
	
	// Don't retry on validation errors - they won't succeed
	if errors.IsInvalid(err) || errors.IsBadRequest(err) {
		return false
	}
	
	// Don't retry on authorization/authentication errors
	if errors.IsUnauthorized(err) || errors.IsForbidden(err) {
		return false
	}
	
	// Don't retry on not found errors for delete operations
	if errors.IsNotFound(err) {
		return false
	}
	
	// By default, don't retry unknown errors
	return false
}