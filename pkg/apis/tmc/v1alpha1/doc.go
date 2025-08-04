/*
Copyright 2025 The KCP Authors.

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

// Package v1alpha1 contains API Schema definitions for the TMC v1alpha1 API group.
// The TMC (Transparent Multi-Cluster) API provides cluster registration and
// workload placement capabilities for KCP with full workspace isolation.
//
// This package defines two primary resources:
//   - ClusterRegistration: Represents physical clusters registered with TMC
//   - WorkloadPlacement: Defines policies for placing workloads across clusters
//
// The TMC API integrates seamlessly with KCP's workspace system, providing
// transparent multi-cluster management while maintaining logical cluster
// boundaries and security isolation.
//
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/kcp-dev/kcp/pkg/apis/tmc
// +k8s:defaulter-gen=TypeMeta
// +groupName=tmc.kcp.io
package v1alpha1