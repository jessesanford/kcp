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
			obj:           testDeployment(),
			strategy:      ServerSideApply,
			wantOperation: "apply",
			wantSuccess:   true,
		},
		"strategic merge on existing resource": {
			obj:      testDeployment(),
			strategy: StrategicMerge,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, testDeployment(), nil
				})
			},
			wantOperation: "update",
			wantSuccess:   true,
		},
		"strategic merge creates new resource": {
			obj:      testDeployment(),
			strategy: StrategicMerge,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewNotFound(schema.GroupResource{}, "test-deployment")
				})
			},
			wantOperation: "create",
			wantSuccess:   true,
		},
		"replace existing resource": {
			obj:      testDeployment(),
			strategy: Replace,
			setupReactor: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					existing := testDeployment()
					existing.SetResourceVersion("123")
					return true, existing, nil
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
				"replicas": 3,
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