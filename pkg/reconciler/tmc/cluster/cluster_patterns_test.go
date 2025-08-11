/*
Copyright 2025 The KCP Authors.

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
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testLogicalCluster = logicalcluster.Name("root:test")
)

// TestClusterQueueKey_String tests the string representation of ClusterQueueKey.
func TestClusterQueueKey_String(t *testing.T) {
	tests := map[string]struct {
		key      ClusterQueueKey
		expected string
	}{
		"basic cluster key": {
			key: ClusterQueueKey{
				ClusterName: testLogicalCluster,
				Name:        "test-cluster",
			},
			expected: "root:test/test-cluster",
		},
		"production cluster": {
			key: ClusterQueueKey{
				ClusterName: logicalcluster.Name("root:prod"),
				Name:        "prod-cluster-01",
			},
			expected: "root:prod/prod-cluster-01",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.key.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// newTestClusterRegistration creates a test ClusterRegistration.
func newTestClusterRegistration(name, location, serverURL string) *tmcv1alpha1.ClusterRegistration {
	cluster := &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 1,
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: testLogicalCluster.String(),
			},
		},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: location,
			ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
				ServerURL: serverURL,
			},
		},
		Status: tmcv1alpha1.ClusterRegistrationStatus{
			Conditions: []conditionsv1alpha1.Condition{},
		},
	}
	return cluster
}

// TestValidateClusterRegistration tests cluster registration validation.
func TestValidateClusterRegistration(t *testing.T) {
	setup := &clusterController{}

	tests := map[string]struct {
		spec        tmcv1alpha1.ClusterRegistrationSpec
		expectError bool
		errorMsg    string
	}{
		"valid specification": {
			spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: "us-west-1",
				ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
					ServerURL: "https://api.cluster.example.com",
				},
			},
			expectError: false,
		},
		"missing location": {
			spec: tmcv1alpha1.ClusterRegistrationSpec{
				ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
					ServerURL: "https://api.cluster.example.com",
				},
			},
			expectError: true,
			errorMsg:    "cluster location must be specified",
		},
		"missing server URL": {
			spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: "us-west-1",
			},
			expectError: true,
			errorMsg:    "cluster server URL must be specified",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cluster := newTestClusterRegistration("test-cluster", tc.spec.Location, tc.spec.ClusterEndpoint.ServerURL)
			cluster.Spec = tc.spec

			resource := convertToCommitterResource(cluster)
			err := setup.validateClusterRegistration(getTestContext(), resource)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}