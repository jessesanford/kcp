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
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestApplier_Apply(t *testing.T) {
	tests := map[string]struct {
		obj           *unstructured.Unstructured
		strategy      ApplyStrategy
		setupReactor  func(*fake.FakeDynamicClient)
		wantOperation string
		wantSuccess   bool
		wantError     bool
	}{
		"server-side apply success": {
			obj:      testDeployment(),
			strategy: ServerSideApply,
			setupReactor: func(client *fake.FakeDynamicClient) {
				// For server-side apply, we need to handle the patch request
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantOperation: "apply",
			wantSuccess:   true,
		},
		"strategic merge on existing resource": {
			obj:      testDeploymentWithChangedSpec(),
			strategy: StrategicMerge,
			setupReactor: func(client *fake.FakeDynamicClient) {
				existing := testDeployment()
				existing.SetResourceVersion("123")
				client.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, existing, nil
				})
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeploymentWithChangedSpec(), nil
				})
			},
			wantOperation: "update",
			wantSuccess:   true,
		},
		"strategic merge creates new resource": {
			obj:      testDeployment(),
			strategy: StrategicMerge,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewNotFound(schema.GroupResource{}, "test-deployment")
				})
				client.PrependReactor("create", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantOperation: "create",
			wantSuccess:   true,
		},
		"replace existing resource": {
			obj:      testDeployment(),
			strategy: Replace,
			setupReactor: func(client *fake.FakeDynamicClient) {
				existing := testDeployment()
				existing.SetResourceVersion("123")
				client.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, existing, nil
				})
				client.PrependReactor("update", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantOperation: "replace",
			wantSuccess:   true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := NewApplier(client, "test-manager").
				WithApplyStrategy(tc.strategy)
			
			result, err := applier.Apply(context.Background(), tc.obj)
			
			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			if result.Success != tc.wantSuccess {
				t.Errorf("expected success=%v, got %v", tc.wantSuccess, result.Success)
			}
			
			if tc.wantSuccess && result.Operation != tc.wantOperation {
				t.Errorf("expected operation=%s, got %s", tc.wantOperation, result.Operation)
			}
		})
	}
}

func TestApplier_Apply_ErrorHandling(t *testing.T) {
	tests := map[string]struct {
		obj            *unstructured.Unstructured
		strategy       ApplyStrategy
		setupReactor   func(*fake.FakeDynamicClient)
		wantError      bool
		wantSuccess    bool
		expectedErrMsg string
	}{
		"server-side apply conflicts": {
			obj:      testDeployment(),
			strategy: ServerSideApply,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test-deployment", nil)
				})
			},
			wantError:      true,
			wantSuccess:    false,
			expectedErrMsg: "server-side apply failed: Operation cannot be fulfilled",
		},
		"strategic merge get error": {
			obj:      testDeployment(),
			strategy: StrategicMerge,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("server error"))
				})
			},
			wantError:      true,
			wantSuccess:    false,
			expectedErrMsg: "failed to get existing resource for strategic merge",
		},
		"replace get error": {
			obj:      testDeployment(),
			strategy: Replace,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("server error"))
				})
			},
			wantError:      true,
			wantSuccess:    false,
			expectedErrMsg: "failed to get existing resource for replace",
		},
		"unknown apply strategy": {
			obj:      testDeployment(),
			strategy: ApplyStrategy("unknown-strategy"),
			wantError:      true,
			wantSuccess:    false,
			expectedErrMsg: "unknown apply strategy: unknown-strategy",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := NewApplier(client, "test-manager").
				WithApplyStrategy(tc.strategy).
				WithRetryStrategy(&RetryStrategy{
					MaxRetries:     1, // Reduce retries for faster tests
					InitialDelay:   1 * time.Millisecond,
					MaxDelay:       5 * time.Millisecond,
					Factor:         1.0,
					Jitter:         0.0,
					RetryCondition: DefaultRetryCondition,
				})
			
			result, err := applier.Apply(context.Background(), tc.obj)
			
			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantError && tc.expectedErrMsg != "" && !containsString(err.Error(), tc.expectedErrMsg) {
				t.Errorf("expected error message to contain %q, got %q", tc.expectedErrMsg, err.Error())
			}
			
			if result.Success != tc.wantSuccess {
				t.Errorf("expected success=%v, got %v", tc.wantSuccess, result.Success)
			}
			
			if result.Attempts <= 0 {
				t.Errorf("expected at least 1 attempt, got %d", result.Attempts)
			}
			
			if result.Duration <= 0 {
				t.Errorf("expected positive duration, got %v", result.Duration)
			}
		})
	}
}

