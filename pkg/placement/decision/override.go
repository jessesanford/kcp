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
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// OverrideManager provides management of placement overrides including
// creation, validation, application, and lifecycle management.
type OverrideManager interface {
	// CreateOverride creates a new placement override with validation
	CreateOverride(ctx context.Context, request *CreateOverrideRequest) (*PlacementOverride, error)
	
	// ApplyOverride applies an override to a placement decision
	ApplyOverride(ctx context.Context, decision *PlacementDecision, override *PlacementOverride) (*PlacementDecision, error)
	
	// ValidateOverride validates an override against policies and constraints
	ValidateOverride(ctx context.Context, override *PlacementOverride) error
	
	// GetActiveOverrides returns active overrides for a placement request
	GetActiveOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error)
	
	// ExpireOverride marks an override as expired
	ExpireOverride(ctx context.Context, overrideID string, reason string) error
	
	// DeleteOverride removes an override
	DeleteOverride(ctx context.Context, overrideID string, reason string) error
	
	// ListOverrides lists overrides with optional filtering
	ListOverrides(ctx context.Context, filter *OverrideFilter) ([]*PlacementOverride, error)
	
	// GetOverrideHistory returns historical override information
	GetOverrideHistory(ctx context.Context, placementID string) ([]*OverrideHistoryEntry, error)
}

// defaultOverrideManager implements OverrideManager.
type defaultOverrideManager struct {
	// Storage for override persistence
	storage OverrideStorage
	
	// Event recorder for Kubernetes events
	eventRecorder record.EventRecorder
	
	// Override validator
	validator OverrideValidator
	
	// Configuration
	config *OverrideManagerConfig
	
	// Active overrides cache
	activeOverrides map[string][]*PlacementOverride
	
	// Override history
	overrideHistory map[string][]*OverrideHistoryEntry
	
	// Synchronization
	mu sync.RWMutex
	
	// Background cleanup
	stopCh chan struct{}
	
	// Audit logger
	auditLogger klog.Logger
}

// NewOverrideManager creates a new override manager with the specified configuration.
func NewOverrideManager(
	storage OverrideStorage,
	eventRecorder record.EventRecorder,
	validator OverrideValidator,
	config *OverrideManagerConfig,
) (OverrideManager, error) {
	if storage == nil {
		return nil, fmt.Errorf("override storage cannot be nil")
	}
	if eventRecorder == nil {
		return nil, fmt.Errorf("event recorder cannot be nil")
	}
	if validator == nil {
		return nil, fmt.Errorf("override validator cannot be nil")
	}
	if config == nil {
		config = DefaultOverrideManagerConfig()
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid override manager configuration: %w", err)
	}
	
	manager := &defaultOverrideManager{
		storage:         storage,
		eventRecorder:   eventRecorder,
		validator:       validator,
		config:          config,
		activeOverrides: make(map[string][]*PlacementOverride),
		overrideHistory: make(map[string][]*OverrideHistoryEntry),
		stopCh:          make(chan struct{}),
		auditLogger:     klog.Background().WithName("override-manager").WithName("audit"),
	}
	
	// Load existing overrides from storage
	if err := manager.loadExistingOverrides(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load existing overrides: %w", err)
	}
	
	// Start background cleanup goroutine
	go manager.cleanupLoop()
	
	return manager, nil
}

