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

// Package engine provides placement algorithms for TMC workload placement.
package engine

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// PlacementRequest represents a request for cluster placement.
type PlacementRequest struct {
	// Policy specifies the placement policy to use
	Policy tmcv1alpha1.PlacementPolicy
	// RequestedClusters is the number of clusters to select
	RequestedClusters int
	// LocationFilter filters clusters by location
	LocationFilter string
	// ResourceRequirements specifies minimum resource requirements
	ResourceRequirements *ResourceRequirements
}

// ResourceRequirements defines minimum resource requirements for placement.
type ResourceRequirements struct {
	CPU    string
	Memory string
}

// PlacementResult contains the result of a placement decision.
type PlacementResult struct {
	// SelectedClusters are the clusters selected for placement
	SelectedClusters []string
	// Reason provides a human-readable explanation for the placement decision
	Reason string
}

// ClusterInfo represents information about a cluster for placement decisions.
type ClusterInfo struct {
	// Name is the cluster name
	Name string
	// Location is the cluster's geographic location
	Location string
	// WorkloadCount is the current number of workloads on the cluster
	WorkloadCount int32
	// Available indicates if the cluster is available for placement
	Available bool
	// CPULoad is the current CPU usage percentage (0-100)
	CPULoad float64
	// MemoryLoad is the current memory usage percentage (0-100)
	MemoryLoad float64
}

// ClusterProvider provides access to cluster information.
type ClusterProvider interface {
	GetAvailableClusters(ctx context.Context) ([]*ClusterInfo, error)
}

// SimplePlacementEngine implements basic placement algorithms.
type SimplePlacementEngine struct {
	clusterProvider ClusterProvider
	rng             *rand.Rand
}

// NewSimplePlacementEngine creates a new simple placement engine.
func NewSimplePlacementEngine(clusterProvider ClusterProvider) *SimplePlacementEngine {
	return &SimplePlacementEngine{
		clusterProvider: clusterProvider,
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectClusters selects clusters based on the placement policy.
func (e *SimplePlacementEngine) SelectClusters(ctx context.Context, request *PlacementRequest) (*PlacementResult, error) {
	clusters, err := e.clusterProvider.GetAvailableClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get available clusters: %w", err)
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no available clusters found")
	}

	// Filter clusters by location if specified
	if request.LocationFilter != "" {
		clusters = e.filterByLocation(clusters, request.LocationFilter)
		if len(clusters) == 0 {
			return nil, fmt.Errorf("no clusters found in location %q", request.LocationFilter)
		}
	}

	// Select clusters based on policy
	var selectedClusters []string
	var reason string

	switch request.Policy {
	case tmcv1alpha1.PlacementPolicyRoundRobin:
		selectedClusters, reason = e.selectRoundRobin(clusters, request.RequestedClusters)
	case tmcv1alpha1.PlacementPolicyLeastLoaded:
		selectedClusters, reason = e.selectLeastLoaded(clusters, request.RequestedClusters)
	case tmcv1alpha1.PlacementPolicyRandom:
		selectedClusters, reason = e.selectRandom(clusters, request.RequestedClusters)
	case tmcv1alpha1.PlacementPolicyLocationAware:
		selectedClusters, reason = e.selectLocationAware(clusters, request.RequestedClusters)
	default:
		return nil, fmt.Errorf("unsupported placement policy: %s", request.Policy)
	}

	if len(selectedClusters) == 0 {
		return nil, fmt.Errorf("no clusters selected using policy %s", request.Policy)
	}

	klog.V(2).InfoS("Placement decision completed",
		"policy", request.Policy,
		"requestedClusters", request.RequestedClusters,
		"availableClusters", len(clusters),
		"selectedClusters", selectedClusters,
		"reason", reason)

	return &PlacementResult{
		SelectedClusters: selectedClusters,
		Reason:           reason,
	}, nil
}

// filterByLocation filters clusters by location.
func (e *SimplePlacementEngine) filterByLocation(clusters []*ClusterInfo, location string) []*ClusterInfo {
	var filtered []*ClusterInfo
	for _, cluster := range clusters {
		if cluster.Location == location {
			filtered = append(filtered, cluster)
		}
	}
	return filtered
}

// selectRoundRobin selects clusters using round-robin algorithm.
func (e *SimplePlacementEngine) selectRoundRobin(clusters []*ClusterInfo, count int) ([]string, string) {
	if count <= 0 || len(clusters) == 0 {
		return nil, "no clusters to select"
	}

	// Sort clusters by name for consistent ordering
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	// Select up to count clusters, cycling through available clusters if needed
	var selected []string
	for i := 0; i < count && i < len(clusters); i++ {
		selected = append(selected, clusters[i].Name)
	}

	return selected, fmt.Sprintf("Selected %d clusters using round-robin", len(selected))
}

// selectLeastLoaded selects clusters with the lowest workload count.
func (e *SimplePlacementEngine) selectLeastLoaded(clusters []*ClusterInfo, count int) ([]string, string) {
	if count <= 0 || len(clusters) == 0 {
		return nil, "no clusters to select"
	}

	// Sort clusters by workload count (ascending)
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].WorkloadCount == clusters[j].WorkloadCount {
			// If workload counts are equal, sort by name for consistency
			return clusters[i].Name < clusters[j].Name
		}
		return clusters[i].WorkloadCount < clusters[j].WorkloadCount
	})

	// Select up to count clusters with lowest workload count
	var selected []string
	for i := 0; i < count && i < len(clusters); i++ {
		selected = append(selected, clusters[i].Name)
	}

	return selected, fmt.Sprintf("Selected %d least loaded clusters", len(selected))
}

