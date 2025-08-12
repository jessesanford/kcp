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
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Evaluator defines the interface for constraint evaluation implementations.
// Each constraint type has its own evaluator that implements this interface.
type Evaluator interface {
	// Evaluate assesses how well a cluster satisfies a specific constraint for a workload.
	// Returns a ConstraintEvaluation with score (0-100) and satisfaction status.
	Evaluate(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error)
}

// AffinityEvaluator evaluates affinity constraints.
// Affinity constraints prefer clusters that match specific criteria.
type AffinityEvaluator struct{}

// Evaluate checks if the cluster satisfies affinity requirements.
func (e *AffinityEvaluator) Evaluate(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error) {
	result := &ConstraintEvaluation{
		Type:   tmcv1alpha1.AffinityConstraintType,
		Weight: constraint.Weight,
	}

	// Check label selector affinity
	if constraint.Parameters.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(constraint.Parameters.LabelSelector)
		if err != nil {
			result.Score = 0
			result.Satisfied = false
			result.Reason = fmt.Sprintf("Invalid label selector: %v", err)
			return result, nil
		}

		if selector.Matches(labels.Set(cluster.Labels)) {
			result.Score = 100
			result.Satisfied = true
			result.Reason = fmt.Sprintf("Cluster %s matches affinity label selector", cluster.Name)
		} else {
			result.Score = 0
			result.Satisfied = false
			result.Reason = fmt.Sprintf("Cluster %s does not match affinity label selector", cluster.Name)
		}
	}

	// Check scope-based affinity
	if constraint.Parameters.Scope != "" {
		switch constraint.Parameters.Scope {
		case "Cluster":
			// Always satisfied for cluster-level affinity
			result.Score = 100
			result.Satisfied = true
			result.Reason = "Cluster-level affinity satisfied"
		case "Zone":
			// Check if cluster has zones
			if len(cluster.Zones) > 0 {
				result.Score = 100
				result.Satisfied = true
				result.Reason = fmt.Sprintf("Zone-level affinity satisfied, cluster has %d zones", len(cluster.Zones))
			} else {
				result.Score = 0
				result.Satisfied = false
				result.Reason = "Zone-level affinity not satisfied, cluster has no zones"
			}
		case "Node":
			// Assume nodes are available for simplicity
			result.Score = 90
			result.Satisfied = true
			result.Reason = "Node-level affinity satisfied"
		}
	}

	// Default to satisfied if no specific criteria
	if constraint.Parameters.LabelSelector == nil && constraint.Parameters.Scope == "" {
		result.Score = 100
		result.Satisfied = true
		result.Reason = "No specific affinity criteria, constraint satisfied"
	}

	return result, nil
}

// AntiAffinityEvaluator evaluates anti-affinity constraints.
// Anti-affinity constraints avoid clusters that match specific criteria.
type AntiAffinityEvaluator struct{}

// Evaluate checks if the cluster violates anti-affinity requirements.
func (e *AntiAffinityEvaluator) Evaluate(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error) {
	result := &ConstraintEvaluation{
		Type:   tmcv1alpha1.AntiAffinityConstraintType,
		Weight: constraint.Weight,
	}

	// Check label selector anti-affinity
	if constraint.Parameters.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(constraint.Parameters.LabelSelector)
		if err != nil {
			result.Score = 0
			result.Satisfied = false
			result.Reason = fmt.Sprintf("Invalid label selector: %v", err)
			return result, nil
		}

		if selector.Matches(labels.Set(cluster.Labels)) {
			result.Score = 0
			result.Satisfied = false
			result.Reason = fmt.Sprintf("Cluster %s matches anti-affinity label selector (violation)", cluster.Name)
		} else {
			result.Score = 100
			result.Satisfied = true
			result.Reason = fmt.Sprintf("Cluster %s does not match anti-affinity label selector", cluster.Name)
		}
	}

	// Check scope-based anti-affinity
	if constraint.Parameters.Scope != "" {
		switch constraint.Parameters.Scope {
		case "Cluster":
			// Anti-affinity at cluster level - score based on cluster characteristics
			result.Score = 80 // Partial satisfaction
			result.Satisfied = true
			result.Reason = "Cluster-level anti-affinity partially satisfied"
		case "Zone":
			// Prefer clusters with multiple zones for better distribution
			if len(cluster.Zones) > 1 {
				result.Score = 100
				result.Satisfied = true
				result.Reason = fmt.Sprintf("Zone-level anti-affinity satisfied, cluster has %d zones for distribution", len(cluster.Zones))
			} else {
				result.Score = 30
				result.Satisfied = false
				result.Reason = "Zone-level anti-affinity not optimal, cluster has limited zones"
			}
		case "Node":
			// Assume multiple nodes available
			result.Score = 90
			result.Satisfied = true
			result.Reason = "Node-level anti-affinity satisfied"
		}
	}

	// Default to satisfied if no specific criteria
	if constraint.Parameters.LabelSelector == nil && constraint.Parameters.Scope == "" {
		result.Score = 100
		result.Satisfied = true
		result.Reason = "No specific anti-affinity criteria, constraint satisfied"
	}

	return result, nil
}

// TopologyEvaluator evaluates topology spreading constraints.
// Topology constraints ensure workloads are distributed across topology domains.
type TopologyEvaluator struct{}