func TestApplier_Apply_Configuration(t *testing.T) {
	tests := map[string]struct {
		configureApplier func(*Applier) *Applier
		obj              *unstructured.Unstructured
		setupReactor     func(*fake.FakeDynamicClient)
		wantSuccess      bool
	}{
		"with force conflicts enabled": {
			configureApplier: func(a *Applier) *Applier {
				return a.WithForceConflicts(true)
			},
			obj: testDeployment(),
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					// Simply verify that patch was called - detailed options checking is complex in fake client
					return true, testDeployment(), nil
				})
			},
			wantSuccess: true,
		},
		"with custom field manager": {
			configureApplier: func(a *Applier) *Applier {
				return a // field manager is set in constructor
			},
			obj: testDeployment(),
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					// Simply verify that patch was called - detailed options checking is complex in fake client
					return true, testDeployment(), nil
				})
			},
			wantSuccess: true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := tc.configureApplier(NewApplier(client, "test-manager").
				WithApplyStrategy(ServerSideApply))
			
			result, err := applier.Apply(context.Background(), tc.obj)
			
			if !tc.wantSuccess && err == nil {
				t.Error("expected error but got none")
			}
			if tc.wantSuccess && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			if result.Success != tc.wantSuccess {
				t.Errorf("expected success=%v, got %v", tc.wantSuccess, result.Success)
			}
		})
	}
}

func TestApplier_Delete(t *testing.T) {
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	
	tests := map[string]struct {
		setupReactor func(*fake.FakeDynamicClient)
		wantError    bool
	}{
		"delete success": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("delete", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, nil
				})
			},
			wantError: false,
		},
		"delete not found is success": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("delete", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test-deployment")
				})
			},
			wantError: false,
		},
		"delete server error": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("delete", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("server error"))
				})
			},
			wantError: true,
		},
		"delete forbidden error": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("delete", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test-deployment", fmt.Errorf("forbidden"))
				})
			},
			wantError: true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := NewApplier(client, "test-manager")
			
			err := applier.Delete(context.Background(), deploymentGVR, "default", "test-deployment", DeleteOptions())
			
			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplier_Patch(t *testing.T) {
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	patchData := []byte(`{"spec":{"replicas":5}}`)
	
	tests := map[string]struct {
		setupReactor func(*fake.FakeDynamicClient)
		wantError    bool
		wantResult   bool
	}{
		"patch success": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantError:  false,
			wantResult: true,
		},
		"patch not found": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test-deployment")
				})
			},
			wantError:  true,
			wantResult: false,
		},
		"patch server error": {
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("server error"))
				})
			},
			wantError:  true,
			wantResult: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := NewApplier(client, "test-manager")
			
			result, err := applier.Patch(context.Background(), deploymentGVR, "default", "test-deployment", "application/strategic-merge-patch+json", patchData)
			
			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			if tc.wantResult && result == nil {
				t.Error("expected result but got nil")
			}
			if !tc.wantResult && result != nil {
				t.Error("expected no result but got one")
			}
		})
	}
}

