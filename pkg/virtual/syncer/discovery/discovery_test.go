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

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic/fake"

	kcpclientsetfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestNewDiscoveryService(t *testing.T) {
	tests := map[string]struct {
		kcpClient       interface{}
		dynamicClient   interface{}
		informerFactory interface{}
		wantError       bool
	}{
		"valid parameters": {
			kcpClient:       kcpclientsetfake.NewSimpleClientset(),
			dynamicClient:   fake.NewSimpleDynamicClient(nil),
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpclientsetfake.NewSimpleClientset(), time.Minute),
			wantError:       false,
		},
		"nil kcpClient": {
			kcpClient:       nil,
			dynamicClient:   fake.NewSimpleDynamicClient(nil),
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpclientsetfake.NewSimpleClientset(), time.Minute),
			wantError:       true,
		},
		"nil dynamicClient": {
			kcpClient:       kcpclientsetfake.NewSimpleClientset(),
			dynamicClient:   nil,
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpclientsetfake.NewSimpleClientset(), time.Minute),
			wantError:       true,
		},
		"nil informerFactory": {
			kcpClient:       kcpclientsetfake.NewSimpleClientset(),
			dynamicClient:   fake.NewSimpleDynamicClient(nil),
			informerFactory: nil,
			wantError:       true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			logger := logr.Discard()

			service, err := NewDiscoveryService(
				logger,
				tc.kcpClient,
				tc.dynamicClient,
				tc.informerFactory,
			)

			if tc.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if service == nil {
				t.Errorf("Expected service instance but got nil")
			}
		})
	}
}

func TestNewDiscoveryCache(t *testing.T) {
	tests := map[string]struct {
		ttl       time.Duration
		wantError bool
	}{
		"valid TTL": {
			ttl:       5 * time.Minute,
			wantError: false,
		},
		"zero TTL": {
			ttl:       0,
			wantError: false, // Should use default
		},
		"negative TTL": {
			ttl:       -1 * time.Minute,
			wantError: false, // Should use default
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			logger := logr.Discard()

			cache, err := NewDiscoveryCache(logger, tc.ttl)

			if tc.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if cache == nil {
				t.Errorf("Expected cache instance but got nil")
			}
		})
	}
}

func TestDiscoveryCacheOperations(t *testing.T) {
	logger := logr.Discard()
	cache, err := NewDiscoveryCache(logger, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test API group operations
	groupName := "apps"
	apiGroup := &metav1.APIGroup{
		Name: groupName,
		Versions: []metav1.GroupVersionForDiscovery{
			{GroupVersion: "apps/v1", Version: "v1"},
		},
	}

	// Initially should not be in cache
	if _, found := cache.GetAPIGroup(groupName); found {
		t.Errorf("Expected group not to be in cache initially")
	}

	// Put in cache
	cache.PutAPIGroup(groupName, apiGroup)

	// Should now be in cache
	if cached, found := cache.GetAPIGroup(groupName); !found {
		t.Errorf("Expected group to be in cache")
	} else if cached.Name != groupName {
		t.Errorf("Expected group name %s, got %s", groupName, cached.Name)
	}

	// Test invalidation
	cache.InvalidateAPIGroup(groupName)
	if _, found := cache.GetAPIGroup(groupName); found {
		t.Errorf("Expected group to be removed after invalidation")
	}
}

func TestVersionNegotiator(t *testing.T) {
	negotiator := NewVersionNegotiator()

	// Test version sorting
	versions := []string{"v1beta1", "v1", "v1alpha1"}
	negotiator.SortVersions(versions)

	// Should be sorted (basic implementation just does string sort)
	expected := []string{"v1", "v1alpha1", "v1beta1"}
	for i, version := range versions {
		if version != expected[i] {
			t.Errorf("Expected version %s at position %d, got %s", expected[i], i, version)
		}
	}

	// Test preferred version selection
	availableVersions := sets.New("v1", "v1beta1", "v1beta2")
	preferred := negotiator.GetPreferredVersion("apps", availableVersions)
	if preferred != "v1" {
		t.Errorf("Expected preferred version v1, got %s", preferred)
	}
}

func TestOpenAPISchemaManager(t *testing.T) {
	logger := logr.Discard()
	manager, err := NewOpenAPISchemaManager(logger)
	if err != nil {
		t.Fatalf("Failed to create schema manager: %v", err)
	}

	// Test starting the manager
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Errorf("Failed to start schema manager: %v", err)
	}

	// Test getting non-existent schema
	resource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if _, found := manager.GetAggregatedSchema(resource); found {
		t.Errorf("Expected schema not to be found initially")
	}

	// Test getting all schemas (should be empty)
	allSchemas := manager.GetAllAggregatedSchemas()
	if len(allSchemas) != 0 {
		t.Errorf("Expected no schemas initially, got %d", len(allSchemas))
	}
}