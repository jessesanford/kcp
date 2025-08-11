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

package physical

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer"
)

func TestNewPhysicalSyncer(t *testing.T) {
	tests := map[string]struct {
		cluster       *syncer.ClusterRegistration
		clusterConfig *rest.Config
		options       *SyncerOptions
		wantError     bool
	}{
		"valid configuration": {
			cluster:       &syncer.ClusterRegistration{},
			clusterConfig: &rest.Config{Host: "https://example.com"},
			options:       DefaultSyncerOptions(),
			wantError:     false,
		},
		"nil cluster": {
			cluster:       nil,
			clusterConfig: &rest.Config{Host: "https://example.com"},
			options:       DefaultSyncerOptions(),
			wantError:     true,
		},
		"nil config": {
			cluster:       &syncer.ClusterRegistration{},
			clusterConfig: nil,
			options:       DefaultSyncerOptions(),
			wantError:     true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewPhysicalSyncer(tc.cluster, tc.clusterConfig, tc.options)
			
			if tc.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultSyncerOptions(t *testing.T) {
	opts := DefaultSyncerOptions()
	
	if opts.ResyncPeriod != 30*time.Second {
		t.Errorf("expected ResyncPeriod 30s, got %v", opts.ResyncPeriod)
	}
	
	if opts.SyncTimeout != 5*time.Minute {
		t.Errorf("expected SyncTimeout 5m, got %v", opts.SyncTimeout)
	}
	
	if opts.RetryStrategy == nil {
		t.Error("expected RetryStrategy to be set")
	}
}

func TestWorkloadStatusPhases(t *testing.T) {
	phases := []syncer.WorkloadPhase{
		syncer.WorkloadPhasePending,
		syncer.WorkloadPhaseDeploying,
		syncer.WorkloadPhaseReady,
		syncer.WorkloadPhaseDegraded,
		syncer.WorkloadPhaseFailed,
		syncer.WorkloadPhaseTerminating,
		syncer.WorkloadPhaseUnknown,
	}
	
	expectedPhases := []syncer.WorkloadPhase{
		"Pending",
		"Deploying", 
		"Ready",
		"Degraded",
		"Failed",
		"Terminating",
		"Unknown",
	}
	
	for i, phase := range phases {
		if phase != expectedPhases[i] {
			t.Errorf("phase %d: expected %s, got %s", i, expectedPhases[i], phase)
		}
	}
}

func TestSyncEventTypes(t *testing.T) {
	eventTypes := []syncer.SyncEventType{
		syncer.SyncEventStarted,
		syncer.SyncEventCompleted,
		syncer.SyncEventFailed,
		syncer.SyncEventSkipped,
	}
	
	expectedTypes := []syncer.SyncEventType{
		"SyncStarted",
		"SyncCompleted",
		"SyncFailed", 
		"SyncSkipped",
	}
	
	for i, eventType := range eventTypes {
		if eventType != expectedTypes[i] {
			t.Errorf("event type %d: expected %s, got %s", i, expectedTypes[i], eventType)
		}
	}
}

func TestWorkspaceAwareNaming(t *testing.T) {
	tests := map[string]struct {
		logicalCluster logicalcluster.Name
		originalName   string
		expectedPrefix string
		expectQualified bool
	}{
		"simple workspace": {
			logicalCluster: logicalcluster.Name("root:workspace-a"),
			originalName:   "my-deployment",
			expectedPrefix: "root-workspace-a",
			expectQualified: true,
		},
		"complex workspace": {
			logicalCluster: logicalcluster.Name("root:org:team:workspace"),
			originalName:   "my-service",
			expectedPrefix: "root-org-team-workspace",
			expectQualified: true,
		},
		"empty workspace": {
			logicalCluster: logicalcluster.Name(""),
			originalName:   "my-pod",
			expectedPrefix: "",
			expectQualified: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			naming := syncer.NewWorkspaceAwareNaming(tc.logicalCluster)
			
			qualifiedName := naming.QualifyName(tc.originalName)
			
			if tc.expectQualified {
				expectedQualifiedName := tc.expectedPrefix + "--" + tc.originalName
				if qualifiedName != expectedQualifiedName {
					t.Errorf("expected qualified name %s, got %s", expectedQualifiedName, qualifiedName)
				}
				
				// Test extraction
				extractedName := naming.ExtractOriginalName(qualifiedName)
				if extractedName != tc.originalName {
					t.Errorf("expected extracted name %s, got %s", tc.originalName, extractedName)
				}
				
				// Test ownership check
				if !naming.IsWorkspaceResource(qualifiedName) {
					t.Error("qualified name should belong to workspace")
				}
			} else {
				if qualifiedName != tc.originalName {
					t.Errorf("expected unmodified name %s, got %s", tc.originalName, qualifiedName)
				}
			}
		})
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	workspaceA := logicalcluster.Name("root:workspace-a")
	workspaceB := logicalcluster.Name("root:workspace-b")
	
	namingA := syncer.NewWorkspaceAwareNaming(workspaceA)
	namingB := syncer.NewWorkspaceAwareNaming(workspaceB)
	
	resourceName := "test-deployment"
	
	qualifiedA := namingA.QualifyName(resourceName)
	qualifiedB := namingB.QualifyName(resourceName)
	
	// Qualified names should be different for different workspaces
	if qualifiedA == qualifiedB {
		t.Error("workspace qualified names should be different for different workspaces")
	}
	
	// Each resource should only be recognized by its own workspace
	if namingA.IsWorkspaceResource(qualifiedB) {
		t.Error("workspace A should not recognize workspace B's resource")
	}
	
	if namingB.IsWorkspaceResource(qualifiedA) {
		t.Error("workspace B should not recognize workspace A's resource")
	}
	
	// Each workspace should recognize its own resources
	if !namingA.IsWorkspaceResource(qualifiedA) {
		t.Error("workspace A should recognize its own resource")
	}
	
	if !namingB.IsWorkspaceResource(qualifiedB) {
		t.Error("workspace B should recognize its own resource")
	}
}

// Mock event handler for testing
type mockEventHandler struct {
	events []syncer.SyncEvent
}

func (m *mockEventHandler) HandleEvent(ctx context.Context, event *syncer.SyncEvent) error {
	m.events = append(m.events, *event)
	return nil
}

func TestSyncerOptionsWithEventHandler(t *testing.T) {
	handler := &mockEventHandler{}
	
	opts := &SyncerOptions{
		ResyncPeriod:   time.Minute,
		EventHandler:   handler,
		SyncTimeout:    time.Minute,
		LogicalCluster: logicalcluster.Name("root:test-workspace"),
	}
	
	if opts.EventHandler == nil {
		t.Error("event handler should be set")
	}
	
	// Test that the handler can receive events
	testEvent := &syncer.SyncEvent{
		Type:           syncer.SyncEventStarted,
		Cluster:        "test",
		LogicalCluster: opts.LogicalCluster,
	}
	
	err := opts.EventHandler.HandleEvent(context.Background(), testEvent)
	if err != nil {
		t.Errorf("unexpected error handling event: %v", err)
	}
	
	if len(handler.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(handler.events))
	}
	
	if handler.events[0].LogicalCluster != opts.LogicalCluster {
		t.Errorf("expected event logical cluster %s, got %s", opts.LogicalCluster, handler.events[0].LogicalCluster)
	}
}