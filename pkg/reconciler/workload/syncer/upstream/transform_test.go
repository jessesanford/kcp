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

package upstream

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

func TestNewResourceTransformer(t *testing.T) {
	workspace := logicalcluster.NewPath("root:test-workspace")
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
			UID:  "12345-abcde-67890",
		},
	}

	transformer := NewResourceTransformer(workspace, syncTarget)

	if transformer == nil {
		t.Fatal("transformer should not be nil")
	}

	if transformer.workspace != workspace {
		t.Errorf("expected workspace %v, got %v", workspace, transformer.workspace)
	}

	if transformer.syncTargetName != "test-sync-target" {
		t.Errorf("expected sync target name %v, got %v", "test-sync-target", transformer.syncTargetName)
	}

	if transformer.syncTargetUID != "12345-abcde-67890" {
		t.Errorf("expected sync target UID %v, got %v", "12345-abcde-67890", transformer.syncTargetUID)
	}

	if transformer.namespaceMapper == nil {
		t.Error("namespace mapper should not be nil")
	}
}

func TestResourceTransformer_TransformFromPhysical(t *testing.T) {
	workspace := logicalcluster.NewPath("root:test-workspace")
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
			UID:  types.UID("12345-abcde-67890"),
		},
	}

	transformer := NewResourceTransformer(workspace, syncTarget)

	tests := map[string]struct {
		input         *unstructured.Unstructured
		expectedAnno  map[string]string
		expectedNS    string
		expectError   bool
	}{
		"pod with namespace": {
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":            "test-pod",
						"namespace":       "test-ns",
						"resourceVersion": "12345",
						"uid":             "pod-uid-123",
						"generation":      int64(5),
					},
					"spec": map[string]interface{}{
						"nodeName":           "node-1",
						"serviceAccountName": "default",
					},
					"status": map[string]interface{}{
						"hostIP": "10.0.0.1",
						"podIP":  "10.244.1.1",
					},
				},
			},
			expectedAnno: map[string]string{
				workloadv1alpha1.InternalSyncTargetUIDAnnotation:  "12345-abcde-67890",
				workloadv1alpha1.InternalSyncTargetNameAnnotation: "test-sync-target",
				workloadv1alpha1.ClusterAnnotation:                "root:test-workspace",
				"kcp.io/upstream-sync-timestamp":                  "1640995200",
				"kcp.io/original-generation":                      "5",
			},
			expectedNS:  "kcp-test-sync-target-test-ns",
			expectError: false,
		},
		"cluster-scoped node": {
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Node",
					"metadata": map[string]interface{}{
						"name": "test-node",
					},
					"status": map[string]interface{}{
						"nodeInfo": map[string]interface{}{
							"machineID":  "machine-123",
							"systemUUID": "system-456",
							"bootID":     "boot-789",
						},
					},
				},
			},
			expectedAnno: map[string]string{
				workloadv1alpha1.InternalSyncTargetUIDAnnotation:  "12345-abcde-67890",
				workloadv1alpha1.InternalSyncTargetNameAnnotation: "test-sync-target",
				workloadv1alpha1.ClusterAnnotation:                "root:test-workspace",
				"kcp.io/upstream-sync-timestamp":                  "1640995200",
			},
			expectedNS:  "",
			expectError: false,
		},
		"nil object": {
			input:       nil,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := transformer.TransformFromPhysical(tc.input)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result should not be nil")
			}

			// Check namespace transformation
			if result.GetNamespace() != tc.expectedNS {
				t.Errorf("expected namespace %v, got %v", tc.expectedNS, result.GetNamespace())
			}

			// Check annotations
			annotations := result.GetAnnotations()
			for key, expectedValue := range tc.expectedAnno {
				if annotations[key] != expectedValue {
					t.Errorf("expected annotation %s=%s, got %s", key, expectedValue, annotations[key])
				}
			}

			// Verify physical fields were cleaned
			if tc.input.GetKind() == "Pod" {
				// Node name should be removed
				if nodeName, found, _ := unstructured.NestedString(result.Object, "spec", "nodeName"); found {
					t.Errorf("nodeName should be removed, found: %s", nodeName)
				}
				
				// Host IP should be removed
				if hostIP, found, _ := unstructured.NestedString(result.Object, "status", "hostIP"); found {
					t.Errorf("hostIP should be removed, found: %s", hostIP)
				}
			}

			if tc.input.GetKind() == "Node" {
				// Machine ID should be removed
				if machineID, found, _ := unstructured.NestedString(result.Object, "status", "nodeInfo", "machineID"); found {
					t.Errorf("machineID should be removed, found: %s", machineID)
				}
			}

			// Verify metadata fields were cleaned
			if uid, found, _ := unstructured.NestedString(result.Object, "metadata", "uid"); found {
				t.Errorf("uid should be removed, found: %s", uid)
			}

			if rv, found, _ := unstructured.NestedString(result.Object, "metadata", "resourceVersion"); found {
				t.Errorf("resourceVersion should be removed, found: %s", rv)
			}
		})
	}
}

