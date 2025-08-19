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

package framework

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TestContext provides a common testing context for KCP integration tests.
// It includes shared configuration, client setup, and utility methods.
type TestContext struct {
	T           *testing.T
	Context     context.Context
	CancelFunc  context.CancelFunc
	Config      *rest.Config
	KubeconfigPath string
	
	// Test configuration
	TestTimeout time.Duration
	PollInterval time.Duration
	
	// Test state
	CleanupFuncs []func()
}

// NewTestContext creates a new test context with default configuration.
func NewTestContext(t *testing.T) *TestContext {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	
	tc := &TestContext{
		T:            t,
		Context:      ctx,
		CancelFunc:   cancel,
		TestTimeout:  10 * time.Minute,
		PollInterval: 1 * time.Second,
		CleanupFuncs: make([]func(), 0),
	}
	
	t.Cleanup(tc.Cleanup)
	
	return tc
}

// NewTestContextWithConfig creates a test context with a specific kubeconfig.
func NewTestContextWithConfig(t *testing.T, kubeconfigPath string) (*TestContext, error) {
	tc := NewTestContext(t)
	tc.KubeconfigPath = kubeconfigPath
	
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		tc.Cleanup()
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}
	
	tc.Config = config
	return tc, nil
}

// AddCleanup adds a cleanup function to be executed when the test completes.
func (tc *TestContext) AddCleanup(cleanup func()) {
	tc.CleanupFuncs = append(tc.CleanupFuncs, cleanup)
}

// Cleanup executes all registered cleanup functions.
func (tc *TestContext) Cleanup() {
	// Cancel the context first
	if tc.CancelFunc != nil {
		tc.CancelFunc()
	}
	
	// Execute cleanup functions in reverse order
	for i := len(tc.CleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					tc.T.Logf("Cleanup function panicked: %v", r)
				}
			}()
			tc.CleanupFuncs[i]()
		}()
	}
}

// Logf logs a formatted message with test context information.
func (tc *TestContext) Logf(format string, args ...interface{}) {
	tc.T.Logf("[%s] %s", tc.T.Name(), fmt.Sprintf(format, args...))
}

// Errorf reports an error with test context information.
func (tc *TestContext) Errorf(format string, args ...interface{}) {
	tc.T.Errorf("[%s] %s", tc.T.Name(), fmt.Sprintf(format, args...))
}

// Fatalf reports a fatal error with test context information.
func (tc *TestContext) Fatalf(format string, args ...interface{}) {
	tc.T.Fatalf("[%s] %s", tc.T.Name(), fmt.Sprintf(format, args...))
}

// WaitFor executes a condition function repeatedly until it returns true,
// an error, or the context timeout is reached.
func (tc *TestContext) WaitFor(conditionMsg string, condition func() (bool, error)) error {
	tc.Logf("Waiting for: %s", conditionMsg)
	
	return wait.PollImmediate(tc.PollInterval, tc.TestTimeout, condition)
}

// WaitForWithContext executes a condition function with a custom context.
func (tc *TestContext) WaitForWithContext(ctx context.Context, conditionMsg string, condition func() (bool, error)) error {
	tc.Logf("Waiting for: %s", conditionMsg)
	
	return wait.PollImmediateUntil(tc.PollInterval, condition, ctx.Done())
}

// Eventually is a convenience method for eventual consistency testing.
func (tc *TestContext) Eventually(conditionMsg string, condition func() bool) {
	tc.Logf("Eventually checking: %s", conditionMsg)
	
	err := wait.PollImmediate(tc.PollInterval, tc.TestTimeout, func() (bool, error) {
		return condition(), nil
	})
	
	if err != nil {
		tc.Fatalf("Condition never became true: %s", conditionMsg)
	}
}

// WithTimeout creates a new context with the specified timeout.
func (tc *TestContext) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(tc.Context, timeout)
}

// WithDeadline creates a new context with the specified deadline.
func (tc *TestContext) WithDeadline(deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(tc.Context, deadline)
}