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

package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
)

// mockController implements the Controller interface for testing
type mockController struct {
	name    string
	started bool
	ready   bool
}

func (m *mockController) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *mockController) Stop() error {
	m.started = false
	return nil
}

func (m *mockController) Name() string {
	return m.name
}

func (m *mockController) Ready() bool {
	return m.ready
}

func TestControllerInterface(t *testing.T) {
	// Verify that mockController implements the Controller interface
	var _ Controller = &mockController{}

	ctrl := &mockController{
		name:  "test-controller",
		ready: true,
	}

	if ctrl.Name() != "test-controller" {
		t.Errorf("expected name 'test-controller', got %s", ctrl.Name())
	}

	if !ctrl.Ready() {
		t.Error("expected controller to be ready")
	}

	ctx := context.Background()
	if err := ctrl.Start(ctx); err != nil {
		t.Errorf("unexpected error starting controller: %v", err)
	}

	if !ctrl.started {
		t.Error("expected controller to be marked as started")
	}

	if err := ctrl.Stop(); err != nil {
		t.Errorf("unexpected error stopping controller: %v", err)
	}

	if ctrl.started {
		t.Error("expected controller to be marked as stopped")
	}
}

// mockReconciler implements the Reconciler interface for testing
type mockReconciler struct {
	logger logr.Logger
}

func (m *mockReconciler) Reconcile(ctx context.Context, key string) error {
	return nil
}

func (m *mockReconciler) SetupWithManager(mgr Manager) error {
	return nil
}

func (m *mockReconciler) GetLogger() logr.Logger {
	return m.logger
}

func TestReconcilerInterface(t *testing.T) {
	// Verify that mockReconciler implements the Reconciler interface
	var _ Reconciler = &mockReconciler{}

	reconciler := &mockReconciler{
		logger: logr.Discard(),
	}

	ctx := context.Background()
	if err := reconciler.Reconcile(ctx, "test/resource"); err != nil {
		t.Errorf("unexpected error reconciling: %v", err)
	}

	logger := reconciler.GetLogger()
	// logr.Discard() returns a valid logger, just check it's not nil interface
	if &logger == nil {
		t.Error("expected valid logger")
	}
}

// mockWorkQueue implements the WorkQueue interface for testing
type mockWorkQueue struct {
	items       []interface{}
	shuttingDown bool
}

func (m *mockWorkQueue) Add(item interface{}) {
	if !m.shuttingDown {
		m.items = append(m.items, item)
	}
}

func (m *mockWorkQueue) Get() (interface{}, bool) {
	if m.shuttingDown {
		return nil, false
	}
	if len(m.items) == 0 {
		return nil, false // No items available, but not shutting down
	}
	item := m.items[0]
	m.items = m.items[1:]
	return item, true
}

func (m *mockWorkQueue) Done(item interface{}) {
	// Mock implementation - nothing to do
}

func (m *mockWorkQueue) Forget(item interface{}) {
	// Mock implementation - nothing to do
}

func (m *mockWorkQueue) Len() int {
	return len(m.items)
}

func (m *mockWorkQueue) ShuttingDown() bool {
	return m.shuttingDown
}

func (m *mockWorkQueue) ShutDown() {
	m.shuttingDown = true
}

func TestWorkQueueInterface(t *testing.T) {
	// Verify that mockWorkQueue implements the WorkQueue interface
	var _ WorkQueue = &mockWorkQueue{}

	queue := &mockWorkQueue{}

	// Test adding and getting items
	queue.Add("test-item")
	if queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", queue.Len())
	}

	item, ok := queue.Get()
	if !ok {
		t.Error("expected queue to be operational")
	}
	if item != "test-item" {
		t.Errorf("expected 'test-item', got %v", item)
	}

	// Test shutdown
	queue.ShutDown()
	if !queue.ShuttingDown() {
		t.Error("expected queue to be shutting down")
	}

	_, ok = queue.Get()
	if ok {
		t.Error("expected queue to be not operational after ShutDown")
	}
}

// mockManager implements the Manager interface for testing
type mockManager struct {
	controllers []Controller
}

func (m *mockManager) AddController(ctrl Controller) error {
	m.controllers = append(m.controllers, ctrl)
	return nil
}

func (m *mockManager) GetClient() dynamic.Interface {
	return nil // Mock implementation
}

func (m *mockManager) GetScheme() *runtime.Scheme {
	return runtime.NewScheme()
}

func (m *mockManager) GetEventRecorder() record.EventRecorder {
	return nil // Mock implementation
}

func (m *mockManager) GetLogger() logr.Logger {
	return logr.Discard()
}

func (m *mockManager) Start(ctx context.Context) error {
	return nil
}

func (m *mockManager) GetControllers() []Controller {
	return m.controllers
}

func TestManagerInterface(t *testing.T) {
	// Verify that mockManager implements the Manager interface
	var _ Manager = &mockManager{}

	manager := &mockManager{}

	ctrl := &mockController{name: "test"}
	if err := manager.AddController(ctrl); err != nil {
		t.Errorf("unexpected error adding controller: %v", err)
	}

	controllers := manager.GetControllers()
	if len(controllers) != 1 {
		t.Errorf("expected 1 controller, got %d", len(controllers))
	}

	if controllers[0] != ctrl {
		t.Error("expected same controller instance")
	}
}