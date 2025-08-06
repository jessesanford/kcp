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

// Package v1alpha1 contains the v1alpha1 version of the TMC API.
//
// The TMC (Transparent Multi-Cluster) API provides Kubernetes-native abstractions
// for managing workloads across multiple clusters through KCP's workspace isolation.
//
// Key APIs:
//   - WorkloadScalingPolicy: Defines scaling policies for multi-cluster workloads
//
// +k8s:deepcopy-gen=package
// +k8s:openapi-gen=true
// +groupName=tmc.kcp.io
package v1alpha1