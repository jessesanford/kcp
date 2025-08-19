package v1alpha1

import (
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

const (
	// DistributionConditionDistributed indicates successful distribution across all locations
	DistributionConditionDistributed = "Distributed"

	// DistributionConditionProgressing indicates ongoing distribution process
	DistributionConditionProgressing = "Progressing"

	// DistributionConditionAvailable indicates workload availability across locations
	DistributionConditionAvailable = "Available"

	// DistributionConditionPaused indicates distribution is paused
	DistributionConditionPaused = "Paused"

	// DistributionConditionFailed indicates distribution has failed
	DistributionConditionFailed = "Failed"
)

// IsDistributed returns true if the WorkloadDistribution is fully distributed
func (d *WorkloadDistribution) IsDistributed() bool {
	return d.Status.Phase == DistributionPhaseDistributed && conditions.IsTrue(d, DistributionConditionDistributed)
}

// IsProgressing returns true if distribution is in progress
func (d *WorkloadDistribution) IsProgressing() bool {
	return d.Status.Phase == DistributionPhaseDistributing || conditions.IsTrue(d, DistributionConditionProgressing)
}

// IsAvailable returns true if the workload is available across locations
func (d *WorkloadDistribution) IsAvailable() bool {
	return conditions.IsTrue(d, DistributionConditionAvailable)
}

// IsPaused returns true if distribution is paused
func (d *WorkloadDistribution) IsPaused() bool {
	return d.Spec.Paused || d.Status.Phase == DistributionPhasePaused
}

// IsFailed returns true if distribution has failed
func (d *WorkloadDistribution) IsFailed() bool {
	return d.Status.Phase == DistributionPhaseFailed || conditions.IsTrue(d, DistributionConditionFailed)
}

// SetCondition sets a condition on the WorkloadDistribution
func (d *WorkloadDistribution) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(d, condition)
}

// GetCondition returns the condition with the specified type
func (d *WorkloadDistribution) GetCondition(conditionType string) *conditionsv1alpha1.Condition {
	return conditions.Get(d, conditionType)
}

// GetLocationStatus returns status for a specific location
func (d *WorkloadDistribution) GetLocationStatus(locationName string) *LocationStatus {
	for i := range d.Status.LocationStatuses {
		if d.Status.LocationStatuses[i].LocationName == locationName {
			return &d.Status.LocationStatuses[i]
		}
	}
	return nil
}

// SetLocationStatus sets or updates status for a specific location
func (d *WorkloadDistribution) SetLocationStatus(status LocationStatus) {
	for i := range d.Status.LocationStatuses {
		if d.Status.LocationStatuses[i].LocationName == status.LocationName {
			d.Status.LocationStatuses[i] = status
			return
		}
	}
	// Location not found, append new status
	d.Status.LocationStatuses = append(d.Status.LocationStatuses, status)
}

// RemoveLocationStatus removes status for a specific location
func (d *WorkloadDistribution) RemoveLocationStatus(locationName string) {
	for i := range d.Status.LocationStatuses {
		if d.Status.LocationStatuses[i].LocationName == locationName {
			d.Status.LocationStatuses = append(d.Status.LocationStatuses[:i],
				d.Status.LocationStatuses[i+1:]...)
			return
		}
	}
}

// CalculateReplicasPerLocation computes how replicas should be distributed across locations
func (d *WorkloadDistribution) CalculateReplicasPerLocation(locations []string) map[string]int32 {
	result := make(map[string]int32)

	// If explicit distributions provided, use them
	if len(d.Spec.Distributions) > 0 {
		for _, dist := range d.Spec.Distributions {
			result[dist.LocationName] = dist.Replicas
		}
		return result
	}

	// Otherwise, distribute evenly across provided locations
	if len(locations) == 0 {
		return result
	}

	baseReplicas := d.Spec.TotalReplicas / int32(len(locations))
	remainder := d.Spec.TotalReplicas % int32(len(locations))

	for i, location := range locations {
		result[location] = baseReplicas
		// Distribute remainder to first N locations
		if int32(i) < remainder {
			result[location]++
		}
	}

	return result
}

// GetSortedLocationsByPriority returns locations sorted by priority for rollout ordering
func (d *WorkloadDistribution) GetSortedLocationsByPriority() []string {
	if len(d.Spec.Distributions) == 0 {
		return nil
	}

	// Create a map of locations with their priorities
	locationPriorities := make(map[string]int32)
	for _, dist := range d.Spec.Distributions {
		priority := int32(50) // default priority
		if dist.Priority != nil {
			priority = *dist.Priority
		}
		locationPriorities[dist.LocationName] = priority
	}

	// Sort locations by priority (lower values = higher priority)
	locations := make([]string, 0, len(locationPriorities))
	for location := range locationPriorities {
		locations = append(locations, location)
	}

	// Simple bubble sort by priority
	for i := 0; i < len(locations)-1; i++ {
		for j := 0; j < len(locations)-i-1; j++ {
			if locationPriorities[locations[j]] > locationPriorities[locations[j+1]] {
				locations[j], locations[j+1] = locations[j+1], locations[j]
			}
		}
	}

	return locations
}

// GetResourceOverrideForLocation returns the resource override for a specific location
func (d *WorkloadDistribution) GetResourceOverrideForLocation(locationName string) *ResourceOverride {
	for i := range d.Spec.ResourceOverrides {
		if d.Spec.ResourceOverrides[i].LocationName == locationName {
			return &d.Spec.ResourceOverrides[i]
		}
	}
	return nil
}

// IsRolloutComplete returns true if rollout is complete based on strategy
func (d *WorkloadDistribution) IsRolloutComplete() bool {
	return d.Status.UpdatedReplicas == d.Spec.TotalReplicas && d.Status.ReadyReplicas == d.Spec.TotalReplicas
}
