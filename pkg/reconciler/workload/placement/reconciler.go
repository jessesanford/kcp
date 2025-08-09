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

package placement

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
)

// reconcile handles the core placement reconciliation logic.
// It processes placement specifications and generates placement decisions
// based on available locations and constraint satisfaction.
func (c *placementController) reconcile(ctx context.Context, placement *workloadv1alpha1.Placement) error {
	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName).WithValues(
		"cluster", logicalcluster.From(placement),
		"placement", placement.Name,
	)

	// Initialize status if needed
	if placement.Status.Conditions == nil {
		placement.Status.Conditions = []metav1.Condition{}
	}

	// Update observed generation
	placement.Status.ObservedGeneration = placement.Generation

	// Validate placement specification
	if err := c.validatePlacementSpec(placement); err != nil {
		logger.V(2).Info("placement specification validation failed", "error", err)
		c.setPlacementCondition(placement, workloadv1alpha1.PlacementValidCondition, metav1.ConditionFalse, workloadv1alpha1.PlacementInvalidReason, err.Error())
		c.setPlacementCondition(placement, workloadv1alpha1.PlacementReadyCondition, metav1.ConditionFalse, workloadv1alpha1.PlacementInvalidReason, "Placement specification is invalid")
		return c.commitStatus(ctx, placement, logger)
	}

	// Mark as valid
	c.setPlacementCondition(placement, workloadv1alpha1.PlacementValidCondition, metav1.ConditionTrue, workloadv1alpha1.PlacementValidReason, "Placement specification is valid")

	// Perform placement scheduling
	decisions, err := c.performPlacementScheduling(ctx, placement, logger)
	if err != nil {
		logger.Error(err, "placement scheduling failed")
		c.setPlacementCondition(placement, workloadv1alpha1.PlacementScheduledCondition, metav1.ConditionFalse, workloadv1alpha1.PlacementSchedulingFailedReason, err.Error())
		c.setPlacementCondition(placement, workloadv1alpha1.PlacementReadyCondition, metav1.ConditionFalse, workloadv1alpha1.PlacementSchedulingFailedReason, "Failed to schedule placement")
		return c.commitStatus(ctx, placement, logger)
	}

	// Update placement decisions
	placement.Status.PlacementDecisions = decisions

	// Mark as scheduled and ready
	c.setPlacementCondition(placement, workloadv1alpha1.PlacementScheduledCondition, metav1.ConditionTrue, workloadv1alpha1.PlacementScheduledReason, fmt.Sprintf("Scheduled to %d clusters", len(decisions)))
	c.setPlacementCondition(placement, workloadv1alpha1.PlacementReadyCondition, metav1.ConditionTrue, workloadv1alpha1.PlacementReadyReason, "Placement is ready")

	logger.V(2).Info("placement reconciled successfully", "decisions", len(decisions))
	return c.commitStatus(ctx, placement, logger)
}

// validatePlacementSpec validates the placement specification for correctness.
func (c *placementController) validatePlacementSpec(placement *workloadv1alpha1.Placement) error {
	spec := &placement.Spec

	// Validate workload reference
	if spec.WorkloadReference.APIVersion == "" {
		return fmt.Errorf("workloadReference.apiVersion is required")
	}
	if spec.WorkloadReference.Kind == "" {
		return fmt.Errorf("workloadReference.kind is required")
	}
	if spec.WorkloadReference.Name == "" {
		return fmt.Errorf("workloadReference.name is required")
	}

	// Validate number of clusters
	if spec.NumberOfClusters != nil && *spec.NumberOfClusters <= 0 {
		return fmt.Errorf("numberOfClusters must be positive")
	}

	// Validate location selector
	if spec.LocationSelector != nil {
		if err := c.validateLocationSelector(spec.LocationSelector); err != nil {
			return fmt.Errorf("invalid locationSelector: %w", err)
		}
	}

	return nil
}

