package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// GetConditions returns the conditions from the PlacementPolicy status.
// This method is required to implement the conditions.Getter interface.
func (p *PlacementPolicy) GetConditions() conditionsv1alpha1.Conditions {
	return p.Status.Conditions
}

// SetConditions sets the conditions on the PlacementPolicy status.
// This method is required to implement the conditions.Setter interface.
func (p *PlacementPolicy) SetConditions(conditions conditionsv1alpha1.Conditions) {
	p.Status.Conditions = conditions
}

// Standard condition types for PlacementPolicy objects.
const (
	// PlacementConditionScheduled indicates whether the placement has been successfully scheduled.
	// This condition is True when all replicas have been assigned to SyncTargets.
	PlacementConditionScheduled conditionsv1alpha1.ConditionType = "Scheduled"

	// PlacementConditionSatisfied indicates whether all placement constraints are satisfied.
	// This includes spread constraints, affinity rules, and resource requirements.
	PlacementConditionSatisfied conditionsv1alpha1.ConditionType = "ConstraintsSatisfied"

	// PlacementConditionSyncTargetsReady indicates whether target SyncTargets are available and ready.
	PlacementConditionSyncTargetsReady conditionsv1alpha1.ConditionType = "SyncTargetsReady"
)

// IsScheduled returns true if the placement policy has been successfully scheduled.
// This means all desired replicas have been assigned to appropriate SyncTargets.
func (p *PlacementPolicy) IsScheduled() bool {
	return p.Status.Phase == PlacementPhaseScheduled &&
		conditions.IsTrue(p, PlacementConditionScheduled)
}

// IsSatisfied returns true if all placement constraints are currently satisfied.
// This includes spread constraints, resource requirements, and affinity preferences.
func (p *PlacementPolicy) IsSatisfied() bool {
	return conditions.IsTrue(p, PlacementConditionSatisfied)
}

// IsFailed returns true if the placement policy is in a failed state.
// This typically means the scheduler could not find a valid placement solution.
func (p *PlacementPolicy) IsFailed() bool {
	return p.Status.Phase == PlacementPhaseFailed
}

// SetCondition sets a condition on the PlacementPolicy status.
// It uses the KCP conditions library for proper condition management.
func (p *PlacementPolicy) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(p, &condition)
}

// GetCondition returns the condition with the specified type.
// Returns nil if the condition is not found.
func (p *PlacementPolicy) GetCondition(conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	return conditions.Get(p, conditionType)
}

// GetPlacementLocations returns all SyncTarget names where replicas are currently placed.
// Useful for tracking the distribution of workloads across locations.
func (p *PlacementPolicy) GetPlacementLocations() []string {
	locations := make([]string, 0, len(p.Status.Placements))
	for _, placement := range p.Status.Placements {
		locations = append(locations, placement.LocationName)
	}
	return locations
}

// GetTotalPlacedReplicas calculates the total number of replicas placed across all locations.
// This should match Status.PlacedReplicas but provides a calculation from placement details.
func (p *PlacementPolicy) GetTotalPlacedReplicas() int32 {
	var total int32
	for _, placement := range p.Status.Placements {
		total += placement.Replicas
	}
	return total
}

// GetPlacementForLocation returns the placement decision for a specific SyncTarget.
// Returns nil if no placement exists for the given location.
func (p *PlacementPolicy) GetPlacementForLocation(locationName string) *PlacementDecision {
	for i := range p.Status.Placements {
		if p.Status.Placements[i].LocationName == locationName {
			return &p.Status.Placements[i]
		}
	}
	return nil
}

// MatchesWorkload checks if this placement policy applies to the specified workload.
// It evaluates API version, kind, name, and label selector criteria.
func (p *PlacementPolicy) MatchesWorkload(apiVersion, kind, name string, workloadLabels map[string]string) bool {
	selector := p.Spec.TargetWorkload

	// Check API version and kind match
	if selector.APIVersion != apiVersion || selector.Kind != kind {
		return false
	}

	// Check name match if specified
	if selector.Name != "" && selector.Name != name {
		return false
	}

	// Check label selector match if specified
	if selector.LabelSelector != nil {
		labelSelector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			return false
		}
		if !labelSelector.Matches(labels.Set(workloadLabels)) {
			return false
		}
	}

	return true
}

