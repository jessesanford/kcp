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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMetadataTransformer_Name(t *testing.T) {
	transformer := NewMetadataTransformer()
	
	expectedName := "metadata-transformer"
	if transformer.Name() != expectedName {
		t.Errorf("Expected transformer name %q, got %q", expectedName, transformer.Name())
	}
}

func TestMetadataTransformer_ShouldTransform(t *testing.T) {
	transformer := NewMetadataTransformer()
	
	tests := map[string]struct {
		obj      runtime.Object
		expected bool
	}{
		"nil object": {
			obj:      nil,
			expected: false,
		},
		"pod object": {
			obj:      &corev1.Pod{},
			expected: true,
		},
		"service object": {
			obj:      &corev1.Service{},
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

func TestMetadataTransformer_TransformForDownstream(t *testing.T) {
	transformer := NewMetadataTransformer()
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
		obj                  runtime.Object
		expectedError        bool
		validateLabels       func(t *testing.T, obj metav1.Object)
		validateAnnotations  func(t *testing.T, obj metav1.Object)
	}{
		"nil object": {
			obj:           nil,
			expectedError: true,
		},
		"non-metadata object": {
			obj:           &runtime.Unknown{},
			expectedError: false,
		},
		"pod with existing metadata": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"app":                     "test-app",
						"app.kubernetes.io/name":  "test",
					},
					Annotations: map[string]string{
						"app.kubernetes.io/version":       "1.0",
						"apis.kcp.io/internal":            "sensitive",
						"kubectl.kubernetes.io/last-applied-configuration": "{}",
					},
					Generation: 5,
				},
			},
			expectedError: false,
			validateLabels: func(t *testing.T, obj metav1.Object) {
				labels := obj.GetLabels()
				
				// Check preserved labels
				if labels["app"] != "test-app" {
					t.Errorf("Expected preserved app label")
				}
				if labels["app.kubernetes.io/name"] != "test" {
					t.Errorf("Expected preserved standard label")
				}
				
				// Check added management labels
				if labels["syncer.kcp.io/managed"] != "true" {
					t.Errorf("Expected syncer managed label")
				}
				if labels["syncer.kcp.io/cluster"] != "test-cluster" {
					t.Errorf("Expected syncer cluster label")
				}
				if labels["syncer.kcp.io/sync-target"] != "test-sync-target" {
					t.Errorf("Expected syncer sync-target label")
				}
				if labels["syncer.kcp.io/last-sync"] == "" {
					t.Errorf("Expected syncer last-sync label")
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				
				// Check preserved annotations
				if annotations["app.kubernetes.io/version"] != "1.0" {
					t.Errorf("Expected preserved app annotation")
				}
				
				// Check removed annotations
				if _, exists := annotations["apis.kcp.io/internal"]; exists {
					t.Errorf("Expected KCP internal annotation to be removed")
				}
				if _, exists := annotations["kubectl.kubernetes.io/last-applied-configuration"]; exists {
					t.Errorf("Expected kubectl annotation to be removed")
				}
				
				// Check added management annotations
				if annotations["syncer.kcp.io/managed"] != "true" {
					t.Errorf("Expected syncer managed annotation")
				}
				if annotations["syncer.kcp.io/cluster"] != "test-cluster" {
					t.Errorf("Expected syncer cluster annotation")
				}
				if annotations["syncer.kcp.io/sync-target"] != "test-sync-target" {
					t.Errorf("Expected syncer sync-target annotation")
				}
				if annotations["syncer.kcp.io/generation"] != "5" {
					t.Errorf("Expected syncer generation annotation, got %q", annotations["syncer.kcp.io/generation"])
				}
				if annotations["syncer.kcp.io/sync-time"] == "" {
					t.Errorf("Expected syncer sync-time annotation")
				}
			},
		},
		"pod without existing metadata": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
			},
			expectedError: false,
			validateLabels: func(t *testing.T, obj metav1.Object) {
				labels := obj.GetLabels()
				
				// Check added management labels
				if labels["syncer.kcp.io/managed"] != "true" {
					t.Errorf("Expected syncer managed label")
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				
				// Check added management annotations
				if annotations["syncer.kcp.io/managed"] != "true" {
					t.Errorf("Expected syncer managed annotation")
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
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateLabels != nil {
				tc.validateLabels(t, metaResult)
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateAnnotations != nil {
				tc.validateAnnotations(t, metaResult)
			}
		})
	}
}

