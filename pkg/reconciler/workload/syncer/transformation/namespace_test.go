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

	"github.com/kcp-dev/logicalcluster/v3"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewNamespaceTransformer(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	transformer := NewNamespaceTransformer(workspace)

	if transformer == nil {
		t.Fatal("NewNamespaceTransformer returned nil")
	}

	if transformer.Name() != "namespace-transformer" {
		t.Errorf("Expected name 'namespace-transformer', got %s", transformer.Name())
	}
}

func TestNamespaceTransformerShouldTransform(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	transformer := NewNamespaceTransformer(workspace)

	tests := []struct {
		name     string
		obj      runtime.Object
		expected bool
	}{
		{
			name: "namespaced pod",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-namespace",
				},
			},
			expected: true,
		},
		{
			name: "cluster-scoped node",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			expected: false,
		},
		{
			name:     "nil object",
			obj:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.ShouldTransform(tt.obj)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNamespaceTransformerDownstream(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	transformer := NewNamespaceTransformer(workspace)

	tests := []struct {
		name              string
		inputNamespace    string
		expectedNamespace string
		expectAnnotation  bool
	}{
		{
			name:              "regular namespace",
			inputNamespace:    "test-namespace",
			expectedNamespace: "root-test-test-namespace",
			expectAnnotation:  true,
		},
		{
			name:              "default namespace",
			inputNamespace:    "default",
			expectedNamespace: "root-test-default",
			expectAnnotation:  true,
		},
		{
			name:              "kube-system preserved",
			inputNamespace:    "kube-system",
			expectedNamespace: "kube-system",
			expectAnnotation:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: tt.inputNamespace,
				},
			}

			target := &SyncTarget{
				Spec: SyncTargetSpec{
					ClusterName: "test-cluster",
				},
			}

			result, err := transformer.TransformForDownstream(context.TODO(), pod, target)
			if err != nil {
				t.Fatalf("TransformForDownstream failed: %v", err)
			}

			resultPod, ok := result.(*corev1.Pod)
			if !ok {
				t.Fatalf("Expected *corev1.Pod, got %T", result)
			}

			if resultPod.Namespace != tt.expectedNamespace {
				t.Errorf("Expected namespace %s, got %s", tt.expectedNamespace, resultPod.Namespace)
			}

			annotations := resultPod.GetAnnotations()
			if tt.expectAnnotation {
				if annotations == nil {
					t.Error("Expected annotations to be set")
				} else if annotations["syncer.kcp.io/original-namespace"] != tt.inputNamespace {
					t.Errorf("Expected original namespace annotation %s, got %s",
						tt.inputNamespace, annotations["syncer.kcp.io/original-namespace"])
				}
			} else {
				if annotations != nil && annotations["syncer.kcp.io/original-namespace"] != "" {
					t.Error("Did not expect original namespace annotation to be set")
				}
			}
		})
	}
}

func TestNamespaceTransformerUpstream(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	transformer := NewNamespaceTransformer(workspace)

	tests := []struct {
		name              string
		inputNamespace    string
		inputAnnotations  map[string]string
		expectedNamespace string
	}{
		{
			name:           "restore from annotation",
			inputNamespace: "root-test-test-namespace",
			inputAnnotations: map[string]string{
				"syncer.kcp.io/original-namespace": "test-namespace",
			},
			expectedNamespace: "test-namespace",
		},
		{
			name:              "remove prefix fallback",
			inputNamespace:    "root-test-other-namespace",
			inputAnnotations:  nil,
			expectedNamespace: "other-namespace",
		},
		{
			name:              "system namespace preserved",
			inputNamespace:    "kube-system",
			inputAnnotations:  nil,
			expectedNamespace: "kube-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Namespace:   tt.inputNamespace,
					Annotations: tt.inputAnnotations,
				},
			}

			source := &SyncTarget{
				Spec: SyncTargetSpec{
					ClusterName: "test-cluster",
				},
			}

			result, err := transformer.TransformForUpstream(context.TODO(), pod, source)
			if err != nil {
				t.Fatalf("TransformForUpstream failed: %v", err)
			}

			resultPod, ok := result.(*corev1.Pod)
			if !ok {
				t.Fatalf("Expected *corev1.Pod, got %T", result)
			}

			if resultPod.Namespace != tt.expectedNamespace {
				t.Errorf("Expected namespace %s, got %s", tt.expectedNamespace, resultPod.Namespace)
			}

			// Annotation should be removed
			annotations := resultPod.GetAnnotations()
			if annotations != nil && annotations["syncer.kcp.io/original-namespace"] != "" {
				t.Error("Original namespace annotation should be removed")
			}
		})
	}
}

func TestGenerateNamespacePrefix(t *testing.T) {
	tests := []struct {
		name      string
		workspace logicalcluster.Name
		expected  string
	}{
		{
			name:      "simple workspace",
			workspace: "root:test",
			expected:  "root-test",
		},
		{
			name:      "nested workspace",
			workspace: "root:org:team",
			expected:  "root-org-team",
		},
		{
			name:      "empty workspace",
			workspace: "",
			expected:  "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNamespacePrefix(tt.workspace)
			if result != tt.expected {
				t.Errorf("Expected prefix %s, got %s", tt.expected, result)
			}
		})
	}
}