// validateLocationSelector validates the location selector specification.
func (c *placementController) validateLocationSelector(selector *workloadv1alpha1.LocationSelector) error {
	// Validate match expressions
	for i, expr := range selector.MatchExpressions {
		if expr.Key == "" {
			return fmt.Errorf("matchExpressions[%d].key is required", i)
		}
		switch expr.Operator {
		case metav1.LabelSelectorOpIn, metav1.LabelSelectorOpNotIn:
			if len(expr.Values) == 0 {
				return fmt.Errorf("matchExpressions[%d] with operator %s requires non-empty values", i, expr.Operator)
			}
		case metav1.LabelSelectorOpExists, metav1.LabelSelectorOpDoesNotExist:
			if len(expr.Values) > 0 {
				return fmt.Errorf("matchExpressions[%d] with operator %s must not have values", i, expr.Operator)
			}
		default:
			return fmt.Errorf("matchExpressions[%d] has unsupported operator: %s", i, expr.Operator)
		}
	}

	return nil
}

// performPlacementScheduling performs the core placement scheduling logic.
func (c *placementController) performPlacementScheduling(ctx context.Context, placement *workloadv1alpha1.Placement, logger klog.Logger) ([]workloadv1alpha1.PlacementDecision, error) {
	clusterName := logicalcluster.From(placement)

	// Get available locations
	locations, err := c.locationLister.Cluster(clusterName).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}

	logger.V(3).Info("evaluating locations for placement", "totalLocations", len(locations))

	// Filter locations based on placement criteria
	candidates, err := c.filterLocationCandidates(placement, locations, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to filter location candidates: %w", err)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable locations found for placement")
	}

	// Score and rank candidates
	scoredCandidates := c.scoreLocationCandidates(placement, candidates, logger)

	// Select final placement decisions
	decisions := c.selectPlacementDecisions(placement, scoredCandidates, logger)

	return decisions, nil
}

// filterLocationCandidates filters locations based on placement specifications.
func (c *placementController) filterLocationCandidates(placement *workloadv1alpha1.Placement, locations []*workloadv1alpha1.Location, logger klog.Logger) ([]*workloadv1alpha1.Location, error) {
	var candidates []*workloadv1alpha1.Location

	for _, location := range locations {
		if c.isLocationCandidate(placement, location, logger) {
			candidates = append(candidates, location)
		}
	}

	logger.V(4).Info("filtered location candidates", "candidates", len(candidates), "total", len(locations))
	return candidates, nil
}

// isLocationCandidate determines if a location is a viable candidate for placement.
func (c *placementController) isLocationCandidate(placement *workloadv1alpha1.Placement, location *workloadv1alpha1.Location, logger klog.Logger) bool {
	// Check if location selector matches
	if placement.Spec.LocationSelector != nil {
		if !c.matchesLocationSelector(placement.Spec.LocationSelector, location) {
			logger.V(5).Info("location filtered out by selector", "location", location.Name)
			return false
		}
	}

	// TODO: Add constraint checking in future PRs
	// For now, we accept all locations that pass the basic selector

	logger.V(5).Info("location is a candidate", "location", location.Name)
	return true
}

