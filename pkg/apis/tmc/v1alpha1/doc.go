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

// Package v1alpha1 contains the status management API types for the TMC (Transport Management Controller).
// This package is specifically focused on resource status tracking, health monitoring, and condition management
// for TMC resources like ClusterRegistration and WorkloadPlacement.
//
// Key Components:
// - Status types: ClusterRegistrationStatus, WorkloadPlacementStatus
// - Resource tracking: ClusterResourceUsage, WorkloadResourceUsage
// - Health monitoring: ClusterHealthMetrics, ClusterConnectionStatus  
// - Condition management: Standardized condition types and reasons
//
// This package follows KCP patterns for status subresource management and integrates
// with the KCP conditions framework for consistent status reporting.
//
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/kcp-dev/kcp/pkg/apis/tmc
// +k8s:openapi-gen=true
// +groupName=tmc.kcp.io

package v1alpha1