// selectRandom selects clusters randomly.
func (e *SimplePlacementEngine) selectRandom(clusters []*ClusterInfo, count int) ([]string, string) {
	if count <= 0 || len(clusters) == 0 {
		return nil, "no clusters to select"
	}

	// Create a copy of cluster names for shuffling
	clusterNames := make([]string, len(clusters))
	for i, cluster := range clusters {
		clusterNames[i] = cluster.Name
	}

	// Shuffle the cluster names
	e.rng.Shuffle(len(clusterNames), func(i, j int) {
		clusterNames[i], clusterNames[j] = clusterNames[j], clusterNames[i]
	})

	// Select up to count clusters
	selectedCount := count
	if selectedCount > len(clusterNames) {
		selectedCount = len(clusterNames)
	}

	selected := clusterNames[:selectedCount]
	return selected, fmt.Sprintf("Randomly selected %d clusters", len(selected))
}

// selectLocationAware selects clusters with location awareness for distribution.
func (e *SimplePlacementEngine) selectLocationAware(clusters []*ClusterInfo, count int) ([]string, string) {
	if count <= 0 || len(clusters) == 0 {
		return nil, "no clusters to select"
	}

	// Group clusters by location
	locationMap := make(map[string][]*ClusterInfo)
	for _, cluster := range clusters {
		locationMap[cluster.Location] = append(locationMap[cluster.Location], cluster)
	}

	// Sort locations for consistent selection
	var locations []string
	for location := range locationMap {
		locations = append(locations, location)
	}
	sort.Strings(locations)

	var selected []string
	remaining := count

	// First pass: distribute one cluster per location
	for _, location := range locations {
		if remaining <= 0 {
			break
		}
		
		locationClusters := locationMap[location]
		// Sort clusters within location by workload count for consistent selection
		sort.Slice(locationClusters, func(i, j int) bool {
			if locationClusters[i].WorkloadCount == locationClusters[j].WorkloadCount {
				return locationClusters[i].Name < locationClusters[j].Name
			}
			return locationClusters[i].WorkloadCount < locationClusters[j].WorkloadCount
		})
		
		selected = append(selected, locationClusters[0].Name)
		remaining--
	}

	// Second pass: fill remaining slots by cycling through locations
	locationIndex := 0
	for remaining > 0 && len(selected) < len(clusters) {
		location := locations[locationIndex%len(locations)]
		locationClusters := locationMap[location]
		
		// Find next unselected cluster in this location
		var found bool
		for _, cluster := range locationClusters {
			alreadySelected := false
			for _, selectedName := range selected {
				if cluster.Name == selectedName {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				selected = append(selected, cluster.Name)
				remaining--
				found = true
				break
			}
		}
		
		// If no more clusters in this location, move to next
		if !found {
			locationIndex++
			if locationIndex >= len(locations) {
				break // No more clusters available
			}
		} else {
			locationIndex++
		}
	}

	// Create summary of selection
	locationCounts := make(map[string]int)
	for _, clusterName := range selected {
		for _, cluster := range clusters {
			if cluster.Name == clusterName {
				locationCounts[cluster.Location]++
				break
			}
		}
	}

	var locationSummary []string
	for location, count := range locationCounts {
		locationSummary = append(locationSummary, fmt.Sprintf("%s:%d", location, count))
	}
	sort.Strings(locationSummary)

	return selected, fmt.Sprintf("Location-aware selection: %s", strings.Join(locationSummary, ", "))
}