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

package rollback

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewEngine(t *testing.T) {
	tests := map[string]struct {
		config      *EngineConfig
		wantErr     bool
		wantEnabled bool
	}{
		"with nil config": {
			config:      nil,
			wantErr:     false,
			wantEnabled: false,
		},
		"with valid config": {
			config: &EngineConfig{
				MaxSnapshots:            5,
				EnableAutomaticTriggers: true,
			},
			wantErr:     false,
			wantEnabled: true,
		},
		"with disabled triggers": {
			config: &EngineConfig{
				MaxSnapshots:            10,
				EnableAutomaticTriggers: false,
			},
			wantErr:     false,
			wantEnabled: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClient(runtime.NewScheme())
			cluster := logicalcluster.Name("test")

			engine, err := NewEngine(client, cluster, tc.config)

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if engine != nil {
				if tc.config == nil {
					if engine.config.MaxSnapshots != 10 {
						t.Errorf("expected default MaxSnapshots 10, got %d", engine.config.MaxSnapshots)
					}
					if engine.config.EnableAutomaticTriggers != false {
						t.Error("expected default EnableAutomaticTriggers false")
					}
				} else {
					if engine.config.EnableAutomaticTriggers != tc.wantEnabled {
						t.Errorf("expected EnableAutomaticTriggers %v, got %v", tc.wantEnabled, engine.config.EnableAutomaticTriggers)
					}
				}
			}
		})
	}
}

func TestEngine_ValidateRollbackRequest(t *testing.T) {
	engine := createTestEngine(t)

	tests := map[string]struct {
		request *RollbackRequest
		wantErr bool
		errMsg  string
	}{
		"valid request": {
			request: &RollbackRequest{
				Spec: RollbackSpec{
					TargetRef: corev1.ObjectReference{
						Name:      "test-deployment",
						Namespace: "default",
					},
					RollbackTo: RollbackTarget{
						SnapshotID: "snap-123",
					},
				},
			},
			wantErr: false,
		},
		"empty deployment name": {
			request: &RollbackRequest{
				Spec: RollbackSpec{
					TargetRef: corev1.ObjectReference{
						Namespace: "default",
					},
					RollbackTo: RollbackTarget{
						SnapshotID: "snap-123",
					},
				},
			},
			wantErr: true,
			errMsg:  "target deployment name cannot be empty",
		},
		"empty snapshot ID": {
			request: &RollbackRequest{
				Spec: RollbackSpec{
					TargetRef: corev1.ObjectReference{
						Name:      "test-deployment",
						Namespace: "default",
					},
					RollbackTo: RollbackTarget{},
				},
			},
			wantErr: true,
			errMsg:  "snapshot ID cannot be empty",
		},
		"invalid timeout": {
			request: &RollbackRequest{
				Spec: RollbackSpec{
					TargetRef: corev1.ObjectReference{
						Name:      "test-deployment",
						Namespace: "default",
					},
					RollbackTo: RollbackTarget{
						SnapshotID: "snap-123",
					},
					TimeoutSeconds: int32Ptr(-1),
				},
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := engine.validateRollbackRequest(tc.request)

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tc.wantErr && err != nil && err.Error() != tc.errMsg {
				t.Errorf("expected error message '%s', got '%s'", tc.errMsg, err.Error())
			}
		})
	}
}

func TestEngine_HasActiveRollback(t *testing.T) {
	engine := createTestEngine(t)

	deploymentRef := corev1.ObjectReference{
		Name:      "test-deployment",
		Namespace: "default",
	}

	// Initially no active rollback
	if engine.hasActiveRollback(deploymentRef) {
		t.Error("expected no active rollback initially")
	}

	// Add active rollback
	execution := &RollbackExecution{
		Request: &RollbackRequest{
			Spec: RollbackSpec{
				TargetRef: deploymentRef,
			},
		},
	}
	engine.activeRollbacks["test-op"] = execution

	// Should now have active rollback
	if !engine.hasActiveRollback(deploymentRef) {
		t.Error("expected active rollback to be found")
	}

	// Different deployment should not have active rollback
	otherRef := corev1.ObjectReference{
		Name:      "other-deployment",
		Namespace: "default",
	}
	if engine.hasActiveRollback(otherRef) {
		t.Error("expected no active rollback for different deployment")
	}
}

