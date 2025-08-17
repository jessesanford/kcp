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
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// OverrideManager provides the interface for managing placement overrides.
type OverrideManager interface {
	// CreateOverride creates a new placement override
	CreateOverride(ctx context.Context, override *PlacementOverride) error
	
	// GetOverride retrieves an override by ID
	GetOverride(ctx context.Context, overrideID string) (*PlacementOverride, error)
	
	// ListOverrides lists all active overrides for a placement
	ListOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error)
	
	// UpdateOverride updates an existing override
	UpdateOverride(ctx context.Context, override *PlacementOverride) error
	
	// DeleteOverride removes an override
	DeleteOverride(ctx context.Context, overrideID string) error
	
	// GetActiveOverrides returns active overrides for a placement (non-expired)
	GetActiveOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error)
	
	// PruneExpiredOverrides removes expired overrides
	PruneExpiredOverrides(ctx context.Context) error
}

// OverrideValidator validates placement overrides.
type OverrideValidator interface {
	// ValidateOverride validates an override before creation or update
	ValidateOverride(ctx context.Context, override *PlacementOverride) error
	
	// CheckConflicts checks for conflicts between multiple overrides
	CheckConflicts(ctx context.Context, overrides []*PlacementOverride) ([]OverrideConflict, error)
}

// OverrideConflict describes a conflict between placement overrides.
type OverrideConflict struct {
	// ConflictType describes the type of conflict
	ConflictType OverrideConflictType
	
	// Description is a human-readable description of the conflict
	Description string
	
	// ConflictingOverrides are the overrides that conflict
	ConflictingOverrides []*PlacementOverride
	
	// Severity indicates the severity of the conflict
	Severity ConflictSeverity
	
	// Resolution suggests how to resolve the conflict
	Resolution string
}

// OverrideConflictType represents different types of override conflicts.
type OverrideConflictType string

const (
	// ConflictTypeContradictory indicates contradictory override directives
	ConflictTypeContradictory OverrideConflictType = "Contradictory"
	
	// ConflictTypePriorityCollision indicates multiple overrides with same priority
	ConflictTypePriorityCollision OverrideConflictType = "PriorityCollision"
	
	// ConflictTypeWorkspaceOverlap indicates overlapping workspace specifications
	ConflictTypeWorkspaceOverlap OverrideConflictType = "WorkspaceOverlap"
)

// inMemoryOverrideManager implements OverrideManager using in-memory storage.
type inMemoryOverrideManager struct {
	mu        sync.RWMutex
	overrides map[string]*PlacementOverride              // overrideID -> override
	byPlacement map[string][]*PlacementOverride          // placementID -> overrides
	validator OverrideValidator
}

// NewInMemoryOverrideManager creates a new in-memory override manager.
func NewInMemoryOverrideManager(validator OverrideValidator) OverrideManager {
	return &inMemoryOverrideManager{
		overrides:   make(map[string]*PlacementOverride),
		byPlacement: make(map[string][]*PlacementOverride),
		validator:   validator,
	}
}

