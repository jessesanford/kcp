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

package registration

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// mockAPIDiscoveryClient implements APIDiscoveryClient for testing
type mockAPIDiscoveryClient struct {
	serverVersion       *version.Info
	serverGroups        *metav1.APIGroupList
	serverResources     []*metav1.APIResourceList
	serverVersionError  error
	serverGroupsError   error
	resourcesError      error
}

func (m *mockAPIDiscoveryClient) ServerVersion() (*version.Info, error) {
	if m.serverVersionError != nil {
		return nil, m.serverVersionError
	}
	return m.serverVersion, nil
}

func (m *mockAPIDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	if m.serverGroupsError != nil {
		return nil, m.serverGroupsError
	}
	return m.serverGroups, nil
}

func (m *mockAPIDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	if m.resourcesError != nil {
		return nil, m.resourcesError
	}
	return m.serverResources, nil
}

// mockClientFactory implements ClientFactory for testing
type mockClientFactory struct {
	kubeClient      *fake.Clientset
	discoveryClient APIDiscoveryClient
	createError     error
}

func (m *mockClientFactory) CreateClient(endpoint tmcv1alpha1.ClusterEndpoint) (kubernetes.Interface, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return m.kubeClient, nil
}

func (m *mockClientFactory) CreateDiscoveryClient(endpoint tmcv1alpha1.ClusterEndpoint) (APIDiscoveryClient, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return m.discoveryClient, nil
}

func TestPerformAPIDiscovery(t *testing.T) {
	tests := map[string]struct {
		discoveryClient  *mockAPIDiscoveryClient
		expectedResult   *DiscoveryResult
		expectedError    bool
		errorContains    string
	}{
		"successful discovery": {
			discoveryClient: &mockAPIDiscoveryClient{
				serverVersion: &version.Info{
					Major: "1", Minor: "28", GitVersion: "v1.28.0",
				},
				serverGroups: &metav1.APIGroupList{
					Groups: []metav1.APIGroup{
						{
							Name: "apps",
							Versions: []metav1.GroupVersionForDiscovery{
								{GroupVersion: "apps/v1", Version: "v1"},
							},
						},
						{
							Name: "networking.k8s.io",
							Versions: []metav1.GroupVersionForDiscovery{
								{GroupVersion: "networking.k8s.io/v1", Version: "v1"},
							},
						},
					},
				},
				serverResources: []*metav1.APIResourceList{
					{
						GroupVersion: "v1",
						APIResources: []metav1.APIResource{
							{Name: "pods", Kind: "Pod"},
							{Name: "services", Kind: "Service"},
						},
					},
					{
						GroupVersion: "apps/v1",
						APIResources: []metav1.APIResource{
							{Name: "deployments", Kind: "Deployment"},
						},
					},
				},
			},
			expectedResult: &DiscoveryResult{
				KubernetesVersion:    "v1.28.0",
				SupportedAPIVersions: []string{"v1", "apps/v1", "networking.k8s.io/v1"},
				AvailableResources:   []string{"pods", "services", "deployments.apps/v1"},
				DetectedFeatures:     []string{"workload-deployment", "networking-services"},
			},
		},
		"version discovery error": {
			discoveryClient: &mockAPIDiscoveryClient{
				serverVersionError: fmt.Errorf("version error"),
			},
			expectedError: true,
			errorContains: "failed to discover Kubernetes version",
		},
		"api versions discovery error": {
			discoveryClient: &mockAPIDiscoveryClient{
				serverVersion: &version.Info{GitVersion: "v1.28.0"},
				serverGroupsError: fmt.Errorf("api versions error"),
			},
			expectedError: true,
			errorContains: "failed to discover API versions",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			result, err := PerformAPIDiscovery(ctx, tc.discoveryClient)

			if tc.expectedError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tc.errorContains != "" && !contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, got %v", tc.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.KubernetesVersion != tc.expectedResult.KubernetesVersion {
				t.Errorf("expected version %q, got %q", tc.expectedResult.KubernetesVersion, result.KubernetesVersion)
			}

			if !stringSlicesEqual(result.SupportedAPIVersions, tc.expectedResult.SupportedAPIVersions) {
				t.Errorf("expected API versions %v, got %v", tc.expectedResult.SupportedAPIVersions, result.SupportedAPIVersions)
			}

			if !stringSlicesEqual(result.DetectedFeatures, tc.expectedResult.DetectedFeatures) {
				t.Errorf("expected features %v, got %v", tc.expectedResult.DetectedFeatures, result.DetectedFeatures)
			}
		})
	}
}

