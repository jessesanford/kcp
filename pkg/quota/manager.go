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

package quota

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

const (
	// DefaultAggregationInterval defines how often quota usage is aggregated
	DefaultAggregationInterval = 30 * time.Second
	
	// MaxBurstMultiplier defines maximum burst allocation multiplier
	MaxBurstMultiplier = 1.5
)

// ResourceName represents a resource type for quota management
type ResourceName string

const (
	// Core compute resources
	ResourceCPU              ResourceName = "cpu"
	ResourceMemory           ResourceName = "memory"
	ResourceStorage          ResourceName = "storage"
	ResourceEphemeralStorage ResourceName = "ephemeral-storage"
	
	// Object count resources
	ResourcePods             ResourceName = "pods"
	ResourceServices         ResourceName = "services"
	ResourceDeployments      ResourceName = "deployments"
)

// QuotaSpec defines resource quotas for a logical cluster or workspace
type QuotaSpec struct {
	// Hard quotas per resource type
	Hard map[ResourceName]resource.Quantity `json:"hard,omitempty"`
	
	// Priority for allocation decisions (higher = more priority)
	Priority int32 `json:"priority,omitempty"`
	
	// BurstAllowed enables temporary quota overages
	BurstAllowed bool `json:"burstAllowed,omitempty"`
}