func TestResourceTransformer_TransformToPhysical(t *testing.T) {
	workspace := logicalcluster.NewPath("root:test-workspace")
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
			UID:  types.UID("12345-abcde-67890"),
		},
	}

	transformer := NewResourceTransformer(workspace, syncTarget)

	tests := map[string]struct {
		input       *unstructured.Unstructured
		expectedNS  string
		expectError bool
	}{
		"kcp pod with logical namespace": {
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "kcp-test-sync-target-test-ns",
						"annotations": map[string]interface{}{
							workloadv1alpha1.InternalSyncTargetUIDAnnotation:  "12345-abcde-67890",
							workloadv1alpha1.InternalSyncTargetNameAnnotation: "test-sync-target",
							workloadv1alpha1.ClusterAnnotation:                "root:test-workspace",
							"kcp.io/upstream-sync-timestamp":                  "1640995200",
							"user-annotation":                                 "keep-me",
						},
						"labels": map[string]interface{}{
							"kcp.io/internal-label": "remove-me",
							"user-label":            "keep-me",
						},
					},
				},
			},
			expectedNS:  "test-ns",
			expectError: false,
		},
		"cluster-scoped resource": {
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Node",
					"metadata": map[string]interface{}{
						"name": "test-node",
						"annotations": map[string]interface{}{
							workloadv1alpha1.InternalSyncTargetUIDAnnotation:  "12345-abcde-67890",
							workloadv1alpha1.InternalSyncTargetNameAnnotation: "test-sync-target",
							"kcp.io/last-synced-generation":                   "5",
						},
					},
				},
			},
			expectedNS:  "",
			expectError: false,
		},
		"nil object": {
			input:       nil,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := transformer.TransformToPhysical(tc.input)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result should not be nil")
			}

			// Check namespace transformation
			if result.GetNamespace() != tc.expectedNS {
				t.Errorf("expected namespace %v, got %v", tc.expectedNS, result.GetNamespace())
			}

			// Verify KCP annotations were removed
			annotations := result.GetAnnotations()
			kcpAnnotations := []string{
				workloadv1alpha1.InternalSyncTargetUIDAnnotation,
				workloadv1alpha1.InternalSyncTargetNameAnnotation,
				workloadv1alpha1.ClusterAnnotation,
				"kcp.io/upstream-sync-timestamp",
				"kcp.io/last-synced-generation",
			}

			for _, annoKey := range kcpAnnotations {
				if _, found := annotations[annoKey]; found {
					t.Errorf("KCP annotation %s should be removed", annoKey)
				}
			}

			// Verify user annotations are preserved
			if tc.input != nil {
				inputAnno := tc.input.GetAnnotations()
				if userAnno, found := inputAnno["user-annotation"]; found {
					if annotations["user-annotation"] != userAnno {
						t.Error("user annotation should be preserved")
					}
				}
			}

			// Verify KCP labels were removed
			labels := result.GetLabels()
			if labels != nil {
				for key := range labels {
					if strings.HasPrefix(key, "kcp.io/") {
						t.Errorf("KCP label %s should be removed", key)
					}
				}
			}
		})
	}
}