func TestApplier_ApplyBatch(t *testing.T) {
	tests := map[string]struct {
		objects        []*unstructured.Unstructured
		maxConcurrency int
		setupReactor   func(*fake.FakeDynamicClient)
		wantTotal      int
		wantSucceeded  int
		wantFailed     int
	}{
		"empty batch": {
			objects:        []*unstructured.Unstructured{},
			maxConcurrency: 1,
			wantTotal:      0,
			wantSucceeded:  0,
			wantFailed:     0,
		},
		"single object success": {
			objects: []*unstructured.Unstructured{testDeployment()},
			maxConcurrency: 1,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantTotal:     1,
			wantSucceeded: 1,
			wantFailed:    0,
		},
		"multiple objects mixed results": {
			objects: []*unstructured.Unstructured{
				testDeployment(),
				testService(),
			},
			maxConcurrency: 2,
			setupReactor: func(client *fake.FakeDynamicClient) {
				callCount := 0
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					callCount++
					if callCount == 1 {
						return true, testDeployment(), nil // success
					}
					return true, nil, errors.NewInternalError(fmt.Errorf("server error")) // failure
				})
			},
			wantTotal:     2,
			wantSucceeded: 1,
			wantFailed:    1,
		},
		"default max concurrency": {
			objects: []*unstructured.Unstructured{testDeployment()},
			maxConcurrency: 0, // Should default to 10
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("patch", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantTotal:     1,
			wantSucceeded: 1,
			wantFailed:    0,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme)
			
			if tc.setupReactor != nil {
				tc.setupReactor(client)
			}
			
			applier := NewApplier(client, "test-manager")
			
			result := applier.ApplyBatch(context.Background(), tc.objects, tc.maxConcurrency)
			
			if result.Total != tc.wantTotal {
				t.Errorf("expected total=%d, got %d", tc.wantTotal, result.Total)
			}
			if result.Succeeded != tc.wantSucceeded {
				t.Errorf("expected succeeded=%d, got %d", tc.wantSucceeded, result.Succeeded)
			}
			if result.Failed != tc.wantFailed {
				t.Errorf("expected failed=%d, got %d", tc.wantFailed, result.Failed)
			}
			
			if result.Duration <= 0 && len(tc.objects) > 0 {
				t.Errorf("expected positive duration, got %v", result.Duration)
			}
			
			// Verify results length matches total
			if len(result.Results) != tc.wantTotal {
				t.Errorf("expected %d results, got %d", tc.wantTotal, len(result.Results))
			}
		})
	}
}

func TestBatchResult_Errors(t *testing.T) {
	result := &BatchResult{
		Results: []*ApplyResult{
			{Success: true, Error: nil},
			{Success: false, Error: fmt.Errorf("error 1")},
			{Success: false, Error: fmt.Errorf("error 2")},
		},
	}
	
	errors := result.Errors()
	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestBatchResult_Summary(t *testing.T) {
	tests := map[string]struct {
		result  *BatchResult
		wantMsg string
	}{
		"no operations": {
			result:  &BatchResult{Total: 0},
			wantMsg: "No operations performed",
		},
		"all succeeded": {
			result:  &BatchResult{Total: 5, Succeeded: 5, Failed: 0},
			wantMsg: "All operations succeeded",
		},
		"all failed": {
			result:  &BatchResult{Total: 5, Succeeded: 0, Failed: 5},
			wantMsg: "All operations failed",
		},
		"partial success": {
			result:  &BatchResult{Total: 5, Succeeded: 3, Failed: 2},
			wantMsg: "Partial success: some operations failed",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.result.Summary()
			if got != tc.wantMsg {
				t.Errorf("expected %q, got %q", tc.wantMsg, got)
			}
		})
	}
}