// QuotaUsage represents current resource usage
type QuotaUsage struct {
	// Used quantities per resource type
	Used map[ResourceName]resource.Quantity `json:"used,omitempty"`
	
	// Timestamp of last update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// QuotaStatus represents quota state with violations
type QuotaStatus struct {
	// Current usage
	Usage QuotaUsage `json:"usage,omitempty"`
	
	// Current quota spec
	Quota QuotaSpec `json:"quota,omitempty"`
	
	// Utilization percentages per resource
	Utilization map[ResourceName]float64 `json:"utilization,omitempty"`
	
	// Violations represent active quota violations
	Violations []QuotaViolation `json:"violations,omitempty"`
}

// QuotaViolation represents a quota limit violation
type QuotaViolation struct {
	// Resource that exceeded quota
	Resource ResourceName `json:"resource"`
	
	// Current usage
	Used resource.Quantity `json:"used"`
	
	// Hard limit that was exceeded
	Limit resource.Quantity `json:"limit"`
	
	// Time when violation occurred
	ViolatedAt metav1.Time `json:"violatedAt"`
}

// AllocationRequest represents a resource allocation request
type AllocationRequest struct {
	// Workspace requesting resources
	Workspace logicalcluster.Name `json:"workspace"`
	
	// Requested resources
	Resources map[ResourceName]resource.Quantity `json:"resources"`
	
	// Priority of the request
	Priority int32 `json:"priority,omitempty"`
	
	// Whether burst allocation is acceptable
	AllowBurst bool `json:"allowBurst,omitempty"`
}

// AllocationResponse represents the result of an allocation request
type AllocationResponse struct {
	// Whether allocation was approved
	Approved bool `json:"approved"`
	
	// Allocated resources (may differ from requested)
	Allocated map[ResourceName]resource.Quantity `json:"allocated,omitempty"`
	
	// Reason for denial or partial allocation
	Reason string `json:"reason,omitempty"`
}

// ClusterCapacity represents available resources on a cluster
type ClusterCapacity struct {
	// Cluster identifier
	ClusterName string `json:"clusterName"`
	
	// Total capacity per resource type
	Capacity map[ResourceName]resource.Quantity `json:"capacity"`
	
	// Currently allocated resources
	Allocated map[ResourceName]resource.Quantity `json:"allocated"`
	
	// Available resources (capacity - allocated)
	Available map[ResourceName]resource.Quantity `json:"available"`
	
	// Cluster health and availability
	Healthy bool `json:"healthy"`
}

// AllocationStrategy defines how resources are allocated
type AllocationStrategy string

const (
	// FairShareStrategy allocates resources proportionally
	FairShareStrategy AllocationStrategy = "FairShare"
	
	// FirstFitStrategy allocates to first available cluster
	FirstFitStrategy AllocationStrategy = "FirstFit"
	
	// BestFitStrategy allocates to cluster with best resource match
	BestFitStrategy AllocationStrategy = "BestFit"
)

// QuotaManager manages resource quotas across workspaces and clusters
type QuotaManager struct {
	// Configuration
	aggregationInterval time.Duration
	strategy           AllocationStrategy
	
	// Thread safety
	mutex sync.RWMutex
	
	// Workspace quotas indexed by logical cluster
	workspaceQuotas map[logicalcluster.Name]*QuotaStatus
	
	// Cluster capacities indexed by cluster name
	clusterCapacities map[string]*ClusterCapacity
	
	// Monitoring and metrics
	metrics *QuotaMetrics
	
	// Event callbacks
	onQuotaViolation   func(workspace logicalcluster.Name, violation QuotaViolation)
	onAllocationDenied func(request AllocationRequest, reason string)
}

// QuotaMetrics tracks quota manager performance and state
type QuotaMetrics struct {
	// Total workspaces under management
	TotalWorkspaces int64 `json:"totalWorkspaces"`
	
	// Total clusters under management
	TotalClusters int64 `json:"totalClusters"`
	
	// Active quota violations
	ActiveViolations int64 `json:"activeViolations"`
	
	// Successful allocations
	SuccessfulAllocations int64 `json:"successfulAllocations"`
	
	// Denied allocations
	DeniedAllocations int64 `json:"deniedAllocations"`
	
	// Average allocation response time
	AvgAllocationTime time.Duration `json:"avgAllocationTime"`
}

// NewQuotaManager creates a new quota manager instance
func NewQuotaManager(opts ...QuotaManagerOption) *QuotaManager {
	qm := &QuotaManager{
		aggregationInterval: DefaultAggregationInterval,
		strategy:           FairShareStrategy,
		workspaceQuotas:    make(map[logicalcluster.Name]*QuotaStatus),
		clusterCapacities:  make(map[string]*ClusterCapacity),
		metrics:           &QuotaMetrics{},
	}
	
	for _, opt := range opts {
		opt(qm)
	}
	
	return qm
}

// QuotaManagerOption configures a QuotaManager
type QuotaManagerOption func(*QuotaManager)

// WithAllocationStrategy sets the resource allocation strategy
func WithAllocationStrategy(strategy AllocationStrategy) QuotaManagerOption {
	return func(qm *QuotaManager) {
		qm.strategy = strategy
	}
}

// WithViolationCallback sets a callback for quota violations
func WithViolationCallback(callback func(logicalcluster.Name, QuotaViolation)) QuotaManagerOption {
	return func(qm *QuotaManager) {
		qm.onQuotaViolation = callback
	}
}

// Start begins quota management operations
func (qm *QuotaManager) Start(ctx context.Context) error {
	klog.Info("Starting quota manager")
	
	// Start aggregation loop
	go wait.UntilWithContext(ctx, qm.aggregateUsage, qm.aggregationInterval)
	
	// Start violation monitoring
	go wait.UntilWithContext(ctx, qm.monitorViolations, time.Minute)
	
	<-ctx.Done()
	klog.Info("Quota manager stopped")
	return nil
}

// SetWorkspaceQuota configures quota limits for a workspace
func (qm *QuotaManager) SetWorkspaceQuota(workspace logicalcluster.Name, spec QuotaSpec) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	status, exists := qm.workspaceQuotas[workspace]
	if !exists {
		status = &QuotaStatus{
			Usage: QuotaUsage{
				Used:        make(map[ResourceName]resource.Quantity),
				LastUpdated: metav1.Now(),
			},
			Utilization: make(map[ResourceName]float64),
			Violations:  []QuotaViolation{},
		}
		qm.workspaceQuotas[workspace] = status
		qm.metrics.TotalWorkspaces++
	}
	
	status.Quota = spec
	qm.updateUtilization(workspace, status)
	
	klog.V(2).Infof("Set quota for workspace %s: %+v", workspace, spec.Hard)
	return nil
}