// CreateOverride creates a new placement override with comprehensive validation.
func (om *defaultOverrideManager) CreateOverride(ctx context.Context, request *CreateOverrideRequest) (*PlacementOverride, error) {
	if request == nil {
		return nil, fmt.Errorf("create override request cannot be nil")
	}
	
	// Validate the request
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid create override request: %w", err)
	}
	
	om.mu.Lock()
	defer om.mu.Unlock()
	
	// Create the override object
	override := &PlacementOverride{
		ID:                 string(uuid.NewUUID()),
		PlacementID:        request.PlacementID,
		OverrideType:       request.OverrideType,
		TargetWorkspaces:   request.TargetWorkspaces,
		ExcludedWorkspaces: request.ExcludedWorkspaces,
		Reason:             request.Reason,
		AppliedBy:          request.AppliedBy,
		CreatedAt:          time.Now(),
		ExpiresAt:          request.ExpiresAt,
		Priority:           request.Priority,
	}
	
	// Validate the override
	if err := om.validator.ValidateOverride(ctx, override); err != nil {
		return nil, fmt.Errorf("override validation failed: %w", err)
	}
	
	// Check for conflicts with existing overrides
	if err := om.checkOverrideConflicts(ctx, override); err != nil {
		return nil, fmt.Errorf("override conflicts detected: %w", err)
	}
	
	// Store the override
	if err := om.storage.StoreOverride(ctx, override); err != nil {
		return nil, fmt.Errorf("failed to store override: %w", err)
	}
	
	// Add to active overrides cache
	om.activeOverrides[override.PlacementID] = append(om.activeOverrides[override.PlacementID], override)
	
	// Sort by priority (higher priority first)
	sort.Slice(om.activeOverrides[override.PlacementID], func(i, j int) bool {
		return om.activeOverrides[override.PlacementID][i].Priority > om.activeOverrides[override.PlacementID][j].Priority
	})
	
	// Record in history
	om.recordOverrideHistory(override, OverrideActionCreated, "Override created successfully")
	
	// Emit audit log
	om.auditOverride(override, "OVERRIDE_CREATED")
	
	// Emit Kubernetes event
	om.emitOverrideEvent(ctx, override, "Normal", "OverrideCreated", 
		fmt.Sprintf("Placement override created: %s", override.Reason))
	
	klog.V(2).InfoS("Placement override created", 
		"overrideID", override.ID,
		"placementID", override.PlacementID,
		"type", override.OverrideType,
		"appliedBy", override.AppliedBy)
	
	return override, nil
}

// ApplyOverride applies an override to a placement decision, modifying the decision according to the override rules.
func (om *defaultOverrideManager) ApplyOverride(ctx context.Context, decision *PlacementDecision, override *PlacementOverride) (*PlacementDecision, error) {
	if decision == nil {
		return nil, fmt.Errorf("placement decision cannot be nil")
	}
	if override == nil {
		return nil, fmt.Errorf("placement override cannot be nil")
	}
	
	om.mu.RLock()
	defer om.mu.RUnlock()
	
	// Check if override is still active
	if om.isOverrideExpired(override) {
		return nil, fmt.Errorf("override %s has expired", override.ID)
	}
	
	klog.V(2).InfoS("Applying placement override",
		"overrideID", override.ID,
		"decisionID", decision.ID,
		"overrideType", override.OverrideType)
	
	// Create a copy of the decision to modify
	modifiedDecision := *decision
	modifiedDecision.Override = override
	
	// Apply the override based on type
	switch override.OverrideType {
	case OverrideTypeForce:
		if err := om.applyForceOverride(&modifiedDecision, override); err != nil {
			return nil, fmt.Errorf("failed to apply force override: %w", err)
		}
	case OverrideTypeExclude:
		om.applyExcludeOverride(&modifiedDecision, override)
	case OverrideTypePrefer:
		om.applyPreferOverride(&modifiedDecision, override)
	case OverrideTypeAvoid:
		om.applyAvoidOverride(&modifiedDecision, override)
	default:
		return nil, fmt.Errorf("unknown override type: %s", override.OverrideType)
	}
	
	// Update decision status
	modifiedDecision.Status = DecisionStatusOverridden
	
	// Add override factor to rationale
	modifiedDecision.DecisionRationale.OverrideFactors = append(
		modifiedDecision.DecisionRationale.OverrideFactors,
		fmt.Sprintf("Applied %s override by %s: %s", override.OverrideType, override.AppliedBy, override.Reason),
	)
	
	// Record the override application
	om.recordOverrideHistory(override, OverrideActionApplied, 
		fmt.Sprintf("Override applied to decision %s", decision.ID))
	
	// Emit audit log
	om.auditOverrideApplication(override, decision, "OVERRIDE_APPLIED")
	
	return &modifiedDecision, nil
}

