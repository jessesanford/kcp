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

package upstream

import (
	"testing"
	"time"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"

	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestNewFactory(t *testing.T) {
	tests := map[string]struct {
		featureEnabled bool
		wantNoop       bool
	}{
		"feature gate enabled": {
			featureEnabled: true,
			wantNoop:       false,
		},
		"feature gate disabled": {
			featureEnabled: false,
			wantNoop:       true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Save original state and restore after test
			originalGate := utilfeature.DefaultMutableFeatureGate
			defer func() {
				utilfeature.DefaultMutableFeatureGate = originalGate
			}()

			// Create new feature gate for this test
			testGate := featuregate.NewFeatureGate()
			utilfeature.DefaultMutableFeatureGate = testGate
			
			// Add our feature gates to test gate
			if err := testGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
				kcpfeatures.UpstreamSync: {Default: tc.featureEnabled, PreRelease: featuregate.Alpha},
			}); err != nil {
				t.Fatalf("failed to add feature gate: %v", err)
			}

			factory := NewFactory()
			
			if tc.wantNoop {
				if _, ok := factory.(*noopFactory); !ok {
					t.Errorf("expected noopFactory when feature disabled, got %T", factory)
				}
			} else {
				if _, ok := factory.(*defaultFactory); !ok {
					t.Errorf("expected defaultFactory when feature enabled, got %T", factory)
				}
			}
		})
	}
}

func TestDefaultFactoryMethods(t *testing.T) {
	factory := &defaultFactory{}
	
	// Test that all methods return appropriate errors indicating implementation is pending
	config := &Config{
		ClusterName:   "test",
		Namespace:     "default",
		SyncInterval:  30 * time.Second,
		MaxRetries:    3,
		CacheSize:     1000,
		EnableMetrics: true,
	}

	t.Run("NewSyncer", func(t *testing.T) {
		syncer, err := factory.NewSyncer(config)
		if err == nil {
			t.Error("expected error for unimplemented method")
		}
		if syncer != nil {
			t.Error("expected nil syncer for unimplemented method")
		}
		if err.Error() != "not implemented: awaiting wave2c-02-core-sync" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("NewWatcher", func(t *testing.T) {
		watcher := factory.NewWatcher(nil, "test-cluster")
		if watcher != nil {
			t.Error("expected nil watcher for unimplemented method")
		}
	})

	t.Run("NewProcessor", func(t *testing.T) {
		processor := factory.NewProcessor(nil)
		if processor != nil {
			t.Error("expected nil processor for unimplemented method")
		}
	})

	t.Run("NewAggregator", func(t *testing.T) {
		aggregator := factory.NewAggregator(tmcv1alpha1.ConflictStrategyUseNewest)
		if aggregator != nil {
			t.Error("expected nil aggregator for unimplemented method")
		}
	})
}

func TestNoopFactoryMethods(t *testing.T) {
	factory := &noopFactory{}
	
	config := &Config{
		ClusterName:   "test",
		Namespace:     "default",
		SyncInterval:  30 * time.Second,
		MaxRetries:    3,
		CacheSize:     1000,
		EnableMetrics: true,
	}

	t.Run("NewSyncer", func(t *testing.T) {
		syncer, err := factory.NewSyncer(config)
		if err != nil {
			t.Errorf("unexpected error from noop factory: %v", err)
		}
		if syncer == nil {
			t.Error("expected noop syncer, got nil")
		}
		if _, ok := syncer.(*noopSyncer); !ok {
			t.Errorf("expected noopSyncer, got %T", syncer)
		}
	})

	t.Run("other methods return nil", func(t *testing.T) {
		if factory.NewWatcher(nil, "test") != nil {
			t.Error("expected nil from noop factory")
		}
		if factory.NewProcessor(nil) != nil {
			t.Error("expected nil from noop factory")
		}
	})
}

func TestNoopSyncer(t *testing.T) {
	syncer := &noopSyncer{}
	
	// Test that all methods are safe to call and return appropriate values
	if syncer.IsReady() {
		t.Error("noop syncer should not be ready")
	}
	
	metrics := syncer.GetMetrics()
	if metrics.SyncTargetsActive != 0 {
		t.Error("noop syncer should have zero active targets")
	}
	
	// These should not panic
	syncer.Stop()
	if err := syncer.Start(nil); err != nil {
		t.Errorf("noop syncer Start should not return error: %v", err)
	}
	if err := syncer.ReconcileSyncTarget(nil, nil); err != nil {
		t.Errorf("noop syncer ReconcileSyncTarget should not return error: %v", err)
	}
}