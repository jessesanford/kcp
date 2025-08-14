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

// Package upstream implements the upstream syncer component for the TMC (Transparent Multi-Cluster) system.
// The upstream syncer is responsible for pulling resources from physical clusters back into KCP workspaces,
// providing observability and status aggregation for workloads running on downstream clusters.
//
// Key responsibilities:
//   - Discover and monitor resources in physical clusters
//   - Pull resource state changes from physical clusters to KCP
//   - Transform physical cluster resources to KCP workspace format
//   - Resolve conflicts between upstream and downstream state
//   - Aggregate status information from multiple physical clusters
//   - Handle network partitions and connectivity issues gracefully
//
// The upstream syncer works in conjunction with the downstream syncer to provide bidirectional
// synchronization between KCP workspaces and physical clusters, enabling true multi-cluster
// workload management with consistent state propagation.
package upstream