func TestDetectClusterFeatures(t *testing.T) {
	tests := map[string]struct {
		resources       []string
		apiVersions     []string
		expectedFeatures []string
	}{
		"basic workload features": {
			resources: []string{"pods", "deployments.apps/v1", "services"},
			apiVersions: []string{"v1", "apps/v1"},
			expectedFeatures: []string{"workload-deployment", "networking-services"},
		},
		"ingress support": {
			resources: []string{"ingresses.networking.k8s.io/v1"},
			apiVersions: []string{"networking.k8s.io/v1"},
			expectedFeatures: []string{"ingress-support"},
		},
		"storage support": {
			resources: []string{"persistentvolumes", "persistentvolumeclaims"},
			apiVersions: []string{"v1"},
			expectedFeatures: []string{"persistent-storage"},
		},
		"network policies": {
			resources: []string{"networkpolicies.networking.k8s.io/v1"},
			apiVersions: []string{"networking.k8s.io/v1"},
			expectedFeatures: []string{"network-policies"},
		},
		"no features detected": {
			resources: []string{"configmaps", "secrets"},
			apiVersions: []string{"v1"},
			expectedFeatures: []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			features := detectClusterFeatures(tc.resources, tc.apiVersions)
			if !stringSlicesEqual(features, tc.expectedFeatures) {
				t.Errorf("expected features %v, got %v", tc.expectedFeatures, features)
			}
		})
	}
}

func TestClusterCapabilityDetector_DetectCapabilities(t *testing.T) {
	tests := map[string]struct {
		cluster          *tmcv1alpha1.ClusterRegistration
		mockFactory      *mockClientFactory
		expectedError    bool
		errorContains    string
		validateResult   func(t *testing.T, capabilities *tmcv1alpha1.ClusterCapabilities)
	}{
		"successful detection": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					ClusterEndpoint: tmcv1alpha1.ClusterEndpoint{
						ServerURL: "https://test-cluster:6443",
					},
				},
			},
			mockFactory: &mockClientFactory{
				kubeClient: fake.NewSimpleClientset(
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
				),
				discoveryClient: &mockAPIDiscoveryClient{
					serverVersion: &version.Info{GitVersion: "v1.28.0"},
					serverGroups: &metav1.APIGroupList{
						Groups: []metav1.APIGroup{
							{
								Name: "apps",
								Versions: []metav1.GroupVersionForDiscovery{
									{GroupVersion: "apps/v1", Version: "v1"},
								},
							},
						},
					},
					serverResources: []*metav1.APIResourceList{
						{
							GroupVersion: "v1",
							APIResources: []metav1.APIResource{{Name: "pods"}},
						},
					},
				},
			},
			validateResult: func(t *testing.T, capabilities *tmcv1alpha1.ClusterCapabilities) {
				if capabilities.KubernetesVersion != "v1.28.0" {
					t.Errorf("expected version v1.28.0, got %s", capabilities.KubernetesVersion)
				}
				if capabilities.NodeCount == nil || *capabilities.NodeCount != 2 {
					t.Errorf("expected node count 2, got %v", capabilities.NodeCount)
				}
				if capabilities.LastDetected == nil {
					t.Error("expected LastDetected to be set")
				}
			},
		},
		"client creation error": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			},
			mockFactory: &mockClientFactory{
				createError: fmt.Errorf("client creation failed"),
			},
			expectedError: true,
			errorContains: "failed to create Kubernetes client",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			detector := NewClusterCapabilityDetector(tc.mockFactory)
			ctx := context.Background()

			capabilities, err := detector.DetectCapabilities(ctx, tc.cluster)

			if tc.expectedError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tc.errorContains != "" && !contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, got %v", tc.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.validateResult != nil {
				tc.validateResult(t, capabilities)
			}
		})
	}
}

func TestShouldRefreshCapabilities(t *testing.T) {
	now := time.Now()
	oldTime := metav1.NewTime(now.Add(-10 * time.Minute))
	recentTime := metav1.NewTime(now.Add(-2 * time.Minute))

	tests := map[string]struct {
		cluster  *tmcv1alpha1.ClusterRegistration
		expected bool
	}{
		"no capabilities": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{},
			},
			expected: true,
		},
		"no last detected": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Capabilities: &tmcv1alpha1.ClusterCapabilities{},
				},
			},
			expected: true,
		},
		"stale capabilities": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Capabilities: &tmcv1alpha1.ClusterCapabilities{
						LastDetected: &oldTime,
					},
				},
			},
			expected: true,
		},
		"recent capabilities": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					Capabilities: &tmcv1alpha1.ClusterCapabilities{
						LastDetected: &recentTime,
					},
				},
			},
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := ShouldRefreshCapabilities(tc.cluster)
			if result != tc.expected {
				t.Errorf("expected %t, got %t", tc.expected, result)
			}
		})
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 stringContains(s, substr))))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}