// HasLocationSelector returns true if the policy has any location selection criteria.
// Used to determine if the policy constrains placement to specific locations.
func (p *PlacementPolicy) HasLocationSelector() bool {
	return len(p.Spec.LocationSelectors) > 0
}

// HasResourceRequirements returns true if the policy specifies resource requirements.
// Used to determine if resource-aware scheduling is needed.
func (p *PlacementPolicy) HasResourceRequirements() bool {
	reqs := p.Spec.ResourceRequirements
	return len(reqs.Requests) > 0 || len(reqs.Limits) > 0
}

// HasSpreadConstraints returns true if the policy has topology spread constraints.
// Used to determine if topology-aware scheduling is required.
func (p *PlacementPolicy) HasSpreadConstraints() bool {
	return len(p.Spec.SpreadConstraints) > 0
}

// HasAffinityRules returns true if the policy has workload affinity or anti-affinity rules.
// Used to determine if affinity-aware scheduling is needed.
func (p *PlacementPolicy) HasAffinityRules() bool {
	return p.Spec.AffinityRules != nil &&
		(len(p.Spec.AffinityRules.WorkloadAffinity) > 0 ||
			len(p.Spec.AffinityRules.WorkloadAntiAffinity) > 0)
}

// RequiresAdvancedScheduling returns true if the policy needs advanced scheduling features.
// This includes spread constraints, affinity rules, or complex location selection.
func (p *PlacementPolicy) RequiresAdvancedScheduling() bool {
	return p.HasSpreadConstraints() ||
		p.HasAffinityRules() ||
		len(p.Spec.LocationSelectors) > 1 ||
		(len(p.Spec.LocationSelectors) == 1 && p.Spec.LocationSelectors[0].CellSelector != nil)
}

// GetDesiredReplicas returns the desired replica count, applying defaults if necessary.
// Used by controllers to determine the target replica count for scheduling.
func (p *PlacementPolicy) GetDesiredReplicas() int32 {
	if p.Spec.Replicas != nil {
		return *p.Spec.Replicas
	}
	// Return strategy-based default
	switch p.Spec.Strategy {
	case PlacementStrategyHighAvailability:
		return 3
	default:
		return 1
	}
}

// NeedsRescheduling returns true if the placement decisions need to be reconsidered.
// This could be due to changed constraints, failed placements, or external factors.
func (p *PlacementPolicy) NeedsRescheduling() bool {
	// Policy is failed and should be retried
	if p.IsFailed() {
		return true
	}

	// Desired replicas don't match placed replicas
	desired := p.GetDesiredReplicas()
	if p.Status.PlacedReplicas != desired {
		return true
	}

	// Constraints are not satisfied
	if !p.IsSatisfied() {
		return true
	}

	return false
}

// UpdatePlacementStatus updates the placement status based on current placement decisions.
// This is typically called by controllers after making placement changes.
func (p *PlacementPolicy) UpdatePlacementStatus(placements []PlacementDecision) {
	p.Status.Placements = placements
	p.Status.PlacedReplicas = p.GetTotalPlacedReplicas()

	// Update phase based on placement results
	desired := p.GetDesiredReplicas()
	if p.Status.PlacedReplicas == desired && desired > 0 {
		p.Status.Phase = PlacementPhaseScheduled
	} else if p.Status.PlacedReplicas == 0 && desired > 0 {
		p.Status.Phase = PlacementPhaseFailed
	} else {
		p.Status.Phase = PlacementPhaseScheduling
	}

	// Update last schedule time
	now := metav1.Now()
	p.Status.LastScheduleTime = &now
}