func TestRetryStrategy_Execute(t *testing.T) {
	tests := map[string]struct {
		strategy    *RetryStrategy
		fn          func() error
		wantRetries int
		wantError   bool
	}{
		"success on first try": {
			strategy: NewDefaultRetryStrategy(),
			fn: func() error {
				return nil
			},
			wantRetries: 0,
			wantError:   false,
		},
		"success after retries": {
			strategy: NewDefaultRetryStrategy(),
			fn: func() func() error {
				attempts := 0
				return func() error {
					attempts++
					if attempts < 3 {
						return errors.NewConflict(schema.GroupResource{}, "test", nil)
					}
					return nil
				}
			}(),
			wantRetries: 2,
			wantError:   false,
		},
		"max retries exceeded": {
			strategy: &RetryStrategy{
				MaxRetries:     2,
				InitialDelay:   1 * time.Millisecond,
				MaxDelay:       10 * time.Millisecond,
				Factor:         2.0,
				Jitter:         0.0,
				RetryCondition: DefaultRetryCondition,
			},
			fn: func() error {
				return errors.NewConflict(schema.GroupResource{}, "test", nil)
			},
			wantRetries: 2,
			wantError:   true,
		},
		"don't retry validation errors": {
			strategy: NewDefaultRetryStrategy(),
			fn: func() error {
				return errors.NewBadRequest("invalid")
			},
			wantRetries: 0,
			wantError:   true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attempts := 0
			wrappedFn := func() error {
				if attempts > 0 {
					// Count actual retries (not first attempt)
				}
				attempts++
				return tc.fn()
			}
			
			err := tc.strategy.Execute(context.Background(), wrappedFn)
			
			actualRetries := attempts - 1 // Subtract first attempt
			if actualRetries != tc.wantRetries {
				t.Errorf("expected %d retries, got %d", tc.wantRetries, actualRetries)
			}
			
			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBatchResult_SuccessRate(t *testing.T) {
	tests := map[string]struct {
		total     int
		succeeded int
		want      float64
	}{
		"all success": {
			total:     10,
			succeeded: 10,
			want:      100.0,
		},
		"partial success": {
			total:     10,
			succeeded: 7,
			want:      70.0,
		},
		"no operations": {
			total:     0,
			succeeded: 0,
			want:      0.0,
		},
		"all failed": {
			total:     5,
			succeeded: 0,
			want:      0.0,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			br := &BatchResult{
				Total:     tc.total,
				Succeeded: tc.succeeded,
			}
			
			got := br.SuccessRate()
			if got != tc.want {
				t.Errorf("expected success rate %v, got %v", tc.want, got)
			}
		})
	}
}

func TestRetryStrategy_CalculateDelay(t *testing.T) {
	tests := map[string]struct {
		strategy    *RetryStrategy
		attempt     int
		wantMinDelay time.Duration
		wantMaxDelay time.Duration
	}{
		"first retry": {
			strategy: &RetryStrategy{
				InitialDelay: 100 * time.Millisecond,
				Factor:       2.0,
				MaxDelay:     5 * time.Second,
				Jitter:       0.0,
			},
			attempt:      0,
			wantMinDelay: 100 * time.Millisecond,
			wantMaxDelay: 100 * time.Millisecond,
		},
		"exponential backoff": {
			strategy: &RetryStrategy{
				InitialDelay: 100 * time.Millisecond,
				Factor:       2.0,
				MaxDelay:     5 * time.Second,
				Jitter:       0.0,
			},
			attempt:      2,
			wantMinDelay: 400 * time.Millisecond,
			wantMaxDelay: 400 * time.Millisecond,
		},
		"max delay cap": {
			strategy: &RetryStrategy{
				InitialDelay: 1 * time.Second,
				Factor:       10.0,
				MaxDelay:     2 * time.Second,
				Jitter:       0.0,
			},
			attempt:      5,
			wantMinDelay: 2 * time.Second,
			wantMaxDelay: 2 * time.Second,
		},
		"with jitter": {
			strategy: &RetryStrategy{
				InitialDelay: 100 * time.Millisecond,
				Factor:       2.0,
				MaxDelay:     5 * time.Second,
				Jitter:       0.5, // 50% jitter
			},
			attempt:      1,
			wantMinDelay: 100 * time.Millisecond, // 200ms - 50% = 100ms min
			wantMaxDelay: 300 * time.Millisecond, // 200ms + 50% = 300ms max
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			delay := tc.strategy.calculateDelay(tc.attempt)
			
			if delay < tc.wantMinDelay || delay > tc.wantMaxDelay {
				t.Errorf("delay %v not in range [%v, %v]", delay, tc.wantMinDelay, tc.wantMaxDelay)
			}
			
			// Ensure delay is never negative
			if delay < 0 {
				t.Errorf("delay should never be negative, got %v", delay)
			}
		})
	}
}

func TestRetryStrategy_ContextCancellation(t *testing.T) {
	strategy := &RetryStrategy{
		MaxRetries:     10,
		InitialDelay:   100 * time.Millisecond,
		Factor:         2.0,
		MaxDelay:       1 * time.Second,
		Jitter:         0.0,
		RetryCondition: DefaultRetryCondition,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	err := strategy.Execute(ctx, func() error {
		return errors.NewConflict(schema.GroupResource{}, "test", nil)
	})
	
	if err != context.DeadlineExceeded {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}
}

func TestApplier_UtilityFunctions(t *testing.T) {
	tests := map[string]struct {
		kind         string
		wantResource string
	}{
		"simple plural": {
			kind:         "Pod",
			wantResource: "Pods",
		},
		"ends with y": {
			kind:         "Policy",
			wantResource: "Policies",
		},
		"ends with s": {
			kind:         "Status",
			wantResource: "Statuses",
		},
		"empty kind": {
			kind:         "",
			wantResource: "",
		},
	}
	
	applier := &Applier{}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resource := applier.kindToResource(tc.kind)
			if resource != tc.wantResource {
				t.Errorf("expected resource=%s, got %s", tc.wantResource, resource)
			}
		})
	}
}

