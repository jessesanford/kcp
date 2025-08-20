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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPipelineCreation(t *testing.T) {
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)
	
	require.NotNil(t, pipeline)
	
	transformers := pipeline.ListTransformers()
	expected := []string{
		"namespace-transformer",
		"metadata-transformer",
		"ownerreference-transformer",
		"secret-transformer",
	}
	
	assert.Equal(t, expected, transformers)
}

func TestTransformerRegistration(t *testing.T) {
	pipeline := NewPipeline(logicalcluster.Name("test"))
	
	// Test adding a custom transformer
	customTransformer := &mockTransformer{name: "custom-transformer"}
	pipeline.RegisterTransformer(customTransformer)
	
	transformers := pipeline.ListTransformers()
	assert.Contains(t, transformers, "custom-transformer")
	
	// Test removing a transformer
	pipeline.RemoveTransformer("custom-transformer")
	transformers = pipeline.ListTransformers()
	assert.NotContains(t, transformers, "custom-transformer")
}

func TestPipelineDownstreamTransformation(t *testing.T) {
	ctx := context.Background()
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)
	
	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
			Namespace:   "test-namespace",
		},
	}
	target.SetName("test-target")
	
	// Test with a ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Data: map[string]string{
			"config.yaml": "test: value",
		},
	}
	
	result, err := pipeline.TransformForDownstream(ctx, configMap, target)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultConfigMap, ok := result.(*corev1.ConfigMap)
	require.True(t, ok)
	
	// Check namespace transformation
	assert.Equal(t, "root-test-default", resultConfigMap.Namespace)
	
	// Check metadata transformation
	assert.Equal(t, "true", resultConfigMap.Labels["syncer.kcp.io/managed"])
	assert.Equal(t, "test-cluster", resultConfigMap.Labels["syncer.kcp.io/cluster"])
	
	// Original data should be preserved
	assert.Equal(t, configMap.Data, resultConfigMap.Data)
}

func TestPipelineUpstreamTransformation(t *testing.T) {
	ctx := context.Background()
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)
	
	source := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	source.SetName("test-source")
	
	// Test with a transformed ConfigMap (as if it came from downstream)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "root-test-default",
			Labels: map[string]string{
				"app":                        "test-app",
				"syncer.kcp.io/managed":      "true",
				"syncer.kcp.io/cluster":      "test-cluster",
			},
			Annotations: map[string]string{
				"syncer.kcp.io/original-namespace": "default",
			},
		},
		Data: map[string]string{
			"config.yaml": "test: value",
		},
	}
	
	result, err := pipeline.TransformForUpstream(ctx, configMap, source)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultConfigMap, ok := result.(*corev1.ConfigMap)
	require.True(t, ok)
	
	// Check namespace restoration
	assert.Equal(t, "default", resultConfigMap.Namespace)
	
	// Check metadata cleanup
	assert.Empty(t, resultConfigMap.Labels["syncer.kcp.io/managed"])
	assert.Empty(t, resultConfigMap.Labels["syncer.kcp.io/cluster"])
	
	// Original app label should be preserved
	assert.Equal(t, "test-app", resultConfigMap.Labels["app"])
	
	// Transformation annotation should be removed
	assert.Empty(t, resultConfigMap.Annotations["syncer.kcp.io/original-namespace"])
}

func TestPipelineWithNilObject(t *testing.T) {
	ctx := context.Background()
	pipeline := NewPipeline(logicalcluster.Name("test"))
	target := &SyncTarget{Spec: SyncTargetSpec{ClusterName: "test"}}
	
	result, err := pipeline.TransformForDownstream(ctx, nil, target)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot transform nil object")
}

func TestPipelineTransformationOrder(t *testing.T) {
	ctx := context.Background()
	workspace := logicalcluster.Name("root:test")
	pipeline := NewPipeline(workspace)
	
	// Add a mock transformer to track execution order
	executionOrder := []string{}
	mockTransformer := &mockTransformerWithTracking{
		name: "tracking-transformer",
		executionOrder: &executionOrder,
	}
	pipeline.RegisterTransformer(mockTransformer)
	
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	target.SetName("test-target")
	
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					UID:        "test-uid",
				},
			},
		},
	}
	
	// Test downstream transformation
	_, err := pipeline.TransformForDownstream(ctx, pod, target)
	require.NoError(t, err)
	
	// Verify that the tracking transformer was called (others may skip based on ShouldTransform)
	assert.Contains(t, executionOrder, "tracking-transformer")
	assert.True(t, len(executionOrder) >= 1, "At least one transformer should execute")
}

// Mock transformer for testing
type mockTransformer struct {
	name string
}

func (m *mockTransformer) Name() string {
	return m.name
}

func (m *mockTransformer) ShouldTransform(obj runtime.Object) bool {
	return obj != nil
}

func (m *mockTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	return obj, nil
}

func (m *mockTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	return obj, nil
}

// Mock transformer that tracks execution order
type mockTransformerWithTracking struct {
	name           string
	executionOrder *[]string
}

func (m *mockTransformerWithTracking) Name() string {
	return m.name
}

func (m *mockTransformerWithTracking) ShouldTransform(obj runtime.Object) bool {
	return obj != nil
}

func (m *mockTransformerWithTracking) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	*m.executionOrder = append(*m.executionOrder, m.name)
	return obj, nil
}

func (m *mockTransformerWithTracking) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	*m.executionOrder = append(*m.executionOrder, m.name)
	return obj, nil
}

// Additional unit tests for specific transformers
func TestNamespaceTransformer(t *testing.T) {
	ctx := context.Background()
	workspace := logicalcluster.Name("root:test")
	transformer := NewNamespaceTransformer(workspace)
	
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	
	// Test downstream transformation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}
	
	result, err := transformer.TransformForDownstream(ctx, pod, target)
	require.NoError(t, err)
	
	resultPod, ok := result.(*corev1.Pod)
	require.True(t, ok)
	assert.Equal(t, "root-test-default", resultPod.Namespace)
	assert.Equal(t, "default", resultPod.Annotations["syncer.kcp.io/original-namespace"])
	
	// Test upstream transformation
	result, err = transformer.TransformForUpstream(ctx, resultPod, target)
	require.NoError(t, err)
	
	finalPod, ok := result.(*corev1.Pod)
	require.True(t, ok)
	assert.Equal(t, "default", finalPod.Namespace)
	assert.Empty(t, finalPod.Annotations["syncer.kcp.io/original-namespace"])
}

func TestSecretTransformer(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	
	// Test with allowed secret type
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}
	
	result, err := transformer.TransformForDownstream(ctx, secret, target)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Test with disallowed secret type
	serviceAccountSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("secret-token"),
		},
	}
	
	result, err = transformer.TransformForDownstream(ctx, serviceAccountSecret, target)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not allowed for synchronization")
}