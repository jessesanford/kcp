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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

// DecisionEngine handles advanced placement decision making with sophisticated
// scoring, constraint evaluation, and selection strategies.
type DecisionEngine struct {
	logger klog.Logger
}

// NewDecisionEngine creates a new decision engine for advanced placement processing.
func NewDecisionEngine(logger klog.Logger) *DecisionEngine {
	return &DecisionEngine{
		logger: logger.WithName("decision-engine"),
	}
}

// DecisionConfig contains configuration for placement decision making.
type DecisionConfig struct {
	// Strategy defines the selection strategy to use
	Strategy SelectionStrategy
	// ScoringWeights define how to weight different scoring criteria
	ScoringWeights ScoringWeights
	// MaxCandidates limits how many locations to consider
	MaxCandidates int
}

// SelectionStrategy defines different strategies for selecting locations.
type SelectionStrategy string

const (
	// SelectionStrategyBalanced spreads workloads evenly across available clusters
	SelectionStrategyBalanced SelectionStrategy = "balanced"
	// SelectionStrategyPacked concentrates workloads on fewer clusters
	SelectionStrategyPacked SelectionStrategy = "packed"
	// SelectionStrategySpread maximizes geographic or zone spread
	SelectionStrategySpread SelectionStrategy = "spread"
	// SelectionStrategyScore selects based purely on highest scores
	SelectionStrategyScore SelectionStrategy = "score"
)

// ScoringWeights define the relative importance of different scoring factors.
type ScoringWeights struct {
	// LocationAffinity weights how well a location matches affinity rules (0-100)
	LocationAffinity int32
	// ResourceCapacity weights available resource capacity (0-100)  
	ResourceCapacity int32
	// WorkloadSpread weights even distribution of workloads (0-100)
	WorkloadSpread int32
	// NetworkLatency weights network performance considerations (0-100)
	NetworkLatency int32
}

// DefaultScoringWeights provides sensible default scoring weights.
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		LocationAffinity: 40,
		ResourceCapacity: 25,
		WorkloadSpread:   20,
		NetworkLatency:   15,
	}
}

// LocationCandidate represents a location being considered for placement
// along with its computed score and evaluation details.
type LocationCandidate struct {
	Location *workloadv1alpha1.Location
	Score    int32
	Reasons  []string
	Details  ScoringDetails
}

// ScoringDetails provides breakdown of how a location's score was calculated.
type ScoringDetails struct {
	AffinityScore    int32
	CapacityScore    int32
	SpreadScore      int32
	LatencyScore     int32
	ConstraintsPassed bool
	ConstraintErrors []string
}

// MakeAdvancedPlacementDecisions creates placement decisions using advanced algorithms
// that consider multiple factors like affinity, capacity, and distribution strategies.
func (e *DecisionEngine) MakeAdvancedPlacementDecisions(
	ctx context.Context,
	placement *workloadv1alpha1.Placement,
	locations []*workloadv1alpha1.Location,
	config DecisionConfig,
) ([]workloadv1alpha1.PlacementDecision, error) {
	
	logger := e.logger.WithValues("placement", placement.Name, "strategy", config.Strategy)
	logger.V(4).Info("starting advanced placement decision making", "locations", len(locations))

	// Step 1: Filter locations based on hard constraints
	eligibleCandidates, err := e.filterLocationsByConstraints(placement, locations)
	if err != nil {
		return nil, fmt.Errorf("failed to filter locations by constraints: %w", err)
	}

	if len(eligibleCandidates) == 0 {
		logger.V(2).Info("no locations passed constraint filtering")
		return []workloadv1alpha1.PlacementDecision{}, nil
	}

	logger.V(4).Info("locations passed constraint filtering", "eligible", len(eligibleCandidates))

	// Step 2: Score all eligible candidates
	err = e.scoreLocationCandidates(placement, eligibleCandidates, config.ScoringWeights)
	if err != nil {
		return nil, fmt.Errorf("failed to score location candidates: %w", err)
	}

	// Step 3: Apply selection strategy to choose final locations
	selectedCandidates := e.selectLocationsByStrategy(placement, eligibleCandidates, config)
	
	logger.V(2).Info("completed location selection", "selected", len(selectedCandidates))

	// Step 4: Convert candidates to placement decisions
	decisions := e.createPlacementDecisions(selectedCandidates)

	logger.V(2).Info("created advanced placement decisions", "count", len(decisions))
	return decisions, nil
}

