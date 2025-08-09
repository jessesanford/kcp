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

package workqueue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// mockProcessor implements ProcessorInterface for testing
type mockProcessor struct {
	mu           sync.Mutex
	processedKeys []string
	shouldError   bool
	processDelay  time.Duration
}

func newMockProcessor() *mockProcessor {
	return &mockProcessor{
		processedKeys: make([]string, 0),
	}
}

func (m *mockProcessor) ProcessItem(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.processDelay > 0 {
		time.Sleep(m.processDelay)
	}
	
	m.processedKeys = append(m.processedKeys, key)
	
	if m.shouldError {
		return fmt.Errorf("mock error for key %s", key)
	}
	
	return nil
}

func (m *mockProcessor) getProcessedKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, len(m.processedKeys))
	copy(keys, m.processedKeys)
	return keys
}

func (m *mockProcessor) setError(shouldError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = shouldError
}

func TestNewManager(t *testing.T) {
	tests := map[string]struct {
		config      *WorkerConfig
		expectError bool
		errorMsg    string
	}{
		"valid config": {
			config: &WorkerConfig{
				Name:        "test-manager",
				Workspace:   "root:test",
				WorkerCount: 2,
				Processor:   newMockProcessor(),
			},
			expectError: false,
		},
		"nil config": {
			config:      nil,
			expectError: true,
			errorMsg:    "WorkerConfig cannot be nil",
		},
		"empty name": {
			config: &WorkerConfig{
				Name:        "",
				Workspace:   "root:test",
				WorkerCount: 1,
				Processor:   newMockProcessor(),
			},
			expectError: true,
			errorMsg:    "Name is required",
		},
		"empty workspace": {
			config: &WorkerConfig{
				Name:        "test-manager",
				Workspace:   "",
				WorkerCount: 1,
				Processor:   newMockProcessor(),
			},
			expectError: true,
			errorMsg:    "Workspace cannot be empty",
		},
		"nil processor": {
			config: &WorkerConfig{
				Name:        "test-manager",
				Workspace:   "root:test",
				WorkerCount: 1,
				Processor:   nil,
			},
			expectError: true,
			errorMsg:    "Processor cannot be nil",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			manager, err := NewManager(tc.config)
			
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errorMsg)
				}
				if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Fatalf("expected error containing %q, got %v", tc.errorMsg, err)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if manager == nil {
				t.Fatal("expected non-nil manager")
			}
		})
	}
}

func TestManagerBasicOperations(t *testing.T) {
	processor := newMockProcessor()
	config := &WorkerConfig{
		Name:        "test-basic",
		Workspace:   "root:test",
		WorkerCount: 1,
		Processor:   processor,
	}
	
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	
	// Test adding items before start
	manager.Add("test-key-1")
	manager.Add("test-key-2")
	
	if manager.Len() != 2 {
		t.Errorf("expected queue length 2, got %d", manager.Len())
	}
	
	// Test shutdown before start
	manager.Shutdown()
	
	if !manager.IsShuttingDown() {
		t.Error("expected queue to be shutting down after Shutdown()")
	}
}