// GetWorkspaceQuota retrieves quota status for a workspace
func (qm *QuotaManager) GetWorkspaceQuota(workspace logicalcluster.Name) (*QuotaStatus, error) {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	status, exists := qm.workspaceQuotas[workspace]
	if !exists {
		return nil, fmt.Errorf("workspace %s not found", workspace)
	}
	
	statusCopy := *status
	return &statusCopy, nil
}

// UpdateWorkspaceUsage updates resource usage for a workspace
func (qm *QuotaManager) UpdateWorkspaceUsage(workspace logicalcluster.Name, usage map[ResourceName]resource.Quantity) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	status, exists := qm.workspaceQuotas[workspace]
	if !exists {
		return fmt.Errorf("workspace %s not found", workspace)
	}
	
	status.Usage.Used = make(map[ResourceName]resource.Quantity)
	for resource, quantity := range usage {
		status.Usage.Used[resource] = quantity
	}
	status.Usage.LastUpdated = metav1.Now()
	
	qm.updateUtilization(workspace, status)
	qm.checkViolations(workspace, status)
	
	return nil
}

// RegisterCluster adds a cluster to quota management
func (qm *QuotaManager) RegisterCluster(clusterName string, capacity map[ResourceName]resource.Quantity) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	clusterCap := &ClusterCapacity{
		ClusterName: clusterName,
		Capacity:    make(map[ResourceName]resource.Quantity),
		Allocated:   make(map[ResourceName]resource.Quantity),
		Available:   make(map[ResourceName]resource.Quantity),
		Healthy:     true,
	}
	
	for resName, quantity := range capacity {
		clusterCap.Capacity[resName] = quantity
		clusterCap.Available[resName] = quantity
		clusterCap.Allocated[resName] = resource.Quantity{}
	}
	
	qm.clusterCapacities[clusterName] = clusterCap
	qm.metrics.TotalClusters++
	
	klog.V(2).Infof("Registered cluster %s with capacity: %+v", clusterName, capacity)
	return nil
}

// UpdateClusterUsage updates resource usage for a cluster
func (qm *QuotaManager) UpdateClusterUsage(clusterName string, allocated map[ResourceName]resource.Quantity) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	clusterCap, exists := qm.clusterCapacities[clusterName]
	if !exists {
		return fmt.Errorf("cluster %s not registered", clusterName)
	}
	
	for resource, quantity := range allocated {
		if capacity, hasCapacity := clusterCap.Capacity[resource]; hasCapacity {
			clusterCap.Allocated[resource] = quantity
			available := capacity.DeepCopy()
			available.Sub(quantity)
			clusterCap.Available[resource] = available
		}
	}
	
	return nil
}

// RequestAllocation processes a resource allocation request
func (qm *QuotaManager) RequestAllocation(request AllocationRequest) (*AllocationResponse, error) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	start := time.Now()
	defer func() {
		qm.updateAllocationMetrics(time.Since(start))
	}()
	
	// Check if workspace has quota defined
	status, exists := qm.workspaceQuotas[request.Workspace]
	if !exists {
		qm.metrics.DeniedAllocations++
		return &AllocationResponse{
			Approved: false,
			Reason:   fmt.Sprintf("No quota defined for workspace %s", request.Workspace),
		}, nil
	}
	
	// Check quota limits
	if !qm.checkQuotaLimits(request, status) {
		qm.metrics.DeniedAllocations++
		return &AllocationResponse{
			Approved: false,
			Reason:   "Request would exceed quota limits",
		}, nil
	}
	
	// Find suitable cluster using allocation strategy
	clusterName, allocated := qm.findClusterForAllocation(request)
	if clusterName == "" {
		qm.metrics.DeniedAllocations++
		return &AllocationResponse{
			Approved: false,
			Reason:   "No cluster has sufficient capacity",
		}, nil
	}
	
	// Update workspace usage projection
	for resource, quantity := range allocated {
		current := status.Usage.Used[resource]
		current.Add(quantity)
		status.Usage.Used[resource] = current
	}
	
	qm.metrics.SuccessfulAllocations++
	return &AllocationResponse{
		Approved:  true,
		Allocated: allocated,
	}, nil
}

