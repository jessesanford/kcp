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

package status

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// AggregatorInterface defines the interface for aggregating cluster status from multiple components.
type AggregatorInterface interface {
	// AggregateClusterStatus aggregates conditions from multiple cluster components
	AggregateClusterStatus(components []ComponentStatus) []conditionsapi.Condition
	
	// ComputeOverallHealth computes overall health based on all conditions
	ComputeOverallHealth(conditions []conditionsapi.Condition) ClusterHealth
	
	// FilterStaleConditions removes conditions that are older than the specified duration
	FilterStaleConditions(conditions []conditionsapi.Condition, maxAge time.Duration) []conditionsapi.Condition
}

// ComponentStatus represents the status of a single cluster component.
type ComponentStatus struct {
	// Name is the component identifier
	Name string
	
	// Conditions are the conditions reported by this component
	Conditions []conditionsapi.Condition
	
	// LastUpdateTime indicates when this component last reported status
	LastUpdateTime metav1.Time
	
	// Critical indicates if this component is critical for cluster operation
	Critical bool
}

// ClusterHealth represents the overall health status of a cluster.
type ClusterHealth string

const (
	// ClusterHealthHealthy indicates all critical components are functioning
	ClusterHealthHealthy ClusterHealth = "Healthy"
	
	// ClusterHealthDegraded indicates some non-critical issues exist
	ClusterHealthDegraded ClusterHealth = "Degraded"
	
	// ClusterHealthUnhealthy indicates critical components have failures
	ClusterHealthUnhealthy ClusterHealth = "Unhealthy"
	
	// ClusterHealthUnknown indicates health cannot be determined
	ClusterHealthUnknown ClusterHealth = "Unknown"
)

// Aggregator implements AggregatorInterface for TMC cluster status aggregation.
type Aggregator struct {
	// healthyThreshold defines the minimum percentage of healthy components required
	healthyThreshold float64
	
	// criticalConditionTypes defines which condition types are critical
	criticalConditionTypes map[conditionsapi.ConditionType]bool
}

// NewAggregator creates a new status aggregator with default settings.
func NewAggregator() AggregatorInterface {
	return NewAggregatorWithConfig(0.8, GetDefaultCriticalConditionTypes())
}

// NewAggregatorWithConfig creates a new aggregator with custom configuration.
//
// Parameters:
//   - healthyThreshold: Minimum percentage (0.0-1.0) of healthy components required for healthy status
//   - criticalConditionTypes: Map of condition types that are considered critical
//
// Returns:
//   - AggregatorInterface: Configured aggregator instance
func NewAggregatorWithConfig(
	healthyThreshold float64,
	criticalConditionTypes map[conditionsapi.ConditionType]bool,
) AggregatorInterface {
	return &Aggregator{
		healthyThreshold:       healthyThreshold,
		criticalConditionTypes: criticalConditionTypes,
	}
}

// GetDefaultCriticalConditionTypes returns the default set of critical condition types.
func GetDefaultCriticalConditionTypes() map[conditionsapi.ConditionType]bool {
	return map[conditionsapi.ConditionType]bool{
		ClusterConnectionCondition:  true,
		ClusterRegistrationCondition: true,
		HeartbeatCondition:          true,
		// PlacementAvailableCondition is important but not critical
		// ResourcesAvailableCondition is important but not critical
		// SyncCondition is important but not critical
	}
}

// AggregateClusterStatus aggregates conditions from multiple cluster components into a unified status.
func (a *Aggregator) AggregateClusterStatus(components []ComponentStatus) []conditionsapi.Condition {
	if len(components) == 0 {
		return []conditionsapi.Condition{
			*conditionsutil.UnknownCondition(
				conditionsapi.ReadyCondition,
				"NoComponents",
				"No component status available",
			),
		}
	}

	// Collect all conditions by type
	conditionsByType := make(map[conditionsapi.ConditionType][]ComponentCondition)
	
	for _, component := range components {
		for _, condition := range component.Conditions {
			componentCondition := ComponentCondition{
				Component: component.Name,
				Condition: condition,
				Critical:  component.Critical,
				LastUpdate: component.LastUpdateTime,
			}
			conditionsByType[condition.Type] = append(conditionsByType[condition.Type], componentCondition)
		}
	}

	// Aggregate each condition type
	var aggregatedConditions []conditionsapi.Condition
	for conditionType, componentConditions := range conditionsByType {
		aggregatedCondition := a.aggregateConditionType(conditionType, componentConditions)
		aggregatedConditions = append(aggregatedConditions, *aggregatedCondition)
	}

	// Sort conditions by type for consistent ordering
	sort.Slice(aggregatedConditions, func(i, j int) bool {
		return string(aggregatedConditions[i].Type) < string(aggregatedConditions[j].Type)
	})

	// Compute overall Ready condition
	readyCondition := a.computeReadyConditionFromAggregated(aggregatedConditions)
	
	// Add Ready condition if it doesn't exist or update it
	found := false
	for i, condition := range aggregatedConditions {
		if condition.Type == conditionsapi.ReadyCondition {
			aggregatedConditions[i] = *readyCondition
			found = true
			break
		}
	}
	if !found {
		aggregatedConditions = append(aggregatedConditions, *readyCondition)
	}

	return aggregatedConditions
}

