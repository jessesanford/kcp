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
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	schedulerapi "github.com/kcp-dev/kcp/pkg/placement/scheduler"
)

// defaultDecisionValidator implements the DecisionValidator interface.
type defaultDecisionValidator struct {
	config ValidationConfig
}

// ValidationConfig provides configuration for decision validation.
type ValidationConfig struct {
	// EnableResourceValidation indicates if resource constraint validation should be enabled
	EnableResourceValidation bool
	
	// EnablePolicyValidation indicates if policy compliance validation should be enabled
	EnablePolicyValidation bool
	
	// EnableConflictChecking indicates if conflict checking should be enabled
	EnableConflictChecking bool
	
	// MaxValidationTime is the maximum time allowed for validation
	MaxValidationTime time.Duration
	
	// ResourceOvercommitThreshold is the maximum resource overcommit allowed (0-1.0)
	ResourceOvercommitThreshold float64
	
	// RequiredLabels are labels that must be present on selected workspaces
	RequiredLabels map[string]string
	
	// ForbiddenLabels are labels that must not be present on selected workspaces
	ForbiddenLabels map[string]string
	
	// MinimumWorkspaces is the minimum number of workspaces required for a valid decision
	MinimumWorkspaces int
	
	// MaximumWorkspaces is the maximum number of workspaces allowed for a valid decision
	MaximumWorkspaces int
	
	// AllowedRegions restricts placement to specific regions
	AllowedRegions []string
	
	// ForbiddenRegions prevents placement in specific regions
	ForbiddenRegions []string
}

// NewDecisionValidator creates a new decision validator with the specified configuration.
func NewDecisionValidator(config ValidationConfig) DecisionValidator {
	return &defaultDecisionValidator{
		config: config,
	}
}

// ValidateDecision validates a placement decision against constraints and policies.
func (v *defaultDecisionValidator) ValidateDecision(ctx context.Context, decision *PlacementDecision) error {
	startTime := time.Now()
	
	klog.V(3).InfoS("Starting decision validation",
		"decisionID", decision.ID,
		"selectedWorkspaces", len(decision.SelectedWorkspaces))
	
	// Apply timeout if configured
	if v.config.MaxValidationTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, v.config.MaxValidationTime)
		defer cancel()
	}
	
	// Basic structural validation
	if err := v.validateStructure(decision); err != nil {
		return fmt.Errorf("decision structure validation failed: %w", err)
	}
	
	// Validate workspace count constraints
	if err := v.validateWorkspaceCount(decision); err != nil {
		return fmt.Errorf("workspace count validation failed: %w", err)
	}
	
	// Validate resource constraints if enabled
	if v.config.EnableResourceValidation {
		if err := v.ValidateResourceConstraints(ctx, decision.SelectedWorkspaces); err != nil {
			return fmt.Errorf("resource constraint validation failed: %w", err)
		}
	}
	
	// Validate policy compliance if enabled
	if v.config.EnablePolicyValidation {
		if err := v.ValidatePolicyCompliance(ctx, decision); err != nil {
			return fmt.Errorf("policy compliance validation failed: %w", err)
		}
	}
	
	// Check for conflicts if enabled
	if v.config.EnableConflictChecking {
		conflicts, err := v.CheckConflicts(ctx, decision)
		if err != nil {
			return fmt.Errorf("conflict checking failed: %w", err)
		}
		
		// Check for critical conflicts that block the decision
		for _, conflict := range conflicts {
			if conflict.Severity == SeverityCritical {
				return fmt.Errorf("critical conflict detected: %s", conflict.Description)
			}
		}
		
		// Log non-critical conflicts
		for _, conflict := range conflicts {
			if conflict.Severity != SeverityCritical {
				klog.V(2).InfoS("Non-critical conflict detected",
					"decisionID", decision.ID,
					"conflictType", conflict.Type,
					"severity", conflict.Severity,
					"description", conflict.Description)
			}
		}
	}
	
	validationDuration := time.Since(startTime)
	klog.V(3).InfoS("Decision validation completed",
		"decisionID", decision.ID,
		"duration", validationDuration)
	
	return nil
}