// ValidateOverride validates an override against system policies and constraints.
func (om *defaultOverrideManager) ValidateOverride(ctx context.Context, override *PlacementOverride) error {
	if override == nil {
		return fmt.Errorf("placement override cannot be nil")
	}
	
	return om.validator.ValidateOverride(ctx, override)
}

// GetActiveOverrides returns all active overrides for a specific placement request.
func (om *defaultOverrideManager) GetActiveOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error) {
	if placementID == "" {
		return nil, fmt.Errorf("placement ID cannot be empty")
	}
	
	om.mu.RLock()
	defer om.mu.RUnlock()
	
	overrides := om.activeOverrides[placementID]
	if len(overrides) == 0 {
		return nil, nil
	}
	
	// Filter out expired overrides
	activeOverrides := make([]*PlacementOverride, 0, len(overrides))
	for _, override := range overrides {
		if !om.isOverrideExpired(override) {
			activeOverrides = append(activeOverrides, override)
		}
	}
	
	return activeOverrides, nil
}

// ExpireOverride marks an override as expired and removes it from active use.
func (om *defaultOverrideManager) ExpireOverride(ctx context.Context, overrideID string, reason string) error {
	if overrideID == "" {
		return fmt.Errorf("override ID cannot be empty")
	}
	
	om.mu.Lock()
	defer om.mu.Unlock()
	
	// Find and expire the override
	var foundOverride *PlacementOverride
	for placementID, overrides := range om.activeOverrides {
		for i, override := range overrides {
			if override.ID == overrideID {
				// Mark as expired
				now := time.Now()
				override.ExpiresAt = &now
				foundOverride = override
				
				// Update storage
				if err := om.storage.UpdateOverride(ctx, override); err != nil {
					return fmt.Errorf("failed to update override expiration: %w", err)
				}
				
				// Remove from active list
				om.activeOverrides[placementID] = append(overrides[:i], overrides[i+1:]...)
				break
			}
		}
		if foundOverride != nil {
			break
		}
	}
	
	if foundOverride == nil {
		return fmt.Errorf("override %s not found", overrideID)
	}
	
	// Record in history
	om.recordOverrideHistory(foundOverride, OverrideActionExpired, reason)
	
	// Emit audit log
	om.auditOverride(foundOverride, "OVERRIDE_EXPIRED")
	
	// Emit Kubernetes event
	om.emitOverrideEvent(ctx, foundOverride, "Normal", "OverrideExpired", 
		fmt.Sprintf("Override expired: %s", reason))
	
	klog.V(2).InfoS("Override expired", "overrideID", overrideID, "reason", reason)
	
	return nil
}

// DeleteOverride permanently removes an override from the system.
func (om *defaultOverrideManager) DeleteOverride(ctx context.Context, overrideID string, reason string) error {
	if overrideID == "" {
		return fmt.Errorf("override ID cannot be empty")
	}
	
	om.mu.Lock()
	defer om.mu.Unlock()
	
	// Find and delete the override
	var foundOverride *PlacementOverride
	for placementID, overrides := range om.activeOverrides {
		for i, override := range overrides {
			if override.ID == overrideID {
				foundOverride = override
				
				// Remove from active list
				om.activeOverrides[placementID] = append(overrides[:i], overrides[i+1:]...)
				break
			}
		}
		if foundOverride != nil {
			break
		}
	}
	
	if foundOverride == nil {
		return fmt.Errorf("override %s not found", overrideID)
	}
	
	// Delete from storage
	if err := om.storage.DeleteOverride(ctx, overrideID); err != nil {
		return fmt.Errorf("failed to delete override from storage: %w", err)
	}
	
	// Record in history
	om.recordOverrideHistory(foundOverride, OverrideActionDeleted, reason)
	
	// Emit audit log
	om.auditOverride(foundOverride, "OVERRIDE_DELETED")
	
	// Emit Kubernetes event
	om.emitOverrideEvent(ctx, foundOverride, "Normal", "OverrideDeleted", 
		fmt.Sprintf("Override deleted: %s", reason))
	
	klog.V(2).InfoS("Override deleted", "overrideID", overrideID, "reason", reason)
	
	return nil
}

