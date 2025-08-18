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

package framework

import (
	"context"
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/logicalcluster/v3"
)

// TestClient provides initialized clients for E2E tests.
type TestClient struct {
	KCPClient     kcpclientset.ClusterInterface
	KubeClient    kubernetes.Interface
	DynamicClient dynamic.Interface
	Config        *rest.Config
	Context       context.Context
}

// NewTestClient creates a new test client with all required interfaces.
func NewTestClient(ctx context.Context, config *rest.Config) (*TestClient, error) {
	if config == nil {
		return nil, fmt.Errorf("rest config cannot be nil")
	}

	kcpClient, err := kcpclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP cluster client: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &TestClient{
		KCPClient:     kcpClient,
		KubeClient:    kubeClient,
		DynamicClient: dynamicClient,
		Config:        config,
		Context:       ctx,
	}, nil
}

// Cluster returns a scoped client for a specific logical cluster.
func (tc *TestClient) Cluster(cluster logicalcluster.Path) *TestClient {
	return &TestClient{
		KCPClient:     tc.KCPClient,
		KubeClient:    tc.KubeClient,
		DynamicClient: tc.DynamicClient,
		Config:        tc.Config,
		Context:       tc.Context,
	}
}