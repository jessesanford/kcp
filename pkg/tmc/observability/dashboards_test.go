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

package observability

import (
	"testing"

	"k8s.io/component-base/featuregate"
	
	"github.com/kcp-dev/kcp/pkg/features"
)

func TestNewDashboardManager(t *testing.T) {
	tests := map[string]struct {
		grafanaURL string
		apiKey     string
		featureEnabled bool
		expectEnabled  bool
	}{
		"creates manager with feature enabled": {
			grafanaURL: "http://grafana:3000",
			apiKey:     "test-key",
			featureEnabled: true,
			expectEnabled: true,
		},
		"creates manager with feature disabled": {
			grafanaURL: "http://grafana:3000", 
			apiKey:     "test-key",
			featureEnabled: false,
			expectEnabled: false,
		},
		"handles empty configuration": {
			grafanaURL: "",
			apiKey:     "",
			featureEnabled: true,
			expectEnabled: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set feature gate for test
			originalGate := features.DefaultFeatureGate
			testGate := featuregate.NewFeatureGate()
			testGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
				TMCDashboards: {Default: tc.featureEnabled, PreRelease: featuregate.Alpha},
			})
			features.DefaultFeatureGate = testGate
			defer func() { features.DefaultFeatureGate = originalGate }()

			dm := NewDashboardManager(tc.grafanaURL, tc.apiKey)

			if dm.grafanaURL != tc.grafanaURL {
				t.Errorf("expected grafanaURL %s, got %s", tc.grafanaURL, dm.grafanaURL)
			}
			if dm.apiKey != tc.apiKey {
				t.Errorf("expected apiKey %s, got %s", tc.apiKey, dm.apiKey)
			}
			if dm.enabled != tc.expectEnabled {
				t.Errorf("expected enabled %v, got %v", tc.expectEnabled, dm.enabled)
			}
		})
	}
}

func TestDashboardManager_IsEnabled(t *testing.T) {
	dm := &DashboardManager{enabled: true}
	if !dm.IsEnabled() {
		t.Error("expected IsEnabled to return true")
	}

	dm.enabled = false
	if dm.IsEnabled() {
		t.Error("expected IsEnabled to return false")
	}
}

func TestLoadEmbeddedDashboards(t *testing.T) {
	tests := map[string]struct {
		enabled bool
		wantNil bool
	}{
		"loads dashboards when enabled": {
			enabled: true,
			wantNil: false,
		},
		"returns nil when disabled": {
			enabled: false,
			wantNil: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dm := &DashboardManager{enabled: tc.enabled}
			
			dashboards, err := dm.LoadEmbeddedDashboards()
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if tc.wantNil && dashboards != nil {
				t.Error("expected nil dashboards when disabled")
			}
			
			if !tc.wantNil && dashboards == nil {
				t.Error("expected non-nil dashboards when enabled")
			}
		})
	}
}