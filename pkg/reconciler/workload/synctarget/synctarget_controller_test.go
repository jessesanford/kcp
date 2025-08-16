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
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-server:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token`
)

func TestBasicReconciliation(t *testing.T) {
	tests := map[string]struct {
		syncTarget      *SyncTarget
		secret          *corev1.Secret
		expectRequeue   bool
		expectCondition bool
		expectPhase     SyncTargetPhase
	}{
		"successful reconciliation with valid kubeconfig": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{
					KubeConfig: "test-secret",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"kubeconfig": []byte(testKubeconfig),
				},
			},
			expectRequeue:   true, // Should requeue for health check
			expectCondition: false, // Connection will fail in test, so condition will be false
			expectPhase:     SyncTargetPhaseNotReady,
		},
		"missing kubeconfig secret": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{
					KubeConfig: "missing-secret",
				},
			},
			expectRequeue:   true,
			expectCondition: false,
			expectPhase:     SyncTargetPhaseNotReady,
		},
		"empty kubeconfig spec": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{},
			},
			expectRequeue:   true,
			expectCondition: false,
			expectPhase:     SyncTargetPhaseNotReady,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))

			var objs []runtime.Object
			objs = append(objs, tc.syncTarget)
			if tc.secret != nil {
				objs = append(objs, tc.secret)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				WithStatusSubresource(tc.syncTarget).
				Build()

			controller := NewController(client, scheme, logicalcluster.Path{})

			// Test reconciliation
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tc.syncTarget.Name,
					Namespace: tc.syncTarget.Namespace,
				},
			}

			result, err := controller.Reconcile(context.TODO(), req)
			require.NoError(t, err)

			if tc.expectRequeue {
				require.True(t, result.RequeueAfter > 0 || result.Requeue)
			}

			// Check the SyncTarget was updated
			updated := &SyncTarget{}
			err = client.Get(context.TODO(), req.NamespacedName, updated)
			require.NoError(t, err)

			// Check finalizer was added
			require.Contains(t, updated.Finalizers, FinalizerName)

			// Check status phase
			require.Equal(t, tc.expectPhase, updated.Status.Phase)

			// Check condition exists
			condition := updated.GetCondition(string(SyncTargetReady))
			require.NotNil(t, condition)

			if tc.expectCondition {
				require.Equal(t, metav1.ConditionTrue, condition.Status)
			} else {
				require.Equal(t, metav1.ConditionFalse, condition.Status)
			}
		})
	}
}

func TestSyncTargetNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	controller := NewController(client, scheme, logicalcluster.Path{})

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	result, err := controller.Reconcile(context.TODO(), req)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)
}

func TestFinalizerHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	now := metav1.Now()
	syncTarget := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-target",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{FinalizerName},
		},
		Spec: SyncTargetSpec{
			KubeConfig: "test-secret",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(syncTarget).
		Build()

	controller := NewController(client, scheme, logicalcluster.Path{})

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      syncTarget.Name,
			Namespace: syncTarget.Namespace,
		},
	}

	result, err := controller.Reconcile(context.TODO(), req)
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, result)

	// Check finalizer was removed
	updated := &SyncTarget{}
	err = client.Get(context.TODO(), req.NamespacedName, updated)
	require.NoError(t, err)
	require.NotContains(t, updated.Finalizers, FinalizerName)
}

func TestSetCondition(t *testing.T) {
	st := &SyncTarget{}

	condition := metav1.Condition{
		Type:               string(SyncTargetReady),
		Status:             metav1.ConditionTrue,
		Reason:             "Connected",
		Message:            "Test condition",
		LastTransitionTime: metav1.Now(),
	}

	st.SetCondition(condition)
	require.Len(t, st.Status.Conditions, 1)
	require.Equal(t, condition.Type, st.Status.Conditions[0].Type)

	// Update existing condition
	updatedCondition := metav1.Condition{
		Type:               string(SyncTargetReady),
		Status:             metav1.ConditionFalse,
		Reason:             "Disconnected",
		Message:            "Updated condition",
		LastTransitionTime: metav1.Now(),
	}

	st.SetCondition(updatedCondition)
	require.Len(t, st.Status.Conditions, 1)
	require.Equal(t, updatedCondition.Status, st.Status.Conditions[0].Status)
	require.Equal(t, updatedCondition.Reason, st.Status.Conditions[0].Reason)
}

func TestGetCondition(t *testing.T) {
	st := &SyncTarget{
		Status: SyncTargetStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(SyncTargetReady),
					Status: metav1.ConditionTrue,
					Reason: "Connected",
				},
			},
		},
	}

	condition := st.GetCondition(string(SyncTargetReady))
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionTrue, condition.Status)

	nonExistent := st.GetCondition("NonExistent")
	require.Nil(t, nonExistent)
}

func TestValidateConnection(t *testing.T) {
	tests := map[string]struct {
		syncTarget  *SyncTarget
		secret      *corev1.Secret
		expectError bool
	}{
		"no kubeconfig specified": {
			syncTarget: &SyncTarget{
				Spec: SyncTargetSpec{},
			},
			expectError: true,
		},
		"secret not found": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{
					KubeConfig: "missing-secret",
				},
			},
			expectError: true,
		},
		"secret missing kubeconfig key": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{
					KubeConfig: "test-secret",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"config": []byte("not kubeconfig"),
				},
			},
			expectError: true,
		},
		"invalid kubeconfig format": {
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: "default",
				},
				Spec: SyncTargetSpec{
					KubeConfig: "test-secret",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"kubeconfig": []byte("invalid yaml"),
				},
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))

			var objs []runtime.Object
			if tc.secret != nil {
				objs = append(objs, tc.secret)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			controller := NewController(client, scheme, logicalcluster.Path{})

			err := controller.validateConnection(context.TODO(), tc.syncTarget)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}