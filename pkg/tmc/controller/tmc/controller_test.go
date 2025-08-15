// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tmc

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestClusterReconciler_ValidateClusterHealth(t *testing.T) {
	tests := map[string]struct {
		cluster *tmcv1alpha1.ClusterRegistration
		wantErr bool
	}{
		"healthy cluster with valid HTTPS endpoint": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-1",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://cluster.example.com",
					},
				},
			},
			wantErr: false,
		},
		"healthy cluster with valid HTTP endpoint": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-2",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "eu-central-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "http://internal-cluster.local:8080",
					},
				},
			},
			wantErr: false,
		},
		"unhealthy cluster with empty endpoint": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-3",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "ap-southeast-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "",
					},
				},
			},
			wantErr: true,
		},
		"unhealthy cluster with invalid URL": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-4",
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "ap-northeast-1",
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "invalid-url",
					},
				},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, tmcv1alpha1.AddToScheme(scheme))
			
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.cluster).
				Build()

			reconciler := NewClusterReconciler(client, logr.Discard())
			err := reconciler.validateClusterHealth(context.Background(), tc.cluster)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPlacementReconciler_PlacementStrategies(t *testing.T) {
	// Create test clusters for placement testing
	clusters := []tmcv1alpha1.ClusterRegistration{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-1"},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{Location: "us-west-1"},
			Status: tmcv1alpha1.ClusterRegistrationStatus{
				Conditions: []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue},
				},
				AllocatedResources: &tmcv1alpha1.ClusterResourceUsage{
					CPU: &[]int64{100}[0],
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-2"},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{Location: "us-west-2"},
			Status: tmcv1alpha1.ClusterRegistrationStatus{
				Conditions: []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue},
				},
				AllocatedResources: &tmcv1alpha1.ClusterResourceUsage{
					CPU: &[]int64{50}[0],
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-3"},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{Location: "eu-central-1"},
			Status: tmcv1alpha1.ClusterRegistrationStatus{
				Conditions: []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue},
				},
				AllocatedResources: &tmcv1alpha1.ClusterResourceUsage{
					CPU: &[]int64{200}[0],
				},
			},
		},
	}

	tests := map[string]struct {
		strategy      tmcv1alpha1.PlacementPolicy
		clusterCount  int
		expectedCount int
		validate      func(t *testing.T, selected []string, clusters []tmcv1alpha1.ClusterRegistration)
	}{
		"round robin placement with 2 clusters": {
			strategy:      tmcv1alpha1.PlacementPolicyRoundRobin,
			clusterCount:  2,
			expectedCount: 2,
			validate: func(t *testing.T, selected []string, clusters []tmcv1alpha1.ClusterRegistration) {
				assert.Len(t, selected, 2)
				assert.Contains(t, selected, "cluster-1")
				assert.Contains(t, selected, "cluster-2")
			},
		},
		"least loaded placement": {
			strategy:      tmcv1alpha1.PlacementPolicyLeastLoaded,
			clusterCount:  2,
			expectedCount: 2,
			validate: func(t *testing.T, selected []string, clusters []tmcv1alpha1.ClusterRegistration) {
				assert.Len(t, selected, 2)
				// Should select cluster-2 (50 CPU) and cluster-1 (100 CPU), not cluster-3 (200 CPU)
				assert.Contains(t, selected, "cluster-2")
				assert.Contains(t, selected, "cluster-1")
				assert.NotContains(t, selected, "cluster-3")
			},
		},
		"random placement": {
			strategy:      tmcv1alpha1.PlacementPolicyRandom,
			clusterCount:  1,
			expectedCount: 1,
			validate: func(t *testing.T, selected []string, clusters []tmcv1alpha1.ClusterRegistration) {
				assert.Len(t, selected, 1)
				// Should select one of the available clusters
				clusterNames := []string{"cluster-1", "cluster-2", "cluster-3"}
				assert.Contains(t, clusterNames, selected[0])
			},
		},
		"location aware placement": {
			strategy:      tmcv1alpha1.PlacementPolicyLocationAware,
			clusterCount:  3,
			expectedCount: 3,
			validate: func(t *testing.T, selected []string, clusters []tmcv1alpha1.ClusterRegistration) {
				assert.Len(t, selected, 3)
				// Should select clusters from different locations
				locations := make(map[string]bool)
				for _, clusterName := range selected {
					for _, cluster := range clusters {
						if cluster.Name == clusterName {
							locations[cluster.Spec.Location] = true
						}
					}
				}
				assert.True(t, len(locations) >= 2, "Should select clusters from different locations")
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			reconciler := NewPlacementReconciler(nil, logr.Discard())

			selected, err := reconciler.applyPlacementStrategy(
				context.Background(),
				&tmcv1alpha1.WorkloadPlacement{
					Spec: tmcv1alpha1.WorkloadPlacementSpec{
						PlacementPolicy:   tc.strategy,
						NumberOfClusters: &[]int32{int32(tc.clusterCount)}[0],
					},
				},
				clusters,
			)

			require.NoError(t, err)
			assert.Len(t, selected, tc.expectedCount)

			if tc.validate != nil {
				tc.validate(t, selected, clusters)
			}
		})
	}
}

