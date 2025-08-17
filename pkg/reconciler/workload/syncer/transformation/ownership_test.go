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

package transformation

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestOwnerReferenceTransformer_Name(t *testing.T) {
	transformer := NewOwnerReferenceTransformer()
	
	expectedName := "ownerreference-transformer"
	if transformer.Name() != expectedName {
		t.Errorf("Expected transformer name %q, got %q", expectedName, transformer.Name())
	}
}

func TestOwnerReferenceTransformer_ShouldTransform(t *testing.T) {
	transformer := NewOwnerReferenceTransformer()
	
	tests := map[string]struct {
		obj      runtime.Object
		expected bool
	}{
		"nil object": {
			obj:      nil,
			expected: false,
		},
		"pod without owner references": {
			obj:      &corev1.Pod{},
			expected: false,
		},
		"pod with owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							UID:        "12345",
						},
					},
				},
			},
			expected: true,
		},
		"non-metadata object": {
			obj:      &runtime.Unknown{},
			expected: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := transformer.ShouldTransform(tc.obj)
			if result != tc.expected {
				t.Errorf("Expected ShouldTransform to return %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestOwnerReferenceTransformer_TransformForDownstream(t *testing.T) {
	transformer := NewOwnerReferenceTransformer()
	ctx := context.Background()
	
	target := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
		},
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	
	tests := map[string]struct {
		obj                      runtime.Object
		expectedError            bool
		validateOwnerReferences  func(t *testing.T, obj metav1.Object)
		validateAnnotations      func(t *testing.T, obj metav1.Object)
	}{
		"nil object": {
			obj:           nil,
			expectedError: true,
		},
		"non-metadata object": {
			obj:           &runtime.Unknown{},
			expectedError: false,
		},
		"pod without owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
			},
			expectedError: false,
		},
		"pod with deployment owner": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							UID:        types.UID("deployment-uid-12345"),
							Controller: boolPtr(true),
						},
					},
				},
			},
			expectedError: false,
			validateOwnerReferences: func(t *testing.T, obj metav1.Object) {
				ownerRefs := obj.GetOwnerReferences()
				if len(ownerRefs) != 1 {
					t.Errorf("Expected 1 owner reference, got %d", len(ownerRefs))
					return
				}
				
				ref := ownerRefs[0]
				if ref.Kind != "Deployment" {
					t.Errorf("Expected owner reference kind Deployment, got %q", ref.Kind)
				}
				if ref.Name != "test-deployment" {
					t.Errorf("Expected owner reference name test-deployment, got %q", ref.Name)
				}
				if ref.UID != "" {
					t.Errorf("Expected transformed owner reference to have empty UID, got %q", ref.UID)
				}
				if ref.Controller == nil || !*ref.Controller {
					t.Errorf("Expected controller flag to be preserved")
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				if annotations == nil {
					t.Errorf("Expected annotations to be created")
					return
				}
				
				if count := annotations["syncer.kcp.io/original-owner-count"]; count != "1" {
					t.Errorf("Expected original owner count annotation to be 1, got %q", count)
				}
			},
		},
		"pod with multiple owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							UID:        types.UID("deployment-uid-12345"),
						},
						{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       "test-service",
							UID:        types.UID("service-uid-67890"),
						},
						{
							APIVersion: "v1",
							Kind:       "PersistentVolume",
							Name:       "test-pv",
							UID:        types.UID("pv-uid-abcde"),
						},
					},
				},
			},
			expectedError: false,
			validateOwnerReferences: func(t *testing.T, obj metav1.Object) {
				ownerRefs := obj.GetOwnerReferences()
				// PersistentVolume should be filtered out (preserveOwnershipTypes[PersistentVolume] = false)
				if len(ownerRefs) != 2 {
					t.Errorf("Expected 2 owner references after filtering PV, got %d", len(ownerRefs))
					return
				}
				
				// Check that all UIDs are cleared
				for i, ref := range ownerRefs {
					if ref.UID != "" {
						t.Errorf("Expected transformed owner reference %d to have empty UID, got %q", i, ref.UID)
					}
				}
				
				// Check that Deployment and Service are preserved
				foundDeployment := false
				foundService := false
				for _, ref := range ownerRefs {
					switch ref.Kind {
					case "Deployment":
						foundDeployment = true
					case "Service":
						foundService = true
					}
				}
				
				if !foundDeployment {
					t.Errorf("Expected Deployment owner reference to be preserved")
				}
				if !foundService {
					t.Errorf("Expected Service owner reference to be preserved")
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				if annotations == nil {
					t.Errorf("Expected annotations to be created")
					return
				}
				
				if count := annotations["syncer.kcp.io/original-owner-count"]; count != "3" {
					t.Errorf("Expected original owner count annotation to be 3, got %q", count)
				}
			},
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := transformer.TransformForDownstream(ctx, tc.obj, target)
			
			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}
			
			// Ensure original object is not modified
			if tc.obj != nil && result == tc.obj {
				t.Errorf("Expected deep copy, got same object")
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateOwnerReferences != nil {
				tc.validateOwnerReferences(t, metaResult)
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateAnnotations != nil {
				tc.validateAnnotations(t, metaResult)
			}
		})
	}
}