func TestEngine_InitializeExecution(t *testing.T) {
	engine := createTestEngine(t)
	ctx := context.Background()

	request := &RollbackRequest{
		Spec: RollbackSpec{
			TargetRef: corev1.ObjectReference{
				Name:      "test-deployment",
				Namespace: "default",
			},
			RollbackTo: RollbackTarget{
				SnapshotID: "snap-123",
			},
		},
	}

	execution := engine.initializeExecution(ctx, request)

	if execution.Request != request {
		t.Error("execution should reference the original request")
	}

	if execution.OperationID == "" {
		t.Error("execution should have an operation ID")
	}

	if execution.Phase != RollbackPhasePending {
		t.Errorf("expected phase %s, got %s", RollbackPhasePending, execution.Phase)
	}

	if execution.Context == nil {
		t.Error("execution should have a context")
	}

	if execution.Cancel == nil {
		t.Error("execution should have a cancel function")
	}
}

func TestEngine_UpdateExecutionStatus(t *testing.T) {
	engine := createTestEngine(t)

	execution := &RollbackExecution{
		Request: &RollbackRequest{
			Status: RollbackStatus{},
		},
	}

	phase := RollbackPhaseRestoring
	message := "Restoring resources"

	engine.updateExecutionStatus(execution, phase, message)

	if execution.Phase != phase {
		t.Errorf("expected phase %s, got %s", phase, execution.Phase)
	}

	if execution.Request.Status.Phase != phase {
		t.Errorf("expected status phase %s, got %s", phase, execution.Request.Status.Phase)
	}

	if execution.Request.Status.Message != message {
		t.Errorf("expected message '%s', got '%s'", message, execution.Request.Status.Message)
	}

	// Test completion
	engine.updateExecutionStatus(execution, RollbackPhaseCompleted, "Done")

	if execution.Request.Status.CompletionTime == nil {
		t.Error("expected completion time to be set")
	}
}

// Benchmark tests

func BenchmarkEngine_ValidateRollbackRequest(b *testing.B) {
	engine := createTestEngine(b)
	
	request := &RollbackRequest{
		Spec: RollbackSpec{
			TargetRef: corev1.ObjectReference{
				Name:      "test-deployment",
				Namespace: "default",
			},
			RollbackTo: RollbackTarget{
				SnapshotID: "snap-123",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.validateRollbackRequest(request)
	}
}

func BenchmarkEngine_HasActiveRollback(b *testing.B) {
	engine := createTestEngine(b)

	deploymentRef := corev1.ObjectReference{
		Name:      "test-deployment",
		Namespace: "default",
	}

	// Add some active rollbacks
	for i := 0; i < 100; i++ {
		execution := &RollbackExecution{
			Request: &RollbackRequest{
				Spec: RollbackSpec{
					TargetRef: corev1.ObjectReference{
						Name:      "deployment-" + string(rune(i)),
						Namespace: "default",
					},
				},
			},
		}
		engine.activeRollbacks[string(rune(i))] = execution
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.hasActiveRollback(deploymentRef)
	}
}

// Helper functions

func createTestEngine(tb testing.TB) *Engine {
	tb.Helper()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	cluster := logicalcluster.Name("test")
	config := &EngineConfig{
		MaxSnapshots:            5,
		EnableAutomaticTriggers: false,
	}

	engine, err := NewEngine(client, cluster, config)
	if err != nil {
		tb.Fatalf("failed to create test engine: %v", err)
	}

	return engine
}

func int32Ptr(i int32) *int32 {
	return &i
}

func TestRollbackPhases(t *testing.T) {
	phases := []RollbackPhase{
		RollbackPhasePending,
		RollbackPhaseValidating,
		RollbackPhaseRestoring,
		RollbackPhaseCompleted,
		RollbackPhaseFailed,
	}

	for _, phase := range phases {
		if string(phase) == "" {
			t.Errorf("phase %v should not be empty string", phase)
		}
	}
}

func TestRestoreStatus(t *testing.T) {
	statuses := []RestoreStatus{
		RestoreStatusRestored,
		RestoreStatusFailed,
		RestoreStatusSkipped,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("status %v should not be empty string", status)
		}
	}
}

func TestTriggerTypes(t *testing.T) {
	types := []TriggerType{
		TriggerTypeHealthCheck,
		TriggerTypeErrorRate,
		TriggerTypeTimeout,
		TriggerTypeManual,
		TriggerTypeSLO,
	}

	for _, triggerType := range types {
		if string(triggerType) == "" {
			t.Errorf("trigger type %v should not be empty string", triggerType)
		}
	}
}

func TestOperationTypes(t *testing.T) {
	types := []OperationType{
		OperationTypeRollback,
		OperationTypeSnapshot,
		OperationTypeRestore,
	}

	for _, opType := range types {
		if string(opType) == "" {
			t.Errorf("operation type %v should not be empty string", opType)
		}
	}
}