// validateStructure validates the basic structure of a placement decision.
func (v *defaultDecisionValidator) validateStructure(decision *PlacementDecision) error {
	if decision == nil {
		return fmt.Errorf("decision cannot be nil")
	}
	
	if decision.ID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}
	
	if decision.RequestID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}
	
	if decision.Status == "" {
		return fmt.Errorf("decision status cannot be empty")
	}
	
	// Validate that decision time is reasonable
	if decision.DecisionTime.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("decision time cannot be in the future")
	}
	
	// Validate selected workspaces
	workspaceMap := make(map[logicalcluster.Name]bool)
	for _, placement := range decision.SelectedWorkspaces {
		if placement == nil {
			return fmt.Errorf("selected workspace placement cannot be nil")
		}
		
		if placement.Workspace.Empty() {
			return fmt.Errorf("selected workspace name cannot be empty")
		}
		
		if workspaceMap[placement.Workspace] {
			return fmt.Errorf("duplicate workspace selection: %s", placement.Workspace)
		}
		workspaceMap[placement.Workspace] = true
		
		// Validate scores are within expected ranges
		if placement.SchedulerScore < 0 || placement.SchedulerScore > 100 {
			return fmt.Errorf("scheduler score out of range for workspace %s: %f", placement.Workspace, placement.SchedulerScore)
		}
		
		if placement.CELScore < 0 || placement.CELScore > 100 {
			return fmt.Errorf("CEL score out of range for workspace %s: %f", placement.Workspace, placement.CELScore)
		}
		
		if placement.FinalScore < 0 || placement.FinalScore > 100 {
			return fmt.Errorf("final score out of range for workspace %s: %f", placement.Workspace, placement.FinalScore)
		}
	}
	
	return nil
}

// validateWorkspaceCount validates workspace count constraints.
func (v *defaultDecisionValidator) validateWorkspaceCount(decision *PlacementDecision) error {
	selectedCount := len(decision.SelectedWorkspaces)
	
	if v.config.MinimumWorkspaces > 0 && selectedCount < v.config.MinimumWorkspaces {
		return fmt.Errorf("insufficient workspaces selected: got %d, minimum required %d", 
			selectedCount, v.config.MinimumWorkspaces)
	}
	
	if v.config.MaximumWorkspaces > 0 && selectedCount > v.config.MaximumWorkspaces {
		return fmt.Errorf("too many workspaces selected: got %d, maximum allowed %d", 
			selectedCount, v.config.MaximumWorkspaces)
	}
	
	return nil
}

// ValidateResourceConstraints validates resource allocation constraints.
func (v *defaultDecisionValidator) ValidateResourceConstraints(ctx context.Context, placements []*WorkspacePlacement) error {
	for _, placement := range placements {
		// Validate resource allocation structure
		if err := v.validateResourceAllocation(placement); err != nil {
			return fmt.Errorf("resource allocation validation failed for workspace %s: %w", 
				placement.Workspace, err)
		}
		
		// Check for resource overcommit
		if err := v.checkResourceOvercommit(placement); err != nil {
			return fmt.Errorf("resource overcommit check failed for workspace %s: %w", 
				placement.Workspace, err)
		}
	}
	
	return nil
}

// validateResourceAllocation validates the resource allocation for a workspace placement.
func (v *defaultDecisionValidator) validateResourceAllocation(placement *WorkspacePlacement) error {
	allocation := &placement.AllocatedResources
	
	// Validate CPU allocation
	if allocation.CPU.Sign() < 0 {
		return fmt.Errorf("CPU allocation cannot be negative: %s", allocation.CPU.String())
	}
	
	// Validate memory allocation
	if allocation.Memory.Sign() < 0 {
		return fmt.Errorf("memory allocation cannot be negative: %s", allocation.Memory.String())
	}
	
	// Validate storage allocation
	if allocation.Storage.Sign() < 0 {
		return fmt.Errorf("storage allocation cannot be negative: %s", allocation.Storage.String())
	}
	
	// Validate reservation ID
	if allocation.ReservationID == "" {
		return fmt.Errorf("reservation ID cannot be empty")
	}
	
	// Validate expiration time
	if allocation.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("resource allocation has already expired: %s", allocation.ExpiresAt)
	}
	
	return nil
}

