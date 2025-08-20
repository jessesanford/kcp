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

package downstream

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestPreserveDownstreamFields(t *testing.T) {
	tests := map[string]struct {
		existing *unstructured.Unstructured
		desired  *unstructured.Unstructured
		want     *unstructured.Unstructured
	}{
		"nil inputs": {
			existing: nil,
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
		},
		"preserve metadata and status": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":            "test",
						"resourceVersion": "12345",
						"uid":             "test-uid",
						"creationTimestamp": "2023-01-01T00:00:00Z",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"labels": map[string]interface{}{
							"app": "test",
						},
						"resourceVersion": "12345",
						"uid":             "test-uid",
						"creationTimestamp": "2023-01-01T00:00:00Z",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := PreserveDownstreamFields(tc.existing, tc.desired)
			if !reflect.DeepEqual(result.Object, tc.want.Object) {
				t.Errorf("PreserveDownstreamFields() = %v, want %v", result.Object, tc.want.Object)
			}
		})
	}
}

func TestPreserveFinalizers(t *testing.T) {
	tests := map[string]struct {
		existing *unstructured.Unstructured
		desired  *unstructured.Unstructured
		want     []string
	}{
		"merge unique finalizers": {
			existing: createObjWithFinalizers([]string{"finalizer1", "finalizer2"}),
			desired:  createObjWithFinalizers([]string{"finalizer2", "finalizer3"}),
			want:     []string{"finalizer1", "finalizer2", "finalizer3"},
		},
		"existing empty finalizers": {
			existing: createObjWithFinalizers([]string{}),
			desired:  createObjWithFinalizers([]string{"finalizer1"}),
			want:     []string{"finalizer1"},
		},
		"desired empty finalizers": {
			existing: createObjWithFinalizers([]string{"finalizer1"}),
			desired:  createObjWithFinalizers([]string{}),
			want:     []string{"finalizer1"},
		},
		"duplicate finalizers": {
			existing: createObjWithFinalizers([]string{"finalizer1", "finalizer1"}),
			desired:  createObjWithFinalizers([]string{"finalizer1", "finalizer2"}),
			want:     []string{"finalizer1", "finalizer2"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			merged := tc.desired.DeepCopy()
			preserveFinalizers(tc.existing, merged)
			
			result := merged.GetFinalizers()
			if !containsAllStrings(result, tc.want) || len(result) != len(tc.want) {
				t.Errorf("preserveFinalizers() finalizers = %v, want %v", result, tc.want)
			}
		})
	}
}

func TestPreserveOwnerReferences(t *testing.T) {
	ownerRef1 := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "owner1",
		UID:        "uid1",
	}
	ownerRef2 := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       "owner2",
		UID:        "uid2",
	}

	tests := map[string]struct {
		existing *unstructured.Unstructured
		desired  *unstructured.Unstructured
		wantLen  int
	}{
		"merge owner references": {
			existing: createObjWithOwnerRefs([]metav1.OwnerReference{ownerRef1}),
			desired:  createObjWithOwnerRefs([]metav1.OwnerReference{ownerRef2}),
			wantLen:  2,
		},
		"override same owner reference": {
			existing: createObjWithOwnerRefs([]metav1.OwnerReference{ownerRef1}),
			desired:  createObjWithOwnerRefs([]metav1.OwnerReference{ownerRef1}),
			wantLen:  1,
		},
		"existing empty owner references": {
			existing: createObjWithOwnerRefs([]metav1.OwnerReference{}),
			desired:  createObjWithOwnerRefs([]metav1.OwnerReference{ownerRef1}),
			wantLen:  1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			merged := tc.desired.DeepCopy()
			preserveOwnerReferences(tc.existing, merged)
			
			result := merged.GetOwnerReferences()
			if len(result) != tc.wantLen {
				t.Errorf("preserveOwnerReferences() length = %d, want %d", len(result), tc.wantLen)
			}
		})
	}
}

func TestPreserveServiceFields(t *testing.T) {
	tests := map[string]struct {
		existing *unstructured.Unstructured
		desired  *unstructured.Unstructured
		wantIP   string
	}{
		"preserve cluster IP": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"clusterIP": "10.0.0.1",
						"ports": []interface{}{
							map[string]interface{}{"port": int64(80)},
						},
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"spec": map[string]interface{}{
						"ports": []interface{}{
							map[string]interface{}{"port": int64(8080)},
						},
					},
				},
			},
			wantIP: "10.0.0.1",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set GVK for type detection
			tc.existing.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Service",
			})
			
			merged := tc.desired.DeepCopy()
			preserveServiceFields(tc.existing, merged)
			
			clusterIP, found, err := unstructured.NestedString(merged.Object, "spec", "clusterIP")
			if err != nil {
				t.Errorf("Error getting clusterIP: %v", err)
			}
			if !found {
				t.Error("clusterIP not found after preservation")
			}
			if clusterIP != tc.wantIP {
				t.Errorf("clusterIP = %s, want %s", clusterIP, tc.wantIP)
			}
		})
	}
}