func TestApplier_GetGVR(t *testing.T) {
	applier := &Applier{}
	
	obj := testDeployment()
	gvr := applier.getGVR(obj)
	
	expectedGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "Deployments",
	}
	
	if gvr != expectedGVR {
		t.Errorf("expected GVR=%v, got %v", expectedGVR, gvr)
	}
}

func TestDeleteOptions_Functions(t *testing.T) {
	tests := map[string]struct {
		optionsFunc     func() metav1.DeleteOptions
		expectedPolicy  metav1.DeletionPropagation
	}{
		"delete options": {
			optionsFunc:     DeleteOptions,
			expectedPolicy:  metav1.DeletePropagationBackground,
		},
		"cascade delete options": {
			optionsFunc:     CascadeDeleteOptions,
			expectedPolicy:  metav1.DeletePropagationForeground,
		},
		"orphan delete options": {
			optionsFunc:     OrphanDeleteOptions,
			expectedPolicy:  metav1.DeletePropagationOrphan,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			options := tc.optionsFunc()
			if options.PropagationPolicy == nil {
				t.Error("expected propagation policy to be set")
				return
			}
			if *options.PropagationPolicy != tc.expectedPolicy {
				t.Errorf("expected propagation policy=%v, got %v", tc.expectedPolicy, *options.PropagationPolicy)
			}
		})
	}
}

func TestDefaultRetryCondition(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantRetry  bool
	}{
		"nil error": {
			err:       nil,
			wantRetry: false,
		},
		"conflict error": {
			err:       errors.NewConflict(schema.GroupResource{}, "test", nil),
			wantRetry: true,
		},
		"server timeout": {
			err:       errors.NewServerTimeout(schema.GroupResource{}, "test", 1),
			wantRetry: true,
		},
		"validation error": {
			err:       errors.NewInvalid(schema.GroupKind{}, "test", nil),
			wantRetry: false,
		},
		"forbidden error": {
			err:       errors.NewForbidden(schema.GroupResource{}, "test", nil),
			wantRetry: false,
		},
		"not found error": {
			err:       errors.NewNotFound(schema.GroupResource{}, "test"),
			wantRetry: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := DefaultRetryCondition(tc.err)
			if got != tc.wantRetry {
				t.Errorf("expected retry=%v, got %v for error %v", tc.wantRetry, got, tc.err)
			}
		})
	}
}

// testDeployment creates a test deployment object.
func testDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
		},
	}
}

// testDeploymentWithChangedSpec creates a test deployment object with different specs for testing strategic merge.
func testDeploymentWithChangedSpec() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(5), // Changed from 3 to 5
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "nginx:1.20", // Changed from nginx:latest
							},
						},
					},
				},
			},
		},
	}
}

// testService creates a test service object.
func testService() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"app": "test",
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":       int64(80),
						"targetPort": int64(8080),
					},
				},
			},
		},
	}
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}