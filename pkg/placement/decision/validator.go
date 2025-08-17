/*
Copyright 2023 The KCP Authors.

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

package decision

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// defaultDecisionValidator implements the DecisionValidator interface.
type defaultDecisionValidator struct {
	capacityTracker schedulerapi.CapacityTracker
}

// NewDecisionValidator creates a new decision validator.
func NewDecisionValidator(capacityTracker schedulerapi.CapacityTracker) DecisionValidator {
	return &defaultDecisionValidator{
		capacityTracker: capacityTracker,
	}
}

// ValidateDecision validates a placement decision against constraints and policies.
func (v *defaultDecisionValidator) ValidateDecision(ctx context.Context, decision *PlacementDecision) error {
	if decision == nil {
		return fmt.Errorf("decision cannot be nil")
	}

	klog.V(3).InfoS("Validating placement decision", "decisionID", decision.ID)

	// Validate resource constraints
	if err := v.ValidateResourceConstraints(ctx, decision.SelectedWorkspaces); err != nil {
		return fmt.Errorf("resource constraint validation failed: %w", err)
	}

	// Validate policy compliance
	if err := v.ValidatePolicyCompliance(ctx, decision); err != nil {
		return fmt.Errorf("policy compliance validation failed: %w", err)
	}

	// Check for conflicts
	conflicts, err := v.CheckConflicts(ctx, decision)
	if err != nil {
		return fmt.Errorf("conflict checking failed: %w", err)
	}

	// Report critical conflicts as validation failures
	for _, conflict := range conflicts {
		if conflict.Severity == SeverityCritical {
			return fmt.Errorf("critical conflict detected: %s", conflict.Description)
		}
	}

	// Log non-critical conflicts as warnings
	for _, conflict := range conflicts {
		if conflict.Severity != SeverityCritical {
			klog.V(2).InfoS("Non-critical conflict detected",
				"decisionID", decision.ID,
				"conflictType", conflict.Type,
				"severity", conflict.Severity,
				"description", conflict.Description)
		}
	}

	klog.V(3).InfoS("Placement decision validation completed", "decisionID", decision.ID)
	return nil
}

// ValidateResourceConstraints validates resource allocation constraints.
func (v *defaultDecisionValidator) ValidateResourceConstraints(ctx context.Context, placements []*WorkspacePlacement) error {
	if len(placements) == 0 {
		return nil // No placements to validate
	}

	for _, placement := range placements {
		if err := v.validateSingleWorkspaceResources(ctx, placement); err != nil {
			return fmt.Errorf("resource validation failed for workspace %s: %w", placement.Workspace, err)
		}
	}

	return nil
}

// validateSingleWorkspaceResources validates resource constraints for a single workspace.
func (v *defaultDecisionValidator) validateSingleWorkspaceResources(ctx context.Context, placement *WorkspacePlacement) error {
	if v.capacityTracker == nil {
		klog.V(3).InfoS("No capacity tracker available, skipping resource validation")
		return nil
	}

	// Get current available capacity
	availableCapacity, err := v.capacityTracker.GetAvailableCapacity(placement.Workspace)
	if err != nil {
		return fmt.Errorf("failed to get available capacity: %w", err)
	}

	if availableCapacity == nil {
		return fmt.Errorf("no capacity information available for workspace %s", placement.Workspace)
	}

	// Check CPU constraint
	if placement.AllocatedResources.CPU.Cmp(availableCapacity.CPU) > 0 {
		return fmt.Errorf("CPU allocation (%s) exceeds available capacity (%s)",
			placement.AllocatedResources.CPU.String(),
			availableCapacity.CPU.String())
	}

	// Check memory constraint
	if placement.AllocatedResources.Memory.Cmp(availableCapacity.Memory) > 0 {
		return fmt.Errorf("memory allocation (%s) exceeds available capacity (%s)",
			placement.AllocatedResources.Memory.String(),
			availableCapacity.Memory.String())
	}

	// Check storage constraint
	if placement.AllocatedResources.Storage.Cmp(availableCapacity.Storage) > 0 {
		return fmt.Errorf("storage allocation (%s) exceeds available capacity (%s)",
			placement.AllocatedResources.Storage.String(),
			availableCapacity.Storage.String())
	}

	// Check custom resource constraints
	for resourceName, allocatedQuantity := range placement.AllocatedResources.CustomResources {
		if availableQuantity, exists := availableCapacity.CustomResources[resourceName]; exists {
			if allocatedQuantity.Cmp(availableQuantity) > 0 {
				return fmt.Errorf("custom resource %s allocation (%s) exceeds available capacity (%s)",
					resourceName,
					allocatedQuantity.String(),
					availableQuantity.String())
			}
		} else {
			return fmt.Errorf("custom resource %s not available in workspace %s", resourceName, placement.Workspace)
		}
	}

	return nil
}

// ValidatePolicyCompliance validates policy compliance for the decision.
func (v *defaultDecisionValidator) ValidatePolicyCompliance(ctx context.Context, decision *PlacementDecision) error {
	// Validate that at least one workspace is selected if required
	if len(decision.SelectedWorkspaces) == 0 && decision.Status == DecisionStatusComplete {
		// Check if this is acceptable based on the original request
		// For now, allow empty selections as they might be valid in some scenarios
		klog.V(2).InfoS("No workspaces selected in decision", "decisionID", decision.ID)
	}

	// Validate workspace selection policies
	for _, placement := range decision.SelectedWorkspaces {
		if err := v.validateWorkspacePolicy(ctx, placement, decision); err != nil {
			return fmt.Errorf("workspace policy validation failed for %s: %w", placement.Workspace, err)
		}
	}

	// Validate override policies
	if decision.Override != nil {
		if err := v.validateOverridePolicy(ctx, decision.Override); err != nil {
			return fmt.Errorf("override policy validation failed: %w", err)
		}
	}

	return nil
}

// validateWorkspacePolicy validates policy compliance for a single workspace placement.
func (v *defaultDecisionValidator) validateWorkspacePolicy(ctx context.Context, placement *WorkspacePlacement, decision *PlacementDecision) error {
	// Validate minimum score requirements
	if placement.FinalScore < 0 {
		return fmt.Errorf("final score cannot be negative: %f", placement.FinalScore)
	}

	// Validate score consistency
	if placement.SchedulerScore < 0 || placement.SchedulerScore > 100 {
		return fmt.Errorf("scheduler score out of range [0-100]: %f", placement.SchedulerScore)
	}

	if placement.CELScore < 0 {
		return fmt.Errorf("CEL score cannot be negative: %f", placement.CELScore)
	}

	// Validate resource allocation has proper identifiers
	if placement.AllocatedResources.ReservationID == "" {
		return fmt.Errorf("allocated resources must have a reservation ID")
	}

	// Validate allocation expiration is in the future
	if placement.AllocatedResources.ExpiresAt.Before(decision.DecisionTime) {
		return fmt.Errorf("resource allocation expires before decision time")
	}

	return nil
}

// validateOverridePolicy validates policy compliance for placement overrides.
func (v *defaultDecisionValidator) validateOverridePolicy(ctx context.Context, override *PlacementOverride) error {
	if override == nil {
		return nil
	}

	// Validate override type
	switch override.OverrideType {
	case OverrideTypeForce, OverrideTypeExclude, OverrideTypePrefer, OverrideTypeAvoid:
		// Valid types
	default:
		return fmt.Errorf("invalid override type: %s", override.OverrideType)
	}

	// Validate that force overrides have target workspaces
	if override.OverrideType == OverrideTypeForce && len(override.TargetWorkspaces) == 0 {
		return fmt.Errorf("force override must specify target workspaces")
	}

	// Validate that exclude overrides have excluded workspaces
	if override.OverrideType == OverrideTypeExclude && len(override.ExcludedWorkspaces) == 0 {
		return fmt.Errorf("exclude override must specify excluded workspaces")
	}

	// Validate override expiration
	if override.ExpiresAt != nil && override.ExpiresAt.Before(override.CreatedAt) {
		return fmt.Errorf("override expiration time cannot be before creation time")
	}

	// Validate applied by is not empty
	if override.AppliedBy == "" {
		return fmt.Errorf("override must specify who applied it")
	}

	return nil
}

// CheckConflicts checks for conflicts with existing placements.
func (v *defaultDecisionValidator) CheckConflicts(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error) {
	var conflicts []ConflictDescription

	// Check for resource overcommitment conflicts
	resourceConflicts, err := v.checkResourceConflicts(ctx, decision)
	if err != nil {
		return nil, fmt.Errorf("failed to check resource conflicts: %w", err)
	}
	conflicts = append(conflicts, resourceConflicts...)

	// Check for affinity/anti-affinity conflicts
	affinityConflicts := v.checkAffinityConflicts(ctx, decision)
	conflicts = append(conflicts, affinityConflicts...)

	// Check for policy conflicts
	policyConflicts := v.checkPolicyConflicts(ctx, decision)
	conflicts = append(conflicts, policyConflicts...)

	return conflicts, nil
}

// checkResourceConflicts checks for resource overcommitment conflicts.
func (v *defaultDecisionValidator) checkResourceConflicts(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error) {
	var conflicts []ConflictDescription

	if v.capacityTracker == nil {
		return conflicts, nil
	}

	for _, placement := range decision.SelectedWorkspaces {
		// Get total capacity
		totalCapacity, err := v.capacityTracker.GetCapacity(placement.Workspace)
		if err != nil {
			klog.V(3).InfoS("Failed to get capacity for conflict checking", 
				"workspace", placement.Workspace, "error", err)
			continue
		}

		// Get available capacity
		availableCapacity, err := v.capacityTracker.GetAvailableCapacity(placement.Workspace)
		if err != nil {
			klog.V(3).InfoS("Failed to get available capacity for conflict checking",
				"workspace", placement.Workspace, "error", err)
			continue
		}

		// Check if allocation would overcommit resources
		conflicts = append(conflicts, v.checkResourceOvercommit(placement, totalCapacity, availableCapacity)...)
	}

	return conflicts, nil
}

// checkResourceOvercommit checks for resource overcommitment in a single workspace.
func (v *defaultDecisionValidator) checkResourceOvercommit(
	placement *WorkspacePlacement,
	totalCapacity, availableCapacity *schedulerapi.ResourceCapacity,
) []ConflictDescription {
	var conflicts []ConflictDescription

	// Check CPU overcommitment
	if placement.AllocatedResources.CPU.Cmp(availableCapacity.CPU) > 0 {
		utilizationPercent := float64(totalCapacity.CPU.MilliValue()-availableCapacity.CPU.MilliValue()) / float64(totalCapacity.CPU.MilliValue()) * 100
		severity := SeverityMedium
		if utilizationPercent > 90 {
			severity = SeverityHigh
		}
		if placement.AllocatedResources.CPU.Cmp(resource.MustParse("0")) > 0 {
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypeResourceOvercommit,
				Description: fmt.Sprintf("CPU allocation would exceed available capacity in workspace %s (requested: %s, available: %s)", 
					placement.Workspace, placement.AllocatedResources.CPU.String(), availableCapacity.CPU.String()),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:          severity,
				ResolutionSuggestion: "Consider reducing resource requirements or selecting a different workspace",
			})
		}
	}

	// Check memory overcommitment
	if placement.AllocatedResources.Memory.Cmp(availableCapacity.Memory) > 0 {
		utilizationPercent := float64(totalCapacity.Memory.Value()-availableCapacity.Memory.Value()) / float64(totalCapacity.Memory.Value()) * 100
		severity := SeverityMedium
		if utilizationPercent > 90 {
			severity = SeverityHigh
		}
		if placement.AllocatedResources.Memory.Cmp(resource.MustParse("0")) > 0 {
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypeResourceOvercommit,
				Description: fmt.Sprintf("Memory allocation would exceed available capacity in workspace %s (requested: %s, available: %s)",
					placement.Workspace, placement.AllocatedResources.Memory.String(), availableCapacity.Memory.String()),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:          severity,
				ResolutionSuggestion: "Consider reducing memory requirements or selecting a different workspace",
			})
		}
	}

	return conflicts
}

// checkAffinityConflicts checks for affinity and anti-affinity conflicts.
func (v *defaultDecisionValidator) checkAffinityConflicts(ctx context.Context, decision *PlacementDecision) []ConflictDescription {
	var conflicts []ConflictDescription

	// This is a placeholder for affinity conflict checking
	// In a real implementation, this would check against existing placements
	// and validate affinity/anti-affinity rules

	workspaceMap := make(map[logicalcluster.Name]bool)
	for _, placement := range decision.SelectedWorkspaces {
		if workspaceMap[placement.Workspace] {
			// Same workspace selected multiple times
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypeAffinityViolation,
				Description: fmt.Sprintf("Workspace %s selected multiple times", placement.Workspace),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:          SeverityLow,
				ResolutionSuggestion: "Review selection algorithm to prevent duplicate selections",
			})
		}
		workspaceMap[placement.Workspace] = true
	}

	return conflicts
}

// checkPolicyConflicts checks for policy violations.
func (v *defaultDecisionValidator) checkPolicyConflicts(ctx context.Context, decision *PlacementDecision) []ConflictDescription {
	var conflicts []ConflictDescription

	// Check for inconsistent scoring
	for _, placement := range decision.SelectedWorkspaces {
		if placement.FinalScore <= 0 {
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypePolicyViolation,
				Description: fmt.Sprintf("Workspace %s selected with zero or negative score: %f", 
					placement.Workspace, placement.FinalScore),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:          SeverityMedium,
				ResolutionSuggestion: "Review scoring algorithm to ensure positive scores for selected workspaces",
			})
		}
	}

	// Check override consistency
	if decision.Override != nil {
		switch decision.Override.OverrideType {
		case OverrideTypeForce:
			// Ensure only target workspaces are selected
			selectedMap := make(map[logicalcluster.Name]bool)
			for _, placement := range decision.SelectedWorkspaces {
				selectedMap[placement.Workspace] = true
			}
			for _, targetWS := range decision.Override.TargetWorkspaces {
				if !selectedMap[targetWS] {
					conflicts = append(conflicts, ConflictDescription{
						Type:        ConflictTypePolicyViolation,
						Description: fmt.Sprintf("Force override specified workspace %s but it was not selected", targetWS),
						AffectedWorkspaces: []logicalcluster.Name{targetWS},
						Severity:          SeverityHigh,
						ResolutionSuggestion: "Ensure force override logic properly selects all target workspaces",
					})
				}
			}

		case OverrideTypeExclude:
			// Ensure excluded workspaces are not selected
			for _, placement := range decision.SelectedWorkspaces {
				for _, excludedWS := range decision.Override.ExcludedWorkspaces {
					if placement.Workspace == excludedWS {
						conflicts = append(conflicts, ConflictDescription{
							Type:        ConflictTypePolicyViolation,
							Description: fmt.Sprintf("Exclude override specified workspace %s but it was selected", excludedWS),
							AffectedWorkspaces: []logicalcluster.Name{excludedWS},
							Severity:          SeverityHigh,
							ResolutionSuggestion: "Ensure exclude override logic properly excludes all specified workspaces",
						})
					}
				}
			}
		}
	}

	return conflicts
}