func TestPreservePVFields(t *testing.T) {
	tests := map[string]struct {
		existing     *unstructured.Unstructured
		desired      *unstructured.Unstructured
		wantClaimRef bool
	}{
		"preserve claim reference": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolume",
					"spec": map[string]interface{}{
						"claimRef": map[string]interface{}{
							"name":      "test-pvc",
							"namespace": "default",
						},
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "PersistentVolume",
					"spec": map[string]interface{}{
						"capacity": map[string]interface{}{
							"storage": "1Gi",
						},
					},
				},
			},
			wantClaimRef: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set GVK for type detection
			tc.existing.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PersistentVolume",
			})
			
			merged := tc.desired.DeepCopy()
			preservePVFields(tc.existing, merged)
			
			_, found, err := unstructured.NestedFieldNoCopy(merged.Object, "spec", "claimRef")
			if err != nil {
				t.Errorf("Error getting claimRef: %v", err)
			}
			if found != tc.wantClaimRef {
				t.Errorf("claimRef found = %v, want %v", found, tc.wantClaimRef)
			}
		})
	}
}

func TestPreserveServerManagedMetadata(t *testing.T) {
	now := metav1.Now()
	managedFields := []metav1.ManagedFieldsEntry{
		{
			Manager:   "test-manager",
			Operation: metav1.ManagedFieldsOperationApply,
		},
	}

	tests := map[string]struct {
		existing *unstructured.Unstructured
		desired  *unstructured.Unstructured
		checks   map[string]interface{}
	}{
		"preserve all server managed metadata": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test",
						"resourceVersion":   "12345",
						"uid":               "test-uid-123",
						"creationTimestamp": now.Time.Format(time.RFC3339),
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
						"labels": map[string]interface{}{
							"app": "updated",
						},
					},
				},
			},
			checks: map[string]interface{}{
				"resourceVersion": "12345",
				"uid":             "test-uid-123",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up existing object with managed fields
			tc.existing.SetManagedFields(managedFields)
			tc.existing.SetUID(types.UID("test-uid-123"))
			tc.existing.SetResourceVersion("12345")
			tc.existing.SetCreationTimestamp(now)

			merged := tc.desired.DeepCopy()
			preserveServerManagedMetadata(tc.existing, merged)
			
			// Verify preserved fields
			if merged.GetResourceVersion() != tc.checks["resourceVersion"].(string) {
				t.Errorf("resourceVersion = %s, want %s", merged.GetResourceVersion(), tc.checks["resourceVersion"].(string))
			}
			
			if string(merged.GetUID()) != tc.checks["uid"].(string) {
				t.Errorf("uid = %s, want %s", merged.GetUID(), tc.checks["uid"].(string))
			}
			
			if len(merged.GetManagedFields()) != len(managedFields) {
				t.Errorf("managedFields length = %d, want %d", len(merged.GetManagedFields()), len(managedFields))
			}
		})
	}
}

func TestPreserveStatus(t *testing.T) {
	tests := map[string]struct {
		existing    *unstructured.Unstructured
		desired     *unstructured.Unstructured
		wantStatus  bool
		statusValue interface{}
	}{
		"preserve existing status": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase":             "Running",
						"replicas":          int64(3),
						"availableReplicas": int64(2),
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
			wantStatus:  true,
			statusValue: map[string]interface{}{
				"phase":             "Running",
				"replicas":          int64(3),
				"availableReplicas": int64(2),
			},
		},
		"no status in existing": {
			existing: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
			wantStatus: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			merged := tc.desired.DeepCopy()
			preserveStatus(tc.existing, merged)
			
			status, found, err := unstructured.NestedFieldNoCopy(merged.Object, "status")
			if err != nil {
				t.Errorf("Error getting status: %v", err)
			}
			
			if found != tc.wantStatus {
				t.Errorf("status found = %v, want %v", found, tc.wantStatus)
			}
			
			if tc.wantStatus && !reflect.DeepEqual(status, tc.statusValue) {
				t.Errorf("status = %v, want %v", status, tc.statusValue)
			}
		})
	}
}

// Helper functions
func createObjWithFinalizers(finalizers []string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}
	obj.SetFinalizers(finalizers)
	return obj
}

func createObjWithOwnerRefs(ownerRefs []metav1.OwnerReference) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}
	obj.SetOwnerReferences(ownerRefs)
	return obj
}

func containsAllStrings(slice []string, expected []string) bool {
	sliceMap := make(map[string]bool)
	for _, item := range slice {
		sliceMap[item] = true
	}
	
	for _, expectedItem := range expected {
		if !sliceMap[expectedItem] {
			return false
		}
	}
	
	return true
}