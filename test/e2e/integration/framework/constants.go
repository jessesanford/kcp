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
	// Test defaults
	DefaultTestPrefix         = "it-"
	DefaultTestPortBase       = 30100
	IntegrationTestNamespace  = "integration-tests"
	
	// Timeouts and polling
	TestTimeout                    = 5 * time.Minute
	TestPollInterval               = 2 * time.Second
	ResourceReadyTimeout           = 5 * time.Minute
	ResourceReadyPollInterval      = 2 * time.Second
	ResourceDeletionTimeout        = 2 * time.Minute
	ResourceDeletionPollInterval   = 2 * time.Second
	
	// Standard labels and annotations
	TestSuiteLabelKey         = "test-suite"
	TestSuiteLabelValue       = "tmc-integration"
	TestRunLabelKey           = "test-run"
	TestWorkspaceLabelKey     = "test-workspace"
	TestNamespaceLabelKey     = "test-namespace"
	TestTypeLabelKey          = "test-type"
	TestNameLabelKey          = "test-name"
	ManagedByLabelKey         = "managed-by"
	ManagedByLabelValue       = "tmc-integration-tests"
	CleanupPolicyLabelKey     = "cleanup-policy"
	AutomaticCleanupPolicyValue = "automatic"
	TestNameAnnotationKey     = "test.kcp.io/test-name"
	TestCreatedAnnotationKey  = "test.kcp.io/created"
	
	// Workspace constants
	UniversalWorkspaceType = "universal"
	RootWorkspacePath      = "root"
	DefaultParentWorkspace = "root:default"
	
	// Condition constants
	ReadyConditionType     = "Ready"
	TrueConditionStatus    = "True"
	FalseConditionStatus   = "False"
	UnknownConditionStatus = "Unknown"
)