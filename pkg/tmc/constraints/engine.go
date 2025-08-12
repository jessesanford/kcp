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

package constraints

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// ConstraintEngine provides constraint evaluation functionality for placement decisions.
// It supports multiple evaluator types and handles priority-based evaluation with metrics.
type ConstraintEngine struct {
	evaluators map[tmcv1alpha1.ConstraintType]Evaluator
	metrics    *EvaluationMetrics
	mutex      sync.RWMutex
}

// NewConstraintEngine creates a new constraint evaluation engine with the specified options.
func NewConstraintEngine(opts EngineOptions) *ConstraintEngine {
	engine := &ConstraintEngine{
		evaluators: make(map[tmcv1alpha1.ConstraintType]Evaluator),
		mutex:      sync.RWMutex{},
	}
	
	if opts.MetricsEnabled {
		engine.metrics = &EvaluationMetrics{}
	}
	
	// Register default evaluators
	engine.RegisterEvaluator(tmcv1alpha1.AffinityConstraintType, &AffinityEvaluator{})
	engine.RegisterEvaluator(tmcv1alpha1.AntiAffinityConstraintType, &AntiAffinityEvaluator{})
	engine.RegisterEvaluator(tmcv1alpha1.TopologyConstraintType, &TopologyEvaluator{})
	engine.RegisterEvaluator(tmcv1alpha1.ResourceConstraintType, &ResourceEvaluator{})
	
	return engine
}

// Evaluate performs constraint evaluation for the given request.
func (e *ConstraintEngine) Evaluate(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	startTime := time.Now()
	defer func() {
		if e.metrics != nil {
			e.updateMetrics(time.Since(startTime))
		}
	}()
	
	result := &EvaluationResult{
		ClusterEvaluations: make([]*ClusterEvaluation, 0, len(req.Clusters)),
		Conflicts:          e.DetectConflicts(req.Constraints),
		Metrics:            e.metrics,
		Timestamp:          metav1.Now(),
	}
	
	// Evaluate each cluster
	for _, cluster := range req.Clusters {
		evaluation, err := e.evaluateCluster(ctx, cluster, req.Constraints, req.Workload)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate cluster %s: %w", cluster.Name, err)
		}
		result.ClusterEvaluations = append(result.ClusterEvaluations, evaluation)
	}
	
	// Sort by score (highest first)
	sort.Slice(result.ClusterEvaluations, func(i, j int) bool {
		return result.ClusterEvaluations[i].Score > result.ClusterEvaluations[j].Score
	})
	
	return result, nil
}

// RegisterEvaluator registers a custom evaluator for a constraint type.
func (e *ConstraintEngine) RegisterEvaluator(constraintType tmcv1alpha1.ConstraintType, evaluator Evaluator) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.evaluators[constraintType] = evaluator
}

// DetectConflicts identifies conflicts between constraints for resolution.
func (e *ConstraintEngine) DetectConflicts(constraints []tmcv1alpha1.PlacementConstraint) []Conflict {
	var conflicts []Conflict
	affinityCount, antiAffinityCount := 0, 0
	
	for _, constraint := range constraints {
		for _, c := range constraint.Spec.Constraints {
			switch c.Type {
			case tmcv1alpha1.AffinityConstraintType:
				affinityCount++
			case tmcv1alpha1.AntiAffinityConstraintType:
				antiAffinityCount++
			}
		}
	}
	
	if affinityCount > 0 && antiAffinityCount > 0 {
		conflicts = append(conflicts, Conflict{
			Type:        ConflictTypeContradictory,
			Description: "Conflicting affinity and anti-affinity constraints",
			Resolution:  "Review constraint priorities and resolve conflicts",
		})
	}
	
	return conflicts
}

// evaluateCluster evaluates a single cluster against all constraints.
func (e *ConstraintEngine) evaluateCluster(ctx context.Context, cluster ClusterSpec, constraints []tmcv1alpha1.PlacementConstraint, workload WorkloadSpec) (*ClusterEvaluation, error) {
	evaluation := &ClusterEvaluation{
		ClusterName:       cluster.Name,
		ConstraintResults: []*ConstraintEvaluation{},
		Violations:        []string{},
		Suitable:          true,
	}
	
	var totalWeightedScore float64
	var totalWeight int32
	
	for _, placementConstraint := range constraints {
		if !e.workloadMatches(workload, placementConstraint.Spec.WorkloadSelector) {
			continue
		}
		
		for _, constraint := range placementConstraint.Spec.Constraints {
			result, err := e.evaluateConstraint(ctx, constraint, cluster, workload)
			if err != nil {
				return nil, err
			}
			
			result.ConstraintName = placementConstraint.Name
			evaluation.ConstraintResults = append(evaluation.ConstraintResults, result)
			
			if placementConstraint.Spec.EnforcementMode == tmcv1alpha1.EnforcementModeStrict && !result.Satisfied {
				evaluation.Violations = append(evaluation.Violations, result.Reason)
				evaluation.Suitable = false
			}
			
			weight := constraint.Weight
			if weight == 0 {
				weight = 50
			}
			totalWeightedScore += float64(weight) * result.Score
			totalWeight += weight
		}
	}
	
	if totalWeight > 0 {
		evaluation.Score = totalWeightedScore / float64(totalWeight)
	}
	
	return evaluation, nil
}

// evaluateConstraint evaluates a single constraint against a cluster.
func (e *ConstraintEngine) evaluateConstraint(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error) {
	e.mutex.RLock()
	evaluator, exists := e.evaluators[constraint.Type]
	e.mutex.RUnlock()
	
	if !exists {
		return &ConstraintEvaluation{
			Type:      constraint.Type,
			Score:     0,
			Satisfied: false,
			Reason:    fmt.Sprintf("No evaluator for constraint type %s", constraint.Type),
			Weight:    constraint.Weight,
		}, nil
	}
	
	return evaluator.Evaluate(ctx, constraint, cluster, workload)
}

// workloadMatches checks if workload matches constraint selector.
func (e *ConstraintEngine) workloadMatches(workload WorkloadSpec, selector tmcv1alpha1.WorkloadSelector) bool {
	if selector.LabelSelector != nil {
		lblSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil || !lblSelector.Matches(labels.Set(workload.Labels)) {
			return false
		}
	}
	
	if selector.NamespaceSelector != nil {
		nsSelector, err := metav1.LabelSelectorAsSelector(selector.NamespaceSelector)
		if err != nil || !nsSelector.Matches(labels.Set(workload.NamespaceLabels)) {
			return false
		}
	}
	
	return true
}

// updateMetrics updates evaluation performance metrics.
func (e *ConstraintEngine) updateMetrics(duration time.Duration) {
	e.metrics.TotalEvaluations++
	if e.metrics.TotalEvaluations == 1 {
		e.metrics.AverageEvaluationTime = duration
	} else {
		alpha := 0.1
		e.metrics.AverageEvaluationTime = time.Duration(
			alpha*float64(duration) + (1-alpha)*float64(e.metrics.AverageEvaluationTime),
		)
	}
}