// matchesLocationSelector checks if a location matches the location selector.
func (c *placementController) matchesLocationSelector(selector *workloadv1alpha1.LocationSelector, location *workloadv1alpha1.Location) bool {
	locationLabels := location.Labels
	if locationLabels == nil {
		locationLabels = make(map[string]string)
	}

	// Check match labels
	for key, value := range selector.MatchLabels {
		if locationLabels[key] != value {
			return false
		}
	}

	// Check match expressions
	for _, expr := range selector.MatchExpressions {
		switch expr.Operator {
		case metav1.LabelSelectorOpIn:
			if !c.labelValueInSlice(locationLabels[expr.Key], expr.Values) {
				return false
			}
		case metav1.LabelSelectorOpNotIn:
			if c.labelValueInSlice(locationLabels[expr.Key], expr.Values) {
				return false
			}
		case metav1.LabelSelectorOpExists:
			if _, exists := locationLabels[expr.Key]; !exists {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if _, exists := locationLabels[expr.Key]; exists {
				return false
			}
		}
	}

	return true
}

// labelValueInSlice checks if a label value is in the given slice.
func (c *placementController) labelValueInSlice(value string, slice []string) bool {
	for _, v := range slice {
		if value == v {
			return true
		}
	}
	return false
}

// scoredLocationCandidate represents a location candidate with its placement score.
type scoredLocationCandidate struct {
	location *workloadv1alpha1.Location
	score    int32
	reason   string
}

// scoreLocationCandidates assigns scores to location candidates based on placement criteria.
func (c *placementController) scoreLocationCandidates(placement *workloadv1alpha1.Placement, candidates []*workloadv1alpha1.Location, logger klog.Logger) []scoredLocationCandidate {
	var scored []scoredLocationCandidate

	for _, location := range candidates {
		score, reason := c.calculateLocationScore(placement, location)
		scored = append(scored, scoredLocationCandidate{
			location: location,
			score:    score,
			reason:   reason,
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	logger.V(4).Info("scored location candidates", "candidates", len(scored))
	return scored
}

// calculateLocationScore calculates a placement score for a location candidate.
func (c *placementController) calculateLocationScore(placement *workloadv1alpha1.Placement, location *workloadv1alpha1.Location) (int32, string) {
	// Base score for all valid candidates
	score := int32(50)
	reasons := []string{"basic candidate"}

	// TODO: Implement sophisticated scoring in future PRs
	// For now, provide basic scoring based on location name
	if strings.Contains(strings.ToLower(location.Name), "primary") {
		score += 20
		reasons = append(reasons, "primary location")
	}

	if strings.Contains(strings.ToLower(location.Name), "prod") {
		score += 15
		reasons = append(reasons, "production location")
	}

	// Ensure score is within valid range
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score, strings.Join(reasons, ", ")
}

// selectPlacementDecisions selects the final placement decisions from scored candidates.
func (c *placementController) selectPlacementDecisions(placement *workloadv1alpha1.Placement, candidates []scoredLocationCandidate, logger klog.Logger) []workloadv1alpha1.PlacementDecision {
	// Determine number of clusters to select
	numClusters := c.determineNumberOfClusters(placement, len(candidates))

	var decisions []workloadv1alpha1.PlacementDecision
	for i := 0; i < numClusters && i < len(candidates); i++ {
		candidate := candidates[i]
		decision := workloadv1alpha1.PlacementDecision{
			ClusterName: candidate.location.Name,
			Location:    candidate.location.Spec.DisplayName,
			Reason:      candidate.reason,
			Score:       &candidate.score,
		}
		decisions = append(decisions, decision)
	}

	logger.V(3).Info("selected placement decisions", "decisions", len(decisions), "requested", numClusters)
	return decisions
}

// determineNumberOfClusters determines how many clusters to place the workload on.
func (c *placementController) determineNumberOfClusters(placement *workloadv1alpha1.Placement, availableCandidates int) int {
	if placement.Spec.NumberOfClusters != nil {
		requested := int(*placement.Spec.NumberOfClusters)
		if requested > availableCandidates {
			return availableCandidates
		}
		return requested
	}

	// Default to 1 cluster if not specified
	return 1
}

// setPlacementCondition sets or updates a condition in the placement status.
func (c *placementController) setPlacementCondition(placement *workloadv1alpha1.Placement, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	// Find existing condition
	for i, existingCondition := range placement.Status.Conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition only if changed
			if existingCondition.Status != status || existingCondition.Reason != reason || existingCondition.Message != message {
				condition.LastTransitionTime = metav1.Now()
				placement.Status.Conditions[i] = condition
			}
			return
		}
	}

	// Add new condition
	condition.LastTransitionTime = metav1.Now()
	placement.Status.Conditions = append(placement.Status.Conditions, condition)
}

// commitStatus commits the placement status changes to the API server.
func (c *placementController) commitStatus(ctx context.Context, placement *workloadv1alpha1.Placement, logger klog.Logger) error {
	clusterName := logicalcluster.From(placement)

	if err := c.committer.Commit(ctx, placement, func(ctx context.Context, placement *workloadv1alpha1.Placement) error {
		_, err := c.kcpClusterClient.Cluster(clusterName.Path()).WorkloadV1alpha1().Placements().UpdateStatus(ctx, placement, metav1.UpdateOptions{})
		return err
	}); err != nil {
		if errors.IsConflict(err) {
			logger.V(3).Info("conflict updating placement status, will retry")
			return nil // Will be retried
		}
		return fmt.Errorf("failed to update placement status: %w", err)
	}

	return nil
}