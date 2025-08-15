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

package workspace

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceLifecycleManager defines the interface for managing workspace lifecycle operations.
// This interface complements WorkspaceProvider by focusing on advanced lifecycle scenarios
// including workspace migration, backup/restore, and complex initialization patterns.
//
// Key Responsibilities:
// - Workspace initialization and setup orchestration
// - Migration between logical clusters or configurations
// - Backup and restore operations
// - Graceful shutdown and cleanup procedures
// - Lifecycle policy enforcement
//
// Thread Safety:
// All methods must be safe for concurrent use across multiple goroutines.
// Long-running operations should support context cancellation.
//
// Error Handling:
// Methods should return structured errors with sufficient context for
// troubleshooting and recovery procedures.
type WorkspaceLifecycleManager interface {
	// InitializeWorkspace performs comprehensive setup of a newly created workspace.
	// This includes resource provisioning, policy application, and readiness verification.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - ref: Workspace to initialize
	//   - config: Initialization configuration and policies
	//
	// Returns:
	//   - *WorkspaceInitializationResult: Details about the initialization process
	//   - error: Issues during workspace initialization
	//
	// Behavior:
	//   - Applies default resource quotas and policies
	//   - Sets up workspace-specific RBAC rules
	//   - Initializes required system resources
	//   - Verifies workspace readiness before completion
	InitializeWorkspace(ctx context.Context, ref WorkspaceReference, config WorkspaceInitializationConfig) (*WorkspaceInitializationResult, error)

	// MigrateWorkspace moves a workspace between logical clusters or updates its configuration.
	// This is a complex operation that may involve data movement and temporary unavailability.
	//
	// Parameters:
	//   - ctx: Context for the long-running migration operation
	//   - source: Current workspace location and configuration
	//   - target: Desired workspace location and configuration
	//   - options: Migration strategy and safety options
	//
	// Returns:
	//   - *WorkspaceMigrationResult: Details about the migration process
	//   - error: Issues during workspace migration
	//
	// Behavior:
	//   - Validates migration feasibility before starting
	//   - Handles data consistency during the transition
	//   - Supports rollback on failure
	//   - Maintains audit trail of migration steps
	MigrateWorkspace(ctx context.Context, source WorkspaceReference, target WorkspaceReference, options WorkspaceMigrationOptions) (*WorkspaceMigrationResult, error)

	// BackupWorkspace creates a point-in-time snapshot of workspace state and data.
	// Backups can be used for disaster recovery or workspace cloning scenarios.
	//
	// Parameters:
	//   - ctx: Context for the backup operation
	//   - ref: Workspace to backup
	//   - config: Backup configuration and storage options
	//
	// Returns:
	//   - *WorkspaceBackupResult: Information about the created backup
	//   - error: Issues during backup creation
	//
	// Behavior:
	//   - Creates consistent snapshot of workspace resources
	//   - Includes metadata, configuration, and user data
	//   - Supports incremental backups where possible
	//   - Validates backup integrity before completion
	BackupWorkspace(ctx context.Context, ref WorkspaceReference, config WorkspaceBackupConfig) (*WorkspaceBackupResult, error)

	// RestoreWorkspace recreates a workspace from a previously created backup.
	// The restored workspace may have a different name or location than the original.
	//
	// Parameters:
	//   - ctx: Context for the restore operation
	//   - backupRef: Reference to the backup to restore from
	//   - targetRef: Desired location and configuration for restored workspace
	//   - options: Restore strategy and conflict resolution
	//
	// Returns:
	//   - *WorkspaceRestoreResult: Details about the restored workspace
	//   - error: Issues during workspace restoration
	//
	// Behavior:
	//   - Validates backup integrity before restoration
	//   - Creates new workspace with restored configuration
	//   - Handles naming conflicts and resource updates
	//   - Verifies restore completeness and correctness
	RestoreWorkspace(ctx context.Context, backupRef WorkspaceBackupReference, targetRef WorkspaceReference, options WorkspaceRestoreOptions) (*WorkspaceRestoreResult, error)

	// FinalizeWorkspaceDeletion performs cleanup operations after workspace deletion.
	// This includes removing external resources and updating related systems.
	//
	// Parameters:
	//   - ctx: Context for cleanup operations
	//   - ref: Reference to the deleted workspace
	//   - options: Cleanup configuration and safety options
	//
	// Returns:
	//   - error: Issues during cleanup operations
	//
	// Behavior:
	//   - Removes external resource dependencies
	//   - Updates related workspaces and policies
	//   - Cleans up cached data and metrics
	//   - Notifies dependent systems of deletion
	FinalizeWorkspaceDeletion(ctx context.Context, ref WorkspaceReference, options WorkspaceCleanupOptions) error

	// GetLifecycleStatus returns the current status of any ongoing lifecycle operations.
	// Used for monitoring long-running operations and providing user feedback.
	//
	// Parameters:
	//   - ctx: Context for the status query
	//   - ref: Workspace to query
	//
	// Returns:
	//   - *WorkspaceLifecycleStatus: Current lifecycle operation status
	//   - error: Issues retrieving lifecycle status
	//
	// Behavior:
	//   - Returns status of any active operations
	//   - Includes progress information where available
	//   - Provides estimated completion times
	//   - Reports any encountered issues or warnings
	GetLifecycleStatus(ctx context.Context, ref WorkspaceReference) (*WorkspaceLifecycleStatus, error)

	// CancelLifecycleOperation attempts to cancel an ongoing lifecycle operation.
	// Not all operations may be safely cancelled once started.
	//
	// Parameters:
	//   - ctx: Context for the cancellation request
	//   - ref: Workspace with operation to cancel
	//   - operationID: Specific operation to cancel
	//
	// Returns:
	//   - bool: Whether the operation was successfully cancelled
	//   - error: Issues with the cancellation request
	//
	// Behavior:
	//   - Attempts graceful operation termination
	//   - Performs necessary cleanup for partial operations
	//   - Updates operation status to reflect cancellation
	//   - May not be supported for all operation types
	CancelLifecycleOperation(ctx context.Context, ref WorkspaceReference, operationID string) (bool, error)
}

