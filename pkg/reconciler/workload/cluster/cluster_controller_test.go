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

package cluster

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/util/conditions"
)

func TestClusterManager_ReconcileCluster(t *testing.T) {
	tests := map[string]struct {
		cluster   *ClusterRegistration
		wantError bool
		wantPhase ClusterRegistrationPhase
	}{
		"successful cluster registration": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					Labels: map[string]string{
						"kcp.io/cluster-type": "edge",
					},
				},
			},
			wantError: false,
			wantPhase: ClusterRegistrationPhaseReady,
		},
		"cluster with invalid location": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cluster",
					Namespace: "test-ns",
				},
				Spec: ClusterRegistrationSpec{
					Location: "", // Invalid empty location
				},
			},
			wantError: true,
			wantPhase: ClusterRegistrationPhaseFailed,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create test cluster manager with mock implementations
			manager := NewClusterManager(
				&mockClientBuilder{},
				&mockCertificateValidator{},
				&mockRBACManager{},
				&mockSyncTargetManager{},
				&mockPlacementNotifier{},
			)

			ctx := context.Background()
			err := manager.ReconcileCluster(ctx, tc.cluster)

			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}

			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tc.cluster.Status.Phase != tc.wantPhase {
				t.Errorf("expected phase %v, got %v", tc.wantPhase, tc.cluster.Status.Phase)
			}

			// Verify conditions are set
			if tc.wantPhase == ClusterRegistrationPhaseReady {
				if !conditions.IsTrue(tc.cluster, ConditionReady) {
					t.Error("expected Ready condition to be true")
				}
			}

			if tc.wantPhase == ClusterRegistrationPhaseFailed {
				if !conditions.IsFalse(tc.cluster, ConditionCredentialsValid) {
					t.Error("expected CredentialsValid condition to be false")
				}
			}
		})
	}
}

func TestClusterManager_validateCredentials(t *testing.T) {
	tests := map[string]struct {
		cluster   *ClusterRegistration
		wantError bool
	}{
		"valid cluster name": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-cluster",
				},
			},
			wantError: true, // Will fail due to empty kubeconfig
		},
		"empty cluster name": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			wantError: true,
		},
	}

	manager := NewClusterManager(
		&mockClientBuilder{},
		&mockCertificateValidator{},
		&mockRBACManager{},
		&mockSyncTargetManager{},
		&mockPlacementNotifier{},
	)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			err := manager.validateCredentials(ctx, tc.cluster)

			if tc.wantError && err == nil {
				t.Error("expected error but got none")
			}

			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClusterManager_detectFeatures(t *testing.T) {
	tests := map[string]struct {
		apiGroups *metav1.APIGroupList
		wantFeatures []string
	}{
		"basic kubernetes features": {
			apiGroups: &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{Name: "apps"},
					{Name: "networking.k8s.io"},
					{Name: "rbac.authorization.k8s.io"},
				},
			},
			wantFeatures: []string{"NetworkPolicies", "RBAC", "Deployments", "StatefulSets", "DaemonSets"},
		},
		"service mesh features": {
			apiGroups: &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{Name: "istio.io"},
					{Name: "linkerd.io"},
				},
			},
			wantFeatures: []string{"Istio", "Linkerd"},
		},
		"empty api groups": {
			apiGroups: &metav1.APIGroupList{
				Groups: []metav1.APIGroup{},
			},
			wantFeatures: []string{},
		},
	}

	manager := NewClusterManager(
		&mockClientBuilder{},
		&mockCertificateValidator{},
		&mockRBACManager{},
		&mockSyncTargetManager{},
		&mockPlacementNotifier{},
	)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			features := manager.detectFeatures(tc.apiGroups)

			if len(features) != len(tc.wantFeatures) {
				t.Errorf("expected %d features, got %d: %v", len(tc.wantFeatures), len(features), features)
			}

			// Check that all expected features are present
			featureSet := make(map[string]bool)
			for _, feature := range features {
				featureSet[feature] = true
			}

			for _, wantFeature := range tc.wantFeatures {
				if !featureSet[wantFeature] {
					t.Errorf("expected feature %s not found in %v", wantFeature, features)
				}
			}
		})
	}
}

// Mock implementations for testing

type mockClientBuilder struct{}

func (m *mockClientBuilder) BuildClient(kubeconfigData []byte) (kubernetes.Interface, error) {
	return fake.NewSimpleClientset(), nil
}

func (m *mockClientBuilder) BuildDiscoveryClient(kubeconfigData []byte) (discovery.DiscoveryInterface, error) {
	client := fake.NewSimpleClientset()
	return client.Discovery(), nil
}

type mockCertificateValidator struct{}

func (m *mockCertificateValidator) ValidateCertificate(certData []byte) error {
	return nil
}

func (m *mockCertificateValidator) ValidateCertificateChain(chainData []byte) error {
	return nil
}

type mockRBACManager struct{}

func (m *mockRBACManager) SetupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error {
	return nil
}

func (m *mockRBACManager) CleanupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error {
	return nil
}

type mockSyncTargetManager struct{}

func (m *mockSyncTargetManager) CreateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}

func (m *mockSyncTargetManager) UpdateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}

func (m *mockSyncTargetManager) DeleteSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}

type mockPlacementNotifier struct{}

func (m *mockPlacementNotifier) NotifyClusterAdded(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}

func (m *mockPlacementNotifier) NotifyClusterUpdated(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}

func (m *mockPlacementNotifier) NotifyClusterRemoved(ctx context.Context, cluster *ClusterRegistration) error {
	return nil
}