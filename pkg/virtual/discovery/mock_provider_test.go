/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMockProvider(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider("test-provider")

	// Test provider name
	if got := provider.Name(); got != "test-provider" {
		t.Errorf("Name() = %v, want %v", got, "test-provider")
	}

	// Test initialization
	config := ProviderConfig{
		CacheEnabled:    true,
		CacheTTL:        60,
		RefreshInterval: 30,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test discovery with default workspace
	result, err := provider.Discover(ctx, "default")
	if err != nil {
		t.Fatalf("Failed to discover resources: %v", err)
	}

	if len(result.Groups) == 0 {
		t.Error("Expected at least one API group")
	}

	if len(result.Resources) == 0 {
		t.Error("Expected at least one resource")
	}

	// Verify default deployment resource exists
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	if resource, ok := result.Resources[deploymentGVR]; !ok {
		t.Error("Expected deployments resource to be present")
	} else {
		if resource.APIResource.Kind != "Deployment" {
			t.Errorf("Expected Deployment kind, got %s", resource.APIResource.Kind)
		}
		if !resource.APIResource.Namespaced {
			t.Error("Expected deployments to be namespaced")
		}
		if !resource.WorkspaceScoped {
			t.Error("Expected deployments to be workspace scoped")
		}
	}

	// Test schema retrieval
	schema, err := provider.GetOpenAPISchema(ctx, "default", deploymentGVR)
	if err != nil {
		t.Fatalf("Failed to get schema: %v", err)
	}

	if len(schema) == 0 {
		t.Error("Expected non-empty schema")
	}

	// Test watch functionality
	ch, err := provider.Watch(ctx, "test-workspace")
	if err != nil {
		t.Fatalf("Failed to watch: %v", err)
	}

	// Trigger refresh to generate an event
	if err := provider.Refresh(ctx, "test-workspace"); err != nil {
		t.Fatalf("Failed to refresh: %v", err)
	}

	// Check for event with timeout
	select {
	case event := <-ch:
		if event.Type != EventTypeResourceUpdated {
			t.Errorf("Expected ResourceUpdated event, got %s", event.Type)
		}
		if event.Workspace != "test-workspace" {
			t.Errorf("Expected workspace %s, got %s", "test-workspace", event.Workspace)
		}
	case <-time.After(time.Second):
		t.Error("Expected to receive an event within timeout")
	}

	// Test adding custom resource
	customGVR := schema.GroupVersionResource{
		Group:    "custom.example.com",
		Version:  "v1",
		Resource: "widgets",
	}

	customResource := ResourceInfo{
		APIResource: metav1.APIResource{
			Name:       "widgets",
			Namespaced: true,
			Kind:       "Widget",
			Verbs:      metav1.Verbs{"get", "list", "create", "update", "delete"},
		},
		WorkspaceScoped: true,
	}

	provider.AddResource("test-workspace", customGVR, customResource)

	// Verify custom resource is discoverable
	customResult, err := provider.Discover(ctx, "test-workspace")
	if err != nil {
		t.Fatalf("Failed to discover custom resources: %v", err)
	}

	if _, ok := customResult.Resources[customGVR]; !ok {
		t.Error("Expected custom widget resource to be present")
	}

	// Test adding custom schema
	customSchema := []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`)
	provider.AddSchema("test-workspace", customGVR, customSchema)

	retrievedSchema, err := provider.GetOpenAPISchema(ctx, "test-workspace", customGVR)
	if err != nil {
		t.Fatalf("Failed to get custom schema: %v", err)
	}

	if string(retrievedSchema) != string(customSchema) {
		t.Error("Retrieved schema does not match added schema")
	}

	// Test cleanup
	if err := provider.Close(ctx); err != nil {
		t.Fatalf("Failed to close provider: %v", err)
	}
}