// checkResourceOvercommit checks if resource allocation would cause overcommit.
func (v *defaultDecisionValidator) checkResourceOvercommit(placement *WorkspacePlacement) error {
	// This is a simplified implementation - in practice, this would check against
	// actual workspace capacity and existing allocations
	
	allocation := &placement.AllocatedResources
	
	// Get resource requirements as raw values for comparison
	cpuMillis := allocation.CPU.MilliValue()
	memoryBytes := allocation.Memory.Value()
	storageBytes := allocation.Storage.Value()
	
	// Define reasonable maximums (these would come from workspace capacity in practice)
	maxCPUMillis := int64(128000)      // 128 CPU cores
	maxMemoryBytes := int64(1024 * 1024 * 1024 * 1024) // 1 TB
	maxStorageBytes := int64(100 * 1024 * 1024 * 1024 * 1024) // 100 TB
	
	// Apply overcommit threshold
	threshold := v.config.ResourceOvercommitThreshold
	if threshold <= 0 {
		threshold = 0.8 // Default to 80% utilization limit
	}
	
	allowedCPU := int64(float64(maxCPUMillis) * threshold)
	allowedMemory := int64(float64(maxMemoryBytes) * threshold)
	allowedStorage := int64(float64(maxStorageBytes) * threshold)
	
	if cpuMillis > allowedCPU {
		return fmt.Errorf("CPU allocation exceeds overcommit threshold: requested %dm, allowed %dm", 
			cpuMillis, allowedCPU)
	}
	
	if memoryBytes > allowedMemory {
		return fmt.Errorf("memory allocation exceeds overcommit threshold: requested %d bytes, allowed %d bytes", 
			memoryBytes, allowedMemory)
	}
	
	if storageBytes > allowedStorage {
		return fmt.Errorf("storage allocation exceeds overcommit threshold: requested %d bytes, allowed %d bytes", 
			storageBytes, allowedStorage)
	}
	
	return nil
}

// ValidatePolicyCompliance validates policy compliance for the decision.
func (v *defaultDecisionValidator) ValidatePolicyCompliance(ctx context.Context, decision *PlacementDecision) error {
	// Validate required labels
	for labelKey, labelValue := range v.config.RequiredLabels {
		if err := v.validateRequiredLabel(decision, labelKey, labelValue); err != nil {
			return err
		}
	}
	
	// Validate forbidden labels
	for labelKey, labelValue := range v.config.ForbiddenLabels {
		if err := v.validateForbiddenLabel(decision, labelKey, labelValue); err != nil {
			return err
		}
	}
	
	// Validate region restrictions
	if err := v.validateRegionRestrictions(decision); err != nil {
		return err
	}
	
	return nil
}

// validateRequiredLabel validates that a required label is present on all selected workspaces.
func (v *defaultDecisionValidator) validateRequiredLabel(decision *PlacementDecision, labelKey, labelValue string) error {
	for _, placement := range decision.SelectedWorkspaces {
		// In a real implementation, we would get workspace labels from the scheduler candidate
		// For now, we assume this check would be implemented based on workspace metadata
		klog.V(4).InfoS("Validating required label",
			"workspace", placement.Workspace,
			"labelKey", labelKey,
			"labelValue", labelValue)
		
		// TODO: Implement actual label checking when workspace metadata is available
	}
	return nil
}

// validateForbiddenLabel validates that a forbidden label is not present on selected workspaces.
func (v *defaultDecisionValidator) validateForbiddenLabel(decision *PlacementDecision, labelKey, labelValue string) error {
	for _, placement := range decision.SelectedWorkspaces {
		// In a real implementation, we would get workspace labels from the scheduler candidate
		// For now, we assume this check would be implemented based on workspace metadata
		klog.V(4).InfoS("Validating forbidden label",
			"workspace", placement.Workspace,
			"labelKey", labelKey,
			"labelValue", labelValue)
		
		// TODO: Implement actual label checking when workspace metadata is available
	}
	return nil
}

// validateRegionRestrictions validates region-based placement restrictions.
func (v *defaultDecisionValidator) validateRegionRestrictions(decision *PlacementDecision) error {
	// Validate allowed regions
	if len(v.config.AllowedRegions) > 0 {
		for _, placement := range decision.SelectedWorkspaces {
			// In a real implementation, we would determine the region from workspace metadata
			// For now, we assume this check would be implemented based on workspace location
			klog.V(4).InfoS("Validating allowed region",
				"workspace", placement.Workspace,
				"allowedRegions", v.config.AllowedRegions)
			
			// TODO: Implement actual region checking when workspace metadata is available
		}
	}
	
	// Validate forbidden regions
	if len(v.config.ForbiddenRegions) > 0 {
		for _, placement := range decision.SelectedWorkspaces {
			// In a real implementation, we would determine the region from workspace metadata
			// For now, we assume this check would be implemented based on workspace location
			klog.V(4).InfoS("Validating forbidden region",
				"workspace", placement.Workspace,
				"forbiddenRegions", v.config.ForbiddenRegions)
			
			// TODO: Implement actual region checking when workspace metadata is available
		}
	}
	
	return nil
}

