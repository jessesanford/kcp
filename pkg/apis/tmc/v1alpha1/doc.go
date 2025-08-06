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

// Package v1alpha1 contains API Schema definitions for the TMC (Transparent Multi-Cluster) v1alpha1 API group.
//
// This package provides the foundational types for transparent multi-cluster workload placement.
// The TMC APIs enable seamless placement and management of workloads across multiple Kubernetes clusters
// through KCP's APIExport system with proper workspace isolation.
//
// Core Types:
// - ClusterRegistration for cluster management and health tracking
// - WorkloadPlacement for placement policies and strategies
//
// +k8s:deepcopy-gen=package
// +groupName=tmc.kcp.io
// +kubebuilder:object:generate=true
package v1alpha1