// ListOverrides returns a list of overrides matching the specified filter criteria.
func (om *defaultOverrideManager) ListOverrides(ctx context.Context, filter *OverrideFilter) ([]*PlacementOverride, error) {
	om.mu.RLock()
	defer om.mu.RUnlock()
	
	// Query storage for overrides
	overrides, err := om.storage.QueryOverrides(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query overrides: %w", err)
	}
	
	return overrides, nil
}

// GetOverrideHistory returns the complete history of override actions for a placement.
func (om *defaultOverrideManager) GetOverrideHistory(ctx context.Context, placementID string) ([]*OverrideHistoryEntry, error) {
	if placementID == "" {
		return nil, fmt.Errorf("placement ID cannot be empty")
	}
	
	om.mu.RLock()
	defer om.mu.RUnlock()
	
	history := om.overrideHistory[placementID]
	if len(history) == 0 {
		// Try to load from storage
		return om.storage.GetOverrideHistory(ctx, placementID)
	}
	
	return history, nil
}

// applyForceOverride applies a force override to a decision.
func (om *defaultOverrideManager) applyForceOverride(decision *PlacementDecision, override *PlacementOverride) error {
	if len(override.TargetWorkspaces) == 0 {
		return fmt.Errorf("force override requires target workspaces")
	}
	
	// Clear existing selections and force placement to target workspaces
	decision.SelectedWorkspaces = decision.SelectedWorkspaces[:0]
	decision.RejectedCandidates = decision.RejectedCandidates[:0]
	
	for _, workspace := range override.TargetWorkspaces {
		placement := &WorkspacePlacement{
			Workspace:     workspace,
			FinalScore:    100.0, // Force override gets maximum score
			SelectionReason: fmt.Sprintf("Forced by override: %s", override.Reason),
		}
		decision.SelectedWorkspaces = append(decision.SelectedWorkspaces, placement)
	}
	
	return nil
}

// applyExcludeOverride applies an exclusion override to a decision.
func (om *defaultOverrideManager) applyExcludeOverride(decision *PlacementDecision, override *PlacementOverride) {
	excludeMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.ExcludedWorkspaces {
		excludeMap[workspace] = true
	}
	
	// Remove excluded workspaces from selected and move to rejected
	var newSelected []*WorkspacePlacement
	for _, placement := range decision.SelectedWorkspaces {
		if excludeMap[placement.Workspace] {
			// Move to rejected candidates
			rejected := &RejectedCandidate{
				Workspace:       placement.Workspace,
				SchedulerScore:  placement.SchedulerScore,
				CELScore:        placement.CELScore,
				FinalScore:      placement.FinalScore,
				RejectionReason: fmt.Sprintf("Excluded by override: %s", override.Reason),
				CELResults:      placement.CELResults,
			}
			decision.RejectedCandidates = append(decision.RejectedCandidates, rejected)
		} else {
			newSelected = append(newSelected, placement)
		}
	}
	decision.SelectedWorkspaces = newSelected
}

// applyPreferOverride applies a preference override to a decision.
func (om *defaultOverrideManager) applyPreferOverride(decision *PlacementDecision, override *PlacementOverride) {
	preferMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.TargetWorkspaces {
		preferMap[workspace] = true
	}
	
	// Boost scores for preferred workspaces
	preferenceBoost := om.config.PreferenceScoreBoost
	for _, placement := range decision.SelectedWorkspaces {
		if preferMap[placement.Workspace] {
			placement.FinalScore = min(100.0, placement.FinalScore+preferenceBoost)
			placement.SelectionReason += fmt.Sprintf(" (preferred by override: %s)", override.Reason)
		}
	}
	
	// Re-sort by final score (highest first)
	sort.Slice(decision.SelectedWorkspaces, func(i, j int) bool {
		return decision.SelectedWorkspaces[i].FinalScore > decision.SelectedWorkspaces[j].FinalScore
	})
}

