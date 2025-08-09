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

// Package v1alpha1 contains the v1alpha1 version of TMC (Topology Management Controller) API types.
//
// TMC provides APIs for managing workload placement across multiple Kubernetes clusters
// within a KCP workspace hierarchy. It enables:
//   - Cluster registration and management
//   - Workload placement policies
//   - Cross-cluster workload synchronization
//   - Load balancing and traffic distribution
//
// The API is designed to integrate seamlessly with KCP's workspace model and APIExport
// system for multi-tenant cluster management.
//
// +k8s:deepcopy-gen=package
// +groupName=tmc.kcp.io

package v1alpha1