func TestMetadataTransformer_TransformForUpstream(t *testing.T) {
	transformer := NewMetadataTransformer()
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
		obj                 runtime.Object
		expectedError       bool
		validateLabels      func(t *testing.T, obj metav1.Object)
		validateAnnotations func(t *testing.T, obj metav1.Object)
	}{
		"nil object": {
			obj:           nil,
			expectedError: true,
		},
		"pod with syncer metadata": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"app":                       "test-app",
						"syncer.kcp.io/managed":     "true",
						"syncer.kcp.io/cluster":     "test-cluster",
						"syncer.kcp.io/sync-target": "test-sync-target",
						"syncer.kcp.io/last-sync":   "1234567890",
					},
					Annotations: map[string]string{
						"app.kubernetes.io/version":   "1.0",
						"syncer.kcp.io/managed":       "true",
						"syncer.kcp.io/cluster":       "test-cluster",
						"syncer.kcp.io/sync-target":   "test-sync-target",
						"syncer.kcp.io/sync-time":     time.Now().Format(time.RFC3339),
						"syncer.kcp.io/generation":    "5",
					},
				},
			},
			expectedError: false,
			validateLabels: func(t *testing.T, obj metav1.Object) {
				labels := obj.GetLabels()
				
				// Check preserved labels
				if labels["app"] != "test-app" {
					t.Errorf("Expected preserved app label")
				}
				
				// Check removed management labels
				for key := range labels {
					if strings.HasPrefix(key, "syncer.kcp.io/") {
						t.Errorf("Expected syncer label %q to be removed", key)
					}
				}
			},
			validateAnnotations: func(t *testing.T, obj metav1.Object) {
				annotations := obj.GetAnnotations()
				
				// Check preserved annotations
				if annotations["app.kubernetes.io/version"] != "1.0" {
					t.Errorf("Expected preserved app annotation")
				}
				
				// Check removed management annotations
				for key := range annotations {
					if strings.HasPrefix(key, "syncer.kcp.io/") {
						t.Errorf("Expected syncer annotation %q to be removed", key)
					}
				}
			},
		},
		"pod with empty labels after cleanup": {
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"syncer.kcp.io/managed": "true",
					},
					Annotations: map[string]string{
						"syncer.kcp.io/managed": "true",
					},
				},
			},
			expectedError: false,
			validateLabels: func(t *testing.T, obj metav1.Object) {
				labels := obj.GetLabels()
				if labels != nil {
					t.Errorf("Expected nil labels map after cleanup, got %v", labels)
				}
			},
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
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateLabels != nil {
				tc.validateLabels(t, metaResult)
			}
			
			if metaResult, ok := result.(metav1.Object); ok && tc.validateAnnotations != nil {
				tc.validateAnnotations(t, metaResult)
			}
		})
	}
}

func TestMetadataTransformer_shouldRemoveAnnotation(t *testing.T) {
	transformer := &metadataTransformer{
		removeAnnotations: map[string]bool{
			"apis.kcp.io/":     true, // prefix match
			"exact-match":      true, // exact match
			"kubectl.kubernetes.io/last-applied-configuration": true,
		},
	}
	
	tests := map[string]struct {
		key      string
		expected bool
	}{
		"exact match": {
			key:      "exact-match",
			expected: true,
		},
		"prefix match": {
			key:      "apis.kcp.io/some-annotation",
			expected: true,
		},
		"kubectl annotation": {
			key:      "kubectl.kubernetes.io/last-applied-configuration",
			expected: true,
		},
		"no match": {
			key:      "app.kubernetes.io/name",
			expected: false,
		},
		"partial match": {
			key:      "apis.kcp",
			expected: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := transformer.shouldRemoveAnnotation(tc.key)
			if result != tc.expected {
				t.Errorf("Expected shouldRemoveAnnotation(%q) to return %v, got %v", tc.key, tc.expected, result)
			}
		})
	}
}

func TestMetadataTransformer_shouldPreserveAnnotation(t *testing.T) {
	transformer := &metadataTransformer{
		preserveAnnotations: map[string]bool{
			"app.kubernetes.io/name":       true, // exact match
			"service.beta.kubernetes.io/aws-load-balancer-": true, // prefix match
			"kubernetes.io/ingress.class":  true, // exact match
		},
	}
	
	tests := map[string]struct {
		key      string
		expected bool
	}{
		"exact match": {
			key:      "app.kubernetes.io/name",
			expected: true,
		},
		"ingress exact match": {
			key:      "kubernetes.io/ingress.class",
			expected: true,
		},
		"aws prefix match": {
			key:      "service.beta.kubernetes.io/aws-load-balancer-type",
			expected: true,
		},
		"no match": {
			key:      "some.other.io/annotation",
			expected: false,
		},
		"partial prefix": {
			key:      "service.beta.kubernetes.io/aws-load-balancer",
			expected: false,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := transformer.shouldPreserveAnnotation(tc.key)
			if result != tc.expected {
				t.Errorf("Expected shouldPreserveAnnotation(%q) to return %v, got %v", tc.key, tc.expected, result)
			}
		})
	}
}