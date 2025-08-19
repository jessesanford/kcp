/*
Copyright 2022 The KCP Authors.

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

package features

import (
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/feature/testing"
)

func TestWorkspaceMountsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "workspace mounts enabled",
			enabled:  true,
			expected: true,
		},
		{
			name:     "workspace mounts disabled",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, WorkspaceMounts, tt.enabled)()
			if got := WorkspaceMountsEnabled(); got != tt.expected {
				t.Errorf("WorkspaceMountsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCacheAPIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "cache APIs enabled",
			enabled:  true,
			expected: true,
		},
		{
			name:     "cache APIs disabled",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, CacheAPIs, tt.enabled)()
			if got := CacheAPIsEnabled(); got != tt.expected {
				t.Errorf("CacheAPIsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDeprecatedAPIExportVirtualWorkspacesUrlsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "deprecated VW URLs enabled",
			enabled:  true,
			expected: true,
		},
		{
			name:     "deprecated VW URLs disabled",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, EnableDeprecatedAPIExportVirtualWorkspacesUrls, tt.enabled)()
			if got := DeprecatedAPIExportVirtualWorkspacesUrlsEnabled(); got != tt.expected {
				t.Errorf("DeprecatedAPIExportVirtualWorkspacesUrlsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetAllEnabledFeatures(t *testing.T) {
	tests := []struct {
		name                    string
		workspaceMounts         bool
		cacheAPIs               bool
		deprecatedVWURLs        bool
		expectedFeatureCount    int
	}{
		{
			name:                 "all features disabled",
			workspaceMounts:      false,
			cacheAPIs:            false,
			deprecatedVWURLs:     false,
			expectedFeatureCount: 0,
		},
		{
			name:                 "workspace mounts only",
			workspaceMounts:      true,
			cacheAPIs:            false,
			deprecatedVWURLs:     false,
			expectedFeatureCount: 1,
		},
		{
			name:                 "cache APIs only",
			workspaceMounts:      false,
			cacheAPIs:            true,
			deprecatedVWURLs:     false,
			expectedFeatureCount: 1,
		},
		{
			name:                 "all features enabled",
			workspaceMounts:      true,
			cacheAPIs:            true,
			deprecatedVWURLs:     true,
			expectedFeatureCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, WorkspaceMounts, tt.workspaceMounts)()
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, CacheAPIs, tt.cacheAPIs)()
			defer testing.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, EnableDeprecatedAPIExportVirtualWorkspacesUrls, tt.deprecatedVWURLs)()
			
			enabled := GetAllEnabledFeatures()
			if len(enabled) != tt.expectedFeatureCount {
				t.Errorf("GetAllEnabledFeatures() returned %d features, want %d", len(enabled), tt.expectedFeatureCount)
			}

			// Verify specific features are present when expected
			enabledMap := make(map[string]bool)
			for _, feature := range enabled {
				enabledMap[string(feature)] = true
			}

			if tt.workspaceMounts {
				if !enabledMap[string(WorkspaceMounts)] {
					t.Error("WorkspaceMounts feature should be in enabled list")
				}
			}
			if tt.cacheAPIs {
				if !enabledMap[string(CacheAPIs)] {
					t.Error("CacheAPIs feature should be in enabled list")
				}
			}
			if tt.deprecatedVWURLs {
				if !enabledMap[string(EnableDeprecatedAPIExportVirtualWorkspacesUrls)] {
					t.Error("EnableDeprecatedAPIExportVirtualWorkspacesUrls feature should be in enabled list")
				}
			}
		})
	}
}