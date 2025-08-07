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

// Package v1alpha1 contains the TMC v1alpha1 API definitions for distributed workload management.
// 
// TMC (Transparent Multi-Cluster) provides APIs for managing workload placement and cluster
// registration in a KCP-based multi-cluster environment.
//
// The main resources are:
// - ClusterRegistration: Represents physical clusters registered for workload management
// - WorkloadPlacement: Defines policies for placing workloads across registered clusters
//
// +k8s:deepcopy-gen=package
// +groupName=tmc.kcp.io
package v1alpha1