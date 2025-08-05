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

package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/placement/engine"
)

// TMCClusterProvider implements the ClusterProvider interface using TMC ClusterRegistration resources.
type TMCClusterProvider struct {
	clusterLister ClusterRegistrationLister
}

// NewTMCClusterProvider creates a new TMC cluster provider.
func NewTMCClusterProvider(clusterLister ClusterRegistrationLister) *TMCClusterProvider {
	return &TMCClusterProvider{
		clusterLister: clusterLister,
	}
}

// GetAvailableClusters returns all available clusters from ClusterRegistration resources.
func (p *TMCClusterProvider) GetAvailableClusters(ctx context.Context) ([]*engine.ClusterInfo, error) {
	clusters, err := p.clusterLister.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster registrations: %w", err)
	}

	var availableClusters []*engine.ClusterInfo
	for _, cluster := range clusters {
		if p.isClusterAvailable(cluster) {
			clusterInfo := p.convertToClusterInfo(cluster)
			availableClusters = append(availableClusters, clusterInfo)
		}
	}

	klog.V(3).InfoS("Retrieved available clusters", 
		"totalClusters", len(clusters),
		"availableClusters", len(availableClusters))

	return availableClusters, nil
}

// isClusterAvailable checks if a cluster is available for placement.
func (p *TMCClusterProvider) isClusterAvailable(cluster *tmcv1alpha1.ClusterRegistration) bool {
	// Check if cluster is ready
	readyCondition := p.findCondition(cluster.Status.Conditions, conditionsv1alpha1.ReadyCondition)
	if readyCondition == nil {
		klog.V(4).InfoS("Cluster missing ready condition", "cluster", cluster.Name)
		return false
	}

	if readyCondition.Status != "True" {
		klog.V(4).InfoS("Cluster not ready", "cluster", cluster.Name, "status", readyCondition.Status)
		return false
	}

	// Check if cluster has recent heartbeat
	if cluster.Status.LastHeartbeatTime != nil {
		// Consider cluster stale if no heartbeat in last 5 minutes
		// This is a simple heuristic - in production you'd want configurable thresholds
		if cluster.Status.LastHeartbeatTime.Time.Add(5 * 60 * 1000000000 /* 5 minutes in nanoseconds */).Before(cluster.CreationTimestamp.Time) {
			klog.V(4).InfoS("Cluster heartbeat stale", "cluster", cluster.Name)
			return false
		}
	}

	return true
}

// convertToClusterInfo converts a ClusterRegistration to engine.ClusterInfo.
func (p *TMCClusterProvider) convertToClusterInfo(cluster *tmcv1alpha1.ClusterRegistration) *engine.ClusterInfo {
	clusterInfo := &engine.ClusterInfo{
		Name:         cluster.Name,
		Location:     cluster.Spec.Location,
		WorkloadCount: cluster.Status.WorkloadCount,
		Available:    true,
	}

	// Convert resource usage to load percentages
	if cluster.Status.ResourceUsage.CPU != "" {
		if cpuLoad, err := p.parsePercentage(cluster.Status.ResourceUsage.CPU); err == nil {
			clusterInfo.CPULoad = cpuLoad
		}
	}

	if cluster.Status.ResourceUsage.Memory != "" {
		if memoryLoad, err := p.parsePercentage(cluster.Status.ResourceUsage.Memory); err == nil {
			clusterInfo.MemoryLoad = memoryLoad
		}
	}

	klog.V(5).InfoS("Converted cluster info",
		"cluster", cluster.Name,
		"location", clusterInfo.Location,
		"workloadCount", clusterInfo.WorkloadCount,
		"cpuLoad", clusterInfo.CPULoad,
		"memoryLoad", clusterInfo.MemoryLoad)

	return clusterInfo
}

// parsePercentage parses a percentage string like "75%" and returns the float value.
func (p *TMCClusterProvider) parsePercentage(percentStr string) (float64, error) {
	// Remove '%' suffix if present
	percentStr = strings.TrimSuffix(percentStr, "%")
	
	// Parse as float
	value, err := strconv.ParseFloat(percentStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse percentage %q: %w", percentStr, err)
	}

	return value, nil
}

// findCondition finds a condition by type in the conditions list.
func (p *TMCClusterProvider) findCondition(conditions conditionsv1alpha1.Conditions, conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}