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

// Package v1alpha1 contains TMC (Transport Management Controller) API types
// for managing session affinity, workload placement, and cluster coordination.
//
// TMC provides APIs for:
// - Session Affinity: Managing workload placement consistency and session stickiness
// - Workload Sessions: Tracking and managing individual workload placement sessions
// - Placement Policies: Defining how workloads should be distributed across clusters
//
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/kcp-dev/kcp/pkg/apis/tmc
// +k8s:defaulter-gen=TypeMeta
// +groupName=tmc.kcp.io
package v1alpha1