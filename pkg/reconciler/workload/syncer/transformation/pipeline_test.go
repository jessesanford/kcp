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

func TestNewPipeline(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	if pipeline == nil {
		t.Fatal("NewPipeline returned nil")
	}

	transformers := pipeline.ListTransformers()
	if len(transformers) != 1 {
		t.Errorf("Expected 1 transformer, got %d", len(transformers))
	}

	if transformers[0] != "namespace-transformer" {
		t.Errorf("Expected namespace-transformer, got %s", transformers[0])
	}
}

func TestPipelineTransformForDownstream(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	// Create a test pod
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
	}

	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
			Namespace:   "target-namespace",
		},
	}

	result, err := pipeline.TransformForDownstream(context.TODO(), pod, target)
	if err != nil {
		t.Fatalf("TransformForDownstream failed: %v", err)
	}

	if result == nil {
		t.Fatal("TransformForDownstream returned nil result")
	}

	// Verify the result is a different object (deep copy)
	if result == pod {
		t.Error("TransformForDownstream should return a copy, not the original")
	}
}

func TestPipelineTransformForUpstream(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	// Create a test pod with transformed namespace
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "root-test-test-namespace",
			Annotations: map[string]string{
				"syncer.kcp.io/original-namespace": "test-namespace",
			},
		},
	}

	source := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
			Namespace:   "source-namespace",
		},
	}

	result, err := pipeline.TransformForUpstream(context.TODO(), pod, source)
	if err != nil {
		t.Fatalf("TransformForUpstream failed: %v", err)
	}

	if result == nil {
		t.Fatal("TransformForUpstream returned nil result")
	}

	// Verify the result is a different object (deep copy)
	if result == pod {
		t.Error("TransformForUpstream should return a copy, not the original")
	}
}

func TestPipelineNilObject(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}

	_, err := pipeline.TransformForDownstream(context.TODO(), nil, target)
	if err == nil {
		t.Error("Expected error for nil object, got none")
	}

	_, err = pipeline.TransformForUpstream(context.TODO(), nil, target)
	if err == nil {
		t.Error("Expected error for nil object, got none")
	}
}

func TestPipelineRegisterTransformer(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	// Create a mock transformer
	mockTransformer := &mockTransformer{name: "test-transformer"}
	
	initialCount := len(pipeline.ListTransformers())
	pipeline.RegisterTransformer(mockTransformer)
	
	newCount := len(pipeline.ListTransformers())
	if newCount != initialCount+1 {
		t.Errorf("Expected %d transformers after registration, got %d", initialCount+1, newCount)
	}

	transformers := pipeline.ListTransformers()
	found := false
	for _, name := range transformers {
		if name == "test-transformer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Registered transformer not found in list")
	}
}

func TestPipelineRemoveTransformer(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)

	initialCount := len(pipeline.ListTransformers())
	
	// Remove the namespace transformer
	pipeline.RemoveTransformer("namespace-transformer")
	
	newCount := len(pipeline.ListTransformers())
	if newCount != initialCount-1 {
		t.Errorf("Expected %d transformers after removal, got %d", initialCount-1, newCount)
	}

	transformers := pipeline.ListTransformers()
	for _, name := range transformers {
		if name == "namespace-transformer" {
			t.Error("Removed transformer still found in list")
		}
	}
}

// mockTransformer is a test helper
type mockTransformer struct {
	name string
}

func (m *mockTransformer) ShouldTransform(obj runtime.Object) bool {
	return true
}

func (m *mockTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	return obj.DeepCopyObject(), nil
}

func (m *mockTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	return obj.DeepCopyObject(), nil
}

func (m *mockTransformer) Name() string {
	return m.name
}