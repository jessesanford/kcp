// Package v1alpha1 contains API Schema definitions for the workload v1alpha1 API group
// +k8s:deepcopy-gen=package
// +k8s:defaulter-gen=TypeMeta
// +k8s:openapi-gen=true
// +groupName=workload.kcp.io

// Package workload provides APIs for managing workload distribution across multiple locations.
// The WorkloadDistribution API allows users to specify how workloads should be distributed
// across SyncTargets, with support for various rollout strategies, resource overrides,
// and fine-grained location-specific configuration.
package v1alpha1