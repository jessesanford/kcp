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

	kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpfakeclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewKCPDiscoveryProvider(t *testing.T) {
	tests := map[string]struct {
		kcpClient       interface{}
		informerFactory kcpinformers.SharedInformerFactory
		workspace       logicalcluster.Name
		wantError       bool
	}{
		"valid parameters": {
			kcpClient:       kcpfakeclient.NewSimpleClientset(),
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpfakeclient.NewSimpleClientset(), time.Minute),
			workspace:       logicalcluster.Name("root:test"),
			wantError:       false,
		},
		"nil kcpClient": {
			kcpClient:       nil,
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpfakeclient.NewSimpleClientset(), time.Minute),
			workspace:       logicalcluster.Name("root:test"),
			wantError:       true,
		},
		"nil informerFactory": {
			kcpClient:       kcpfakeclient.NewSimpleClientset(),
			informerFactory: nil,
			workspace:       logicalcluster.Name("root:test"),
			wantError:       true,
		},
		"empty workspace": {
			kcpClient:       kcpfakeclient.NewSimpleClientset(),
			informerFactory: kcpinformers.NewSharedInformerFactory(kcpfakeclient.NewSimpleClientset(), time.Minute),
			workspace:       logicalcluster.Name(""),
			wantError:       true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var kcpClient kcpclient.ClusterInterface
			if tc.kcpClient != nil {
				kcpClient = tc.kcpClient.(kcpclient.ClusterInterface)
			}
			provider, err := NewKCPDiscoveryProvider(kcpClient, tc.informerFactory, tc.workspace)
			
			if tc.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if provider != nil {
					t.Errorf("expected nil provider on error, got %v", provider)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if provider == nil {
					t.Errorf("expected provider, got nil")
				}
			}
		})
	}
}

func TestKCPDiscoveryProvider_Discover(t *testing.T) {
	// Create a fake client 
	fakeClient := kcpfakeclient.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(fakeClient, time.Minute)

	provider, err := NewKCPDiscoveryProvider(fakeClient, informerFactory, logicalcluster.Name("root:test"))
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test discovery with empty store
	ctx := context.Background()
	resources, err := provider.Discover(ctx, logicalcluster.Name("root:test"))
	if err != nil {
		t.Errorf("Discover failed: %v", err)
	}

	// With empty store, we should get empty results
	if len(resources) != 0 {
		t.Errorf("Expected no resources with empty store, got %d", len(resources))
	}
}

func TestKCPDiscoveryProvider_StartStop(t *testing.T) {
	fakeClient := kcpfakeclient.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(fakeClient, time.Minute)

	provider, err := NewKCPDiscoveryProvider(fakeClient, informerFactory, logicalcluster.Name("root:test"))
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test starting the provider
	ctx := context.Background()
	err = provider.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	// Test starting already started provider
	err = provider.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already started provider")
	}

	// Test stopping the provider
	provider.Stop()

	// Test stopping already stopped provider (should not panic)
	provider.Stop()
}

// Additional tests will be added in subsequent PRs
// This basic test coverage validates core functionality for this PR split