// ComponentCondition represents a condition from a specific component.
type ComponentCondition struct {
	Component  string
	Condition  conditionsapi.Condition
	Critical   bool
	LastUpdate metav1.Time
}

// aggregateConditionType aggregates conditions of the same type from multiple components.
func (a *Aggregator) aggregateConditionType(
	conditionType conditionsapi.ConditionType,
	componentConditions []ComponentCondition,
) *conditionsapi.Condition {
	if len(componentConditions) == 0 {
		return conditionsutil.UnknownCondition(
			conditionType,
			"NoData",
			"No component data available",
		)
	}

	// Count conditions by status
	trueCount := 0
	falseCount := 0
	unknownCount := 0
	criticalFalseCount := 0
	
	var falseConditions []ComponentCondition
	var criticalFalseConditions []ComponentCondition
	
	mostRecentTime := componentConditions[0].LastUpdate
	
	for _, cc := range componentConditions {
		if cc.LastUpdate.After(mostRecentTime.Time) {
			mostRecentTime = cc.LastUpdate
		}
		
		switch cc.Condition.Status {
		case corev1.ConditionTrue:
			trueCount++
		case corev1.ConditionFalse:
			falseCount++
			falseConditions = append(falseConditions, cc)
			if cc.Critical || a.criticalConditionTypes[conditionType] {
				criticalFalseCount++
				criticalFalseConditions = append(criticalFalseConditions, cc)
			}
		case corev1.ConditionUnknown:
			unknownCount++
		}
	}

	totalCount := len(componentConditions)
	
	// Determine aggregated status
	var status corev1.ConditionStatus
	var reason string
	var message string
	var severity conditionsapi.ConditionSeverity

	// If any critical components have False status, the overall condition is False
	if criticalFalseCount > 0 {
		status = corev1.ConditionFalse
		reason = "CriticalComponentsFailed"
		severity = conditionsapi.ConditionSeverityError
		
		componentNames := make([]string, len(criticalFalseConditions))
		for i, cc := range criticalFalseConditions {
			componentNames[i] = cc.Component
		}
		message = fmt.Sprintf("Critical components failed: %v", componentNames)
		
	} else if unknownCount > 0 && float64(trueCount)/float64(totalCount) < a.healthyThreshold {
		// If too many components are unknown, overall status is unknown
		status = corev1.ConditionUnknown
		reason = "InsufficientData"
		message = fmt.Sprintf("%d/%d components have unknown status", unknownCount, totalCount)
		
	} else if falseCount > 0 && float64(trueCount)/float64(totalCount) < a.healthyThreshold {
		// If too many non-critical components are false, status is degraded
		status = corev1.ConditionFalse
		reason = "ComponentsDegraded"
		severity = conditionsapi.ConditionSeverityWarning
		
		componentNames := make([]string, len(falseConditions))
		for i, cc := range falseConditions {
			componentNames[i] = cc.Component
		}
		message = fmt.Sprintf("Some components degraded: %v", componentNames)
		
	} else {
		// Majority of components are healthy
		status = corev1.ConditionTrue
		reason = "ComponentsHealthy"
		
		if falseCount > 0 {
			message = fmt.Sprintf("%d/%d components healthy, %d degraded", trueCount, totalCount, falseCount)
		} else {
			message = fmt.Sprintf("All %d components healthy", totalCount)
		}
	}

	return &conditionsapi.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: mostRecentTime,
		Reason:             reason,
		Message:            message,
		Severity:           severity,
	}
}

