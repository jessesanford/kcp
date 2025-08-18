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

package framework

import "time"

const (
	// Test environment constants
	
	// DefaultTestPrefix is the default prefix for test resource names
	DefaultTestPrefix = "it-"
	
	// DefaultTestPortBase is the default base port for test services
	DefaultTestPortBase = 30100
	
	// IntegrationTestNamespace is the dedicated namespace for TMC integration tests
	IntegrationTestNamespace = "integration-tests"
)

const (
	// Timeout and polling constants
	
	// TestTimeout is the default timeout for integration test operations
	TestTimeout = 5 * time.Minute
	
	// TestPollInterval is the default polling interval for integration tests
	TestPollInterval = 2 * time.Second
	
	// ResourceReadyTimeout is the timeout for waiting for resources to become ready
	ResourceReadyTimeout = 5 * time.Minute
	
	// ResourceReadyPollInterval is the polling interval for resource ready checks
	ResourceReadyPollInterval = 2 * time.Second
	
	// ResourceDeletionTimeout is the timeout for waiting for resource deletion
	ResourceDeletionTimeout = 2 * time.Minute
	
	// ResourceDeletionPollInterval is the polling interval for resource deletion checks
	ResourceDeletionPollInterval = 2 * time.Second
)

const (
	// Standard test labels and annotations
	
	// TestSuiteLabelKey is the key for the test suite label
	TestSuiteLabelKey = "test-suite"
	
	// TestSuiteLabelValue is the value for the TMC integration test suite label
	TestSuiteLabelValue = "tmc-integration"
	
	// TestRunLabelKey is the key for the test run label
	TestRunLabelKey = "test-run"
	
	// TestWorkspaceLabelKey is the key for the test workspace label
	TestWorkspaceLabelKey = "test-workspace"
	
	// TestNamespaceLabelKey is the key for the test namespace label
	TestNamespaceLabelKey = "test-namespace"
	
	// TestTypeLabelKey is the key for the test type label
	TestTypeLabelKey = "test-type"
	
	// TestNameLabelKey is the key for the test name label
	TestNameLabelKey = "test-name"
	
	// ManagedByLabelKey is the key for the managed-by label
	ManagedByLabelKey = "managed-by"
	
	// ManagedByLabelValue is the value for the managed-by label for TMC integration tests
	ManagedByLabelValue = "tmc-integration-tests"
	
	// CleanupPolicyLabelKey is the key for the cleanup policy label
	CleanupPolicyLabelKey = "cleanup-policy"
	
	// AutomaticCleanupPolicyValue is the value for automatic cleanup policy
	AutomaticCleanupPolicyValue = "automatic"
	
	// TestNameAnnotationKey is the key for the test name annotation
	TestNameAnnotationKey = "test.kcp.io/test-name"
	
	// TestCreatedAnnotationKey is the key for the test creation time annotation
	TestCreatedAnnotationKey = "test.kcp.io/created"
)

const (
	// Workspace and resource constants
	
	// UniversalWorkspaceType is the workspace type for universal workspaces
	UniversalWorkspaceType = "universal"
	
	// RootWorkspacePath is the path for the root workspace
	RootWorkspacePath = "root"
	
	// DefaultParentWorkspace is the default parent workspace for test workspaces
	DefaultParentWorkspace = "root:default"
)

const (
	// Condition types and statuses
	
	// ReadyConditionType is the condition type for readiness
	ReadyConditionType = "Ready"
	
	// TrueConditionStatus is the condition status for true
	TrueConditionStatus = "True"
	
	// FalseConditionStatus is the condition status for false
	FalseConditionStatus = "False"
	
	// UnknownConditionStatus is the condition status for unknown
	UnknownConditionStatus = "Unknown"
)