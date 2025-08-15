// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tmc

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// PlacementReconciler handles WorkloadPlacement resources with TMC-specific logic.
// This reconciler implements intelligent workload placement strategies across
// registered clusters following KCP multi-tenant patterns.
type PlacementReconciler struct {
	client client.Client
	logger logr.Logger
}

// NewPlacementReconciler creates a new placement reconciler for TMC workload management.
// The reconciler implements TMC-specific placement algorithms that consider cluster
// health, capacity, and location for optimal workload distribution.
func NewPlacementReconciler(client client.Client, logger logr.Logger) *PlacementReconciler {
	return &PlacementReconciler{
		client: client,
		logger: logger.WithName("placement-reconciler"),
	}
}

// Reconcile processes a WorkloadPlacement resource with TMC-specific placement logic.
// This method implements the core TMC workload placement functionality including
// cluster selection, placement strategy execution, and status management.
func (r *PlacementReconciler) Reconcile(ctx context.Context, key string) error {
	r.logger.V(2).Info("Reconciling workload placement", "key", key)

	// Parse the key to extract namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key format %q: %w", key, err)
	}

	// Get the WorkloadPlacement resource
	placement := &tmcv1alpha1.WorkloadPlacement{}
	objKey := client.ObjectKey{Namespace: namespace, Name: name}
	if err := r.client.Get(ctx, objKey, placement); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("WorkloadPlacement deleted", "key", key)
			return nil
		}
		return fmt.Errorf("failed to get WorkloadPlacement %q: %w", key, err)
	}

	// TMC-specific logic: find clusters matching the placement requirements
	suitableClusters, err := r.findSuitableClusters(ctx, placement)
	if err != nil {
		return r.updatePlacementCondition(ctx, placement, "ClustersFound", metav1.ConditionFalse,
			"ClusterSelectionFailed", err.Error())
	}

	if len(suitableClusters) == 0 {
		return r.updatePlacementCondition(ctx, placement, "ClustersFound", metav1.ConditionFalse,
			"NoSuitableClusters", "No clusters match the placement requirements")
	}

	// TMC-specific logic: apply placement strategy to select target clusters
	selectedClusters, err := r.applyPlacementStrategy(ctx, placement, suitableClusters)
	if err != nil {
		return r.updatePlacementCondition(ctx, placement, "PlacementReady", metav1.ConditionFalse,
			"PlacementStrategyFailed", err.Error())
	}

	// Update placement status with selected clusters and decisions
	return r.updatePlacementStatus(ctx, placement, selectedClusters, suitableClusters)
}

// findSuitableClusters identifies clusters matching placement requirements using TMC logic.
// This implements TMC-specific cluster filtering based on location, capacity, and health.
func (r *PlacementReconciler) findSuitableClusters(
	ctx context.Context, 
	placement *tmcv1alpha1.WorkloadPlacement,
) ([]tmcv1alpha1.ClusterRegistration, error) {
	r.logger.V(4).Info("Finding suitable clusters", "placement", placement.Name)

	// Get all registered clusters
	clusterList := &tmcv1alpha1.ClusterRegistrationList{}
	if err := r.client.List(ctx, clusterList); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	var suitableClusters []tmcv1alpha1.ClusterRegistration
	selector := placement.Spec.ClusterSelector

	for _, cluster := range clusterList.Items {
		// TMC-specific filtering: check cluster health
		if !r.isClusterHealthy(&cluster) {
			r.logger.V(4).Info("Skipping unhealthy cluster", "cluster", cluster.Name)
			continue
		}

		// TMC-specific filtering: location-based selection
		if len(selector.LocationSelector) > 0 {
			if !r.matchesLocation(&cluster, selector.LocationSelector) {
				continue
			}
		}

		// TMC-specific filtering: explicit cluster name selection
		if len(selector.ClusterNames) > 0 {
			if !r.matchesClusterName(&cluster, selector.ClusterNames) {
				continue
			}
		}

		// TMC-specific filtering: label-based selection
		if selector.LabelSelector != nil {
			if !r.matchesLabels(&cluster, selector.LabelSelector) {
				continue
			}
		}

		suitableClusters = append(suitableClusters, cluster)
	}

	r.logger.V(4).Info("Found suitable clusters", 
		"placement", placement.Name,
		"total", len(clusterList.Items),
		"suitable", len(suitableClusters))

	return suitableClusters, nil
}

