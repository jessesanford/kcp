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

// Package upstream implements synchronization of resources from physical
// clusters back to KCP workspaces. This enables status aggregation and
// state reconciliation for workloads running on edge clusters.
//
// The upstream syncer watches SyncTarget resources and establishes connections
// to their associated physical clusters to pull resource states back into KCP.
// This provides a bidirectional synchronization mechanism where KCP can not
// only push workloads to edge clusters but also monitor their actual state.
//
// Architecture:
//
//   - UpstreamSyncController: Main controller that watches SyncTarget resources
//   - ResourceTransformer: Handles transformation between physical and logical formats (PR 2)
//   - ConflictResolver: Manages conflicts between KCP and physical cluster state (PR 3)
//   - UpstreamSyncer: Orchestrates the actual sync operations (PR 3)
//
// This package follows KCP's workspace isolation principles, ensuring that
// resources from different physical clusters are properly isolated within
// their respective logical clusters.
package upstream