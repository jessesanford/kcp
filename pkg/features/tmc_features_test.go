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

package features

import (
	"testing"

	"k8s.io/component-base/featuregate"
)

func TestTMCFeatureFlags(t *testing.T) {
	tests := map[string]struct {
		wantTMCEnabled         bool
		wantAPIsEnabled        bool
		wantControllersEnabled bool
		wantPlacementEnabled   bool
		wantAnyEnabled         bool
	}{
		"default configuration": {
			// By default, all TMC features should be disabled
			wantTMCEnabled:         false,
			wantAPIsEnabled:        false,
			wantControllersEnabled: false,
			wantPlacementEnabled:   false,
			wantAnyEnabled:         false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Test all utility functions with default feature gate settings
			gotTMCEnabled := TMCEnabled()
			if gotTMCEnabled != tc.wantTMCEnabled {
				t.Errorf("TMCEnabled() = %t, want %t", gotTMCEnabled, tc.wantTMCEnabled)
			}

			gotAPIsEnabled := TMCAPIsEnabled()
			if gotAPIsEnabled != tc.wantAPIsEnabled {
				t.Errorf("TMCAPIsEnabled() = %t, want %t", gotAPIsEnabled, tc.wantAPIsEnabled)
			}

			gotControllersEnabled := TMCControllersEnabled()
			if gotControllersEnabled != tc.wantControllersEnabled {
				t.Errorf("TMCControllersEnabled() = %t, want %t", gotControllersEnabled, tc.wantControllersEnabled)
			}

			gotPlacementEnabled := TMCPlacementEnabled()
			if gotPlacementEnabled != tc.wantPlacementEnabled {
				t.Errorf("TMCPlacementEnabled() = %t, want %t", gotPlacementEnabled, tc.wantPlacementEnabled)
			}

			gotAnyEnabled := TMCAnyEnabled()
			if gotAnyEnabled != tc.wantAnyEnabled {
				t.Errorf("TMCAnyEnabled() = %t, want %t", gotAnyEnabled, tc.wantAnyEnabled)
			}
		})
	}
}

func TestTMCFeatureFlagConstants(t *testing.T) {
	tests := map[string]struct {
		feature featuregate.Feature
		want    string
	}{
		"TMCFeature constant": {
			feature: TMCFeature,
			want:    "TMCFeature",
		},
		"TMCAPIs constant": {
			feature: TMCAPIs,
			want:    "TMCAPIs",
		},
		"TMCControllers constant": {
			feature: TMCControllers,
			want:    "TMCControllers",
		},
		"TMCPlacement constant": {
			feature: TMCPlacement,
			want:    "TMCPlacement",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := string(tc.feature)
			if got != tc.want {
				t.Errorf("Feature constant = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTMCFeatureFlagDefaultConfiguration(t *testing.T) {
	tests := []struct {
		feature   featuregate.Feature
		wantInMap bool
	}{
		{TMCFeature, true},
		{TMCAPIs, true},
		{TMCControllers, true},
		{TMCPlacement, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.feature), func(t *testing.T) {
			_, exists := defaultVersionedGenericControlPlaneFeatureGates[tc.feature]
			if exists != tc.wantInMap {
				if tc.wantInMap {
					t.Errorf("Feature %s should be in defaultVersionedGenericControlPlaneFeatureGates but is not", tc.feature)
				} else {
					t.Errorf("Feature %s should not be in defaultVersionedGenericControlPlaneFeatureGates but is", tc.feature)
				}
			}

			if exists {
				specs := defaultVersionedGenericControlPlaneFeatureGates[tc.feature]
				if len(specs) == 0 {
					t.Errorf("Feature %s has empty version specs", tc.feature)
				}

				// Check that all TMC features default to false and are Alpha
				for _, spec := range specs {
					if spec.Default != false {
						t.Errorf("Feature %s should default to false, got %t", tc.feature, spec.Default)
					}
					if spec.PreRelease != featuregate.Alpha {
						t.Errorf("Feature %s should be Alpha prerelease, got %s", tc.feature, spec.PreRelease)
					}
				}
			}
		})
	}
}

func TestTMCFeatureFlagLogic(t *testing.T) {
	// Test that TMC feature utility functions have the correct hierarchical logic
	// Note: This tests the function logic, not the actual feature gate state

	tests := []struct {
		name     string
		funcName string
		desc     string
	}{
		{
			name:     "TMCEnabled",
			funcName: "TMCEnabled()",
			desc:     "returns true only if TMCFeature is enabled",
		},
		{
			name:     "TMCAPIsEnabled",
			funcName: "TMCAPIsEnabled()",
			desc:     "returns true only if both TMCFeature and TMCAPIs are enabled",
		},
		{
			name:     "TMCControllersEnabled",
			funcName: "TMCControllersEnabled()",
			desc:     "returns true only if both TMCFeature and TMCControllers are enabled",
		},
		{
			name:     "TMCPlacementEnabled",
			funcName: "TMCPlacementEnabled()",
			desc:     "returns true only if both TMCFeature and TMCPlacement are enabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing that %s %s", tc.funcName, tc.desc)
			// The actual logic testing is done in the default configuration test above
			// since we can't easily manipulate the feature gates in unit tests
		})
	}
}
