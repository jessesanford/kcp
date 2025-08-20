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

package tmc

import (
	"context"
	"time"
)

// RetryStrategy defines the configuration for retry behavior
type RetryStrategy struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
}

// DefaultRetryStrategy returns a default retry strategy for TMC operations
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
	}
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func(ctx context.Context) error

// ExecuteWithRetry executes a retryable operation using the given retry strategy
func ExecuteWithRetry(operation RetryableOperation, strategy *RetryStrategy) error {
	if strategy == nil {
		strategy = DefaultRetryStrategy()
	}

	ctx := context.TODO()
	
	var lastErr error
	interval := strategy.InitialInterval

	for attempt := 0; attempt <= strategy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(interval)
			
			// Increase interval for next attempt, up to max
			interval = time.Duration(float64(interval) * strategy.Multiplier)
			if interval > strategy.MaxInterval {
				interval = strategy.MaxInterval
			}
		}

		err := operation(ctx)
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Check if this is a retryable error
		if !isRetryableError(err) {
			return err
		}
	}

	// All retries exhausted
	return NewTMCError(TMCErrorTypeSyncFailure, "retry-executor", "execute").
		WithMessage("all retry attempts exhausted").
		WithCause(lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if tmcErr, ok := err.(*TMCError); ok {
		// Don't retry validation errors
		return tmcErr.Type != TMCErrorTypeResourceValidation
	}
	
	// By default, treat unknown errors as retryable
	return true
}