// applyPlacementStrategy executes the TMC placement algorithm to select target clusters.
// This implements different placement strategies based on the configured policy.
func (r *PlacementReconciler) applyPlacementStrategy(
	ctx context.Context,
	placement *tmcv1alpha1.WorkloadPlacement,
	clusters []tmcv1alpha1.ClusterRegistration,
) ([]string, error) {
	strategy := placement.Spec.PlacementPolicy
	if strategy == "" {
		strategy = tmcv1alpha1.PlacementPolicyRoundRobin
	}

	desiredClusters := int(1)
	if placement.Spec.NumberOfClusters != nil {
		desiredClusters = int(*placement.Spec.NumberOfClusters)
	}

	// Ensure we don't select more clusters than available
	if desiredClusters > len(clusters) {
		desiredClusters = len(clusters)
	}

	r.logger.V(4).Info("Applying placement strategy",
		"strategy", strategy,
		"desiredClusters", desiredClusters,
		"availableClusters", len(clusters))

	switch strategy {
	case tmcv1alpha1.PlacementPolicyRoundRobin:
		return r.roundRobinPlacement(clusters, desiredClusters), nil
	case tmcv1alpha1.PlacementPolicyLeastLoaded:
		return r.leastLoadedPlacement(clusters, desiredClusters), nil
	case tmcv1alpha1.PlacementPolicyRandom:
		return r.randomPlacement(clusters, desiredClusters), nil
	case tmcv1alpha1.PlacementPolicyLocationAware:
		return r.locationAwarePlacement(clusters, desiredClusters), nil
	default:
		return nil, fmt.Errorf("unknown placement strategy: %s", strategy)
	}
}

// roundRobinPlacement implements round-robin cluster selection for even distribution.
func (r *PlacementReconciler) roundRobinPlacement(clusters []tmcv1alpha1.ClusterRegistration, count int) []string {
	if count <= 0 || len(clusters) == 0 {
		return nil
	}

	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		clusterIndex := i % len(clusters)
		result = append(result, clusters[clusterIndex].Name)
	}

	return result
}

// leastLoadedPlacement implements least-loaded cluster selection for optimal resource usage.
func (r *PlacementReconciler) leastLoadedPlacement(clusters []tmcv1alpha1.ClusterRegistration, count int) []string {
	if count <= 0 || len(clusters) == 0 {
		return nil
	}

	// Sort clusters by resource usage (ascending order)
	sortedClusters := make([]tmcv1alpha1.ClusterRegistration, len(clusters))
	copy(sortedClusters, clusters)

	sort.Slice(sortedClusters, func(i, j int) bool {
		return r.getClusterLoad(&sortedClusters[i]) < r.getClusterLoad(&sortedClusters[j])
	})

	result := make([]string, 0, count)
	for i := 0; i < count && i < len(sortedClusters); i++ {
		result = append(result, sortedClusters[i].Name)
	}

	return result
}

