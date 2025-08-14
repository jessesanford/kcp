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

package synctarget

import (
	"context"
	"fmt"
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncTargetDeepCopy(t *testing.T) {
	original := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-target",
			Namespace: "default",
		},
		Spec: SyncTargetSpec{
			KubeConfig: "test-secret",
			VirtualWorkspaces: []VirtualWorkspace{
				{URL: "https://example.com/api"},
			},
		},
		Status: SyncTargetStatus{
			Phase: SyncTargetPhaseReady,
			Conditions: []metav1.Condition{
				{
					Type:   string(SyncTargetReady),
					Status: metav1.ConditionTrue,
					Reason: "Connected",
				},
			},
		},
	}

	copied := original.DeepCopy()
	require.NotNil(t, copied)
	require.Equal(t, original.Name, copied.Name)
	require.Equal(t, original.Spec.KubeConfig, copied.Spec.KubeConfig)
	require.Len(t, copied.Spec.VirtualWorkspaces, 1)
	require.Equal(t, original.Status.Phase, copied.Status.Phase)
	require.Len(t, copied.Status.Conditions, 1)

	// Modify original to ensure deep copy independence
	original.Spec.KubeConfig = "modified"
	require.NotEqual(t, original.Spec.KubeConfig, copied.Spec.KubeConfig)
}

func TestSyncTargetConditions(t *testing.T) {
	st := &SyncTarget{}

	// Test adding first condition
	condition := metav1.Condition{
		Type:   string(SyncTargetReady),
		Status: metav1.ConditionTrue,
		Reason: "Connected",
	}

	st.SetCondition(condition)
	require.Len(t, st.Status.Conditions, 1)
	require.Equal(t, condition.Type, st.Status.Conditions[0].Type)

	// Test updating existing condition
	updatedCondition := metav1.Condition{
		Type:   string(SyncTargetReady),
		Status: metav1.ConditionFalse,
		Reason: "Disconnected",
	}

	st.SetCondition(updatedCondition)
	require.Len(t, st.Status.Conditions, 1)
	require.Equal(t, updatedCondition.Status, st.Status.Conditions[0].Status)
	require.Equal(t, updatedCondition.Reason, st.Status.Conditions[0].Reason)

	// Test getting condition
	retrieved := st.GetCondition(string(SyncTargetReady))
	require.NotNil(t, retrieved)
	require.Equal(t, updatedCondition.Status, retrieved.Status)

	// Test getting non-existent condition
	nonExistent := st.GetCondition("NonExistent")
	require.Nil(t, nonExistent)
}

func TestSyncTargetFinalizers(t *testing.T) {
	st := &SyncTarget{}

	// Test adding finalizer
	require.False(t, st.HasFinalizer(FinalizerName))
	st.AddFinalizer(FinalizerName)
	require.True(t, st.HasFinalizer(FinalizerName))
	require.Len(t, st.Finalizers, 1)

	// Test adding same finalizer again (should not duplicate)
	st.AddFinalizer(FinalizerName)
	require.Len(t, st.Finalizers, 1)

	// Test removing finalizer
	st.RemoveFinalizer(FinalizerName)
	require.False(t, st.HasFinalizer(FinalizerName))
	require.Len(t, st.Finalizers, 0)
}

func TestNewController(t *testing.T) {
	workspace := logicalcluster.Path{}
	controller := NewController(workspace)

	require.NotNil(t, controller)
	require.NotNil(t, controller.ControllerFoundation)
	require.Equal(t, workspace, controller.workspace)
}

func TestControllerReconcile(t *testing.T) {
	workspace := logicalcluster.Path{}
	controller := NewController(workspace)

	tests := []struct {
		name string
		key  string
		expectError bool
	}{
		{
			name: "valid key format",
			key:  "default/test-target",
			expectError: false,
		},
		{
			name: "cluster scoped resource",
			key:  "test-target",
			expectError: false,
		},
		{
			name: "invalid key format",
			key:  "",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := controller.reconcile(context.TODO(), test.key)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConnection(t *testing.T) {
	controller := NewController(logicalcluster.Path{})

	tests := []struct {
		name string
		syncTarget *SyncTarget
		expectError bool
	}{
		{
			name: "no kubeconfig specified",
			syncTarget: &SyncTarget{
				Spec: SyncTargetSpec{},
			},
			expectError: true,
		},
		{
			name: "kubeconfig specified",
			syncTarget: &SyncTarget{
				Spec: SyncTargetSpec{
					KubeConfig: "test-secret",
				},
			},
			expectError: false, // Placeholder implementation returns nil
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := controller.validateConnection(context.TODO(), test.syncTarget)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateStatusOperations(t *testing.T) {
	controller := NewController(logicalcluster.Path{})
	st := &SyncTarget{}

	// Test updateStatusReady
	err := controller.updateStatusReady(context.TODO(), st)
	require.NoError(t, err)
	require.Equal(t, SyncTargetPhaseReady, st.Status.Phase)
	require.NotNil(t, st.Status.LastHeartbeat)

	condition := st.GetCondition(string(SyncTargetReady))
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionTrue, condition.Status)

	// Test updateStatusError
	testErr := fmt.Errorf("test error")
	err = controller.updateStatusError(context.TODO(), st, testErr)
	require.NoError(t, err)
	require.Equal(t, SyncTargetPhaseNotReady, st.Status.Phase)

	condition = st.GetCondition(string(SyncTargetReady))
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Contains(t, condition.Message, "test error")
}

func TestHandleDeletion(t *testing.T) {
	controller := NewController(logicalcluster.Path{})
	st := &SyncTarget{}

	// Add finalizer first
	st.AddFinalizer(FinalizerName)
	require.True(t, st.HasFinalizer(FinalizerName))

	// Handle deletion
	err := controller.handleDeletion(context.TODO(), st)
	require.NoError(t, err)
	require.False(t, st.HasFinalizer(FinalizerName))
}