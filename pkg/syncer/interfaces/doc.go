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

// Package interfaces defines the core interfaces for the TMC (Transparent Multi-Cluster)
// syncer component. These interfaces provide the contract for synchronizing resources
// between logical clusters in KCP and physical Kubernetes clusters.
//
// The syncer is responsible for:
//   - Bidirectional synchronization of resources between logical and physical clusters
//   - Transformation of resources to adapt them for their target environment
//   - Conflict detection and resolution during synchronization
//   - Status collection and aggregation across multiple placements
//   - Workspace-aware resource management
//
// Core Interfaces:
//
// SyncEngine - The main synchronization engine that orchestrates resource synchronization
// between clusters. It manages the sync queue, processes operations, and coordinates
// with other components.
//
// ResourceTransformer - Handles the transformation of resources as they move between
// logical and physical clusters. This includes adding/removing annotations, labels,
// and adapting resources for their target environment.
//
// ConflictResolver - Detects and resolves conflicts that occur during synchronization.
// Supports multiple resolution strategies including server-side apply, three-way merge,
// and custom resolution logic.
//
// StatusCollector - Collects and aggregates status information from sync operations
// across multiple clusters and workspaces. Provides metrics and health monitoring.
//
// Architecture:
//
// The syncer operates as a controller in both the KCP control plane and in physical
// clusters (via deployed syncer pods). It watches for resources that need to be
// synchronized based on placement decisions and manages their lifecycle across clusters.
//
//	┌─────────────────┐     ┌─────────────────┐
//	│  Logical        │     │  Physical       │
//	│  Cluster (KCP)  │◄───►│  Cluster        │
//	└─────────────────┘     └─────────────────┘
//	        │                        │
//	        ▼                        ▼
//	┌─────────────────┐     ┌─────────────────┐
//	│  Sync Engine    │     │  Sync Engine    │
//	│  (Downstream)   │     │  (Upstream)     │
//	└─────────────────┘     └─────────────────┘
//	        │                        │
//	        ▼                        ▼
//	┌─────────────────────────────────────────┐
//	│         Resource Transformer            │
//	│         Conflict Resolver                │
//	│         Status Collector                 │
//	└─────────────────────────────────────────┘
//
// Usage:
//
// Implementations of these interfaces are used by the TMC placement controller
// and the syncer controller to manage workload distribution across clusters:
//
//	engine := syncer.NewSyncEngine(config)
//	engine.Start(ctx)
//	defer engine.Stop(ctx)
//
//	operation := interfaces.SyncOperation{
//	    Direction: interfaces.SyncDirectionDownstream,
//	    SourceCluster: logicalCluster,
//	    TargetCluster: physicalCluster,
//	    GVR: schema.GroupVersionResource{
//	        Group: "apps",
//	        Version: "v1",
//	        Resource: "deployments",
//	    },
//	    Namespace: "default",
//	    Name: "my-app",
//	}
//
//	engine.EnqueueSyncOperation(operation)
package interfaces