// WorkspaceInitializationConfig defines parameters for workspace initialization.
// Controls how new workspaces are set up and configured.
type WorkspaceInitializationConfig struct {
	// TemplateSource specifies a template workspace to copy configuration from.
	// Optional - if not specified, default initialization is used.
	TemplateSource *WorkspaceReference `json:"templateSource,omitempty"`

	// InitialResources contains Kubernetes resources to create during initialization.
	// These are applied after basic workspace setup is complete.
	InitialResources []InitialResource `json:"initialResources,omitempty"`

	// PolicySets defines which policy sets to apply during initialization.
	// Policies control resource quotas, security settings, and access rules.
	PolicySets []string `json:"policySets,omitempty"`

	// CustomInitializers specifies additional initialization steps to perform.
	// These run after standard initialization but before readiness checks.
	CustomInitializers []WorkspaceInitializer `json:"customInitializers,omitempty"`

	// TimeoutDuration specifies maximum time to wait for initialization completion.
	TimeoutDuration time.Duration `json:"timeoutDuration,omitempty"`

	// RequiredCapabilities lists API capabilities that must be available before
	// the workspace is considered ready.
	RequiredCapabilities []WorkspaceCapability `json:"requiredCapabilities,omitempty"`
}

// InitialResource defines a Kubernetes resource to create during workspace initialization.
type InitialResource struct {
	// APIVersion specifies the API version for this resource.
	APIVersion string `json:"apiVersion"`

	// Kind identifies the resource type.
	Kind string `json:"kind"`

	// Metadata contains resource metadata including name and labels.
	Metadata metav1.ObjectMeta `json:"metadata"`

	// Spec contains the resource specification.
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// WorkspaceInitializer defines a custom initialization step.
type WorkspaceInitializer struct {
	// Name identifies this initializer for logging and error reporting.
	Name string `json:"name"`

	// Type specifies the kind of initialization to perform.
	Type WorkspaceInitializerType `json:"type"`

	// Configuration contains type-specific initialization parameters.
	Configuration map[string]interface{} `json:"configuration,omitempty"`

	// Dependencies lists other initializers that must complete first.
	Dependencies []string `json:"dependencies,omitempty"`

	// TimeoutDuration specifies maximum execution time for this initializer.
	TimeoutDuration time.Duration `json:"timeoutDuration,omitempty"`
}

// WorkspaceInitializerType categorizes different kinds of initialization operations.
type WorkspaceInitializerType string

const (
	// InitializerTypeResource creates or updates Kubernetes resources.
	InitializerTypeResource WorkspaceInitializerType = "Resource"

	// InitializerTypePolicy applies security or resource policies.
	InitializerTypePolicy WorkspaceInitializerType = "Policy"

	// InitializerTypeExternal calls out to external systems or APIs.
	InitializerTypeExternal WorkspaceInitializerType = "External"

	// InitializerTypeCustom runs workspace-specific initialization logic.
	InitializerTypeCustom WorkspaceInitializerType = "Custom"
)

// WorkspaceInitializationResult contains details about workspace initialization.
type WorkspaceInitializationResult struct {
	// InitializationID uniquely identifies this initialization operation.
	InitializationID string `json:"initializationId"`

	// StartTime indicates when initialization began.
	StartTime time.Time `json:"startTime"`

	// CompletionTime indicates when initialization finished.
	CompletionTime time.Time `json:"completionTime"`

	// Status indicates whether initialization was successful.
	Status WorkspaceOperationStatus `json:"status"`

	// CompletedSteps lists initialization steps that completed successfully.
	CompletedSteps []string `json:"completedSteps,omitempty"`

	// FailedSteps lists initialization steps that encountered errors.
	FailedSteps []WorkspaceOperationError `json:"failedSteps,omitempty"`

	// AppliedPolicies lists policy sets that were successfully applied.
	AppliedPolicies []string `json:"appliedPolicies,omitempty"`

	// CreatedResources lists resources that were created during initialization.
	CreatedResources []WorkspaceResourceReference `json:"createdResources,omitempty"`

	// Messages contains additional information about the initialization process.
	Messages []string `json:"messages,omitempty"`
}

// WorkspaceMigrationOptions configures workspace migration behavior.
type WorkspaceMigrationOptions struct {
	// Strategy specifies the migration approach to use.
	Strategy WorkspaceMigrationStrategy `json:"strategy"`

	// PreserveSources indicates whether to keep the source workspace after migration.
	// If false, the source workspace is deleted after successful migration.
	PreserveSources bool `json:"preserveSources"`

	// DataMigrationMode controls how workspace data is transferred.
	DataMigrationMode DataMigrationMode `json:"dataMigrationMode"`

	// ValidationMode specifies how thoroughly to validate the migration.
	ValidationMode ValidationMode `json:"validationMode"`

	// AllowDataLoss permits migration even if some data cannot be transferred.
	// Should be used with extreme caution.
	AllowDataLoss bool `json:"allowDataLoss"`

	// TimeoutDuration specifies maximum time to wait for migration completion.
	TimeoutDuration time.Duration `json:"timeoutDuration,omitempty"`

	// RollbackOnFailure automatically reverts changes if migration fails.
	RollbackOnFailure bool `json:"rollbackOnFailure"`
}

// WorkspaceMigrationStrategy defines different approaches to workspace migration.
type WorkspaceMigrationStrategy string

const (
	// MigrationStrategyBlueGreen creates a new workspace and switches traffic after validation.
	MigrationStrategyBlueGreen WorkspaceMigrationStrategy = "BlueGreen"

	// MigrationStrategyInPlace updates the existing workspace configuration directly.
	MigrationStrategyInPlace WorkspaceMigrationStrategy = "InPlace"

	// MigrationStrategyRolling gradually moves resources to the new configuration.
	MigrationStrategyRolling WorkspaceMigrationStrategy = "Rolling"
)

// DataMigrationMode controls how workspace data is handled during migration.
type DataMigrationMode string

const (
	// DataMigrationModeSnapshot creates a backup before migration for rollback.
	DataMigrationModeSnapshot DataMigrationMode = "Snapshot"

	// DataMigrationModeDirect moves data without creating intermediate copies.
	DataMigrationModeDirect DataMigrationMode = "Direct"

	// DataMigrationModeIncremental synchronizes data changes during migration.
	DataMigrationModeIncremental DataMigrationMode = "Incremental"
)

// ValidationMode specifies how thoroughly to validate migration results.
type ValidationMode string

const (
	// ValidationModeNone skips validation for fastest migration.
	ValidationModeNone ValidationMode = "None"

	// ValidationModeBasic performs essential validation checks.
	ValidationModeBasic ValidationMode = "Basic"

	// ValidationModeComplete performs comprehensive validation of all data.
	ValidationModeComplete ValidationMode = "Complete"
)

// WorkspaceMigrationResult contains details about a workspace migration operation.
type WorkspaceMigrationResult struct {
	// MigrationID uniquely identifies this migration operation.
	MigrationID string `json:"migrationId"`

	// StartTime indicates when migration began.
	StartTime time.Time `json:"startTime"`

	// CompletionTime indicates when migration finished.
	CompletionTime time.Time `json:"completionTime"`

	// Status indicates whether migration was successful.
	Status WorkspaceOperationStatus `json:"status"`

	// SourceWorkspace identifies the original workspace.
	SourceWorkspace WorkspaceReference `json:"sourceWorkspace"`

	// TargetWorkspace identifies the destination workspace.
	TargetWorkspace WorkspaceReference `json:"targetWorkspace"`

	// MigratedResources lists resources that were successfully migrated.
	MigratedResources []WorkspaceResourceReference `json:"migratedResources,omitempty"`

	// FailedResources lists resources that could not be migrated.
	FailedResources []WorkspaceOperationError `json:"failedResources,omitempty"`

	// DataTransferSize indicates the amount of data moved during migration.
	DataTransferSize int64 `json:"dataTransferSize,omitempty"`

	// Messages contains additional information about the migration process.
	Messages []string `json:"messages,omitempty"`
}

// WorkspaceBackupConfig defines parameters for workspace backup operations.
type WorkspaceBackupConfig struct {
	// BackupType specifies the kind of backup to create.
	BackupType WorkspaceBackupType `json:"backupType"`

	// StorageLocation specifies where to store the backup.
	StorageLocation string `json:"storageLocation"`

	// IncludeData determines whether user data is included in the backup.
	IncludeData bool `json:"includeData"`

	// CompressionEnabled reduces backup size at the cost of processing time.
	CompressionEnabled bool `json:"compressionEnabled"`

	// EncryptionEnabled protects backup data with encryption.
	EncryptionEnabled bool `json:"encryptionEnabled"`

	// RetentionPolicy specifies how long to keep this backup.
	RetentionPolicy *WorkspaceBackupRetention `json:"retentionPolicy,omitempty"`

	// Labels contains metadata to attach to the backup.
	Labels map[string]string `json:"labels,omitempty"`
}

// WorkspaceBackupType categorizes different kinds of backup operations.
type WorkspaceBackupType string

const (
	// BackupTypeFull includes all workspace configuration and data.
	BackupTypeFull WorkspaceBackupType = "Full"

	// BackupTypeConfiguration includes only workspace metadata and settings.
	BackupTypeConfiguration WorkspaceBackupType = "Configuration"

	// BackupTypeIncremental includes only changes since the last backup.
	BackupTypeIncremental WorkspaceBackupType = "Incremental"
)

// WorkspaceBackupRetention defines how long to keep backup data.
type WorkspaceBackupRetention struct {
	// Duration specifies how long to keep this backup.
	Duration time.Duration `json:"duration"`

	// AutoCleanup enables automatic deletion when retention expires.
	AutoCleanup bool `json:"autoCleanup"`
}

// WorkspaceBackupResult contains information about a completed backup operation.
type WorkspaceBackupResult struct {
	// BackupID uniquely identifies this backup.
	BackupID string `json:"backupId"`

	// BackupReference provides a stable reference to this backup.
	BackupReference WorkspaceBackupReference `json:"backupReference"`

	// CreationTime indicates when the backup was created.
	CreationTime time.Time `json:"creationTime"`

	// BackupSize indicates the size of the backup data.
	BackupSize int64 `json:"backupSize"`

	// Status indicates whether backup creation was successful.
	Status WorkspaceOperationStatus `json:"status"`

	// IncludedResources lists resources that were backed up.
	IncludedResources []WorkspaceResourceReference `json:"includedResources,omitempty"`

	// ExcludedResources lists resources that were skipped during backup.
	ExcludedResources []WorkspaceResourceReference `json:"excludedResources,omitempty"`

	// Checksum provides integrity verification for the backup data.
	Checksum string `json:"checksum,omitempty"`

	// Messages contains additional information about the backup process.
	Messages []string `json:"messages,omitempty"`
}

// WorkspaceBackupReference provides a stable reference to a backup.
type WorkspaceBackupReference struct {
	// BackupID uniquely identifies the backup.
	BackupID string `json:"backupId"`

	// BackupName provides a human-readable backup identifier.
	BackupName string `json:"backupName,omitempty"`

	// StorageLocation specifies where the backup is stored.
	StorageLocation string `json:"storageLocation"`

	// CreationTime indicates when the backup was created.
	CreationTime time.Time `json:"creationTime"`

	// SourceWorkspace identifies the workspace that was backed up.
	SourceWorkspace WorkspaceReference `json:"sourceWorkspace"`
}

// WorkspaceRestoreOptions configures workspace restoration behavior.
type WorkspaceRestoreOptions struct {
	// RestoreMode specifies how to handle conflicts during restoration.
	RestoreMode WorkspaceRestoreMode `json:"restoreMode"`

	// NameTransform specifies how to modify resource names during restoration.
	NameTransform *WorkspaceNameTransform `json:"nameTransform,omitempty"`

	// ResourceFilter limits which resources are restored from the backup.
	ResourceFilter *WorkspaceResourceFilter `json:"resourceFilter,omitempty"`

	// ValidationMode specifies how thoroughly to validate restoration.
	ValidationMode ValidationMode `json:"validationMode"`

	// TimeoutDuration specifies maximum time to wait for restore completion.
	TimeoutDuration time.Duration `json:"timeoutDuration,omitempty"`

	// PreserveOriginalTimestamps keeps original creation and modification times.
	PreserveOriginalTimestamps bool `json:"preserveOriginalTimestamps"`
}

// WorkspaceRestoreMode defines different approaches to workspace restoration.
type WorkspaceRestoreMode string

const (
	// RestoreModeCreateNew creates a new workspace with restored data.
	RestoreModeCreateNew WorkspaceRestoreMode = "CreateNew"

	// RestoreModeOverwrite replaces an existing workspace with backup data.
	RestoreModeOverwrite WorkspaceRestoreMode = "Overwrite"

	// RestoreModeMerge combines backup data with existing workspace data.
	RestoreModeMerge WorkspaceRestoreMode = "Merge"
)

// WorkspaceNameTransform defines how to modify resource names during restoration.
type WorkspaceNameTransform struct {
	// Prefix adds a prefix to all restored resource names.
	Prefix string `json:"prefix,omitempty"`

	// Suffix adds a suffix to all restored resource names.
	Suffix string `json:"suffix,omitempty"`

	// NamespaceMapping maps original namespaces to new namespace names.
	NamespaceMapping map[string]string `json:"namespaceMapping,omitempty"`
}

// WorkspaceResourceFilter limits which resources are included in operations.
type WorkspaceResourceFilter struct {
	// IncludeResources lists resource types to include.
	IncludeResources []string `json:"includeResources,omitempty"`

	// ExcludeResources lists resource types to exclude.
	ExcludeResources []string `json:"excludeResources,omitempty"`

	// LabelSelector filters resources based on labels.
	LabelSelector string `json:"labelSelector,omitempty"`

	// NamespaceFilter limits operations to specific namespaces.
	NamespaceFilter []string `json:"namespaceFilter,omitempty"`
}

// WorkspaceRestoreResult contains details about a workspace restoration operation.
type WorkspaceRestoreResult struct {
	// RestoreID uniquely identifies this restoration operation.
	RestoreID string `json:"restoreId"`

	// StartTime indicates when restoration began.
	StartTime time.Time `json:"startTime"`

	// CompletionTime indicates when restoration finished.
	CompletionTime time.Time `json:"completionTime"`

	// Status indicates whether restoration was successful.
	Status WorkspaceOperationStatus `json:"status"`

	// RestoredWorkspace identifies the workspace that was restored.
	RestoredWorkspace WorkspaceReference `json:"restoredWorkspace"`

	// SourceBackup identifies the backup that was restored from.
	SourceBackup WorkspaceBackupReference `json:"sourceBackup"`

	// RestoredResources lists resources that were successfully restored.
	RestoredResources []WorkspaceResourceReference `json:"restoredResources,omitempty"`

	// SkippedResources lists resources that were not restored.
	SkippedResources []WorkspaceResourceReference `json:"skippedResources,omitempty"`

	// FailedResources lists resources that could not be restored.
	FailedResources []WorkspaceOperationError `json:"failedResources,omitempty"`

	// Messages contains additional information about the restoration process.
	Messages []string `json:"messages,omitempty"`
}

// WorkspaceCleanupOptions configures workspace cleanup behavior after deletion.
type WorkspaceCleanupOptions struct {
	// CleanupMode specifies the thoroughness of cleanup operations.
	CleanupMode WorkspaceCleanupMode `json:"cleanupMode"`

	// PreserveBackups indicates whether to keep backups of the deleted workspace.
	PreserveBackups bool `json:"preserveBackups"`

	// CleanupTimeoutDuration specifies maximum time for cleanup operations.
	CleanupTimeoutDuration time.Duration `json:"cleanupTimeoutDuration,omitempty"`

	// NotifyDependentSystems indicates whether to notify related systems.
	NotifyDependentSystems bool `json:"notifyDependentSystems"`
}

// WorkspaceCleanupMode defines different levels of cleanup after workspace deletion.
type WorkspaceCleanupMode string

const (
	// CleanupModeStandard performs normal cleanup operations.
	CleanupModeStandard WorkspaceCleanupMode = "Standard"

	// CleanupModeAggressive removes all traces of the workspace.
	CleanupModeAggressive WorkspaceCleanupMode = "Aggressive"

	// CleanupModeMinimal performs only essential cleanup operations.
	CleanupModeMinimal WorkspaceCleanupMode = "Minimal"
)

// WorkspaceLifecycleStatus represents the current status of lifecycle operations.
type WorkspaceLifecycleStatus struct {
	// ActiveOperations lists currently running lifecycle operations.
	ActiveOperations []WorkspaceLifecycleOperation `json:"activeOperations,omitempty"`

	// RecentOperations lists recently completed operations.
	RecentOperations []WorkspaceLifecycleOperation `json:"recentOperations,omitempty"`

	// PendingOperations lists operations waiting to start.
	PendingOperations []WorkspaceLifecycleOperation `json:"pendingOperations,omitempty"`

	// LastUpdateTime indicates when this status was last refreshed.
	LastUpdateTime time.Time `json:"lastUpdateTime"`
}

// WorkspaceLifecycleOperation represents a single lifecycle operation.
type WorkspaceLifecycleOperation struct {
	// OperationID uniquely identifies this operation.
	OperationID string `json:"operationId"`

	// Type indicates the kind of lifecycle operation.
	Type WorkspaceLifecycleOperationType `json:"type"`

	// Status indicates the current state of the operation.
	Status WorkspaceOperationStatus `json:"status"`

	// StartTime indicates when the operation began.
	StartTime time.Time `json:"startTime"`

	// EstimatedCompletion provides an estimate of when the operation will finish.
	EstimatedCompletion *time.Time `json:"estimatedCompletion,omitempty"`

	// Progress indicates operation completion percentage (0-100).
	Progress int `json:"progress,omitempty"`

	// CurrentStep describes what the operation is currently doing.
	CurrentStep string `json:"currentStep,omitempty"`

	// Messages contains status updates and error information.
	Messages []string `json:"messages,omitempty"`
}

// WorkspaceLifecycleOperationType categorizes different lifecycle operations.
type WorkspaceLifecycleOperationType string

const (
	// OperationTypeInitialization represents workspace initialization.
	OperationTypeInitialization WorkspaceLifecycleOperationType = "Initialization"

	// OperationTypeMigration represents workspace migration.
	OperationTypeMigration WorkspaceLifecycleOperationType = "Migration"

	// OperationTypeBackup represents workspace backup creation.
	OperationTypeBackup WorkspaceLifecycleOperationType = "Backup"

	// OperationTypeRestore represents workspace restoration.
	OperationTypeRestore WorkspaceLifecycleOperationType = "Restore"

	// OperationTypeCleanup represents post-deletion cleanup.
	OperationTypeCleanup WorkspaceLifecycleOperationType = "Cleanup"
)

// WorkspaceOperationStatus indicates the state of a lifecycle operation.
type WorkspaceOperationStatus string

const (
	// OperationStatusPending indicates the operation has not started yet.
	OperationStatusPending WorkspaceOperationStatus = "Pending"

	// OperationStatusRunning indicates the operation is currently executing.
	OperationStatusRunning WorkspaceOperationStatus = "Running"

	// OperationStatusCompleted indicates the operation finished successfully.
	OperationStatusCompleted WorkspaceOperationStatus = "Completed"

	// OperationStatusFailed indicates the operation encountered an error.
	OperationStatusFailed WorkspaceOperationStatus = "Failed"

	// OperationStatusCancelled indicates the operation was cancelled.
	OperationStatusCancelled WorkspaceOperationStatus = "Cancelled"
)

// WorkspaceOperationError provides details about operation failures.
type WorkspaceOperationError struct {
	// Step identifies where in the operation the error occurred.
	Step string `json:"step"`

	// ErrorMessage describes what went wrong.
	ErrorMessage string `json:"errorMessage"`

	// ErrorCode provides a machine-readable error identifier.
	ErrorCode string `json:"errorCode,omitempty"`

	// Timestamp indicates when the error occurred.
	Timestamp time.Time `json:"timestamp"`

	// Retryable indicates whether this error can be resolved by retrying.
	Retryable bool `json:"retryable"`
}

// WorkspaceResourceReference identifies a specific resource within a workspace.
type WorkspaceResourceReference struct {
	// APIVersion specifies the API version for this resource.
	APIVersion string `json:"apiVersion"`

	// Kind identifies the resource type.
	Kind string `json:"kind"`

	// Namespace specifies the resource namespace (if applicable).
	Namespace string `json:"namespace,omitempty"`

	// Name identifies the specific resource instance.
	Name string `json:"name"`

	// LogicalCluster identifies the logical cluster containing this resource.
	LogicalCluster logicalcluster.Name `json:"logicalCluster"`
}