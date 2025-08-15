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

// Package v1alpha1 contains the v1alpha1 version of TMC (Topology Management Controller) API contracts.
//
// TMC provides interface contracts for managing workload placement across multiple Kubernetes clusters
// within a KCP workspace hierarchy. This package defines pure interface contracts without implementations,
// enabling:
//   - Cluster registration and management contracts
//   - Workload placement policy interfaces
//   - Cross-cluster workload synchronization contracts
//   - Load balancing and traffic distribution interfaces
//
// The interfaces are designed to integrate seamlessly with KCP's workspace model and APIExport
// system for multi-tenant cluster management. All implementations must comply with these contracts.
//
// This package contains NO implementations - only interface definitions, constants, and contracts
// that define the behavior expected by TMC controllers and other components.
package v1alpha1

const (
	// TMCFeatureGate controls all TMC functionality
	// When disabled, all TMC-related features should be inactive
	TMCFeatureGate = "TMCAlpha"

	// TMCAPIContractsVersion indicates the current version of the API contracts
	TMCAPIContractsVersion = "v1alpha1"

	// GroupName is the API group name for TMC resources
	GroupName = "tmc.kcp.io"
)