func TestClusterReconciler_UpdateCapabilities(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, tmcv1alpha1.AddToScheme(scheme))

	cluster := &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: "us-west-1",
			ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: "https://cluster.example.com",
			},
		},
		Status: tmcv1alpha1.ClusterRegistrationStatus{},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		WithStatusSubresource(cluster).
		Build()

	reconciler := NewClusterReconciler(client, logr.Discard())

	// Test capability detection and update
	err := reconciler.updateClusterCapabilities(context.Background(), cluster)
	require.NoError(t, err)

	// Verify capabilities were updated
	assert.NotNil(t, cluster.Status.Capabilities)
	assert.Equal(t, "v1.28.0", cluster.Status.Capabilities.KubernetesVersion)
	assert.NotNil(t, cluster.Status.Capabilities.NodeCount)
	assert.Equal(t, int32(3), *cluster.Status.Capabilities.NodeCount)
	assert.Contains(t, cluster.Status.Capabilities.SupportedAPIVersions, "v1")
	assert.Contains(t, cluster.Status.Capabilities.Features, "cni")
	assert.NotNil(t, cluster.Status.Capabilities.LastDetected)

	// Verify the timestamp is recent
	timeDiff := time.Since(cluster.Status.Capabilities.LastDetected.Time)
	assert.True(t, timeDiff < time.Minute, "LastDetected should be recent")
}

func TestTMCController_FeatureGateValidation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, tmcv1alpha1.AddToScheme(scheme))

	config := &TMCControllerConfig{
		Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme:   scheme,
		Logger:   logr.Discard(),
		Workspace: logicalcluster.Name("root:test"),
	}

	// Test with feature gate disabled (should fail)
	controller, err := NewTMCController(config)
	assert.Error(t, err)
	assert.Nil(t, controller)
	assert.Contains(t, err.Error(), "TMC feature gate")

	// Note: Testing with feature gate enabled would require modifying the features package
	// which is beyond the scope of this unit test
}

func TestPlacementReconciler_ClusterFiltering(t *testing.T) {
	reconciler := NewPlacementReconciler(nil, logr.Discard())

	tests := map[string]struct {
		cluster *tmcv1alpha1.ClusterRegistration
		healthy bool
	}{
		"healthy cluster with ready condition": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue},
					},
				},
			},
			healthy: true,
		},
		"unhealthy cluster without ready condition": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionFalse},
					},
				},
			},
			healthy: false,
		},
		"unhealthy cluster with no conditions": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Conditions: []metav1.Condition{},
				},
			},
			healthy: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			healthy := reconciler.isClusterHealthy(tc.cluster)
			assert.Equal(t, tc.healthy, healthy)
		})
	}
}

func TestPlacementReconciler_LocationMatching(t *testing.T) {
	reconciler := NewPlacementReconciler(nil, logr.Discard())

	cluster := &tmcv1alpha1.ClusterRegistration{
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: "us-west-1",
		},
	}

	tests := map[string]struct {
		locations []string
		matches   bool
	}{
		"exact location match": {
			locations: []string{"us-west-1"},
			matches:   true,
		},
		"multiple locations with match": {
			locations: []string{"us-east-1", "us-west-1", "eu-central-1"},
			matches:   true,
		},
		"no location match": {
			locations: []string{"us-east-1", "eu-central-1"},
			matches:   false,
		},
		"empty location list": {
			locations: []string{},
			matches:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			matches := reconciler.matchesLocation(cluster, tc.locations)
			assert.Equal(t, tc.matches, matches)
		})
	}
}