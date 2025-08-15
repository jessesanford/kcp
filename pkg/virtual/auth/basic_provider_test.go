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

package auth

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBasicProvider(t *testing.T) {
	ctx := context.Background()
	provider := NewBasicProvider("test-provider")

	// Test provider name
	if provider.Name() != "test-provider" {
		t.Errorf("Expected provider name 'test-provider', got %s", provider.Name())
	}

	// Test initialization
	config := ProviderConfig{
		CacheEnabled: true,
		CacheTTL:     60,
		AuditEnabled: false,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test admin authorization for write operations
	adminReq := &Request{
		User:      "admin",
		Groups:    []string{"system:masters"},
		Workspace: "default",
		Resource: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Verb: "create",
	}

	decision, err := provider.Authorize(ctx, adminReq)
	if err != nil {
		t.Fatalf("Failed to authorize admin: %v", err)
	}

	if !decision.Allowed {
		t.Errorf("Expected admin to be authorized, got: %s", decision.Reason)
	}

	// Test viewer authorization for write (should be denied)
	viewerWriteReq := &Request{
		User:      "viewer",
		Groups:    []string{"system:viewers"},
		Workspace: "default",
		Resource: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Verb: "create",
	}

	decision, err = provider.Authorize(ctx, viewerWriteReq)
	if err != nil {
		t.Fatalf("Failed to authorize viewer for write: %v", err)
	}

	if decision.Allowed {
		t.Error("Expected viewer to be denied for create operation")
	}

	// Test viewer authorization for read (should be allowed)
	viewerReadReq := &Request{
		User:      "viewer",
		Groups:    []string{"system:viewers"},
		Workspace: "default",
		Resource: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Verb: "get",
	}

	decision, err = provider.Authorize(ctx, viewerReadReq)
	if err != nil {
		t.Fatalf("Failed to authorize viewer for read: %v", err)
	}

	if !decision.Allowed {
		t.Errorf("Expected viewer to be allowed for get operation, got: %s", decision.Reason)
	}
}

func TestBasicProviderPermissions(t *testing.T) {
	ctx := context.Background()
	provider := NewBasicProvider("test-provider")

	config := ProviderConfig{
		CacheEnabled: false,
		AuditEnabled: false,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test getting admin permissions
	permissions, err := provider.GetPermissions(ctx, "default", "admin")
	if err != nil {
		t.Fatalf("Failed to get admin permissions: %v", err)
	}

	if len(permissions) == 0 {
		t.Error("Expected admin to have permissions")
	}

	// Verify admin has wildcard permissions
	hasWildcard := false
	for _, perm := range permissions {
		if len(perm.Verbs) > 0 && perm.Verbs[0] == "*" {
			hasWildcard = true
			break
		}
	}

	if !hasWildcard {
		t.Error("Expected admin to have wildcard permissions")
	}

	// Test getting permissions for non-existent user
	permissions, err = provider.GetPermissions(ctx, "default", "nonexistent")
	if err != nil {
		t.Fatalf("Failed to get permissions for non-existent user: %v", err)
	}

	if len(permissions) != 0 {
		t.Error("Expected no permissions for non-existent user")
	}
}

func TestBasicProviderCaching(t *testing.T) {
	ctx := context.Background()
	provider := NewBasicProvider("test-provider")

	config := ProviderConfig{
		CacheEnabled: true,
		CacheTTL:     3600, // 1 hour
		AuditEnabled: false,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	req := &Request{
		User:      "admin",
		Workspace: "default",
		Resource: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Verb: "get",
	}

	// First request - cache miss
	decision1, err := provider.Authorize(ctx, req)
	if err != nil {
		t.Fatalf("Failed to authorize first request: %v", err)
	}

	// Second request - should hit cache
	decision2, err := provider.Authorize(ctx, req)
	if err != nil {
		t.Fatalf("Failed to authorize second request: %v", err)
	}

	// Both should be allowed
	if !decision1.Allowed || !decision2.Allowed {
		t.Error("Expected both requests to be authorized")
	}

	// Test cache refresh
	if err := provider.RefreshCache(ctx, "default"); err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}
}

func TestBasicProviderResourceMatching(t *testing.T) {
	ctx := context.Background()
	provider := NewBasicProvider("test-provider")

	config := ProviderConfig{
		CacheEnabled: false,
		AuditEnabled: false,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	tests := map[string]struct {
		user      string
		resource  schema.GroupVersionResource
		verb      string
		allowed   bool
		workspace string
	}{
		"admin access to deployments": {
			user: "admin",
			resource: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			verb:      "create",
			allowed:   true,
			workspace: "default",
		},
		"viewer read access": {
			user: "viewer",
			resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			verb:      "list",
			allowed:   true,
			workspace: "default",
		},
		"viewer write access denied": {
			user: "viewer",
			resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			verb:      "delete",
			allowed:   false,
			workspace: "default",
		},
		"editor create access": {
			user: "editor",
			resource: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			verb:      "create",
			allowed:   true,
			workspace: "default",
		},
		"editor delete access denied": {
			user: "editor",
			resource: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			verb:      "delete",
			allowed:   false,
			workspace: "default",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := &Request{
				User:      tc.user,
				Workspace: tc.workspace,
				Resource:  tc.resource,
				Verb:      tc.verb,
			}

			decision, err := provider.Authorize(ctx, req)
			if err != nil {
				t.Fatalf("Authorization failed: %v", err)
			}

			if decision.Allowed != tc.allowed {
				t.Errorf("Expected allowed=%v, got %v. Reason: %s",
					tc.allowed, decision.Allowed, decision.Reason)
			}
		})
	}
}

func TestBasicProviderClose(t *testing.T) {
	ctx := context.Background()
	provider := NewBasicProvider("test-provider")

	config := ProviderConfig{
		CacheEnabled: true,
		CacheTTL:     60,
	}

	if err := provider.Initialize(ctx, config); err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test clean shutdown
	if err := provider.Close(ctx); err != nil {
		t.Errorf("Failed to close provider: %v", err)
	}
}