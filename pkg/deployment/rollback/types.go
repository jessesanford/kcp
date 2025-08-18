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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RollbackRequest represents a request to rollback a deployment to a previous state.
type RollbackRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RollbackSpec   `json:"spec,omitempty"`
	Status RollbackStatus `json:"status,omitempty"`
}

// RollbackSpec defines the specification for a rollback operation.
type RollbackSpec struct {
	// TargetRef identifies the deployment to rollback
	TargetRef corev1.ObjectReference `json:"targetRef"`

	// RollbackTo specifies which snapshot/version to restore
	RollbackTo RollbackTarget `json:"rollbackTo"`

	// Reason provides the reason for the rollback
	Reason string `json:"reason"`

	// AutoTriggered indicates if this rollback was automatically triggered
	AutoTriggered bool `json:"autoTriggered,omitempty"`

	// DryRun indicates if this is a dry-run rollback (validation only)
	DryRun bool `json:"dryRun,omitempty"`

	// RestoreTraffic specifies if traffic routing should be restored
	RestoreTraffic bool `json:"restoreTraffic,omitempty"`

	// TimeoutSeconds specifies the rollback operation timeout
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// RollbackTarget specifies the target state for rollback.
type RollbackTarget struct {
	// Version is the deployment version to rollback to
	Version string `json:"version,omitempty"`

	// SnapshotID is the specific snapshot identifier
	SnapshotID string `json:"snapshotID"`

	// Timestamp represents when the snapshot was created
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// ConfigHash is the hash of the configuration at snapshot time
	ConfigHash string `json:"configHash,omitempty"`
}

// RollbackStatus represents the current status of a rollback operation.
type RollbackStatus struct {
	// Phase represents the current phase of the rollback
	Phase RollbackPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StartTime represents when the rollback started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime represents when the rollback completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RestoredResources lists the resources that were restored
	RestoredResources []RestoredResource `json:"restoredResources,omitempty"`

	// Message provides human-readable status information
	Message string `json:"message,omitempty"`
}

// RollbackPhase represents the phase of rollback execution.
type RollbackPhase string

const (
	// RollbackPhasePending indicates the rollback is pending
	RollbackPhasePending RollbackPhase = "Pending"
	// RollbackPhaseValidating indicates snapshot validation is in progress
	RollbackPhaseValidating RollbackPhase = "Validating"
	// RollbackPhaseRestoring indicates resources are being restored
	RollbackPhaseRestoring RollbackPhase = "Restoring"
	// RollbackPhaseCompleted indicates the rollback completed successfully
	RollbackPhaseCompleted RollbackPhase = "Completed"
	// RollbackPhaseFailed indicates the rollback failed
	RollbackPhaseFailed RollbackPhase = "Failed"
)

// RestoredResource represents a resource that was restored during rollback.
type RestoredResource struct {
	// Reference to the restored resource
	Reference corev1.ObjectReference `json:"reference"`
	// Status of the restoration
	Status RestoreStatus `json:"status"`
	// Message provides additional details
	Message string `json:"message,omitempty"`
}

// RestoreStatus represents the status of a resource restoration.
type RestoreStatus string

const (
	// RestoreStatusRestored indicates successful restoration
	RestoreStatusRestored RestoreStatus = "Restored"
	// RestoreStatusFailed indicates restoration failed
	RestoreStatusFailed RestoreStatus = "Failed"
	// RestoreStatusSkipped indicates restoration was skipped
	RestoreStatusSkipped RestoreStatus = "Skipped"
)

// DeploymentSnapshot captures the state of a deployment at a specific point in time.
type DeploymentSnapshot struct {
	// ID is a unique identifier for this snapshot
	ID string `json:"id"`

	// Version is the deployment version
	Version string `json:"version"`

	// CreatedAt is when this snapshot was created
	CreatedAt metav1.Time `json:"createdAt"`

	// DeploymentRef identifies the deployment
	DeploymentRef corev1.ObjectReference `json:"deploymentRef"`

	// Resources contains the raw resource definitions
	Resources []runtime.RawExtension `json:"resources"`

	// Configuration contains key-value configuration pairs
	Configuration map[string]string `json:"configuration,omitempty"`

	// Secrets contains encrypted secret references
	Secrets map[string]string `json:"secrets,omitempty"`

	// TrafficConfig contains traffic routing configuration
	TrafficConfig *TrafficConfiguration `json:"trafficConfig,omitempty"`

	// ConfigHash is a hash of the configuration for quick comparison
	ConfigHash string `json:"configHash"`

	// Labels for categorizing snapshots
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations for additional metadata
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TrafficConfiguration captures traffic routing state.
type TrafficConfiguration struct {
	// ServiceSelectors maps service names to their selectors
	ServiceSelectors map[string]map[string]string `json:"serviceSelectors,omitempty"`

	// IngressRules contains ingress routing rules
	IngressRules []IngressRule `json:"ingressRules,omitempty"`

	// WeightDistribution contains traffic weight distribution
	WeightDistribution map[string]int32 `json:"weightDistribution,omitempty"`
}

// IngressRule represents an ingress routing rule.
type IngressRule struct {
	Host string `json:"host"`
	Path string `json:"path"`
	Backend string `json:"backend"`
}

// RollbackTrigger defines conditions for automatic rollback.
type RollbackTrigger struct {
	// Name identifies this trigger
	Name string `json:"name"`

	// Type specifies the trigger type
	Type TriggerType `json:"type"`

	// Conditions define when this trigger fires
	Conditions TriggerConditions `json:"conditions"`

	// Enabled controls if this trigger is active
	Enabled bool `json:"enabled"`

	// CooldownDuration prevents rapid repeated triggers
	CooldownDuration *metav1.Duration `json:"cooldownDuration,omitempty"`
}

// TriggerType specifies the type of rollback trigger.
type TriggerType string

const (
	// TriggerTypeHealthCheck triggers on health check failures
	TriggerTypeHealthCheck TriggerType = "HealthCheck"
	// TriggerTypeErrorRate triggers on high error rates
	TriggerTypeErrorRate TriggerType = "ErrorRate"
	// TriggerTypeTimeout triggers on deployment timeout
	TriggerTypeTimeout TriggerType = "Timeout"
	// TriggerTypeManual requires manual approval
	TriggerTypeManual TriggerType = "Manual"
	// TriggerTypeSLO triggers on SLO violations
	TriggerTypeSLO TriggerType = "SLO"
)

// TriggerConditions define the specific conditions for trigger activation.
type TriggerConditions struct {
	// ErrorRateThreshold for error rate triggers (percentage)
	ErrorRateThreshold *float64 `json:"errorRateThreshold,omitempty"`

	// HealthCheckFailures for health check triggers
	HealthCheckFailures *int32 `json:"healthCheckFailures,omitempty"`

	// TimeoutDuration for timeout triggers
	TimeoutDuration *metav1.Duration `json:"timeoutDuration,omitempty"`

	// SLOThreshold for SLO violation triggers
	SLOThreshold *float64 `json:"sloThreshold,omitempty"`

	// EvaluationWindow specifies the time window for evaluation
	EvaluationWindow *metav1.Duration `json:"evaluationWindow,omitempty"`
}

// RollbackHistory tracks the history of rollback operations.
type RollbackHistory struct {
	// DeploymentRef identifies the deployment
	DeploymentRef corev1.ObjectReference `json:"deploymentRef"`

	// Operations contains the history of rollback operations
	Operations []RollbackOperation `json:"operations"`

	// CreatedAt is when this history was first created
	CreatedAt metav1.Time `json:"createdAt"`

	// LastUpdated is when this history was last modified
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// RollbackOperation represents a single rollback operation in history.
type RollbackOperation struct {
	// ID uniquely identifies this operation
	ID string `json:"id"`

	// Type indicates the type of operation
	Type OperationType `json:"type"`

	// StartTime when the operation started
	StartTime metav1.Time `json:"startTime"`

	// EndTime when the operation completed (if completed)
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// Success indicates if the operation succeeded
	Success bool `json:"success"`

	// FromSnapshot is the snapshot before the operation
	FromSnapshot string `json:"fromSnapshot,omitempty"`

	// ToSnapshot is the snapshot after the operation
	ToSnapshot string `json:"toSnapshot,omitempty"`

	// Reason for the operation
	Reason string `json:"reason"`

	// TriggeredBy indicates who/what triggered this operation
	TriggeredBy string `json:"triggeredBy"`

	// Duration of the operation
	Duration time.Duration `json:"duration"`

	// Error message if the operation failed
	Error string `json:"error,omitempty"`
}

// OperationType specifies the type of rollback operation.
type OperationType string

const (
	// OperationTypeRollback represents a rollback to previous state
	OperationTypeRollback OperationType = "Rollback"
	// OperationTypeSnapshot represents taking a snapshot
	OperationTypeSnapshot OperationType = "Snapshot"
	// OperationTypeRestore represents restoring from snapshot
	OperationTypeRestore OperationType = "Restore"
)

// EngineConfig provides configuration for the rollback engine.
type EngineConfig struct {
	// MaxSnapshots specifies the maximum number of snapshots to retain
	MaxSnapshots int `json:"maxSnapshots,omitempty"`

	// SnapshotRetentionDuration specifies how long to keep snapshots
	SnapshotRetentionDuration *metav1.Duration `json:"snapshotRetentionDuration,omitempty"`

	// DefaultTimeout for rollback operations
	DefaultTimeout *metav1.Duration `json:"defaultTimeout,omitempty"`

	// EnableAutomaticTriggers controls if automatic triggers are enabled
	EnableAutomaticTriggers bool `json:"enableAutomaticTriggers,omitempty"`

	// Triggers defines the configured automatic triggers
	Triggers []RollbackTrigger `json:"triggers,omitempty"`

	// EncryptionKey for encrypting sensitive data in snapshots
	EncryptionKey string `json:"encryptionKey,omitempty"`
}