// applyAvoidOverride applies an avoidance override to a decision.
func (om *defaultOverrideManager) applyAvoidOverride(decision *PlacementDecision, override *PlacementOverride) {
	avoidMap := make(map[logicalcluster.Name]bool)
	for _, workspace := range override.TargetWorkspaces {
		avoidMap[workspace] = true
	}
	
	// Reduce scores for avoided workspaces
	avoidancePenalty := om.config.AvoidanceScorePenalty
	for _, placement := range decision.SelectedWorkspaces {
		if avoidMap[placement.Workspace] {
			placement.FinalScore = max(0.0, placement.FinalScore-avoidancePenalty)
			placement.SelectionReason += fmt.Sprintf(" (avoided by override: %s)", override.Reason)
		}
	}
	
	// Re-sort by final score (highest first)
	sort.Slice(decision.SelectedWorkspaces, func(i, j int) bool {
		return decision.SelectedWorkspaces[i].FinalScore > decision.SelectedWorkspaces[j].FinalScore
	})
}

// checkOverrideConflicts checks for conflicts with existing overrides.
func (om *defaultOverrideManager) checkOverrideConflicts(ctx context.Context, override *PlacementOverride) error {
	existingOverrides := om.activeOverrides[override.PlacementID]
	
	for _, existing := range existingOverrides {
		// Skip expired overrides
		if om.isOverrideExpired(existing) {
			continue
		}
		
		// Check for conflicting override types
		if om.areOverridesConflicting(override, existing) {
			return fmt.Errorf("override conflicts with existing override %s (%s)", 
				existing.ID, existing.OverrideType)
		}
	}
	
	return nil
}

// areOverridesConflicting determines if two overrides conflict with each other.
func (om *defaultOverrideManager) areOverridesConflicting(override1, override2 *PlacementOverride) bool {
	// Force overrides conflict with each other
	if override1.OverrideType == OverrideTypeForce && override2.OverrideType == OverrideTypeForce {
		return true
	}
	
	// Check workspace overlaps for specific conflict patterns
	overlap := om.hasWorkspaceOverlap(override1, override2)
	
	// Force and exclude on same workspaces conflict
	if (override1.OverrideType == OverrideTypeForce && override2.OverrideType == OverrideTypeExclude) ||
		(override1.OverrideType == OverrideTypeExclude && override2.OverrideType == OverrideTypeForce) {
		return overlap
	}
	
	// Prefer and avoid on same workspaces conflict
	if (override1.OverrideType == OverrideTypePrefer && override2.OverrideType == OverrideTypeAvoid) ||
		(override1.OverrideType == OverrideTypeAvoid && override2.OverrideType == OverrideTypePrefer) {
		return overlap
	}
	
	return false
}

// hasWorkspaceOverlap checks if two overrides have overlapping workspaces.
func (om *defaultOverrideManager) hasWorkspaceOverlap(override1, override2 *PlacementOverride) bool {
	workspaces1 := make(map[logicalcluster.Name]bool)
	for _, ws := range override1.TargetWorkspaces {
		workspaces1[ws] = true
	}
	for _, ws := range override1.ExcludedWorkspaces {
		workspaces1[ws] = true
	}
	
	// Check overlap with override2's workspaces
	for _, ws := range override2.TargetWorkspaces {
		if workspaces1[ws] {
			return true
		}
	}
	for _, ws := range override2.ExcludedWorkspaces {
		if workspaces1[ws] {
			return true
		}
	}
	
	return false
}

// isOverrideExpired checks if an override has expired.
func (om *defaultOverrideManager) isOverrideExpired(override *PlacementOverride) bool {
	if override.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*override.ExpiresAt)
}

// recordOverrideHistory records an override action in the history.
func (om *defaultOverrideManager) recordOverrideHistory(override *PlacementOverride, action OverrideAction, message string) {
	entry := &OverrideHistoryEntry{
		OverrideID:  override.ID,
		PlacementID: override.PlacementID,
		Action:      action,
		Timestamp:   time.Now(),
		Message:     message,
		AppliedBy:   override.AppliedBy,
	}
	
	om.overrideHistory[override.PlacementID] = append(om.overrideHistory[override.PlacementID], entry)
}