// CreateOverride creates a new placement override.
func (m *inMemoryOverrideManager) CreateOverride(ctx context.Context, override *PlacementOverride) error {
	if override == nil {
		return fmt.Errorf("override cannot be nil")
	}

	// Generate ID if not provided
	if override.ID == "" {
		override.ID = string(uuid.NewUUID())
	}

	// Set creation time if not provided
	if override.CreatedAt.IsZero() {
		override.CreatedAt = time.Now()
	}

	// Validate the override
	if m.validator != nil {
		if err := m.validator.ValidateOverride(ctx, override); err != nil {
			return fmt.Errorf("override validation failed: %w", err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	klog.V(2).InfoS("Creating placement override",
		"overrideID", override.ID,
		"placementID", override.PlacementID,
		"overrideType", override.OverrideType)

	// Check if override already exists
	if _, exists := m.overrides[override.ID]; exists {
		return fmt.Errorf("override with ID %s already exists", override.ID)
	}

	// Check for conflicts with existing overrides
	if m.validator != nil {
		existingOverrides := m.getActiveOverridesForPlacementLocked(override.PlacementID)
		allOverrides := append(existingOverrides, override)
		
		conflicts, err := m.validator.CheckConflicts(ctx, allOverrides)
		if err != nil {
			return fmt.Errorf("conflict checking failed: %w", err)
		}

		// Report critical conflicts as creation failures
		for _, conflict := range conflicts {
			if conflict.Severity == SeverityCritical {
				return fmt.Errorf("critical override conflict: %s", conflict.Description)
			}
		}

		// Log non-critical conflicts as warnings
		for _, conflict := range conflicts {
			if conflict.Severity != SeverityCritical {
				klog.V(2).InfoS("Override conflict detected",
					"overrideID", override.ID,
					"conflictType", conflict.ConflictType,
					"severity", conflict.Severity,
					"description", conflict.Description)
			}
		}
	}

	// Store the override
	m.overrides[override.ID] = override
	m.byPlacement[override.PlacementID] = append(m.byPlacement[override.PlacementID], override)

	// Sort overrides by priority (highest first)
	m.sortOverridesByPriority(m.byPlacement[override.PlacementID])

	klog.V(3).InfoS("Override created successfully", "overrideID", override.ID)

	return nil
}

// GetOverride retrieves an override by ID.
func (m *inMemoryOverrideManager) GetOverride(ctx context.Context, overrideID string) (*PlacementOverride, error) {
	if overrideID == "" {
		return nil, fmt.Errorf("override ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	override, exists := m.overrides[overrideID]
	if !exists {
		return nil, fmt.Errorf("override not found: %s", overrideID)
	}

	// Return a copy to prevent external modification
	overrideCopy := *override
	return &overrideCopy, nil
}

// ListOverrides lists all overrides for a placement (including expired ones).
func (m *inMemoryOverrideManager) ListOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error) {
	if placementID == "" {
		return nil, fmt.Errorf("placement ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	overrides, exists := m.byPlacement[placementID]
	if !exists {
		return []*PlacementOverride{}, nil
	}

	// Return copies to prevent external modification
	result := make([]*PlacementOverride, len(overrides))
	for i, override := range overrides {
		overrideCopy := *override
		result[i] = &overrideCopy
	}

	return result, nil
}

// UpdateOverride updates an existing override.
func (m *inMemoryOverrideManager) UpdateOverride(ctx context.Context, override *PlacementOverride) error {
	if override == nil {
		return fmt.Errorf("override cannot be nil")
	}

	if override.ID == "" {
		return fmt.Errorf("override ID cannot be empty")
	}

	// Validate the override
	if m.validator != nil {
		if err := m.validator.ValidateOverride(ctx, override); err != nil {
			return fmt.Errorf("override validation failed: %w", err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	klog.V(2).InfoS("Updating placement override", "overrideID", override.ID)

	// Check if override exists
	existingOverride, exists := m.overrides[override.ID]
	if !exists {
		return fmt.Errorf("override not found: %s", override.ID)
	}

	oldPlacementID := existingOverride.PlacementID
	newPlacementID := override.PlacementID

	// Check for conflicts with other overrides
	if m.validator != nil {
		// Get existing overrides for the new placement (excluding this one)
		existingOverrides := []*PlacementOverride{}
		for _, o := range m.getActiveOverridesForPlacementLocked(newPlacementID) {
			if o.ID != override.ID {
				existingOverrides = append(existingOverrides, o)
			}
		}
		allOverrides := append(existingOverrides, override)
		
		conflicts, err := m.validator.CheckConflicts(ctx, allOverrides)
		if err != nil {
			return fmt.Errorf("conflict checking failed: %w", err)
		}

		// Report critical conflicts as update failures
		for _, conflict := range conflicts {
			if conflict.Severity == SeverityCritical {
				return fmt.Errorf("critical override conflict: %s", conflict.Description)
			}
		}
	}

	// Update the override
	m.overrides[override.ID] = override

	// Update placement index if placement ID changed
	if oldPlacementID != newPlacementID {
		// Remove from old placement
		if oldOverrides, exists := m.byPlacement[oldPlacementID]; exists {
			for i, o := range oldOverrides {
				if o.ID == override.ID {
					m.byPlacement[oldPlacementID] = append(oldOverrides[:i], oldOverrides[i+1:]...)
					break
				}
			}
			// Clean up empty entries
			if len(m.byPlacement[oldPlacementID]) == 0 {
				delete(m.byPlacement, oldPlacementID)
			}
		}

		// Add to new placement
		m.byPlacement[newPlacementID] = append(m.byPlacement[newPlacementID], override)
		m.sortOverridesByPriority(m.byPlacement[newPlacementID])
	} else {
		// Just update in place and re-sort
		for i, o := range m.byPlacement[newPlacementID] {
			if o.ID == override.ID {
				m.byPlacement[newPlacementID][i] = override
				break
			}
		}
		m.sortOverridesByPriority(m.byPlacement[newPlacementID])
	}

	klog.V(3).InfoS("Override updated successfully", "overrideID", override.ID)

	return nil
}

// DeleteOverride removes an override.
func (m *inMemoryOverrideManager) DeleteOverride(ctx context.Context, overrideID string) error {
	if overrideID == "" {
		return fmt.Errorf("override ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	klog.V(2).InfoS("Deleting placement override", "overrideID", overrideID)

	// Check if override exists
	override, exists := m.overrides[overrideID]
	if !exists {
		return fmt.Errorf("override not found: %s", overrideID)
	}

	// Remove from main map
	delete(m.overrides, overrideID)

	// Remove from placement index
	if placementOverrides, exists := m.byPlacement[override.PlacementID]; exists {
		for i, o := range placementOverrides {
			if o.ID == overrideID {
				m.byPlacement[override.PlacementID] = append(placementOverrides[:i], placementOverrides[i+1:]...)
				break
			}
		}
		
		// Clean up empty entries
		if len(m.byPlacement[override.PlacementID]) == 0 {
			delete(m.byPlacement, override.PlacementID)
		}
	}

	klog.V(3).InfoS("Override deleted successfully", "overrideID", overrideID)

	return nil
}

// GetActiveOverrides returns active overrides for a placement (non-expired).
func (m *inMemoryOverrideManager) GetActiveOverrides(ctx context.Context, placementID string) ([]*PlacementOverride, error) {
	if placementID == "" {
		return nil, fmt.Errorf("placement ID cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getActiveOverridesForPlacementLocked(placementID), nil
}

// getActiveOverridesForPlacementLocked returns active overrides for a placement (must be called with lock held).
func (m *inMemoryOverrideManager) getActiveOverridesForPlacementLocked(placementID string) []*PlacementOverride {
	overrides, exists := m.byPlacement[placementID]
	if !exists {
		return []*PlacementOverride{}
	}

	now := time.Now()
	activeOverrides := []*PlacementOverride{}

	for _, override := range overrides {
		// Check if override is expired
		if override.ExpiresAt == nil || override.ExpiresAt.After(now) {
			// Return a copy to prevent external modification
			overrideCopy := *override
			activeOverrides = append(activeOverrides, &overrideCopy)
		}
	}

	return activeOverrides
}

// PruneExpiredOverrides removes expired overrides.
func (m *inMemoryOverrideManager) PruneExpiredOverrides(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	klog.V(2).InfoS("Pruning expired overrides")

	now := time.Now()
	prunedCount := 0

	// Collect expired override IDs
	expiredIDs := []string{}
	for overrideID, override := range m.overrides {
		if override.ExpiresAt != nil && override.ExpiresAt.Before(now) {
			expiredIDs = append(expiredIDs, overrideID)
		}
	}

	// Remove expired overrides
	for _, overrideID := range expiredIDs {
		override := m.overrides[overrideID]
		
		// Remove from main map
		delete(m.overrides, overrideID)

		// Remove from placement index
		if placementOverrides, exists := m.byPlacement[override.PlacementID]; exists {
			for i, o := range placementOverrides {
				if o.ID == overrideID {
					m.byPlacement[override.PlacementID] = append(placementOverrides[:i], placementOverrides[i+1:]...)
					break
				}
			}
			
			// Clean up empty entries
			if len(m.byPlacement[override.PlacementID]) == 0 {
				delete(m.byPlacement, override.PlacementID)
			}
		}

		prunedCount++
	}

	klog.V(2).InfoS("Override pruning completed", "prunedCount", prunedCount)

	return nil
}

// sortOverridesByPriority sorts overrides by priority in descending order (highest priority first).
func (m *inMemoryOverrideManager) sortOverridesByPriority(overrides []*PlacementOverride) {
	sort.Slice(overrides, func(i, j int) bool {
		if overrides[i].Priority == overrides[j].Priority {
			// If priorities are equal, sort by creation time (newest first)
			return overrides[i].CreatedAt.After(overrides[j].CreatedAt)
		}
		return overrides[i].Priority > overrides[j].Priority
	})
}

// defaultOverrideValidator implements OverrideValidator with basic validation rules.
type defaultOverrideValidator struct{}

// NewDefaultOverrideValidator creates a new default override validator.
func NewDefaultOverrideValidator() OverrideValidator {
	return &defaultOverrideValidator{}
}

// ValidateOverride validates an override before creation or update.
func (v *defaultOverrideValidator) ValidateOverride(ctx context.Context, override *PlacementOverride) error {
	if override == nil {
		return fmt.Errorf("override cannot be nil")
	}

	// Validate placement ID
	if override.PlacementID == "" {
		return fmt.Errorf("placement ID cannot be empty")
	}

	// Validate override type
	switch override.OverrideType {
	case OverrideTypeForce, OverrideTypeExclude, OverrideTypePrefer, OverrideTypeAvoid:
		// Valid types
	default:
		return fmt.Errorf("invalid override type: %s", override.OverrideType)
	}

	// Validate type-specific requirements
	switch override.OverrideType {
	case OverrideTypeForce, OverrideTypePrefer:
		if len(override.TargetWorkspaces) == 0 {
			return fmt.Errorf("%s override must specify target workspaces", override.OverrideType)
		}
	case OverrideTypeExclude, OverrideTypeAvoid:
		if len(override.ExcludedWorkspaces) == 0 {
			return fmt.Errorf("%s override must specify excluded workspaces", override.OverrideType)
		}
	}

	// Validate workspace names
	for _, workspace := range override.TargetWorkspaces {
		if workspace == "" {
			return fmt.Errorf("target workspace name cannot be empty")
		}
	}
	for _, workspace := range override.ExcludedWorkspaces {
		if workspace == "" {
			return fmt.Errorf("excluded workspace name cannot be empty")
		}
	}

	// Validate expiration time
	if override.ExpiresAt != nil && override.ExpiresAt.Before(override.CreatedAt) {
		return fmt.Errorf("expiration time cannot be before creation time")
	}

	// Validate applied by
	if override.AppliedBy == "" {
		return fmt.Errorf("applied by cannot be empty")
	}

	// Validate reason
	if override.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}

	return nil
}

// CheckConflicts checks for conflicts between multiple overrides.
func (v *defaultOverrideValidator) CheckConflicts(ctx context.Context, overrides []*PlacementOverride) ([]OverrideConflict, error) {
	var conflicts []OverrideConflict

	if len(overrides) < 2 {
		return conflicts, nil // No conflicts with fewer than 2 overrides
	}

	// Check for contradictory overrides
	conflicts = append(conflicts, v.checkContradictoryOverrides(overrides)...)

	// Check for priority collisions
	conflicts = append(conflicts, v.checkPriorityCollisions(overrides)...)

	// Check for workspace overlap conflicts
	conflicts = append(conflicts, v.checkWorkspaceOverlaps(overrides)...)

	return conflicts, nil
}

// checkContradictoryOverrides checks for contradictory override directives.
func (v *defaultOverrideValidator) checkContradictoryOverrides(overrides []*PlacementOverride) []OverrideConflict {
	var conflicts []OverrideConflict

	// Build workspace sets for each override type
	forceWorkspaces := make(map[logicalcluster.Name][]*PlacementOverride)
	excludeWorkspaces := make(map[logicalcluster.Name][]*PlacementOverride)

	for _, override := range overrides {
		switch override.OverrideType {
		case OverrideTypeForce:
			for _, ws := range override.TargetWorkspaces {
				forceWorkspaces[ws] = append(forceWorkspaces[ws], override)
			}
		case OverrideTypeExclude:
			for _, ws := range override.ExcludedWorkspaces {
				excludeWorkspaces[ws] = append(excludeWorkspaces[ws], override)
			}
		}
	}

	// Check for workspaces that are both forced and excluded
	for workspace, forceOverrides := range forceWorkspaces {
		if excludeOverrides, exists := excludeWorkspaces[workspace]; exists {
			conflictingOverrides := append(forceOverrides, excludeOverrides...)
			conflicts = append(conflicts, OverrideConflict{
				ConflictType: ConflictTypeContradictory,
				Description: fmt.Sprintf("Workspace %s is both forced and excluded by different overrides", workspace),
				ConflictingOverrides: conflictingOverrides,
				Severity: SeverityCritical,
				Resolution: "Remove either the force or exclude directive for this workspace",
			})
		}
	}

	return conflicts
}

// checkPriorityCollisions checks for multiple overrides with the same priority.
func (v *defaultOverrideValidator) checkPriorityCollisions(overrides []*PlacementOverride) []OverrideConflict {
	var conflicts []OverrideConflict

	priorityMap := make(map[int32][]*PlacementOverride)
	for _, override := range overrides {
		priorityMap[override.Priority] = append(priorityMap[override.Priority], override)
	}

	for priority, priorityOverrides := range priorityMap {
		if len(priorityOverrides) > 1 {
			conflicts = append(conflicts, OverrideConflict{
				ConflictType: ConflictTypePriorityCollision,
				Description: fmt.Sprintf("Multiple overrides have the same priority %d", priority),
				ConflictingOverrides: priorityOverrides,
				Severity: SeverityMedium,
				Resolution: "Assign different priorities to resolve application order ambiguity",
			})
		}
	}

	return conflicts
}

// checkWorkspaceOverlaps checks for overlapping workspace specifications.
func (v *defaultOverrideValidator) checkWorkspaceOverlaps(overrides []*PlacementOverride) []OverrideConflict {
	var conflicts []OverrideConflict

	// Check for prefer/avoid conflicts on the same workspace
	preferWorkspaces := make(map[logicalcluster.Name][]*PlacementOverride)
	avoidWorkspaces := make(map[logicalcluster.Name][]*PlacementOverride)

	for _, override := range overrides {
		switch override.OverrideType {
		case OverrideTypePrefer:
			for _, ws := range override.TargetWorkspaces {
				preferWorkspaces[ws] = append(preferWorkspaces[ws], override)
			}
		case OverrideTypeAvoid:
			for _, ws := range override.ExcludedWorkspaces {
				avoidWorkspaces[ws] = append(avoidWorkspaces[ws], override)
			}
		}
	}

	// Check for workspaces that are both preferred and avoided
	for workspace, preferOverrides := range preferWorkspaces {
		if avoidOverrides, exists := avoidWorkspaces[workspace]; exists {
			conflictingOverrides := append(preferOverrides, avoidOverrides...)
			conflicts = append(conflicts, OverrideConflict{
				ConflictType: ConflictTypeWorkspaceOverlap,
				Description: fmt.Sprintf("Workspace %s is both preferred and avoided by different overrides", workspace),
				ConflictingOverrides: conflictingOverrides,
				Severity: SeverityMedium,
				Resolution: "Remove conflicting prefer/avoid directives or use different priorities",
			})
		}
	}

	return conflicts
}