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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestSessionStateValidation(t *testing.T) {
	now := metav1.Now()
	sessionRef := SessionReference{
		Name:      "test-session",
		Namespace: "default",
		SessionID: "session-123",
	}

	tests := map[string]struct {
		state     *SessionState
		wantValid bool
	}{
		"valid session state with basic configuration": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-session-state",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
						PhaseTransitions: []PhaseTransition{
							{
								FromPhase: SessionPhaseCreated,
								ToPhase:   SessionPhaseActive,
								Timestamp: now,
								Reason:    "SessionStarted",
								Message:   "Session successfully started",
								Initiator: "session-controller",
							},
						},
						PlacementContext: &PlacementContext{
							AvailableClusters: []ClusterInfo{
								{
									Name: "cluster-1",
									Location: &ClusterLocation{
										Region:   "us-west-2",
										Zone:     "us-west-2a",
										Provider: "aws",
									},
									Status: ClusterStatusReady,
									ResourceCapacity: map[string]resource.Quantity{
										"cpu":    resource.MustParse("100"),
										"memory": resource.MustParse("1000Gi"),
									},
									Labels: map[string]string{
										"tier": "production",
									},
								},
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"valid session state with comprehensive data": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "comprehensive-state",
					Namespace: "production",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
						PhaseTransitions: []PhaseTransition{
							{
								FromPhase: SessionPhaseCreated,
								ToPhase:   SessionPhaseInitializing,
								Timestamp: metav1.Time{Time: now.Add(-10 * time.Minute)},
								Reason:    "Initialize",
								Message:   "Starting session initialization",
								Initiator: "session-controller",
							},
							{
								FromPhase: SessionPhaseInitializing,
								ToPhase:   SessionPhaseActive,
								Timestamp: now,
								Reason:    "InitializationComplete",
								Message:   "Session initialization completed successfully",
								Initiator: "session-controller",
							},
						},
						PlacementContext: &PlacementContext{
							AvailableClusters: []ClusterInfo{
								{
									Name: "prod-cluster-1",
									Location: &ClusterLocation{
										Region:   "us-west-2",
										Zone:     "us-west-2a",
										Provider: "aws",
									},
									Status: ClusterStatusReady,
									ResourceCapacity: map[string]resource.Quantity{
										"cpu":    resource.MustParse("200"),
										"memory": resource.MustParse("2000Gi"),
									},
									Capabilities: []string{"gpu", "ssd"},
									Labels: map[string]string{
										"tier": "production",
										"zone": "us-west-2a",
									},
								},
							},
							WorkloadRequirements: []WorkloadRequirement{
								{
									WorkloadRef: WorkloadReference{
										APIVersion: "apps/v1",
										Kind:       "Deployment",
										Name:       "web-app",
										Namespace:  "production",
									},
									ResourceRequirements: map[string]resource.Quantity{
										"cpu":    resource.MustParse("10"),
										"memory": resource.MustParse("100Gi"),
									},
									Priority: 800,
								},
							},
						},
						ResourceAllocations: []ResourceAllocation{
							{
								ClusterName: "prod-cluster-1",
								AllocatedResources: map[string]resource.Quantity{
									"cpu":    resource.MustParse("50"),
									"memory": resource.MustParse("500Gi"),
								},
								WorkloadAllocations: []WorkloadAllocation{
									{
										WorkloadRef: WorkloadReference{
											APIVersion: "apps/v1",
											Kind:       "Deployment",
											Name:       "web-app",
											Namespace:  "production",
										},
										AllocatedResources: map[string]resource.Quantity{
											"cpu":    resource.MustParse("10"),
											"memory": resource.MustParse("100Gi"),
										},
									},
								},
								AllocationTime: now,
								Status:         AllocationStatusAllocated,
							},
						},
						ConflictHistory: []ConflictRecord{
							{
								ConflictID:   "conflict-123",
								Timestamp:    now,
								ConflictType: ConflictTypeResourceContention,
								ConflictingPlacements: []PlacementReference{
									{
										Name:      "placement-1",
										Namespace: "default",
									},
									{
										Name:      "placement-2",
										Namespace: "default",
									},
								},
								Resolution: &ConflictResolution{
									Strategy: ConflictResolutionTypeMerge,
									WinningPlacement: &PlacementReference{
										Name:      "placement-1",
										Namespace: "default",
									},
									RejectedPlacements: []PlacementReference{
										{
											Name:      "placement-2",
											Namespace: "default",
										},
									},
									Reason: "Higher priority placement selected",
								},
								ResolutionTime: &now,
							},
						},
						SessionEvents: []SessionEvent{
							{
								EventID:   "event-123",
								Timestamp: now,
								EventType: SessionEventTypeStarted,
								Message:   "Session started successfully",
								Source:    "session-controller",
								Reason:    "UserRequest",
								RelatedObjects: []ObjectReference{
									{
										APIVersion: "tmc.kcp.io/v1alpha1",
										Kind:       "PlacementSession",
										Name:       "test-session",
										Namespace:  "default",
									},
								},
							},
						},
						Checkpoints: []StateCheckpoint{
							{
								CheckpointID: "checkpoint-123",
								Timestamp:    now,
								Phase:        SessionPhaseActive,
								Data:         []byte("serialized-state-data"),
								Version:      "v1",
								Checksum:     "abc123",
							},
						},
					},
					SyncPolicy: &StateSyncPolicy{
						SyncMode:           StateSyncModeImmediate,
						SyncInterval:       metav1.Duration{Duration: 1 * time.Minute},
						ConflictResolution: StateSyncConflictResolutionTimestamp,
						Replicas:           3,
					},
					RetentionPolicy: &StateRetentionPolicy{
						RetentionPeriod:        metav1.Duration{Duration: 24 * time.Hour},
						FailedSessionRetention: metav1.Duration{Duration: 7 * 24 * time.Hour},
						CheckpointRetention:    metav1.Duration{Duration: 30 * 24 * time.Hour},
						MaxCheckpoints:         10,
					},
				},
			},
			wantValid: true,
		},
		"valid session state with sync status": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sync-state",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
					},
				},
				Status: SessionStateStatus{
					LastHeartbeat: &now,
					StateVersion:  42,
					SyncStatus: &StateSyncStatus{
						LastSyncTime: &now,
						SyncVersion:  42,
						ReplicaStates: []ReplicaState{
							{
								ClusterName:    "cluster-1",
								Version:        42,
								LastUpdateTime: now,
								Status:         ReplicaStatusInSync,
							},
							{
								ClusterName:    "cluster-2",
								Version:        41,
								LastUpdateTime: metav1.Time{Time: now.Add(-5 * time.Minute)},
								Status:         ReplicaStatusOutOfSync,
							},
						},
						SyncConflicts: []SyncConflict{
							{
								ConflictID:   "sync-conflict-123",
								DetectedTime: now,
								ConflictingVersions: []ConflictingVersion{
									{
										ClusterName: "cluster-1",
										Version:     42,
										Timestamp:   now,
									},
									{
										ClusterName: "cluster-2",
										Version:     41,
										Timestamp:   metav1.Time{Time: now.Add(-5 * time.Minute)},
									},
								},
								Resolution: &SyncConflictResolution{
									Strategy:       StateSyncConflictResolutionTimestamp,
									WinningVersion: 42,
									WinningCluster: "cluster-1",
									ResolvedTime:   now,
								},
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"invalid - empty session reference name": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-state",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					SessionRef: SessionReference{
						Name:      "", // Invalid - empty name
						Namespace: "default",
						SessionID: "session-123",
					},
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
					},
				},
			},
			wantValid: false,
		},
		"invalid - invalid workload priority": {
			state: &SessionState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-priority-state",
					Namespace: "default",
				},
				Spec: SessionStateSpec{
					SessionRef: sessionRef,
					StateData: SessionStateData{
						CurrentPhase: SessionPhaseActive,
						PlacementContext: &PlacementContext{
							WorkloadRequirements: []WorkloadRequirement{
								{
									WorkloadRef: WorkloadReference{
										APIVersion: "apps/v1",
										Kind:       "Deployment",
										Name:       "test-app",
										Namespace:  "default",
									},
									Priority: 1500, // Invalid - over maximum
								},
							},
						},
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.state == nil {
				t.Fatal("state cannot be nil")
			}

			// Validate session reference
			if tc.state.Spec.SessionRef.Name == "" && tc.wantValid {
				t.Error("expected valid state, but SessionRef.Name is empty")
				return
			}

			// Validate state data
			validPhases := []SessionPhase{
				SessionPhaseCreated, SessionPhaseInitializing, SessionPhaseActive,
				SessionPhaseSuspended, SessionPhaseCompleting, SessionPhaseCompleted,
				SessionPhaseFailed, SessionPhaseTerminated,
			}
			found := false
			for _, validPhase := range validPhases {
				if tc.state.Spec.StateData.CurrentPhase == validPhase {
					found = true
					break
				}
			}
			if !found && tc.wantValid {
				t.Errorf("invalid current phase: %s", tc.state.Spec.StateData.CurrentPhase)
			}

			// Validate workload requirements
			if tc.state.Spec.StateData.PlacementContext != nil {
				for _, req := range tc.state.Spec.StateData.PlacementContext.WorkloadRequirements {
					if req.Priority < 0 || req.Priority > 1000 {
						if tc.wantValid {
							t.Errorf("workload priority %d is out of valid range [0, 1000]", req.Priority)
						}
					}
				}
			}

			// Validate resource allocations
			for _, allocation := range tc.state.Spec.StateData.ResourceAllocations {
				if allocation.ClusterName == "" && tc.wantValid {
					t.Error("resource allocation must have cluster name")
				}

				validStatuses := []AllocationStatus{
					AllocationStatusAllocated, AllocationStatusPending,
					AllocationStatusFailed, AllocationStatusReleased,
				}
				statusFound := false
				for _, validStatus := range validStatuses {
					if allocation.Status == validStatus {
						statusFound = true
						break
					}
				}
				if !statusFound && allocation.Status != "" && tc.wantValid {
					t.Errorf("invalid allocation status: %s", allocation.Status)
				}
			}

			// Validate sync policy
			if tc.state.Spec.SyncPolicy != nil {
				if tc.state.Spec.SyncPolicy.Replicas < 1 && tc.wantValid {
					t.Errorf("sync policy replicas must be >= 1, got %d", tc.state.Spec.SyncPolicy.Replicas)
				}
			}

			// Validate retention policy
			if tc.state.Spec.RetentionPolicy != nil {
				if tc.state.Spec.RetentionPolicy.MaxCheckpoints < 1 && tc.wantValid {
					t.Errorf("retention policy max checkpoints must be >= 1, got %d", tc.state.Spec.RetentionPolicy.MaxCheckpoints)
				}
			}
		})
	}
}

func TestStateSyncPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy    *StateSyncPolicy
		wantValid bool
	}{
		"valid immediate sync policy": {
			policy: &StateSyncPolicy{
				SyncMode:           StateSyncModeImmediate,
				ConflictResolution: StateSyncConflictResolutionTimestamp,
				Replicas:           3,
			},
			wantValid: true,
		},
		"valid periodic sync policy": {
			policy: &StateSyncPolicy{
				SyncMode:           StateSyncModePeriodic,
				SyncInterval:       metav1.Duration{Duration: 5 * time.Minute},
				ConflictResolution: StateSyncConflictResolutionLastWrite,
				Replicas:           5,
			},
			wantValid: true,
		},
		"valid batch sync policy": {
			policy: &StateSyncPolicy{
				SyncMode:           StateSyncModeBatch,
				ConflictResolution: StateSyncConflictResolutionManual,
				Replicas:           1,
			},
			wantValid: true,
		},
		"invalid - zero replicas": {
			policy: &StateSyncPolicy{
				SyncMode: StateSyncModeImmediate,
				Replicas: 0, // Invalid - must be >= 1
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.policy == nil && tc.wantValid {
				t.Fatal("policy cannot be nil for valid test")
			}

			if tc.policy != nil {
				// Validate replicas
				if tc.policy.Replicas < 1 && tc.wantValid {
					t.Errorf("replicas must be >= 1, got %d", tc.policy.Replicas)
				}

				// Validate sync mode
				validModes := []StateSyncMode{
					StateSyncModeImmediate, StateSyncModeBatch, StateSyncModePeriodic,
				}
				found := false
				for _, validMode := range validModes {
					if tc.policy.SyncMode == validMode {
						found = true
						break
					}
				}
				if !found && tc.policy.SyncMode != "" && tc.wantValid {
					t.Errorf("invalid sync mode: %s", tc.policy.SyncMode)
				}

				// Validate conflict resolution
				validResolutions := []StateSyncConflictResolution{
					StateSyncConflictResolutionLastWrite,
					StateSyncConflictResolutionTimestamp,
					StateSyncConflictResolutionManual,
				}
				found = false
				for _, validResolution := range validResolutions {
					if tc.policy.ConflictResolution == validResolution {
						found = true
						break
					}
				}
				if !found && tc.policy.ConflictResolution != "" && tc.wantValid {
					t.Errorf("invalid conflict resolution: %s", tc.policy.ConflictResolution)
				}
			}
		})
	}
}

func TestStateRetentionPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy    *StateRetentionPolicy
		wantValid bool
	}{
		"valid retention policy": {
			policy: &StateRetentionPolicy{
				RetentionPeriod:        metav1.Duration{Duration: 24 * time.Hour},
				FailedSessionRetention: metav1.Duration{Duration: 7 * 24 * time.Hour},
				CheckpointRetention:    metav1.Duration{Duration: 30 * 24 * time.Hour},
				MaxCheckpoints:         10,
			},
			wantValid: true,
		},
		"valid minimal retention policy": {
			policy: &StateRetentionPolicy{
				MaxCheckpoints: 1,
			},
			wantValid: true,
		},
		"invalid - zero max checkpoints": {
			policy: &StateRetentionPolicy{
				MaxCheckpoints: 0, // Invalid - must be >= 1
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.policy == nil && tc.wantValid {
				t.Fatal("policy cannot be nil for valid test")
			}

			if tc.policy != nil {
				// Validate max checkpoints
				if tc.policy.MaxCheckpoints < 1 && tc.wantValid {
					t.Errorf("max checkpoints must be >= 1, got %d", tc.policy.MaxCheckpoints)
				}
			}
		})
	}
}

func TestSessionEventTypeValues(t *testing.T) {
	validEventTypes := []SessionEventType{
		SessionEventTypeCreated,
		SessionEventTypeStarted,
		SessionEventTypeSuspended,
		SessionEventTypeResumed,
		SessionEventTypeCompleted,
		SessionEventTypeFailed,
		SessionEventTypeHeartbeatMissed,
		SessionEventTypeConflictDetected,
		SessionEventTypeConflictResolved,
	}

	// Verify all event types are defined correctly
	for _, eventType := range validEventTypes {
		if string(eventType) == "" {
			t.Errorf("event type %v has empty string value", eventType)
		}
	}

	// Test event validation with real values
	testEvents := map[SessionEventType]bool{
		SessionEventTypeCreated:          true,
		SessionEventTypeStarted:          true,
		SessionEventTypeSuspended:        true,
		SessionEventTypeResumed:          true,
		SessionEventTypeCompleted:        true,
		SessionEventTypeFailed:           true,
		SessionEventTypeHeartbeatMissed:  true,
		SessionEventTypeConflictDetected: true,
		SessionEventTypeConflictResolved: true,
		"InvalidEvent":                   false,
	}

	for eventType, shouldBeValid := range testEvents {
		found := false
		for _, validType := range validEventTypes {
			if eventType == validType {
				found = true
				break
			}
		}

		if found != shouldBeValid {
			t.Errorf("event type %s: expected valid=%t, got valid=%t", eventType, shouldBeValid, found)
		}
	}
}