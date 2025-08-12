/*
Copyright 2025 The KCP Authors.

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

package binding

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Binder handles the binding of workload sessions to placement targets.
// It manages session lifecycle, target selection, and affinity policies.
type Binder struct {
	resolver *Resolver
}

// NewBinder creates a new session binder with the provided resolver.
func NewBinder(resolver *Resolver) *Binder {
	if resolver == nil {
		panic("resolver cannot be nil")
	}
	return &Binder{
		resolver: resolver,
	}
}

// BindingRequest represents a request to bind a workload to placement targets.
type BindingRequest struct {
	// WorkloadReference identifies the workload to bind
	WorkloadReference tmcv1alpha1.WorkloadReference
	
	// Policy defines the session affinity policy to apply
	Policy *tmcv1alpha1.SessionAffinityPolicy
	
	// Candidates are the available placement targets
	Candidates []tmcv1alpha1.PlacementTarget
	
	// Context provides additional binding context
	Context BindingContext
}

// BindingContext provides additional context for session binding.
type BindingContext struct {
	// Namespace is the namespace where the session should be created
	Namespace string
	
	// ExistingSessions are current sessions for the workload
	ExistingSessions []tmcv1alpha1.SessionState
	
	// Timestamp when the binding request was made
	RequestTime time.Time
}

// BindingResult represents the result of a session binding operation.
type BindingResult struct {
	// SessionState is the created or updated session
	SessionState *tmcv1alpha1.SessionState
	
	// SelectedTargets are the targets selected for placement
	SelectedTargets []tmcv1alpha1.PlacementTarget
	
	// Reason explains why targets were selected
	Reason string
	
	// Conditions represent any conditions from the binding process
	Conditions []metav1.Condition
}

// Bind performs session binding according to the provided request.
// It creates or updates session state to bind workloads to appropriate targets.
func (b *Binder) Bind(ctx context.Context, req *BindingRequest) (*BindingResult, error) {
	if req == nil {
		return nil, fmt.Errorf("binding request cannot be nil")
	}
	
	if err := b.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid binding request: %w", err)
	}
	
	klog.V(2).InfoS("Processing session binding request", 
		"workload", req.WorkloadReference.Name,
		"namespace", req.Context.Namespace,
		"candidates", len(req.Candidates))
	
	// Check for existing sessions and apply affinity policy
	selectedTargets, reason, err := b.selectTargets(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to select targets: %w", err)
	}
	
	// Create or update session state
	sessionState, err := b.createSessionState(req, selectedTargets)
	if err != nil {
		return nil, fmt.Errorf("failed to create session state: %w", err)
	}
	
	conditions := b.generateConditions(req, selectedTargets)
	
	return &BindingResult{
		SessionState:    sessionState,
		SelectedTargets: selectedTargets,
		Reason:          reason,
		Conditions:      conditions,
	}, nil
}

// selectTargets chooses the appropriate placement targets based on affinity policy.
func (b *Binder) selectTargets(ctx context.Context, req *BindingRequest) ([]tmcv1alpha1.PlacementTarget, string, error) {
	// If no policy provided, use default selection
	if req.Policy == nil {
		targets, err := b.resolver.ResolveDefaultTargets(ctx, req.Candidates)
		return targets, "default target selection", err
	}
	
	// Apply session affinity policy
	switch req.Policy.Spec.AffinityType {
	case tmcv1alpha1.ClusterAffinity:
		return b.applyClusterAffinity(ctx, req)
	case tmcv1alpha1.NodeAffinity:
		return b.applyNodeAffinity(ctx, req)
	case tmcv1alpha1.WorkspaceAffinity:
		return b.applyWorkspaceAffinity(ctx, req)
	default:
		// Fallback to cluster affinity
		return b.applyClusterAffinity(ctx, req)
	}
}

// applyClusterAffinity applies cluster-level session affinity.
func (b *Binder) applyClusterAffinity(ctx context.Context, req *BindingRequest) ([]tmcv1alpha1.PlacementTarget, string, error) {
	// Check if we have existing sessions for this workload
	existingTarget := b.findExistingClusterTarget(req.Context.ExistingSessions)
	if existingTarget != nil {
		// Apply stickiness factor
		stickinessFactor := float64(0.5) // default
		if req.Policy.Spec.StickinessFactor != nil {
			stickinessFactor = *req.Policy.Spec.StickinessFactor
		}
		
		// Check if existing target is still viable
		if b.isTargetViable(existingTarget, req.Candidates, stickinessFactor) {
			klog.V(3).InfoS("Maintaining cluster affinity",
				"workload", req.WorkloadReference.Name,
				"cluster", existingTarget.ClusterName)
			return []tmcv1alpha1.PlacementTarget{*existingTarget}, "cluster affinity maintained", nil
		}
	}
	
	// Select new target based on policy
	targets, err := b.resolver.ResolveClusterTargets(ctx, req.Candidates, req.Policy)
	return targets, "cluster affinity applied", err
}

// applyNodeAffinity applies node-level session affinity.
func (b *Binder) applyNodeAffinity(ctx context.Context, req *BindingRequest) ([]tmcv1alpha1.PlacementTarget, string, error) {
	// Check if we have existing sessions with node affinity
	existingTarget := b.findExistingNodeTarget(req.Context.ExistingSessions)
	if existingTarget != nil {
		stickinessFactor := float64(0.5) // default
		if req.Policy.Spec.StickinessFactor != nil {
			stickinessFactor = *req.Policy.Spec.StickinessFactor
		}
		
		if b.isTargetViable(existingTarget, req.Candidates, stickinessFactor) {
			klog.V(3).InfoS("Maintaining node affinity",
				"workload", req.WorkloadReference.Name,
				"cluster", existingTarget.ClusterName,
				"nodeSelector", existingTarget.NodeSelector)
			return []tmcv1alpha1.PlacementTarget{*existingTarget}, "node affinity maintained", nil
		}
	}
	
	targets, err := b.resolver.ResolveNodeTargets(ctx, req.Candidates, req.Policy)
	return targets, "node affinity applied", err
}

// applyWorkspaceAffinity applies workspace-level session affinity.
func (b *Binder) applyWorkspaceAffinity(ctx context.Context, req *BindingRequest) ([]tmcv1alpha1.PlacementTarget, string, error) {
	// Workspace affinity considers the namespace context
	targets, err := b.resolver.ResolveWorkspaceTargets(ctx, req.Candidates, req.Policy, req.Context.Namespace)
	return targets, "workspace affinity applied", err
}

// findExistingClusterTarget finds an existing cluster placement target.
func (b *Binder) findExistingClusterTarget(sessions []tmcv1alpha1.SessionState) *tmcv1alpha1.PlacementTarget {
	for _, session := range sessions {
		if session.Status.CurrentPlacement != nil && session.Status.Phase == tmcv1alpha1.SessionActive {
			return session.Status.CurrentPlacement
		}
	}
	return nil
}

// findExistingNodeTarget finds an existing node placement target.
func (b *Binder) findExistingNodeTarget(sessions []tmcv1alpha1.SessionState) *tmcv1alpha1.PlacementTarget {
	for _, session := range sessions {
		if session.Status.CurrentPlacement != nil && 
		   session.Status.Phase == tmcv1alpha1.SessionActive &&
		   len(session.Status.CurrentPlacement.NodeSelector) > 0 {
			return session.Status.CurrentPlacement
		}
	}
	return nil
}

// isTargetViable checks if an existing target is still viable based on candidates and stickiness.
func (b *Binder) isTargetViable(existing *tmcv1alpha1.PlacementTarget, candidates []tmcv1alpha1.PlacementTarget, stickinessFactor float64) bool {
	// Check if the existing target is still in the candidate list
	for _, candidate := range candidates {
		if candidate.ClusterName == existing.ClusterName {
			// Apply stickiness factor - higher values prefer existing placement
			return true // Simplified for now - could add more sophisticated logic
		}
	}
	return false
}

// createSessionState creates a new session state resource.
func (b *Binder) createSessionState(req *BindingRequest, targets []tmcv1alpha1.PlacementTarget) (*tmcv1alpha1.SessionState, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets selected for session")
	}
	
	sessionID := b.generateSessionID(req.WorkloadReference, req.Context.RequestTime)
	
	// Calculate expiration time
	var expiresAt *metav1.Time
	if req.Policy != nil && req.Policy.Spec.SessionTTL != nil {
		expiry := req.Context.RequestTime.Add(req.Policy.Spec.SessionTTL.Duration)
		expiresAt = &metav1.Time{Time: expiry}
	}
	
	session := &tmcv1alpha1.SessionState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", req.WorkloadReference.Name, sessionID[:8]),
			Namespace: req.Context.Namespace,
			Labels: map[string]string{
				"tmc.kcp.io/workload-name": req.WorkloadReference.Name,
				"tmc.kcp.io/workload-kind": req.WorkloadReference.Kind,
				"tmc.kcp.io/session-id":    sessionID,
			},
		},
		Spec: tmcv1alpha1.SessionStateSpec{
			WorkloadReference: req.WorkloadReference,
			PlacementTargets:  targets,
			SessionID:         sessionID,
			CreatedAt:         metav1.NewTime(req.Context.RequestTime),
			ExpiresAt:         expiresAt,
		},
		Status: tmcv1alpha1.SessionStateStatus{
			Phase:            tmcv1alpha1.SessionPending,
			CurrentPlacement: &targets[0], // Primary target
		},
	}
	
	return session, nil
}

// generateSessionID creates a unique session identifier.
func (b *Binder) generateSessionID(workload tmcv1alpha1.WorkloadReference, timestamp time.Time) string {
	// Generate a session ID based on workload and timestamp
	return fmt.Sprintf("%s-%s-%d", workload.Kind, workload.Name, timestamp.Unix())
}

// generateConditions creates conditions for the binding result.
func (b *Binder) generateConditions(req *BindingRequest, targets []tmcv1alpha1.PlacementTarget) []metav1.Condition {
	conditions := []metav1.Condition{}
	
	if len(targets) > 0 {
		conditions = append(conditions, metav1.Condition{
			Type:   "Bound",
			Status: metav1.ConditionTrue,
			Reason: "TargetsSelected",
			Message: fmt.Sprintf("Successfully bound to %d target(s)", len(targets)),
			LastTransitionTime: metav1.NewTime(req.Context.RequestTime),
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:   "Bound",
			Status: metav1.ConditionFalse,
			Reason: "NoTargetsAvailable",
			Message: "No suitable targets found for binding",
			LastTransitionTime: metav1.NewTime(req.Context.RequestTime),
		})
	}
	
	return conditions
}

// validateRequest validates the binding request parameters.
func (b *Binder) validateRequest(req *BindingRequest) error {
	if req.WorkloadReference.Name == "" {
		return fmt.Errorf("workload name cannot be empty")
	}
	
	if req.WorkloadReference.Kind == "" {
		return fmt.Errorf("workload kind cannot be empty")
	}
	
	if req.Context.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	
	if req.Context.RequestTime.IsZero() {
		return fmt.Errorf("request time cannot be zero")
	}
	
	return nil
}