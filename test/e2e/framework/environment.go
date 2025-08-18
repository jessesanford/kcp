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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

// TestEnvironment provides isolated test environment with cleanup.
type TestEnvironment struct {
	t       kcptesting.TestingT
	clients *TestClient
	cleanup []func() error
}

// NewTestEnvironment creates isolated test environment.
func NewTestEnvironment(t kcptesting.TestingT) *TestEnvironment {
	t.Helper()
	
	server := kcptesting.SharedKcpServer(t)
	config := server.BaseConfig(t)
	
	ctx := context.Background()
	clients, err := NewTestClient(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create test clients: %v", err)
	}
	
	env := &TestEnvironment{
		t:       t,
		clients: clients,
		cleanup: []func() error{},
	}
	
	t.Cleanup(func() {
		env.Cleanup()
	})
	
	return env
}

// Clients returns the test client interfaces.
func (e *TestEnvironment) Clients() *TestClient {
	return e.clients
}

// AddCleanup registers a cleanup function.
func (e *TestEnvironment) AddCleanup(fn func() error) {
	e.cleanup = append(e.cleanup, fn)
}

// Cleanup tears down the test environment.
func (e *TestEnvironment) Cleanup() {
	e.t.Helper()
	
	for i := len(e.cleanup) - 1; i >= 0; i-- {
		if err := e.cleanup[i](); err != nil {
			e.t.Errorf("Cleanup function %d failed: %v", i, err)
		}
	}
}

// CreateTestWorkspace creates a test workspace.
func CreateTestWorkspace(t kcptesting.TestingT, env *TestEnvironment, name string) (*tenancyv1alpha1.Workspace, error) {
	t.Helper()
	
	workspace := &tenancyv1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: tenancyv1alpha1.WorkspaceSpec{
			Type: &tenancyv1alpha1.WorkspaceTypeReference{
				Name: "universal",
				Path: "root",
			},
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	created, err := env.Clients().KCPClient.Cluster(logicalcluster.NewPath("root")).TenancyV1alpha1().Workspaces().Create(ctx, workspace, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace %s: %w", name, err)
	}
	
	env.AddCleanup(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return env.Clients().KCPClient.Cluster(logicalcluster.NewPath("root")).TenancyV1alpha1().Workspaces().Delete(ctx, name, metav1.DeleteOptions{})
	})
	
	return created, nil
}

// CreateTestNamespace creates a test namespace.
func CreateTestNamespace(t kcptesting.TestingT, env *TestEnvironment, name string) (*corev1.Namespace, error) {
	t.Helper()
	
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	created, err := env.Clients().KubeClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create namespace %s: %w", name, err)
	}
	
	env.AddCleanup(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return env.Clients().KubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	})
	
	return created, nil
}