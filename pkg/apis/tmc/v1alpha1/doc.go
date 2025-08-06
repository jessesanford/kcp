/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the TMC (Transparent Multi-Cluster) v1alpha1 API group.
//
// This package provides the foundational types and shared utilities for session-based workload placement management.
// The TMC APIs enable coordinated multi-cluster placement with session management, state persistence,
// decision coordination, and comprehensive validation frameworks.
//
// Shared Foundation Types:
// - WorkloadSelector for workload selection criteria
// - ClusterSelector for cluster targeting
// - WorkloadType for Kubernetes resource type specification
// - ObjectReference for cross-resource references
//
// +k8s:deepcopy-gen=package
// +groupName=tmc.kcp.io
// +kubebuilder:object:generate=true
package v1alpha1