// Evaluate checks if the cluster supports proper topology spreading.
func (e *TopologyEvaluator) Evaluate(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error) {
	result := &ConstraintEvaluation{
		Type:   tmcv1alpha1.TopologyConstraintType,
		Weight: constraint.Weight,
	}

	topologyKey := constraint.Parameters.TopologyKey
	maxSkew := constraint.Parameters.MaxSkew

	if topologyKey == "" {
		result.Score = 0
		result.Satisfied = false
		result.Reason = "Topology key not specified"
		return result, nil
	}

	// Check if cluster supports the topology key
	switch topologyKey {
	case "topology.kubernetes.io/zone":
		if len(cluster.Zones) == 0 {
			result.Score = 0
			result.Satisfied = false
			result.Reason = "Cluster has no zones for topology spreading"
		} else if len(cluster.Zones) == 1 {
			result.Score = 50
			result.Satisfied = maxSkew >= 1
			result.Reason = fmt.Sprintf("Cluster has 1 zone, limited topology spreading (maxSkew: %d)", maxSkew)
		} else {
			result.Score = 100
			result.Satisfied = true
			result.Reason = fmt.Sprintf("Cluster has %d zones for optimal topology spreading", len(cluster.Zones))
		}
	case "kubernetes.io/hostname":
		// Assume multiple nodes available
		result.Score = 90
		result.Satisfied = true
		result.Reason = "Node-level topology spreading supported"
	case "topology.kubernetes.io/region":
		// Single cluster typically in one region
		result.Score = 60
		result.Satisfied = maxSkew >= 1
		result.Reason = "Region-level topology limited to single cluster region"
	default:
		// Check if topology key matches cluster labels
		if _, exists := cluster.Labels[topologyKey]; exists {
			result.Score = 80
			result.Satisfied = true
			result.Reason = fmt.Sprintf("Custom topology key %s found in cluster labels", topologyKey)
		} else {
			result.Score = 20
			result.Satisfied = false
			result.Reason = fmt.Sprintf("Custom topology key %s not found in cluster", topologyKey)
		}
	}

	return result, nil
}

// ResourceEvaluator evaluates resource-based constraints.
// Resource constraints ensure clusters have sufficient resources for workloads.
type ResourceEvaluator struct{}

// Evaluate checks if the cluster has sufficient resources.
func (e *ResourceEvaluator) Evaluate(ctx context.Context, constraint tmcv1alpha1.Constraint, cluster ClusterSpec, workload WorkloadSpec) (*ConstraintEvaluation, error) {
	result := &ConstraintEvaluation{
		Type:   tmcv1alpha1.ResourceConstraintType,
		Weight: constraint.Weight,
	}

	requiredResources := constraint.Parameters.Resources
	scores := []float64{}
	reasons := []string{}

	// Evaluate CPU requirements
	if requiredResources.CPU != nil && !requiredResources.CPU.IsZero() {
		if cluster.Resources.CPU != nil && !cluster.Resources.CPU.IsZero() {
			cpuRatio := float64(cluster.Resources.CPU.MilliValue()) / float64(requiredResources.CPU.MilliValue())
			if cpuRatio >= 1.0 {
				scores = append(scores, 100)
				reasons = append(reasons, "CPU requirements satisfied")
			} else {
				scores = append(scores, cpuRatio*100)
				reasons = append(reasons, fmt.Sprintf("CPU partially satisfied (%.1f%% available)", cpuRatio*100))
			}
		} else {
			scores = append(scores, 0)
			reasons = append(reasons, "Cluster CPU capacity unknown")
		}
	}

	// Evaluate Memory requirements
	if requiredResources.Memory != nil && !requiredResources.Memory.IsZero() {
		if cluster.Resources.Memory != nil && !cluster.Resources.Memory.IsZero() {
			memRatio := float64(cluster.Resources.Memory.Value()) / float64(requiredResources.Memory.Value())
			if memRatio >= 1.0 {
				scores = append(scores, 100)
				reasons = append(reasons, "Memory requirements satisfied")
			} else {
				scores = append(scores, memRatio*100)
				reasons = append(reasons, fmt.Sprintf("Memory partially satisfied (%.1f%% available)", memRatio*100))
			}
		} else {
			scores = append(scores, 0)
			reasons = append(reasons, "Cluster memory capacity unknown")
		}
	}

	// Evaluate Storage requirements
	if requiredResources.Storage != nil && !requiredResources.Storage.IsZero() {
		if cluster.Resources.Storage != nil && !cluster.Resources.Storage.IsZero() {
			storageRatio := float64(cluster.Resources.Storage.Value()) / float64(requiredResources.Storage.Value())
			if storageRatio >= 1.0 {
				scores = append(scores, 100)
				reasons = append(reasons, "Storage requirements satisfied")
			} else {
				scores = append(scores, storageRatio*100)
				reasons = append(reasons, fmt.Sprintf("Storage partially satisfied (%.1f%% available)", storageRatio*100))
			}
		} else {
			scores = append(scores, 0)
			reasons = append(reasons, "Cluster storage capacity unknown")
		}
	}

	// Calculate overall resource satisfaction score
	if len(scores) == 0 {
		result.Score = 100
		result.Satisfied = true
		result.Reason = "No specific resource requirements"
	} else {
		// Take the minimum score as the overall score (weakest link)
		result.Score = slices.Min(scores)
		result.Satisfied = result.Score >= 100
		result.Reason = strings.Join(reasons, ", ")
	}

	return result, nil
}

// Helper function to compare resource quantities safely.
func compareResourceQuantities(available, required *resource.Quantity) float64 {
	if required == nil || required.IsZero() {
		return 1.0 // No requirement
	}
	if available == nil || available.IsZero() {
		return 0.0 // No capacity
	}
	
	return float64(available.MilliValue()) / float64(required.MilliValue())
}