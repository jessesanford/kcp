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

// Package tmcexport provides the TMC APIExport controller for managing
// the availability of TMC APIs within KCP workspaces.
//
// The TMC APIExport controller ensures that TMC APIs (Cluster, WorkloadPlacement,
// WorkloadPlacementAdvanced) are properly exported and available for consumption
// within workspaces. It manages the lifecycle of the tmc.kcp.io APIExport resource
// and maintains its status based on the availability of TMC resources.
//
// The controller follows KCP patterns for workspace-aware resource management
// and integrates with the existing APIExport system for proper API binding
// and consumption.
package tmcexport