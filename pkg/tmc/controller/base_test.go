// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// mockReconciler implements Reconciler for testing
type mockReconciler struct {
	reconcileFunc func(ctx context.Context, key string) error
}

func (m *mockReconciler) Reconcile(ctx context.Context, key string) error {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(ctx, key)
	}
	return nil
}

func TestNewBaseController(t *testing.T) {
	tests := []struct {
		name    string
		config  *BaseControllerConfig
		wantErr bool
	}{
		{
			name:    "nil config should panic",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty workspace should panic",
			config: &BaseControllerConfig{
				Name:      "test",
				Workspace: logicalcluster.Name(""),
				Reconciler: &mockReconciler{},
			},
			wantErr: true,
		},
		{
			name: "nil reconciler should panic",
			config: &BaseControllerConfig{
				Name:       "test",
				Workspace:  logicalcluster.Name("root:test"),
				Reconciler: nil,
			},
			wantErr: true,
		},
		{
			name: "valid config should succeed",
			config: &BaseControllerConfig{
				Name:      "test",
				Workspace: logicalcluster.Name("root:test"),
				Reconciler: &mockReconciler{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantErr {
					t.Errorf("NewBaseController() panic = %v, wantErr %v", r != nil, tt.wantErr)
				}
			}()

			controller := NewBaseController(tt.config)
			if !tt.wantErr && controller == nil {
				t.Error("NewBaseController() returned nil controller")
			}
		})
	}
}

func TestBaseControllerLifecycle(t *testing.T) {
	config := &BaseControllerConfig{
		Name:        "test-controller",
		Workspace:   logicalcluster.Name("root:test"),
		WorkerCount: 1,
		Reconciler:  &mockReconciler{},
	}

	controller := NewBaseController(config)

	// Test initial state
	if controller.Name() != "test-controller" {
		t.Errorf("Expected name 'test-controller', got %s", controller.Name())
	}

	if !controller.IsHealthy() {
		t.Error("Controller should start healthy")
	}

	// Test shutdown before start
	ctx := context.Background()
	if err := controller.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown before start should not error, got %v", err)
	}

	// Start controller in goroutine since it blocks
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startErr := make(chan error, 1)
	go func() {
		startErr <- controller.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Test shutdown
	shutdownCtx := context.Background()
	if err := controller.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Wait for start to complete
	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("Start failed: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Start did not complete in time")
	}
}