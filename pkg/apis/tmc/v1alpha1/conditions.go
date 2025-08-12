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
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// Condition types for TMC resource status management
const (
	// ClusterRegistration condition types
	
	// ClusterConnectionCondition indicates whether the cluster is connected and reachable
	ClusterConnectionCondition conditionsv1alpha1.ConditionType = "Connected"
	
	// ClusterRegistrationCondition indicates whether the cluster is properly registered
	ClusterRegistrationCondition conditionsv1alpha1.ConditionType = "Registered"
	
	// HeartbeatCondition indicates whether cluster heartbeats are being received
	HeartbeatCondition conditionsv1alpha1.ConditionType = "HeartbeatHealthy"
	
	// ResourcesAvailableCondition indicates whether the cluster has sufficient resources
	ResourcesAvailableCondition conditionsv1alpha1.ConditionType = "ResourcesAvailable"
	
	// CapabilitiesDetectedCondition indicates whether cluster capabilities have been detected
	CapabilitiesDetectedCondition conditionsv1alpha1.ConditionType = "CapabilitiesDetected"

	// WorkloadPlacement condition types
	
	// PlacementAvailableCondition indicates whether workload placement is available
	PlacementAvailableCondition conditionsv1alpha1.ConditionType = "PlacementAvailable"
	
	// SchedulingCondition indicates whether workload scheduling is functioning
	SchedulingCondition conditionsv1alpha1.ConditionType = "Scheduling"
	
	// SyncedCondition indicates whether placement status is synced with actual state
	SyncedCondition conditionsv1alpha1.ConditionType = "Synced"
	
	// ConflictResolutionCondition indicates the status of placement conflict resolution
	ConflictResolutionCondition conditionsv1alpha1.ConditionType = "ConflictResolution"
)

// Condition reasons for ClusterRegistration
const (
	// Connection-related reasons
	ClusterConnectedReason        = "ClusterConnected"
	ClusterDisconnectedReason     = "ClusterDisconnected"
	ClusterConnectionTimeoutReason = "ConnectionTimeout"
	ClusterConnectionErrorReason   = "ConnectionError"
	
	// Registration-related reasons
	ClusterRegisteredReason         = "ClusterRegistered"
	ClusterRegistrationFailedReason = "RegistrationFailed"
	ClusterUnregisteredReason       = "ClusterUnregistered"
	
	// Heartbeat-related reasons
	HeartbeatHealthyReason    = "HeartbeatHealthy"
	HeartbeatMissedReason     = "HeartbeatMissed"
	HeartbeatStaleReason      = "HeartbeatStale"
	HeartbeatFailedReason     = "HeartbeatFailed"
	
	// Resource-related reasons
	ResourcesAvailableReason     = "ResourcesAvailable"
	InsufficientResourcesReason  = "InsufficientResources"
	ResourcesUnavailableReason   = "ResourcesUnavailable"
	
	// Capabilities-related reasons
	CapabilitiesDetectedReason   = "CapabilitiesDetected"
	CapabilitiesDetectionFailedReason = "CapabilitiesDetectionFailed"
	CapabilitiesStaleReason      = "CapabilitiesStale"
)

// Condition reasons for WorkloadPlacement
const (
	// Placement availability reasons
	PlacementAvailableReason     = "PlacementAvailable"
	PlacementUnavailableReason   = "PlacementUnavailable"
	PlacementMaintenanceReason   = "MaintenanceWindow"
	
	// Scheduling-related reasons
	SchedulingActiveReason       = "SchedulingActive"
	SchedulingFailedReason       = "SchedulingFailed"
	SchedulingDisabledReason     = "SchedulingDisabled"
	NoSuitableClustersReason     = "NoSuitableClusters"
	
	// Sync-related reasons
	SyncedReason                 = "Synced"
	SyncFailedReason             = "SyncFailed"
	SyncInProgressReason         = "SyncInProgress"
	
	// Conflict resolution reasons
	ConflictResolvedReason       = "ConflictResolved"
	ConflictDetectedReason       = "ConflictDetected"
	ConflictResolutionFailedReason = "ConflictResolutionFailed"
	ConflictResolutionInProgressReason = "ConflictResolutionInProgress"
)