func TestOwnerReferenceTransformer_TransformForUpstream(t *testing.T) {
	transformer := NewOwnerReferenceTransformer()
	ctx := context.Background()
	
	source := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
		},
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	
	tests := map[string]struct {
		obj                     runtime.Object
		expectedError           bool
		validateOwnerReferences func(t *testing.T, obj metav1.Object)
		validateAnnotations     func(t *testing.T, obj metav1.Object)
	}{
		"nil object": {
			obj:           nil,
			expectedError: true,
		},
		"pod without owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
			},
			expectedError: false,
		},
		"pod with preserved owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							UID:        types.UID("deployment-uid-12345"),
						},
						{
							APIVersion: "v1",
							Kind:       "Service",
							Name:       "test-service",
							UID:        types.UID("service-uid-67890"),
						},
					},
					Annotations: map[string]string{
						"syncer.kcp.io/original-owner-count": "2",
						"app.kubernetes.io/version":          "1.0",
					},
				},
			},
			expectedError: false,
			validateOwnerReferences: func(t *testing.T, obj metav1.Object) {
				ownerRefs := obj.GetOwnerReferences()
				if len(ownerRefs) != 2 {
					t.Errorf("Expected 2 owner references, got %d", len(ownerRefs))
					return
				}
				
				// Both Deployment and Service should be preserved for upstream
				foundDeployment := false
				foundService := false
				for _, ref := range ownerRefs {
					switch ref.Kind {
					case "Deployment":
						foundDeployment = true
					case "Service":
						foundService = true
					}
				}
				
				if !foundDeployment {
					t.Errorf("Expected Deployment owner reference to be preserved")
				}
				if !foundService {
					t.Errorf("Expected Service owner reference to be preserved")
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				if annotations == nil {
					t.Errorf("Expected annotations to exist")
					return
				}
				
				// Transformation annotation should be removed
				if _, exists := annotations["syncer.kcp.io/original-owner-count"]; exists {
					t.Errorf("Expected original owner count annotation to be removed")
				}
				
				// App annotation should be preserved
				if version := annotations["app.kubernetes.io/version"]; version != "1.0" {
					t.Errorf("Expected app version annotation to be preserved, got %q", version)
				}
			},
		},
		"pod with filtered owner references": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							UID:        types.UID("deployment-uid-12345"),
						},
						{
							APIVersion: "custom.io/v1",
							Kind:       "CustomResource",
							Name:       "test-custom",
							UID:        types.UID("custom-uid-67890"),
						},
					},
				},
			},
			expectedError: false,
			validateOwnerReferences: func(t *testing.T, obj metav1.Object) {
				ownerRefs := obj.GetOwnerReferences()
				if len(ownerRefs) != 1 {
					t.Errorf("Expected 1 owner reference after filtering, got %d", len(ownerRefs))
					return
				}
				
				// Only Deployment should be preserved (CustomResource not in preserve list)
				if ownerRefs[0].Kind != "Deployment" {
					t.Errorf("Expected only Deployment to be preserved, got %q", ownerRefs[0].Kind)
				}
			},
		},
		"pod with only syncer annotation": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"syncer.kcp.io/original-owner-count": "1",
					},
				},
			},
			expectedError: false,
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				if annotations != nil {
					t.Errorf("Expected nil annotations map after cleanup, got %v", annotations)
				}
			},
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := transformer.TransformForUpstream(ctx, tc.obj, source)
			
			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}
			
			// Ensure original object is not modified
			if tc.obj != nil && result == tc.obj {
				t.Errorf("Expected deep copy, got same object")
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateOwnerReferences != nil {
				tc.validateOwnerReferences(t, metaResult)
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateAnnotations != nil {
				tc.validateAnnotations(t, metaResult)
			}
		})
	}
}

func TestOwnerReferenceTransformer_shouldPreserveForUpstream(t *testing.T) {
	transformer := &ownerReferenceTransformer{}
	
	tests := map[string]struct {
		ref      metav1.OwnerReference
		expected bool
	}{
		"deployment": {
			ref: metav1.OwnerReference{
				Kind: "Deployment",
			},
			expected: true,
		},
		"service": {
			ref: metav1.OwnerReference{
				Kind: "Service",
			},
			expected: true,
		},
		"custom resource": {
			ref: metav1.OwnerReference{
				Kind: "CustomResource",
			},
			expected: false,
		},
		"node": {
			ref: metav1.OwnerReference{
				Kind: "Node",
			},
			expected: false,
		},
		"statefulset": {
			ref: metav1.OwnerReference{
				Kind: "StatefulSet",
			},
			expected: true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := transformer.shouldPreserveForUpstream(tc.ref)
			if result != tc.expected {
				t.Errorf("Expected shouldPreserveForUpstream(%q) to return %v, got %v", tc.ref.Kind, tc.expected, result)
			}
		})
	}
}

func TestOwnerReferenceTransformer_generateCrossClusterUID(t *testing.T) {
	transformer := &ownerReferenceTransformer{}
	
	originalUID := types.UID("original-uid-12345")
	clusterName := "test-cluster"
	
	result := transformer.generateCrossClusterUID(originalUID, clusterName)
	
	expectedUID := types.UID("original-uid-12345-test-cluster")
	if result != expectedUID {
		t.Errorf("Expected generated UID %q, got %q", expectedUID, result)
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}