// GetMetrics returns current quota manager metrics
func (qm *QuotaManager) GetMetrics() *QuotaMetrics {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	metricsCopy := *qm.metrics
	return &metricsCopy
}

// aggregateUsage periodically aggregates resource usage across clusters
func (qm *QuotaManager) aggregateUsage(ctx context.Context) {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	// In a real implementation, this would query cluster usage via APIs
	for workspace := range qm.workspaceQuotas {
		klog.V(5).Infof("Aggregating usage for workspace %s", workspace)
	}
}

// monitorViolations checks for and handles quota violations
func (qm *QuotaManager) monitorViolations(ctx context.Context) {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	violations := 0
	for workspace, status := range qm.workspaceQuotas {
		for _, violation := range status.Violations {
			klog.Warningf("Quota violation for workspace %s, resource %s", workspace, violation.Resource)
			violations++
		}
	}
	
	qm.metrics.ActiveViolations = int64(violations)
}

// updateUtilization calculates utilization percentages for a workspace
func (qm *QuotaManager) updateUtilization(workspace logicalcluster.Name, status *QuotaStatus) {
	for resource, hardLimit := range status.Quota.Hard {
		used, hasUsage := status.Usage.Used[resource]
		if hasUsage && !hardLimit.IsZero() {
			utilization := float64(used.MilliValue()) / float64(hardLimit.MilliValue()) * 100
			status.Utilization[resource] = math.Min(utilization, 100.0)
		} else {
			status.Utilization[resource] = 0.0
		}
	}
}

// checkViolations identifies quota violations for a workspace
func (qm *QuotaManager) checkViolations(workspace logicalcluster.Name, status *QuotaStatus) {
	var violations []QuotaViolation
	
	for resource, used := range status.Usage.Used {
		hardLimit, hasHardLimit := status.Quota.Hard[resource]
		if hasHardLimit && used.Cmp(hardLimit) > 0 {
			violation := QuotaViolation{
				Resource:   resource,
				Used:       used,
				Limit:      hardLimit,
				ViolatedAt: metav1.Now(),
			}
			
			violations = append(violations, violation)
			
			if qm.onQuotaViolation != nil {
				qm.onQuotaViolation(workspace, violation)
			}
		}
	}
	
	status.Violations = violations
}

// checkQuotaLimits verifies if a request fits within quota limits
func (qm *QuotaManager) checkQuotaLimits(request AllocationRequest, status *QuotaStatus) bool {
	for resource, requested := range request.Resources {
		hardLimit, hasHardLimit := status.Quota.Hard[resource]
		if !hasHardLimit {
			continue
		}
		
		used := status.Usage.Used[resource]
		total := used.DeepCopy()
		total.Add(requested)
		
		if total.Cmp(hardLimit) > 0 {
			if request.AllowBurst && status.Quota.BurstAllowed {
				burstLimit := hardLimit.DeepCopy()
				burstLimit.SetMilli(int64(float64(burstLimit.MilliValue()) * MaxBurstMultiplier))
				if total.Cmp(burstLimit) > 0 {
					return false
				}
			} else {
				return false
			}
		}
	}
	
	return true
}

// findClusterForAllocation selects a cluster for resource allocation
func (qm *QuotaManager) findClusterForAllocation(request AllocationRequest) (string, map[ResourceName]resource.Quantity) {
	switch qm.strategy {
	case FairShareStrategy:
		return qm.fairShareAllocation(request)
	case BestFitStrategy:
		return qm.bestFitAllocation(request)
	default:
		return qm.firstFitAllocation(request)
	}
}

