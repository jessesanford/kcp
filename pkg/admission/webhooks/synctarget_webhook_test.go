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

package webhooks

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestSyncTargetWebhook_Admit(t *testing.T) {
	tests := []struct {
		name        string
		operation   admission.Operation
		object      *unstructured.Unstructured
		wantLabels  map[string]string
		wantError   bool
	}{
		{
			name:      "create synctarget adds default labels",
			operation: admission.Create,
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
					},
					"spec": map[string]interface{}{
						"apiServerURL": "https://kubernetes.example.com:6443",
					},
				},
			},
			wantLabels: map[string]string{
				TMCManagedLabel:    "true",
				TMCWorkspaceLabel:  "root:test",
				TMCSchedulingLabel: "enabled",
			},
			wantError: false,
		},
		{
			name:      "create synctarget with incomplete URL adds https",
			operation: admission.Create,
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
					},
					"spec": map[string]interface{}{
						"apiServerURL": "kubernetes.example.com:6443",
					},
				},
			},
			wantError: false,
		},
		{
			name:      "update synctarget preserves existing config",
			operation: admission.Update,
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
						"labels": map[string]interface{}{
							TMCManagedLabel: "true",
						},
					},
					"spec": map[string]interface{}{
						"apiServerURL": "https://kubernetes.example.com:6443",
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhookInterface, err := NewSyncTargetWebhook(nil)
			if err != nil {
				t.Fatalf("NewSyncTargetWebhook() error = %v", err)
			}

			webhook, ok := webhookInterface.(admission.MutationInterface)
			if !ok {
				t.Fatalf("webhook does not implement MutationInterface")
			}

			ctx := context.Background()
			ctx = genericapirequest.WithCluster(ctx, genericapirequest.Cluster{Name: logicalcluster.Name("root:test")})

			attrs := &testAdmissionAttributes{
				operation: tt.operation,
				object:    tt.object,
				resource:  schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "synctargets"},
				userInfo:  &user.DefaultInfo{Name: "test-user"},
			}

			err = webhook.Admit(ctx, attrs, nil)
			if (err != nil) != tt.wantError {
				t.Errorf("SyncTargetWebhook.Admit() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantLabels != nil {
				labels := tt.object.GetLabels()
				for key, expectedValue := range tt.wantLabels {
					if actualValue, found := labels[key]; !found || actualValue != expectedValue {
						t.Errorf("Expected label %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}
			}

			// Check if HTTPS scheme was added when needed
			if tt.operation == admission.Create {
				spec, found, err := unstructured.NestedMap(tt.object.Object, "spec")
				if err != nil || !found {
					t.Errorf("Failed to get spec: %v", err)
					return
				}
				if apiServerURL, found, err := unstructured.NestedString(spec, "apiServerURL"); found && err == nil {
					if apiServerURL == "kubernetes.example.com:6443" {
						// Should have been changed to https://
						t.Errorf("Expected URL to be prefixed with https://, got: %s", apiServerURL)
					}
				}
			}
		})
	}
}

func TestSyncTargetWebhook_Validate(t *testing.T) {
	tests := []struct {
		name      string
		object    *unstructured.Unstructured
		wantError bool
	}{
		{
			name: "valid synctarget passes validation",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "valid-cluster-name",
						"labels": map[string]interface{}{
							TMCManagedLabel:    "true",
							TMCSchedulingLabel: "enabled",
						},
						"annotations": map[string]interface{}{
							TMCCapacityAnnotation: `{"cpu":"1000m","memory":"1Gi","pods":"100"}`,
						},
					},
					"spec": map[string]interface{}{
						"apiServerURL": "https://kubernetes.example.com:6443",
					},
				},
			},
			wantError: false,
		},
		{
			name: "missing apiServerURL fails validation",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
					},
					"spec": map[string]interface{}{},
				},
			},
			wantError: true,
		},
		{
			name: "invalid URL scheme fails validation",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
					},
					"spec": map[string]interface{}{
						"apiServerURL": "http://kubernetes.example.com:6443",
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid cluster name fails validation",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "ab", // too short
					},
					"spec": map[string]interface{}{
						"apiServerURL": "https://kubernetes.example.com:6443",
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid capacity annotation fails validation",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "SyncTarget",
					"metadata": map[string]interface{}{
						"name": "test-cluster",
						"annotations": map[string]interface{}{
							TMCCapacityAnnotation: "invalid-json",
						},
					},
					"spec": map[string]interface{}{
						"apiServerURL": "https://kubernetes.example.com:6443",
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhookInterface, err := NewSyncTargetWebhook(nil)
			if err != nil {
				t.Fatalf("NewSyncTargetWebhook() error = %v", err)
			}

			webhook, ok := webhookInterface.(admission.ValidationInterface)
			if !ok {
				t.Fatalf("webhook does not implement ValidationInterface")
			}

			ctx := context.Background()
			ctx = genericapirequest.WithCluster(ctx, genericapirequest.Cluster{Name: logicalcluster.Name("root:test")})

			attrs := &testAdmissionAttributes{
				operation: admission.Create,
				object:    tt.object,
				resource:  schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "synctargets"},
				userInfo:  &user.DefaultInfo{Name: "test-user"},
			}

			err = webhook.Validate(ctx, attrs, nil)
			if (err != nil) != tt.wantError {
				t.Errorf("SyncTargetWebhook.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// testAdmissionAttributes implements admission.Attributes for testing
type testAdmissionAttributes struct {
	operation admission.Operation
	object    runtime.Object
	oldObject runtime.Object
	resource  schema.GroupVersionResource
	userInfo  user.Info
}

func (a *testAdmissionAttributes) GetOperation() admission.Operation { return a.operation }
func (a *testAdmissionAttributes) GetObject() runtime.Object         { return a.object }
func (a *testAdmissionAttributes) GetOldObject() runtime.Object      { return a.oldObject }
func (a *testAdmissionAttributes) GetName() string                   { return "" }
func (a *testAdmissionAttributes) GetNamespace() string              { return "" }
func (a *testAdmissionAttributes) GetResource() schema.GroupVersionResource { return a.resource }
func (a *testAdmissionAttributes) GetSubresource() string            { return "" }
func (a *testAdmissionAttributes) GetUserInfo() user.Info            { return a.userInfo }
func (a *testAdmissionAttributes) GetKind() schema.GroupVersionKind  { return schema.GroupVersionKind{} }
func (a *testAdmissionAttributes) IsDryRun() bool                    { return false }
func (a *testAdmissionAttributes) GetAnnotations(key string) []string { return nil }
func (a *testAdmissionAttributes) AddAnnotation(key, value string) error { return nil }
func (a *testAdmissionAttributes) AddAnnotationWithLevel(key, value string, level audit.Level) error { return nil }
func (a *testAdmissionAttributes) GetReinvocationContext() admission.ReinvocationContext { return nil }
func (a *testAdmissionAttributes) GetCluster() logicalcluster.Name { return logicalcluster.Name("root:test") }
func (a *testAdmissionAttributes) GetOperationOptions() runtime.Object { return nil }