// auditOverride logs an audit entry for an override action.
func (om *defaultOverrideManager) auditOverride(override *PlacementOverride, action string) {
	om.auditLogger.Info("Override audit",
		"action", action,
		"overrideID", override.ID,
		"placementID", override.PlacementID,
		"type", override.OverrideType,
		"appliedBy", override.AppliedBy,
		"reason", override.Reason,
		"priority", override.Priority,
	)
}

// auditOverrideApplication logs an audit entry for override application.
func (om *defaultOverrideManager) auditOverrideApplication(override *PlacementOverride, decision *PlacementDecision, action string) {
	om.auditLogger.Info("Override application audit",
		"action", action,
		"overrideID", override.ID,
		"decisionID", decision.ID,
		"type", override.OverrideType,
		"selectedWorkspaces", len(decision.SelectedWorkspaces),
		"rejectedCandidates", len(decision.RejectedCandidates),
	)
}

// emitOverrideEvent emits a Kubernetes event for an override action.
func (om *defaultOverrideManager) emitOverrideEvent(ctx context.Context, override *PlacementOverride, eventType, reason, message string) {
	// For now, we'll log the event since we don't have a specific object to attach it to
	// In a real implementation, this would be attached to the placement object
	klog.V(3).InfoS("Override event", 
		"type", eventType,
		"reason", reason,
		"message", message,
		"overrideID", override.ID,
		"placementID", override.PlacementID,
	)
}

// loadExistingOverrides loads existing overrides from storage.
func (om *defaultOverrideManager) loadExistingOverrides(ctx context.Context) error {
	overrides, err := om.storage.QueryOverrides(ctx, &OverrideFilter{})
	if err != nil {
		return fmt.Errorf("failed to load overrides from storage: %w", err)
	}
	
	for _, override := range overrides {
		// Skip expired overrides
		if om.isOverrideExpired(override) {
			continue
		}
		
		om.activeOverrides[override.PlacementID] = append(om.activeOverrides[override.PlacementID], override)
	}
	
	// Sort all override lists by priority
	for placementID := range om.activeOverrides {
		sort.Slice(om.activeOverrides[placementID], func(i, j int) bool {
			return om.activeOverrides[placementID][i].Priority > om.activeOverrides[placementID][j].Priority
		})
	}
	
	return nil
}

// cleanupLoop runs periodic cleanup operations.
func (om *defaultOverrideManager) cleanupLoop() {
	ticker := time.NewTicker(om.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			om.performCleanup()
		case <-om.stopCh:
			return
		}
	}
}

// performCleanup performs periodic cleanup of expired overrides.
func (om *defaultOverrideManager) performCleanup() {
	om.mu.Lock()
	defer om.mu.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), om.config.CleanupTimeout)
	defer cancel()
	
	// Clean up expired overrides
	for placementID, overrides := range om.activeOverrides {
		var activeOverrides []*PlacementOverride
		
		for _, override := range overrides {
			if om.isOverrideExpired(override) {
				// Record expiration in history
				om.recordOverrideHistory(override, OverrideActionExpired, "Automatic expiration during cleanup")
				
				klog.V(4).InfoS("Override expired during cleanup", "overrideID", override.ID)
			} else {
				activeOverrides = append(activeOverrides, override)
			}
		}
		
		if len(activeOverrides) == 0 {
			delete(om.activeOverrides, placementID)
		} else {
			om.activeOverrides[placementID] = activeOverrides
		}
	}
	
	// Cleanup storage
	if err := om.storage.CleanupExpiredOverrides(ctx); err != nil {
		klog.ErrorS(err, "Failed to cleanup expired overrides from storage")
	}
}

// Stop stops the override manager and its background operations.
func (om *defaultOverrideManager) Stop() {
	close(om.stopCh)
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two float64 values.
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}