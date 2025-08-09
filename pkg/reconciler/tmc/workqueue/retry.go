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
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// RetryManager handles intelligent retry logic for work items.
type RetryManager struct {
	policy RetryPolicy
}

// NewRetryManager creates a new retry manager with the specified policy.
func NewRetryManager(policy RetryPolicy) *RetryManager {
	return &RetryManager{
		policy: policy,
	}
}

// ShouldRetry determines if a work item should be retried based on the error and attempt count.
func (r *RetryManager) ShouldRetry(item *WorkItem, err error) bool {
	// Check if we've exceeded max attempts
	if item.Attempts >= r.policy.MaxAttempts {
		klog.V(4).Infof("Work item %s exceeded max attempts (%d), not retrying", 
			item.Key, r.policy.MaxAttempts)
		return false
	}

	// Check if the error is retryable
	if !r.isRetryableError(err) {
		klog.V(4).Infof("Work item %s failed with non-retryable error: %v", 
			item.Key, err)
		return false
	}

	return true
}

// GetRetryDelay calculates the delay before retrying a work item.
func (r *RetryManager) GetRetryDelay(item *WorkItem, err error) time.Duration {
	// Check for error-specific backoff override
	for _, matcher := range r.policy.RetryableErrors {
		if matcher.Matches(err) {
			if override := matcher.GetBackoffOverride(); override != nil {
				return *override
			}
		}
	}

	// Calculate exponential backoff
	delay := r.calculateExponentialBackoff(item.Attempts)

	// Add jitter if enabled
	if r.policy.Jitter {
		delay = r.addJitter(delay)
	}

	// Ensure delay is within bounds
	if delay < r.policy.BaseDelay {
		delay = r.policy.BaseDelay
	}
	if delay > r.policy.MaxDelay {
		delay = r.policy.MaxDelay
	}

	klog.V(6).Infof("Calculated retry delay for work item %s: %v (attempt %d)", 
		item.Key, delay, item.Attempts)

	return delay
}

// RetryWithBackoff retries a work item with exponential backoff.
func (r *RetryManager) RetryWithBackoff(ctx context.Context, item *WorkItem, processor ProcessorFunc) error {
	var lastErr error

	backoff := wait.Backoff{
		Duration: r.policy.BaseDelay,
		Factor:   r.policy.BackoffFactor,
		Jitter:   0.1,
		Steps:    r.policy.MaxAttempts,
		Cap:      r.policy.MaxDelay,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		item.Attempts++
		item.LastAttemptAt = time.Now()

		klog.V(6).Infof("Retrying work item %s (attempt %d/%d)", 
			item.Key, item.Attempts, r.policy.MaxAttempts)

		err := processor(ctx, item)
		if err == nil {
			klog.V(4).Infof("Work item %s succeeded after %d attempts", 
				item.Key, item.Attempts)
			return true, nil // Success, stop retrying
		}

		lastErr = err
		item.LastError = err

		// Check if we should continue retrying
		if !r.ShouldRetry(item, err) {
			klog.V(4).Infof("Work item %s failed permanently after %d attempts: %v", 
				item.Key, item.Attempts, err)
			return false, err // Permanent failure, stop retrying
		}

		klog.V(6).Infof("Work item %s failed (attempt %d/%d), will retry: %v", 
			item.Key, item.Attempts, r.policy.MaxAttempts, err)
		return false, nil // Temporary failure, continue retrying
	})

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("retry timeout exceeded for work item %s: %w", item.Key, lastErr)
		}
		return err
	}

	return lastErr
}

// isRetryableError checks if an error should trigger a retry.
func (r *RetryManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// If no specific retryable errors are configured, retry all errors
	if len(r.policy.RetryableErrors) == 0 {
		return true
	}

	// Check if any matcher matches the error
	for _, matcher := range r.policy.RetryableErrors {
		if matcher.Matches(err) {
			return true
		}
	}

	return false
}

// calculateExponentialBackoff calculates the delay using exponential backoff.
func (r *RetryManager) calculateExponentialBackoff(attempts int) time.Duration {
	if attempts <= 0 {
		return r.policy.BaseDelay
	}

	// Calculate exponential backoff: baseDelay * (backoffFactor ^ (attempts - 1))
	multiplier := math.Pow(r.policy.BackoffFactor, float64(attempts-1))
	delay := time.Duration(float64(r.policy.BaseDelay) * multiplier)

	return delay
}

// addJitter adds randomness to the delay to avoid thundering herd.
func (r *RetryManager) addJitter(delay time.Duration) time.Duration {
	// Add up to 25% jitter
	jitterRange := float64(delay) * 0.25
	jitter := time.Duration(rand.Float64() * jitterRange)
	
	// Randomly add or subtract jitter
	if rand.Float64() < 0.5 {
		return delay + jitter
	}
	return delay - jitter
}

// Common error matchers

// RetryableErrorMatcher is a simple error matcher based on error messages.
type RetryableErrorMatcher struct {
	ErrorSubstrings []string
	BackoffOverride *time.Duration
}

// Matches checks if the error message contains any of the specified substrings.
func (m *RetryableErrorMatcher) Matches(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	for _, substring := range m.ErrorSubstrings {
		if len(substring) > 0 && len(errMsg) > 0 {
			// Simple substring check (could be enhanced with regex)
			for i := 0; i <= len(errMsg)-len(substring); i++ {
				if errMsg[i:i+len(substring)] == substring {
					return true
				}
			}
		}
	}

	return false
}

// GetBackoffOverride returns the custom backoff duration for this error type.
func (m *RetryableErrorMatcher) GetBackoffOverride() *time.Duration {
	return m.BackoffOverride
}

// NetworkErrorMatcher matches common network-related errors.
type NetworkErrorMatcher struct {
	BackoffOverride *time.Duration
}

// Matches checks if the error is network-related.
func (m *NetworkErrorMatcher) Matches(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network unreachable",
		"no route to host",
		"timeout",
		"EOF",
		"broken pipe",
	}

	for _, networkErr := range networkErrors {
		if len(networkErr) > 0 && len(errMsg) > 0 {
			// Simple substring check
			for i := 0; i <= len(errMsg)-len(networkErr); i++ {
				if errMsg[i:i+len(networkErr)] == networkErr {
					return true
				}
			}
		}
	}

	return false
}

// GetBackoffOverride returns the custom backoff duration for network errors.
func (m *NetworkErrorMatcher) GetBackoffOverride() *time.Duration {
	return m.BackoffOverride
}

// ResourceConflictMatcher matches Kubernetes resource conflict errors.
type ResourceConflictMatcher struct {
	BackoffOverride *time.Duration
}

// Matches checks if the error is a resource conflict.
func (m *ResourceConflictMatcher) Matches(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	conflictErrors := []string{
		"conflict",
		"the object has been modified",
		"operation cannot be fulfilled",
		"resource version",
	}

	for _, conflictErr := range conflictErrors {
		if len(conflictErr) > 0 && len(errMsg) > 0 {
			// Simple substring check
			for i := 0; i <= len(errMsg)-len(conflictErr); i++ {
				if errMsg[i:i+len(conflictErr)] == conflictErr {
					return true
				}
			}
		}
	}

	return false
}

// GetBackoffOverride returns the custom backoff duration for resource conflicts.
func (m *ResourceConflictMatcher) GetBackoffOverride() *time.Duration {
	return m.BackoffOverride
}