// CheckConflicts checks for conflicts with existing placements.
func (v *defaultDecisionValidator) CheckConflicts(ctx context.Context, decision *PlacementDecision) ([]ConflictDescription, error) {
	var conflicts []ConflictDescription
	
	// Check for resource overcommit conflicts
	resourceConflicts := v.checkResourceConflicts(decision)
	conflicts = append(conflicts, resourceConflicts...)
	
	// Check for affinity rule conflicts
	affinityConflicts := v.checkAffinityConflicts(decision)
	conflicts = append(conflicts, affinityConflicts...)
	
	// Check for policy conflicts
	policyConflicts := v.checkPolicyConflicts(decision)
	conflicts = append(conflicts, policyConflicts...)
	
	return conflicts, nil
}

// checkResourceConflicts checks for resource-related conflicts.
func (v *defaultDecisionValidator) checkResourceConflicts(decision *PlacementDecision) []ConflictDescription {
	var conflicts []ConflictDescription
	
	for _, placement := range decision.SelectedWorkspaces {
		// Simulate checking for resource overcommit
		allocation := &placement.AllocatedResources
		
		// Check if allocation seems unreasonably large
		if allocation.CPU.MilliValue() > 64000 { // More than 64 CPU cores
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypeResourceOvercommit,
				Description: fmt.Sprintf("High CPU allocation detected in workspace %s: %s", placement.Workspace, allocation.CPU.String()),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:    SeverityMedium,
				ResolutionSuggestion: "Consider reducing CPU requirements or selecting additional workspaces",
			})
		}
		
		// Check memory allocation
		if allocation.Memory.Value() > 512*1024*1024*1024 { // More than 512GB
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypeResourceOvercommit,
				Description: fmt.Sprintf("High memory allocation detected in workspace %s: %s", placement.Workspace, allocation.Memory.String()),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:    SeverityMedium,
				ResolutionSuggestion: "Consider reducing memory requirements or selecting additional workspaces",
			})
		}
	}
	
	return conflicts
}

// checkAffinityConflicts checks for affinity and anti-affinity rule conflicts.
func (v *defaultDecisionValidator) checkAffinityConflicts(decision *PlacementDecision) []ConflictDescription {
	var conflicts []ConflictDescription
	
	// Simulate checking for affinity conflicts
	// In a real implementation, this would check against actual affinity rules
	
	selectedWorkspaces := make(map[logicalcluster.Name]bool)
	for _, placement := range decision.SelectedWorkspaces {
		selectedWorkspaces[placement.Workspace] = true
	}
	
	// Example: Check if multiple workspaces are selected when anti-affinity is preferred
	if len(selectedWorkspaces) > 1 {
		var workspaceNames []logicalcluster.Name
		for workspace := range selectedWorkspaces {
			workspaceNames = append(workspaceNames, workspace)
		}
		
		conflicts = append(conflicts, ConflictDescription{
			Type:        ConflictTypeAntiAffinityViolation,
			Description: "Multiple workspaces selected may violate anti-affinity preferences",
			AffectedWorkspaces: workspaceNames,
			Severity:    SeverityLow,
			ResolutionSuggestion: "Consider if workload spreading is desired or if consolidation is preferred",
		})
	}
	
	return conflicts
}

// checkPolicyConflicts checks for policy-related conflicts.
func (v *defaultDecisionValidator) checkPolicyConflicts(decision *PlacementDecision) []ConflictDescription {
	var conflicts []ConflictDescription
	
	// Simulate checking for policy conflicts
	// In a real implementation, this would check against actual governance policies
	
	for _, placement := range decision.SelectedWorkspaces {
		// Example: Check if placement violates hypothetical security policies
		if placement.FinalScore < 60 {
			conflicts = append(conflicts, ConflictDescription{
				Type:        ConflictTypePolicyViolation,
				Description: fmt.Sprintf("Workspace %s selected with low confidence score: %.2f", placement.Workspace, placement.FinalScore),
				AffectedWorkspaces: []logicalcluster.Name{placement.Workspace},
				Severity:    SeverityLow,
				ResolutionSuggestion: "Review placement criteria or consider alternative workspaces",
			})
		}
	}
	
	return conflicts
}