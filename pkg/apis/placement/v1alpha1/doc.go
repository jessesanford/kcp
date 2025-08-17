// Package v1alpha1 contains placement policy types for the KCP TMC implementation.
//
// The placement package defines APIs for workload placement and scheduling across
// multiple SyncTargets in a KCP workspace. It provides:
//
// - PlacementPolicy: Defines how workloads should be placed across locations
// - Placement strategies: Singleton, HighAvailability, Spread, Binpack
// - Location selection: Direct, label-based, and cell-based selectors
// - Resource requirements: CPU, memory, and custom resource constraints
// - Spread constraints: Topology-aware distribution policies
// - Affinity rules: Co-location and anti-affinity preferences
//
// These types follow KCP conventions for workspace-scoped APIs and provide
// the foundation for intelligent workload distribution in multi-location
// environments.
//
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +k8s:openapi-gen=true
//
// +groupName=placement.kcp.io
package v1alpha1