// filterLocationsByConstraints filters locations based on hard placement constraints
// such as affinity/anti-affinity rules and tolerations.
func (e *DecisionEngine) filterLocationsByConstraints(
	placement *workloadv1alpha1.Placement,
	locations []*workloadv1alpha1.Location,
) ([]*LocationCandidate, error) {

	var candidates []*LocationCandidate

	for _, location := range locations {
		candidate := &LocationCandidate{
			Location: location,
			Details: ScoringDetails{
				ConstraintsPassed: true,
			},
		}

		// Check location selector matching
		if !e.locationMatchesAdvancedSelector(location, placement.Spec.LocationSelector) {
			candidate.Details.ConstraintsPassed = false
			candidate.Details.ConstraintErrors = append(candidate.Details.ConstraintErrors,
				"location does not match selector criteria")
			continue
		}

		// Check placement constraints
		if placement.Spec.Constraints != nil {
			if err := e.evaluateConstraints(candidate, placement.Spec.Constraints); err != nil {
				candidate.Details.ConstraintsPassed = false
				candidate.Details.ConstraintErrors = append(candidate.Details.ConstraintErrors, err.Error())
				continue
			}
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// locationMatchesAdvancedSelector checks if a location matches selector with full
// MatchExpressions support.
func (e *DecisionEngine) locationMatchesAdvancedSelector(
	location *workloadv1alpha1.Location, 
	selector *workloadv1alpha1.LocationSelector,
) bool {
	if selector == nil {
		return true
	}

	// Check match labels
	if selector.MatchLabels != nil {
		for key, value := range selector.MatchLabels {
			if location.Labels[key] != value {
				return false
			}
		}
	}

	// Check match expressions
	if selector.MatchExpressions != nil {
		for _, expr := range selector.MatchExpressions {
			if !e.evaluateLabelSelectorRequirement(location.Labels, expr) {
				return false
			}
		}
	}

	return true
}

// evaluateLabelSelectorRequirement evaluates a single label selector requirement.
func (e *DecisionEngine) evaluateLabelSelectorRequirement(
	labels map[string]string, 
	req metav1.LabelSelectorRequirement,
) bool {
	switch req.Operator {
	case metav1.LabelSelectorOpIn:
		value, exists := labels[req.Key]
		if !exists {
			return false
		}
		for _, v := range req.Values {
			if value == v {
				return true
			}
		}
		return false

	case metav1.LabelSelectorOpNotIn:
		value, exists := labels[req.Key]
		if !exists {
			return true
		}
		for _, v := range req.Values {
			if value == v {
				return false
			}
		}
		return true

	case metav1.LabelSelectorOpExists:
		_, exists := labels[req.Key]
		return exists

	case metav1.LabelSelectorOpDoesNotExist:
		_, exists := labels[req.Key]
		return !exists

	default:
		e.logger.V(2).Info("unknown label selector operator", "operator", req.Operator)
		return false
	}
}

// evaluateConstraints checks if a location candidate satisfies placement constraints.
func (e *DecisionEngine) evaluateConstraints(
	candidate *LocationCandidate,
	constraints *workloadv1alpha1.PlacementConstraints,
) error {
	
	// Check affinity constraints
	if constraints.Affinity != nil {
		if err := e.evaluateAffinityConstraints(candidate, constraints.Affinity); err != nil {
			return fmt.Errorf("affinity constraint failed: %w", err)
		}
	}

	// Check tolerations (placeholder for future implementation)
	if constraints.Tolerations != nil && len(constraints.Tolerations) > 0 {
		if err := e.evaluateTolerations(candidate, constraints.Tolerations); err != nil {
			return fmt.Errorf("toleration constraint failed: %w", err)
		}
	}

	return nil
}

// evaluateAffinityConstraints checks if a location satisfies affinity rules.
func (e *DecisionEngine) evaluateAffinityConstraints(
	candidate *LocationCandidate,
	affinity *workloadv1alpha1.PlacementAffinity,
) error {
	
	location := candidate.Location

	// Check required affinity (hard constraint)
	if affinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, term := range affinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			if !e.nodeSelectionTermMatches(location, term) {
				return fmt.Errorf("required affinity constraint not satisfied")
			}
		}
	}

	return nil
}

// nodeSelectionTermMatches checks if a location matches a node selector term.
func (e *DecisionEngine) nodeSelectionTermMatches(
	location *workloadv1alpha1.Location,
	term corev1.NodeSelectorTerm,
) bool {
	
	// Check match expressions
	for _, expr := range term.MatchExpressions {
		if !e.evaluateNodeSelectorRequirement(location.Labels, expr) {
			return false
		}
	}

	// Check match fields (if any - location-specific logic)
	for _, field := range term.MatchFields {
		if !e.evaluateNodeSelectorFieldRequirement(location, field) {
			return false
		}
	}

	return true
}

// evaluateNodeSelectorRequirement evaluates node selector requirements.
func (e *DecisionEngine) evaluateNodeSelectorRequirement(
	labels map[string]string,
	req corev1.NodeSelectorRequirement,
) bool {
	switch req.Operator {
	case corev1.NodeSelectorOpIn:
		value, exists := labels[req.Key]
		if !exists {
			return false
		}
		for _, v := range req.Values {
			if value == v {
				return true
			}
		}
		return false

	case corev1.NodeSelectorOpNotIn:
		value, exists := labels[req.Key]
		if !exists {
			return true
		}
		for _, v := range req.Values {
			if value == v {
				return false
			}
		}
		return true

	case corev1.NodeSelectorOpExists:
		_, exists := labels[req.Key]
		return exists

	case corev1.NodeSelectorOpDoesNotExist:
		_, exists := labels[req.Key]
		return !exists

	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		// Numeric comparisons - implement if needed for location properties
		return false

	default:
		return false
	}
}

// evaluateNodeSelectorFieldRequirement evaluates field-based requirements.
func (e *DecisionEngine) evaluateNodeSelectorFieldRequirement(
	location *workloadv1alpha1.Location,
	req corev1.NodeSelectorRequirement,
) bool {
	// Implement location-specific field matching if needed
	// For now, return true as locations may not have specific field requirements
	return true
}

// evaluateTolerations checks if location taints are tolerated.
func (e *DecisionEngine) evaluateTolerations(
	candidate *LocationCandidate,
	tolerations []corev1.Toleration,
) error {
	// Placeholder implementation - location taints would need to be defined
	// in the Location API first. For now, assume all tolerations pass.
	return nil
}