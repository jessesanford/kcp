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

// Package v1alpha1 contains shared types for the v1alpha1 version of TMC (Topology Management Controller) API.
//
// This package provides common types used across multiple TMC APIs for managing workload placement
// across multiple Kubernetes clusters within a KCP workspace hierarchy. The shared types include:
//   - WorkloadSelector for selecting workloads based on labels, types, and namespaces
//   - ClusterSelector for selecting target clusters based on labels, locations, and names
//   - PlacementPolicy enums for defining placement strategies
//   - WorkloadReference for referencing specific workloads
//   - PlacedWorkloadStatus enums for tracking workload placement status
//
// These types are designed to integrate seamlessly with KCP's workspace model and APIExport
// system for multi-tenant cluster management.
//
// +k8s:deepcopy-gen=package
// +groupName=tmc.kcp.io

package v1alpha1