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

package framework

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// TestClient provides unified access to all client interfaces needed for TMC integration tests.
type TestClient struct {
	t *testing.T

	// Core clients
	KcpClusterClient kcpclientset.ClusterInterface
	KubeClient       kubernetes.Interface
	DynamicClient    dynamic.Interface
	DiscoveryClient  discovery.DiscoveryInterface

	// Configuration
	Config       *rest.Config
	Workspace    logicalcluster.Name
	TestPrefix   string
	TestPortBase int
}

// NewTestClient creates a new TestClient with all necessary client interfaces initialized.
func NewTestClient(t *testing.T, config *rest.Config, workspace logicalcluster.Name, testPrefix string, testPortBase int) (*TestClient, error) {
	t.Helper()

	if testPrefix == "" {
		testPrefix = DefaultTestPrefix
	}
	if testPortBase == 0 {
		testPortBase = DefaultTestPortBase
	}

	// Initialize cluster-aware KCP client
	kcpClusterClient, err := kcpclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP cluster client: %w", err)
	}

	// Initialize standard Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Initialize dynamic client for generic resource operations
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Initialize discovery client for API discovery
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return &TestClient{
		t:               t,
		KcpClusterClient: kcpClusterClient,
		KubeClient:      kubeClient,
		DynamicClient:   dynamicClient,
		DiscoveryClient: discoveryClient,
		Config:          config,
		Workspace:       workspace,
		TestPrefix:      testPrefix,
		TestPortBase:    testPortBase,
	}, nil
}

// ClusterFor returns a cluster-scoped client for the given logical cluster.
func (tc *TestClient) ClusterFor(cluster logicalcluster.Name) kcpclientset.Interface {
	return tc.KcpClusterClient.Cluster(cluster.Path())
}

// DynamicFor returns a dynamic client scoped to the given logical cluster and GVR.
func (tc *TestClient) DynamicFor(cluster logicalcluster.Name, gvr schema.GroupVersionResource) dynamic.ResourceInterface {
	return tc.DynamicClient.Resource(gvr).Cluster(cluster.Path())
}

// WithTestPrefix returns a resource name prefixed with the test prefix.
func (tc *TestClient) WithTestPrefix(name string) string {
	return tc.TestPrefix + name
}

// AllocateTestPort returns the next available test port for this test.
func (tc *TestClient) AllocateTestPort() int {
	// Simple sequential allocation for integration tests
	// In a more complex scenario, we might track allocated ports
	return tc.TestPortBase
}

// WaitForAPIGroup waits for a specific API group to become available.
func (tc *TestClient) WaitForAPIGroup(ctx context.Context, groupName string) error {
	tc.t.Helper()
	
	tc.t.Logf("Waiting for API group %s to become available", groupName)
	
	// Poll for API group availability
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			groups, err := tc.DiscoveryClient.ServerGroups()
			if err != nil {
				tc.t.Logf("Error discovering server groups: %v", err)
				continue
			}
			
			for _, group := range groups.Groups {
				if group.Name == groupName {
					tc.t.Logf("API group %s is now available", groupName)
					return nil
				}
			}
		}
	}
}

// CreateTestNamespace creates a namespace with test prefix for resource isolation.
func (tc *TestClient) CreateTestNamespace(ctx context.Context, cluster logicalcluster.Name, namespaceName string) error {
	tc.t.Helper()
	
	namespace := &metav1.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: tc.WithTestPrefix(namespaceName),
			Labels: map[string]string{
				TestSuiteLabelKey: TestSuiteLabelValue,
				TestRunLabelKey:   tc.t.Name(),
			},
		},
	}
	
	kubeClient := tc.KubeClient
	_, err := kubeClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test namespace %s: %w", namespace.Name, err)
	}
	
	tc.t.Logf("Created test namespace %s in cluster %s", namespace.Name, cluster)
	return nil
}

// CleanupTestResources removes all resources created by this test client.
func (tc *TestClient) CleanupTestResources(ctx context.Context) error {
	tc.t.Helper()
	
	tc.t.Logf("Cleaning up test resources with prefix %s", tc.TestPrefix)
	
	// Delete test namespaces
	namespaces, err := tc.KubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: TestSuiteLabelKey + "=" + TestSuiteLabelValue + "," + TestRunLabelKey + "=" + tc.t.Name(),
	})
	if err != nil {
		return fmt.Errorf("failed to list test namespaces: %w", err)
	}
	
	for _, namespace := range namespaces.Items {
		err := tc.KubeClient.CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			tc.t.Logf("Warning: failed to delete namespace %s: %v", namespace.Name, err)
		} else {
			tc.t.Logf("Deleted test namespace %s", namespace.Name)
		}
	}
	
	return nil
}