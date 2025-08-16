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

package mocks

import (
	"context"
	"sync"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

// MockSyncEngine provides a mock implementation of interfaces.SyncEngine for testing.
type MockSyncEngine struct {
	mu sync.RWMutex

	// Function hooks for testing
	StartFunc                 func(ctx context.Context) error
	StopFunc                  func(ctx context.Context) error
	EnqueueSyncOperationFunc  func(operation interfaces.SyncOperation) error
	ProcessSyncOperationFunc  func(ctx context.Context, operation interfaces.SyncOperation) interfaces.SyncStatus
	GetSyncStatusFunc         func(operationID string) (interfaces.SyncStatus, bool)
	ListPendingOperationsFunc func(workspace logicalcluster.Name, direction interfaces.SyncDirection) []interfaces.SyncOperation
	GetMetricsFunc            func() interfaces.SyncMetrics
	IsHealthyFunc             func() error

	// State tracking for verification
	StartCalled   bool
	StopCalled    bool
	EnqueuedOps   []interfaces.SyncOperation
	ProcessedOps  []interfaces.SyncOperation
	StatusQueries []string
}

// NewMockSyncEngine creates a new mock sync engine with default implementations.
func NewMockSyncEngine() *MockSyncEngine {
	return &MockSyncEngine{
		EnqueuedOps:   make([]interfaces.SyncOperation, 0),
		ProcessedOps:  make([]interfaces.SyncOperation, 0),
		StatusQueries: make([]string, 0),
	}
}

func (m *MockSyncEngine) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartCalled = true
	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}
	return nil
}

func (m *MockSyncEngine) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StopCalled = true
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}
	return nil
}

func (m *MockSyncEngine) EnqueueSyncOperation(operation interfaces.SyncOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EnqueuedOps = append(m.EnqueuedOps, operation)
	if m.EnqueueSyncOperationFunc != nil {
		return m.EnqueueSyncOperationFunc(operation)
	}
	return nil
}

func (m *MockSyncEngine) ProcessSyncOperation(ctx context.Context, operation interfaces.SyncOperation) interfaces.SyncStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ProcessedOps = append(m.ProcessedOps, operation)
	if m.ProcessSyncOperationFunc != nil {
		return m.ProcessSyncOperationFunc(ctx, operation)
	}
	return interfaces.SyncStatus{
		Operation: operation,
		Result:    interfaces.SyncResultSuccess,
		Message:   "mock success",
	}
}

func (m *MockSyncEngine) GetSyncStatus(operationID string) (interfaces.SyncStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StatusQueries = append(m.StatusQueries, operationID)
	if m.GetSyncStatusFunc != nil {
		return m.GetSyncStatusFunc(operationID)
	}
	return interfaces.SyncStatus{}, false
}

func (m *MockSyncEngine) ListPendingOperations(workspace logicalcluster.Name, direction interfaces.SyncDirection) []interfaces.SyncOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ListPendingOperationsFunc != nil {
		return m.ListPendingOperationsFunc(workspace, direction)
	}
	return []interfaces.SyncOperation{}
}

func (m *MockSyncEngine) GetMetrics() interfaces.SyncMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc()
	}
	return interfaces.SyncMetrics{}
}

func (m *MockSyncEngine) IsHealthy() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return nil
}

// Reset clears all recorded state for reuse in tests.
func (m *MockSyncEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartCalled = false
	m.StopCalled = false
	m.EnqueuedOps = make([]interfaces.SyncOperation, 0)
	m.ProcessedOps = make([]interfaces.SyncOperation, 0)
	m.StatusQueries = make([]string, 0)
}
