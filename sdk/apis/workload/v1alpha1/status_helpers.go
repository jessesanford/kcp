package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

const (
	// StatusAggregationConditionHealthy indicates overall health
	StatusAggregationConditionHealthy = "Healthy"

	// StatusAggregationConditionAggregated indicates aggregation done
	StatusAggregationConditionAggregated = "Aggregated"
)

// IsHealthy returns true if aggregated status is healthy
func (s *WorkloadStatusAggregation) IsHealthy() bool {
	return conditions.IsTrue(s, StatusAggregationConditionHealthy)
}

// IsAggregated returns true if aggregation is complete
func (s *WorkloadStatusAggregation) IsAggregated() bool {
	return conditions.IsTrue(s, StatusAggregationConditionAggregated)
}

// SetCondition sets a condition
func (s *WorkloadStatusAggregation) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(s, condition)
}

// GetLocationStatus returns status for a location
func (s *WorkloadStatusAggregation) GetLocationStatus(locationName string) *AggregatedLocationStatus {
	for i := range s.Status.LocationStatuses {
		if s.Status.LocationStatuses[i].LocationName == locationName {
			return &s.Status.LocationStatuses[i]
		}
	}
	return nil
}

// CalculateAggregatedPhase determines overall phase
func (s *WorkloadStatusAggregation) CalculateAggregatedPhase() WorkloadPhase {
	if len(s.Status.LocationStatuses) == 0 {
		return WorkloadPhaseUnknown
	}

	policy := s.Spec.AggregationPolicy
	if policy == nil {
		policy = &AggregationPolicy{Strategy: AggregationStrategyPessimistic}
	}

	switch policy.Strategy {
	case AggregationStrategyOptimistic:
		// If any location is running, overall is running
		for _, loc := range s.Status.LocationStatuses {
			if loc.Phase == WorkloadPhaseRunning || loc.Phase == WorkloadPhaseSucceeded {
				return loc.Phase
			}
		}
		return WorkloadPhaseFailed

	case AggregationStrategyPessimistic:
		// If any location is failed, overall is failed
		for _, loc := range s.Status.LocationStatuses {
			if loc.Phase == WorkloadPhaseFailed {
				return WorkloadPhaseFailed
			}
		}
		// All must be running/succeeded
		return WorkloadPhaseRunning

	case AggregationStrategyMajority:
		// Count phases and return majority
		phaseCounts := make(map[WorkloadPhase]int)
		for _, loc := range s.Status.LocationStatuses {
			phaseCounts[loc.Phase]++
		}

		var maxPhase WorkloadPhase
		maxCount := 0
		for phase, count := range phaseCounts {
			if count > maxCount {
				maxPhase = phase
				maxCount = count
			}
		}
		return maxPhase

	default:
		return WorkloadPhaseUnknown
	}
}

// GetHealthPercentage returns health percentage
func (s *WorkloadStatusAggregation) GetHealthPercentage() float64 {
	if s.Status.TotalLocations == 0 {
		return 0
	}
	return float64(s.Status.HealthyLocations) / float64(s.Status.TotalLocations) * 100
}

// UpdateLocationStatus updates status for a specific location
func (s *WorkloadStatusAggregation) UpdateLocationStatus(locationName string, phase WorkloadPhase, healthy bool, extractedFields runtime.RawExtension) {
	// Find existing location status
	for i := range s.Status.LocationStatuses {
		if s.Status.LocationStatuses[i].LocationName == locationName {
			s.Status.LocationStatuses[i].Phase = phase
			s.Status.LocationStatuses[i].Healthy = healthy
			s.Status.LocationStatuses[i].ExtractedFields = extractedFields
			return
		}
	}

	// Add new location status
	s.Status.LocationStatuses = append(s.Status.LocationStatuses, AggregatedLocationStatus{
		LocationName:    locationName,
		Phase:           phase,
		Healthy:         healthy,
		ExtractedFields: extractedFields,
	})
}

// RecalculateHealthCounts recalculates health counts from location statuses
func (s *WorkloadStatusAggregation) RecalculateHealthCounts() {
	s.Status.TotalLocations = int32(len(s.Status.LocationStatuses))
	s.Status.HealthyLocations = 0
	s.Status.UnhealthyLocations = 0

	for _, loc := range s.Status.LocationStatuses {
		if loc.Healthy {
			s.Status.HealthyLocations++
		} else {
			s.Status.UnhealthyLocations++
		}
	}
}

// IsMinimumHealthMet checks if minimum health requirements are met
func (s *WorkloadStatusAggregation) IsMinimumHealthMet() bool {
	if s.Spec.AggregationPolicy == nil {
		return s.Status.HealthyLocations > 0
	}

	policy := s.Spec.AggregationPolicy

	if policy.RequireAllLocations {
		return s.Status.HealthyLocations == s.Status.TotalLocations
	}

	if policy.MinLocations > 0 {
		return s.Status.HealthyLocations >= policy.MinLocations
	}

	// Default to at least one healthy location
	return s.Status.HealthyLocations > 0
}
