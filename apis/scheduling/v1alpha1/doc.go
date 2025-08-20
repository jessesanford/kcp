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

// Package v1alpha1 contains the foundation scheduling and resource management API types
// for KCP workspaces. These types provide the core abstractions for scheduling decisions,
// resource allocation, and cluster topology management across multiple clusters and workspaces.
//
// +k8s:deepcopy-gen=package
// +k8s:protobuf-gen=package
// +k8s:openapi-gen=true
// +groupName=scheduling.kcp.io

package v1alpha1 // import "github.com/kcp-dev/kcp/apis/scheduling/v1alpha1"