func TestResourceTransformer_ShouldTransformResource(t *testing.T) {
	workspace := logicalcluster.NewPath("root:test-workspace")
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
			UID:  types.UID("12345-abcde-67890"),
		},
	}

	transformer := NewResourceTransformer(workspace, syncTarget)

	tests := map[string]struct {
		gvr             schema.GroupVersionResource
		obj             *unstructured.Unstructured
		expectedResult  bool
	}{
		"regular pod": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-pod",
					},
				},
			},
			expectedResult: true,
		},
		"kcp internal resource": {
			gvr: schema.GroupVersionResource{Group: "tenancy.kcp.io", Version: "v1alpha1", Resource: "workspaces"},
			obj: &unstructured.Unstructured{},
			expectedResult: false,
		},
		"events": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"},
			obj: &unstructured.Unstructured{},
			expectedResult: false,
		},
		"secrets": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-secret",
					},
				},
			},
			expectedResult: false,
		},
		"deployment": {
			gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			obj: &unstructured.Unstructured{},
			expectedResult: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := transformer.ShouldTransformResource(tc.gvr, tc.obj)
			if result != tc.expectedResult {
				t.Errorf("expected %v, got %v", tc.expectedResult, result)
			}
		})
	}
}

func TestResourceTransformer_BidirectionalTransform(t *testing.T) {
	workspace := logicalcluster.NewPath("root:test-workspace")
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
			UID:  types.UID("12345-abcde-67890"),
		},
	}

	transformer := NewResourceTransformer(workspace, syncTarget)

	originalPod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-namespace",
				"annotations": map[string]interface{}{
					"user-annotation": "should-be-preserved",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test-container",
						"image": "test-image:latest",
					},
				},
			},
		},
	}

	// Transform from physical to KCP format
	kcpPod, err := transformer.TransformFromPhysical(originalPod)
	if err != nil {
		t.Fatalf("failed to transform from physical: %v", err)
	}

	// Verify KCP transformation
	if kcpPod.GetNamespace() != "kcp-test-sync-target-test-namespace" {
		t.Errorf("expected KCP namespace, got %s", kcpPod.GetNamespace())
	}

	kcpAnnotations := kcpPod.GetAnnotations()
	if kcpAnnotations[workloadv1alpha1.InternalSyncTargetNameAnnotation] != "test-sync-target" {
		t.Error("expected sync target annotation in KCP format")
	}

	if kcpAnnotations["user-annotation"] != "should-be-preserved" {
		t.Error("user annotation should be preserved")
	}

	// Transform back to physical format
	physicalPod, err := transformer.TransformToPhysical(kcpPod)
	if err != nil {
		t.Fatalf("failed to transform to physical: %v", err)
	}

	// Verify physical transformation
	if physicalPod.GetNamespace() != "test-namespace" {
		t.Errorf("expected original namespace, got %s", physicalPod.GetNamespace())
	}

	physicalAnnotations := physicalPod.GetAnnotations()
	if physicalAnnotations["user-annotation"] != "should-be-preserved" {
		t.Error("user annotation should be preserved after round trip")
	}

	// KCP annotations should be removed
	if _, found := physicalAnnotations[workloadv1alpha1.InternalSyncTargetNameAnnotation]; found {
		t.Error("KCP annotations should be removed in physical format")
	}

	// Spec should be preserved
	originalContainers, _, _ := unstructured.NestedSlice(originalPod.Object, "spec", "containers")
	physicalContainers, _, _ := unstructured.NestedSlice(physicalPod.Object, "spec", "containers")

	if len(originalContainers) != len(physicalContainers) {
		t.Error("container spec should be preserved")
	}
}

func TestTransformationResult(t *testing.T) {
	obj := &unstructured.Unstructured{}

	t.Run("successful result", func(t *testing.T) {
		result := NewTransformationResult(obj)
		if result.Transformed != obj {
			t.Error("transformed object should match")
		}
		if result.Skipped {
			t.Error("should not be skipped")
		}
		if result.Error != nil {
			t.Error("should not have error")
		}
	})

	t.Run("skipped result", func(t *testing.T) {
		result := NewSkippedResult("test reason")
		if !result.Skipped {
			t.Error("should be skipped")
		}
		if result.Reason != "test reason" {
			t.Error("reason should match")
		}
	})

	t.Run("error result", func(t *testing.T) {
		testErr := fmt.Errorf("test error")
		result := NewErrorResult(testErr)
		if result.Error != testErr {
			t.Error("error should match")
		}
	})
}