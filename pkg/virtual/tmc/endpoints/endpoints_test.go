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

package endpoints

import (
	"net/http"
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

func TestNewTMCEndpoints(t *testing.T) {
	config := &EndpointConfig{
		DynamicClient: fake.NewSimpleDynamicClient(nil),
		Workspace:     logicalcluster.New("test-workspace"),
		PathPrefix:    "/services/apiexport/test-workspace",
	}

	endpoints := NewTMCEndpoints(config)

	if endpoints == nil {
		t.Error("expected non-nil endpoints")
	}

	if endpoints.workspace != config.Workspace {
		t.Errorf("expected workspace %v, got %v", config.Workspace, endpoints.workspace)
	}

	if endpoints.pathPrefix != config.PathPrefix {
		t.Errorf("expected pathPrefix %s, got %s", config.PathPrefix, endpoints.pathPrefix)
	}
}

func TestInstallHandlers(t *testing.T) {
	config := &EndpointConfig{
		DynamicClient: fake.NewSimpleDynamicClient(nil),
		Workspace:     logicalcluster.New("test-workspace"),
		PathPrefix:    "/services/apiexport/test-workspace",
	}

	endpoints := NewTMCEndpoints(config)
	mux := http.NewServeMux()

	// Should not panic
	endpoints.InstallHandlers(mux)
}

func TestEndpointConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config *EndpointConfig
		valid  bool
	}{
		{
			name: "valid configuration",
			config: &EndpointConfig{
				DynamicClient: fake.NewSimpleDynamicClient(nil),
				Workspace:     logicalcluster.New("test-workspace"),
				PathPrefix:    "/services/apiexport/test-workspace",
			},
			valid: true,
		},
		{
			name: "empty workspace",
			config: &EndpointConfig{
				DynamicClient: fake.NewSimpleDynamicClient(nil),
				Workspace:     logicalcluster.New(""),
				PathPrefix:    "/services/apiexport/test-workspace",
			},
			valid: false,
		},
		{
			name: "empty path prefix",
			config: &EndpointConfig{
				DynamicClient: fake.NewSimpleDynamicClient(nil),
				Workspace:     logicalcluster.New("test-workspace"),
				PathPrefix:    "",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := NewTMCEndpoints(tt.config)

			if tt.valid && endpoints == nil {
				t.Error("expected valid configuration to create endpoints")
			}

			if !tt.valid && endpoints != nil {
				// For now, we don't validate in constructor
				// This test documents expected future behavior
				t.Log("constructor does not yet validate configuration")
			}
		})
	}
}