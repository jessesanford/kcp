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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	workload "github.com/kcp-dev/kcp/pkg/apis/workload"
)

func TestSchemeRegistration(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(AddToScheme(scheme))

	t.Run("SyncTarget is registered", func(t *testing.T) {
		// Verify GVK registration for SyncTarget
		gvks, _, err := scheme.ObjectKinds(&SyncTarget{})
		require.NoError(t, err)
		require.Len(t, gvks, 1)
		expected := SchemeGroupVersion.WithKind("SyncTarget")
		assert.Equal(t, expected, gvks[0])
	})

	t.Run("SyncTargetList is registered", func(t *testing.T) {
		// Verify GVK registration for SyncTargetList
		gvks, _, err := scheme.ObjectKinds(&SyncTargetList{})
		require.NoError(t, err)
		require.Len(t, gvks, 1)
		expected := SchemeGroupVersion.WithKind("SyncTargetList")
		assert.Equal(t, expected, gvks[0])
	})

	t.Run("scheme can create SyncTarget objects", func(t *testing.T) {
		obj, err := scheme.New(SchemeGroupVersion.WithKind("SyncTarget"))
		require.NoError(t, err)
		
		syncTarget, ok := obj.(*SyncTarget)
		require.True(t, ok, "expected *SyncTarget, got %T", obj)
		assert.NotNil(t, syncTarget)
	})

	t.Run("scheme can create SyncTargetList objects", func(t *testing.T) {
		obj, err := scheme.New(SchemeGroupVersion.WithKind("SyncTargetList"))
		require.NoError(t, err)
		
		syncTargetList, ok := obj.(*SyncTargetList)
		require.True(t, ok, "expected *SyncTargetList, got %T", obj)
		assert.NotNil(t, syncTargetList)
	})
}

func TestSchemeGroupVersion(t *testing.T) {
	t.Run("correct group and version", func(t *testing.T) {
		assert.Equal(t, workload.GroupName, SchemeGroupVersion.Group)
		assert.Equal(t, "v1alpha1", SchemeGroupVersion.Version)
	})

	t.Run("Kind function returns correct GroupKind", func(t *testing.T) {
		gk := Kind("SyncTarget")
		expected := schema.GroupKind{
			Group: workload.GroupName,
			Kind:  "SyncTarget",
		}
		assert.Equal(t, expected, gk)
	})

	t.Run("Resource function returns correct GroupResource", func(t *testing.T) {
		gr := Resource("synctargets")
		expected := schema.GroupResource{
			Group:    workload.GroupName,
			Resource: "synctargets",
		}
		assert.Equal(t, expected, gr)
	})
}

func TestDeepCopyGeneration(t *testing.T) {
	t.Run("SyncTarget can be deep copied", func(t *testing.T) {
		original := &SyncTarget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-synctarget",
				Namespace: "test-namespace",
			},
			Spec: SyncTargetSpec{
				ClusterRef: ClusterReference{
					Name:      "test-cluster",
					Workspace: "test-workspace",
				},
				Location: "us-west-2",
			},
		}

		// Test DeepCopy
		copied := original.DeepCopy()
		require.NotNil(t, copied)
		assert.Equal(t, original.Name, copied.Name)
		assert.Equal(t, original.Spec.ClusterRef.Name, copied.Spec.ClusterRef.Name)
		
		// Verify it's a deep copy, not shallow
		copied.Spec.ClusterRef.Name = "modified"
		assert.NotEqual(t, original.Spec.ClusterRef.Name, copied.Spec.ClusterRef.Name)
	})

	t.Run("SyncTargetList can be deep copied", func(t *testing.T) {
		original := &SyncTargetList{
			Items: []SyncTarget{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "sync1"},
					Spec:       SyncTargetSpec{ClusterRef: ClusterReference{Name: "cluster1"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "sync2"},
					Spec:       SyncTargetSpec{ClusterRef: ClusterReference{Name: "cluster2"}},
				},
			},
		}

		copied := original.DeepCopy()
		require.NotNil(t, copied)
		assert.Len(t, copied.Items, 2)
		assert.Equal(t, original.Items[0].Name, copied.Items[0].Name)
		
		// Verify deep copy
		copied.Items[0].Name = "modified"
		assert.NotEqual(t, original.Items[0].Name, copied.Items[0].Name)
	})
}

func TestAddToScheme(t *testing.T) {
	t.Run("AddToScheme succeeds", func(t *testing.T) {
		scheme := runtime.NewScheme()
		err := AddToScheme(scheme)
		require.NoError(t, err)
		
		// Verify both types are registered
		_, err = scheme.New(SchemeGroupVersion.WithKind("SyncTarget"))
		assert.NoError(t, err)
		
		_, err = scheme.New(SchemeGroupVersion.WithKind("SyncTargetList"))
		assert.NoError(t, err)
	})

	t.Run("SchemeBuilder works correctly", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(SchemeBuilder.AddToScheme(scheme))
		
		// Should be able to create objects
		obj, err := scheme.New(SchemeGroupVersion.WithKind("SyncTarget"))
		require.NoError(t, err)
		assert.IsType(t, &SyncTarget{}, obj)
	})
}