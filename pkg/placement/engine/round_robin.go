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

package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// RoundRobinEngine implements PlacementEngine using a round-robin strategy.
// It distributes workloads evenly across available clusters, maintaining
// placement state to ensure fair distribution over time.
type RoundRobinEngine struct {
	// mu protects access to placement state across concurrent operations
	mu sync.RWMutex
	
	// lastPlacement tracks the last cluster used for placement per cluster selector
	// Key format: "labelSelector:<hash>|locationSelector:<locations>|clusterNames:<names>"
	lastPlacement map[string]string
}

// NewRoundRobinEngine creates a new round-robin placement engine.
func NewRoundRobinEngine() *RoundRobinEngine {
	return &RoundRobinEngine{
		lastPlacement: make(map[string]string),
	}
}

// SelectClusters implements PlacementEngine.SelectClusters using round-robin distribution.
// It filters clusters based on the cluster selector and returns clusters in round-robin order.
func (e *RoundRobinEngine) SelectClusters(ctx context.Context,
	workload *tmcv1alpha1.WorkloadPlacement,
	clusters []*tmcv1alpha1.ClusterRegistration,
) ([]PlacementDecision, error) {
	if workload == nil {
		return nil, fmt.Errorf("workload placement cannot be nil")
	}

	// Filter clusters based on cluster selector
	eligibleClusters, err := e.filterClusters(clusters, &workload.Spec.ClusterSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to filter clusters: %w", err)
	}

	if len(eligibleClusters) == 0 {
		return nil, fmt.Errorf("no eligible clusters found matching selector")
	}

	// Sort clusters for consistent ordering
	sort.Slice(eligibleClusters, func(i, j int) bool {
		return eligibleClusters[i].Name < eligibleClusters[j].Name
	})

	// Determine number of clusters to select
	requestedClusters := int(1) // default
	if workload.Spec.NumberOfClusters != nil {
		requestedClusters = int(*workload.Spec.NumberOfClusters)
	}

	if requestedClusters > len(eligibleClusters) {
		requestedClusters = len(eligibleClusters)
	}

	// Generate selector key for placement tracking
	selectorKey := e.generateSelectorKey(&workload.Spec.ClusterSelector)

	// Select clusters using round-robin
	decisions := e.selectRoundRobin(eligibleClusters, requestedClusters, selectorKey)

	return decisions, nil
}

// filterClusters filters the provided clusters based on the cluster selector.
func (e *RoundRobinEngine) filterClusters(clusters []*tmcv1alpha1.ClusterRegistration,
	selector *tmcv1alpha1.ClusterSelector) ([]*tmcv1alpha1.ClusterRegistration, error) {
	
	if selector == nil {
		return clusters, nil
	}

	var filtered []*tmcv1alpha1.ClusterRegistration

	for _, cluster := range clusters {
		// Check explicit cluster names
		if len(selector.ClusterNames) > 0 {
			found := false
			for _, name := range selector.ClusterNames {
				if cluster.Name == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check location selector
		if len(selector.LocationSelector) > 0 {
			found := false
			for _, location := range selector.LocationSelector {
				if cluster.Spec.Location == location {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check label selector
		if selector.LabelSelector != nil {
			labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
			if err != nil {
				return nil, fmt.Errorf("invalid label selector: %w", err)
			}
			if !labelSelector.Matches(labels.Set(cluster.Labels)) {
				continue
			}
		}

		filtered = append(filtered, cluster)
	}

	return filtered, nil
}

// selectRoundRobin performs round-robin selection on eligible clusters.
func (e *RoundRobinEngine) selectRoundRobin(clusters []*tmcv1alpha1.ClusterRegistration,
	count int, selectorKey string) []PlacementDecision {
	
	e.mu.Lock()
	defer e.mu.Unlock()

	var decisions []PlacementDecision
	clusterNames := make([]string, len(clusters))
	for i, cluster := range clusters {
		clusterNames[i] = cluster.Name
	}

	// Find starting position based on last placement
	startIndex := 0
	if lastCluster, exists := e.lastPlacement[selectorKey]; exists {
		for i, name := range clusterNames {
			if name == lastCluster {
				startIndex = (i + 1) % len(clusterNames)
				break
			}
		}
	}

	// Select clusters in round-robin order
	selectedClusters := make(map[string]bool)
	currentIndex := startIndex

	for len(decisions) < count && len(selectedClusters) < len(clusters) {
		clusterName := clusterNames[currentIndex]
		if !selectedClusters[clusterName] {
			decisions = append(decisions, PlacementDecision{
				ClusterName: clusterName,
				Score:       100 - len(decisions)*10, // Decrease score for secondary selections
				Reason:      fmt.Sprintf("Round-robin selection (position %d)", len(decisions)+1),
			})
			selectedClusters[clusterName] = true
		}
		currentIndex = (currentIndex + 1) % len(clusterNames)
	}

	// Update last placement
	if len(decisions) > 0 {
		e.lastPlacement[selectorKey] = decisions[len(decisions)-1].ClusterName
	}

	return decisions
}

// generateSelectorKey creates a unique key for tracking placement state per selector.
func (e *RoundRobinEngine) generateSelectorKey(selector *tmcv1alpha1.ClusterSelector) string {
	if selector == nil {
		return "default"
	}

	key := ""
	
	if selector.LabelSelector != nil {
		key += fmt.Sprintf("labels:%+v", selector.LabelSelector)
	}
	
	if len(selector.LocationSelector) > 0 {
		key += fmt.Sprintf("|locations:%v", selector.LocationSelector)
	}
	
	if len(selector.ClusterNames) > 0 {
		key += fmt.Sprintf("|names:%v", selector.ClusterNames)
	}
	
	if key == "" {
		return "default"
	}
	
	return key
}