// computeReadyConditionFromAggregated computes the Ready condition based on aggregated conditions.
func (a *Aggregator) computeReadyConditionFromAggregated(conditions []conditionsapi.Condition) *conditionsapi.Condition {
	criticalErrors := 0
	warnings := 0
	unknowns := 0
	
	var criticalErrorTypes []string
	var warningTypes []string
	var unknownTypes []string
	
	for _, condition := range conditions {
		if condition.Type == conditionsapi.ReadyCondition {
			continue // Skip Ready condition to avoid recursion
		}
		
		switch condition.Status {
		case corev1.ConditionFalse:
			if a.criticalConditionTypes[condition.Type] || condition.Severity == conditionsapi.ConditionSeverityError {
				criticalErrors++
				criticalErrorTypes = append(criticalErrorTypes, string(condition.Type))
			} else {
				warnings++
				warningTypes = append(warningTypes, string(condition.Type))
			}
		case corev1.ConditionUnknown:
			if a.criticalConditionTypes[condition.Type] {
				unknowns++
				unknownTypes = append(unknownTypes, string(condition.Type))
			}
		}
	}

	// Determine Ready condition status
	if criticalErrors > 0 {
		return conditionsutil.FalseCondition(
			conditionsapi.ReadyCondition,
			"CriticalConditionsFailed",
			conditionsapi.ConditionSeverityError,
			"Critical conditions failed: %v", criticalErrorTypes,
		)
	}
	
	if unknowns > 0 {
		return conditionsutil.UnknownCondition(
			conditionsapi.ReadyCondition,
			"CriticalConditionsUnknown",
			"Critical conditions unknown: %v", unknownTypes,
		)
	}
	
	message := "Cluster is ready"
	if warnings > 0 {
		message = fmt.Sprintf("Cluster is ready with warnings: %v", warningTypes)
	}
	
	return &conditionsapi.Condition{
		Type:    conditionsapi.ReadyCondition,
		Status:  corev1.ConditionTrue,
		Reason:  "ClusterReady",
		Message: message,
	}
}

// ComputeOverallHealth computes the overall health status based on conditions.
func (a *Aggregator) ComputeOverallHealth(conditions []conditionsapi.Condition) ClusterHealth {
	if len(conditions) == 0 {
		return ClusterHealthUnknown
	}

	criticalErrors := 0
	warnings := 0
	unknowns := 0
	total := 0

	for _, condition := range conditions {
		// Skip the Ready condition as it's computed from others
		if condition.Type == conditionsapi.ReadyCondition {
			continue
		}
		
		total++
		
		switch condition.Status {
		case corev1.ConditionFalse:
			if a.criticalConditionTypes[condition.Type] || condition.Severity == conditionsapi.ConditionSeverityError {
				criticalErrors++
			} else {
				warnings++
			}
		case corev1.ConditionUnknown:
			if a.criticalConditionTypes[condition.Type] {
				unknowns++
			}
		}
	}

	// Determine overall health
	if criticalErrors > 0 {
		return ClusterHealthUnhealthy
	}
	
	if unknowns > 0 && float64(unknowns)/float64(total) > (1.0-a.healthyThreshold) {
		return ClusterHealthUnknown
	}
	
	if warnings > 0 && float64(warnings)/float64(total) > (1.0-a.healthyThreshold) {
		return ClusterHealthDegraded
	}

	return ClusterHealthHealthy
}

// FilterStaleConditions removes conditions that are older than the specified duration.
func (a *Aggregator) FilterStaleConditions(conditions []conditionsapi.Condition, maxAge time.Duration) []conditionsapi.Condition {
	if maxAge <= 0 {
		return conditions
	}

	now := time.Now()
	cutoff := now.Add(-maxAge)
	
	var filtered []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.LastTransitionTime.After(cutoff) {
			filtered = append(filtered, condition)
		}
	}

	return filtered
}

// Health helper functions

// IsHealthy returns true if the cluster health is Healthy.
func IsHealthy(health ClusterHealth) bool {
	return health == ClusterHealthHealthy
}

// IsDegraded returns true if the cluster health is Degraded.
func IsDegraded(health ClusterHealth) bool {
	return health == ClusterHealthDegraded
}

// IsUnhealthy returns true if the cluster health is Unhealthy.
func IsUnhealthy(health ClusterHealth) bool {
	return health == ClusterHealthUnhealthy
}

// IsUnknown returns true if the cluster health is Unknown.
func IsUnknown(health ClusterHealth) bool {
	return health == ClusterHealthUnknown
}