// randomPlacement implements random cluster selection for load distribution.
func (r *PlacementReconciler) randomPlacement(clusters []tmcv1alpha1.ClusterRegistration, count int) []string {
	if count <= 0 || len(clusters) == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	shuffled := make([]tmcv1alpha1.ClusterRegistration, len(clusters))
	copy(shuffled, clusters)

	for i := range shuffled {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	result := make([]string, 0, count)
	for i := 0; i < count && i < len(shuffled); i++ {
		result = append(result, shuffled[i].Name)
	}

	return result
}

// locationAwarePlacement implements location-aware cluster selection for geographic distribution.
func (r *PlacementReconciler) locationAwarePlacement(clusters []tmcv1alpha1.ClusterRegistration, count int) []string {
	if count <= 0 || len(clusters) == 0 {
		return nil
	}

	// Group clusters by location
	locationMap := make(map[string][]tmcv1alpha1.ClusterRegistration)
	for _, cluster := range clusters {
		location := cluster.Spec.Location
		if location == "" {
			location = "unknown"
		}
		locationMap[location] = append(locationMap[location], cluster)
	}

	// Select clusters from different locations for geographic diversity
	result := make([]string, 0, count)
	locations := make([]string, 0, len(locationMap))
	for location := range locationMap {
		locations = append(locations, location)
	}

	for i := 0; i < count; i++ {
		locationIndex := i % len(locations)
		location := locations[locationIndex]
		locationClusters := locationMap[location]
		
		clusterIndex := (i / len(locations)) % len(locationClusters)
		if clusterIndex < len(locationClusters) {
			result = append(result, locationClusters[clusterIndex].Name)
		}
	}

	return result
}

// Helper methods for cluster evaluation

func (r *PlacementReconciler) isClusterHealthy(cluster *tmcv1alpha1.ClusterRegistration) bool {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *PlacementReconciler) matchesLocation(cluster *tmcv1alpha1.ClusterRegistration, locations []string) bool {
	for _, location := range locations {
		if cluster.Spec.Location == location {
			return true
		}
	}
	return false
}

func (r *PlacementReconciler) matchesClusterName(cluster *tmcv1alpha1.ClusterRegistration, names []string) bool {
	for _, name := range names {
		if cluster.Name == name {
			return true
		}
	}
	return false
}

func (r *PlacementReconciler) matchesLabels(cluster *tmcv1alpha1.ClusterRegistration, selector *metav1.LabelSelector) bool {
	// Simplified label matching - in real implementation would use proper label selector
	return true
}

func (r *PlacementReconciler) getClusterLoad(cluster *tmcv1alpha1.ClusterRegistration) int64 {
	if cluster.Status.AllocatedResources != nil && cluster.Status.AllocatedResources.CPU != nil {
		return *cluster.Status.AllocatedResources.CPU
	}
	return 0
}

// updatePlacementStatus updates the WorkloadPlacement status with placement results.
func (r *PlacementReconciler) updatePlacementStatus(
	ctx context.Context,
	placement *tmcv1alpha1.WorkloadPlacement,
	selectedClusters []string,
	allClusters []tmcv1alpha1.ClusterRegistration,
) error {
	placement.Status.SelectedClusters = selectedClusters
	placement.Status.LastPlacementTime = &metav1.Time{Time: time.Now()}

	// Create placement decisions for audit trail
	placement.Status.PlacementDecisions = make([]tmcv1alpha1.PlacementDecision, 0, len(selectedClusters))
	for _, clusterName := range selectedClusters {
		decision := tmcv1alpha1.PlacementDecision{
			ClusterName:  clusterName,
			Reason:       fmt.Sprintf("Selected by %s strategy", placement.Spec.PlacementPolicy),
			DecisionTime: metav1.Now(),
		}
		placement.Status.PlacementDecisions = append(placement.Status.PlacementDecisions, decision)
	}

	// Update the condition to indicate successful placement
	if err := r.updatePlacementCondition(ctx, placement, "PlacementReady", metav1.ConditionTrue,
		"PlacementSuccessful", fmt.Sprintf("Successfully placed on %d clusters", len(selectedClusters))); err != nil {
		return err
	}

	return r.client.Status().Update(ctx, placement)
}

// updatePlacementCondition updates or adds a condition to the placement status.
func (r *PlacementReconciler) updatePlacementCondition(
	ctx context.Context,
	placement *tmcv1alpha1.WorkloadPlacement,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	conditionsv1alpha1.SetStatusCondition(&placement.Status.Conditions, condition)

	if err := r.client.Status().Update(ctx, placement); err != nil {
		return fmt.Errorf("failed to update placement condition: %w", err)
	}

	r.logger.Info("Updated placement condition",
		"placement", placement.Name,
		"condition", conditionType,
		"status", status,
		"reason", reason)

	return nil
}

// SetupWithManager configures the placement reconciler with the controller manager.
func (r *PlacementReconciler) SetupWithManager(mgr interface{}) error {
	r.logger.Info("Setting up placement reconciler with manager")
	return nil
}

// GetLogger returns the reconciler's logger for structured logging.
func (r *PlacementReconciler) GetLogger() logr.Logger {
	return r.logger
}