// fairShareAllocation implements fair share allocation strategy
func (qm *QuotaManager) fairShareAllocation(request AllocationRequest) (string, map[ResourceName]resource.Quantity) {
	type clusterUtilization struct {
		name        string
		utilization float64
	}
	
	var clusters []clusterUtilization
	for name, capacity := range qm.clusterCapacities {
		if !capacity.Healthy {
			continue
		}
		
		totalUtil := 0.0
		resourceCount := 0
		
		for resource, cap := range capacity.Capacity {
			if !cap.IsZero() {
				alloc := capacity.Allocated[resource]
				util := float64(alloc.MilliValue()) / float64(cap.MilliValue())
				totalUtil += util
				resourceCount++
			}
		}
		
		avgUtil := 0.0
		if resourceCount > 0 {
			avgUtil = totalUtil / float64(resourceCount)
		}
		
		clusters = append(clusters, clusterUtilization{name: name, utilization: avgUtil})
	}
	
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].utilization < clusters[j].utilization
	})
	
	for _, cluster := range clusters {
		if allocated := qm.tryAllocateOnCluster(cluster.name, request.Resources); allocated != nil {
			return cluster.name, allocated
		}
	}
	
	return "", nil
}

// firstFitAllocation implements first-fit allocation strategy
func (qm *QuotaManager) firstFitAllocation(request AllocationRequest) (string, map[ResourceName]resource.Quantity) {
	for clusterName, capacity := range qm.clusterCapacities {
		if !capacity.Healthy {
			continue
		}
		
		if allocated := qm.tryAllocateOnCluster(clusterName, request.Resources); allocated != nil {
			return clusterName, allocated
		}
	}
	
	return "", nil
}

// bestFitAllocation implements best-fit allocation strategy
func (qm *QuotaManager) bestFitAllocation(request AllocationRequest) (string, map[ResourceName]resource.Quantity) {
	bestCluster := ""
	var bestAllocated map[ResourceName]resource.Quantity
	bestWaste := math.MaxFloat64
	
	for clusterName, capacity := range qm.clusterCapacities {
		if !capacity.Healthy {
			continue
		}
		
		allocated := qm.tryAllocateOnCluster(clusterName, request.Resources)
		if allocated == nil {
			continue
		}
		
		waste := 0.0
		for resource, available := range capacity.Available {
			requested, hasRequested := request.Resources[resource]
			if hasRequested {
				wasteAmount := float64(available.MilliValue() - requested.MilliValue())
				if wasteAmount >= 0 {
					waste += wasteAmount
				}
			}
		}
		
		if waste < bestWaste {
			bestWaste = waste
			bestCluster = clusterName
			bestAllocated = allocated
		}
	}
	
	return bestCluster, bestAllocated
}

// tryAllocateOnCluster attempts to allocate resources on a specific cluster
func (qm *QuotaManager) tryAllocateOnCluster(clusterName string, requested map[ResourceName]resource.Quantity) map[ResourceName]resource.Quantity {
	capacity, exists := qm.clusterCapacities[clusterName]
	if !exists || !capacity.Healthy {
		return nil
	}
	
	allocated := make(map[ResourceName]resource.Quantity)
	
	for resource, quantity := range requested {
		available, hasCapacity := capacity.Available[resource]
		if !hasCapacity || available.Cmp(quantity) < 0 {
			return nil
		}
		allocated[resource] = quantity
	}
	
	return allocated
}

// updateAllocationMetrics updates performance metrics for allocations
func (qm *QuotaManager) updateAllocationMetrics(duration time.Duration) {
	if qm.metrics.AvgAllocationTime == 0 {
		qm.metrics.AvgAllocationTime = duration
	} else {
		alpha := 0.1
		qm.metrics.AvgAllocationTime = time.Duration(
			float64(qm.metrics.AvgAllocationTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
}