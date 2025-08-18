package decision

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// DecisionOverride manages placement decision overrides and emergency interventions
type DecisionOverride struct {
	// Client for interacting with KCP
	kcpClient kcpclientset.ClusterInterface

	// Event recorder for generating events
	eventRecorder record.EventRecorder

	// Override policies and rules
	overridePolicies map[string]*OverridePolicy
	policiesMutex    sync.RWMutex

	// Active overrides cache
	activeOverrides map[string]*ActiveOverride
	overridesMutex  sync.RWMutex

	// Override history for audit
	overrideHistory []*OverrideRecord
	historyMutex    sync.RWMutex

	// Metrics collector
	metricsCollector OverrideMetricsCollector

	// Configuration
	maxHistorySize   int
	defaultTTL       time.Duration
	emergencyEnabled bool

	// Authorization checker
	authChecker OverrideAuthChecker

	// Decision validator
	validator OverrideValidator

	// Notification system
	notifier OverrideNotifier
}

// NewDecisionOverride creates a new decision override manager
func NewDecisionOverride(
	kcpClient kcpclientset.ClusterInterface,
	eventRecorder record.EventRecorder,
	metricsCollector OverrideMetricsCollector,
	authChecker OverrideAuthChecker,
	validator OverrideValidator,
	notifier OverrideNotifier,
) *DecisionOverride {
	return &DecisionOverride{
		kcpClient:        kcpClient,
		eventRecorder:    eventRecorder,
		metricsCollector: metricsCollector,
		authChecker:      authChecker,
		validator:        validator,
		notifier:         notifier,
		overridePolicies: make(map[string]*OverridePolicy),
		activeOverrides:  make(map[string]*ActiveOverride),
		overrideHistory:  make([]*OverrideRecord, 0),
		maxHistorySize:   1000,
		defaultTTL:       24 * time.Hour,
		emergencyEnabled: true,
	}
}

// ApplyOverride applies an override to a placement decision
func (o *DecisionOverride) ApplyOverride(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
	overrideSpec *OverrideSpec,
	userID string,
) (*placementv1alpha1.PlacementDecision, *ActiveOverride, error) {
	logger := klog.FromContext(ctx)
	startTime := time.Now()

	// Find matching policies
	matchingPolicies := o.findMatchingPolicies(ctx, placement, overrideSpec)
	if len(matchingPolicies) == 0 {
		return nil, nil, fmt.Errorf("no matching override policies found")
	}

	// Sort by priority (highest first)
	sort.Slice(matchingPolicies, func(i, j int) bool {
		return matchingPolicies[i].Priority > matchingPolicies[j].Priority
	})

	policy := matchingPolicies[0]

	// Check authorization
	if o.authChecker != nil {
		canCreate, err := o.authChecker.CanCreateOverride(ctx, userID, policy)
		if err != nil {
			return nil, nil, fmt.Errorf("authorization check failed: %w", err)
		}
		if !canCreate {
			return nil, nil, fmt.Errorf("user %s not authorized to create override", userID)
		}
	}

	// Create active override
	override := &ActiveOverride{
		ID:         o.generateOverrideID(),
		PolicyName: policy.Name,
		TargetName: placement.Name,
		Created:    startTime,
		Placement:  placement.DeepCopy(),
		Original:   decision.DeepCopy(),
		Reason:     overrideSpec.Reason,
		Emergency:  overrideSpec.Emergency,
		UserID:     userID,
		Source:     overrideSpec.Source,
		Status:     OverrideStatusPending,
		Metrics:    OverrideMetrics{},
		History:    []OverrideEvent{},
	}

	// Set expiration if TTL is specified
	if policy.TTL != nil {
		expiresAt := startTime.Add(*policy.TTL)
		override.ExpiresAt = &expiresAt
	} else {
		expiresAt := startTime.Add(o.defaultTTL)
		override.ExpiresAt = &expiresAt
	}

	// Check if approval is required
	if o.authChecker != nil && o.authChecker.RequiresApproval(ctx, override) {
		override.ApprovalRequired = true
		override.Status = OverrideStatusPending
	}

	// Validate override
	if o.validator != nil {
		if err := o.validator.ValidateOverride(ctx, override); err != nil {
			return nil, nil, fmt.Errorf("override validation failed: %w", err)
		}
	}

	// Apply override rules to decision
	modifiedDecision, err := o.applyOverrideRules(ctx, decision, policy, overrideSpec)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to apply override rules: %w", err)
	}

	override.Override = modifiedDecision

	// Store active override
	o.storeActiveOverride(override)

	// Record metrics
	if o.metricsCollector != nil {
		o.metricsCollector.RecordOverride(ctx, override)
		if override.Emergency {
			o.metricsCollector.RecordEmergencyOverride(policy.SeverityLevel)
		}
	}

	// Send notifications
	if o.notifier != nil {
		if override.Emergency {
			if err := o.notifier.NotifyEmergencyOverride(ctx, override); err != nil {
				logger.Error(err, "Failed to send emergency override notification")
			}
		} else {
			if err := o.notifier.NotifyOverrideCreated(ctx, override); err != nil {
				logger.Error(err, "Failed to send override creation notification")
			}
		}
	}

	// Generate event
	eventType := corev1.EventTypeNormal
	if override.Emergency {
		eventType = corev1.EventTypeWarning
	}
	o.eventRecorder.Eventf(placement, eventType, "OverrideApplied",
		"Applied %s override: %s", policy.Name, override.Reason)

	// Record in history
	o.recordOverrideEvent(override, "created", "success", userID, overrideSpec.Source, nil)

	logger.Info("Applied placement override",
		"override", override.ID,
		"policy", policy.Name,
		"placement", placement.Name,
		"emergency", override.Emergency,
		"user", userID,
	)

	return modifiedDecision, override, nil
}

// ApproveOverride approves a pending override
func (o *DecisionOverride) ApproveOverride(ctx context.Context, overrideID, userID string) error {
	logger := klog.FromContext(ctx)

	o.overridesMutex.Lock()
	defer o.overridesMutex.Unlock()

	override, exists := o.activeOverrides[overrideID]
	if !exists {
		return fmt.Errorf("override not found: %s", overrideID)
	}

	if override.Status != OverrideStatusPending {
		return fmt.Errorf("override %s is not pending approval", overrideID)
	}

	// Check approval authorization
	if o.authChecker != nil {
		canApprove, err := o.authChecker.CanApproveOverride(ctx, userID, override)
		if err != nil {
			return fmt.Errorf("approval authorization check failed: %w", err)
		}
		if !canApprove {
			return fmt.Errorf("user %s not authorized to approve override", userID)
		}
	}

	// Update override status
	now := time.Now()
	override.Status = OverrideStatusActive
	override.ApprovedBy = append(override.ApprovedBy, userID)
	override.ApprovedAt = &now

	// Send notification
	if o.notifier != nil {
		if err := o.notifier.NotifyOverrideApproved(ctx, override); err != nil {
			logger.Error(err, "Failed to send override approval notification")
		}
	}

	// Generate event
	o.eventRecorder.Eventf(override.Placement, corev1.EventTypeNormal, "OverrideApproved",
		"Override %s approved by %s", overrideID, userID)

	// Record in history
	o.recordOverrideEvent(override, "approved", "success", userID, "api", nil)

	logger.Info("Approved placement override",
		"override", overrideID,
		"user", userID,
	)

	return nil
}

// RejectOverride rejects a pending override
func (o *DecisionOverride) RejectOverride(ctx context.Context, overrideID, userID, reason string) error {
	logger := klog.FromContext(ctx)

	o.overridesMutex.Lock()
	defer o.overridesMutex.Unlock()

	override, exists := o.activeOverrides[overrideID]
	if !exists {
		return fmt.Errorf("override not found: %s", overrideID)
	}

	if override.Status != OverrideStatusPending {
		return fmt.Errorf("override %s is not pending approval", overrideID)
	}

	// Update override status
	now := time.Now()
	override.Status = OverrideStatusRejected
	override.RejectedBy = append(override.RejectedBy, userID)
	override.RejectedAt = &now

	// Generate event
	o.eventRecorder.Eventf(override.Placement, corev1.EventTypeWarning, "OverrideRejected",
		"Override %s rejected by %s: %s", overrideID, userID, reason)

	// Record in history
	metadata := map[string]interface{}{"reason": reason}
	o.recordOverrideEvent(override, "rejected", "success", userID, "api", metadata)

	logger.Info("Rejected placement override",
		"override", overrideID,
		"user", userID,
		"reason", reason,
	)

	return nil
}

// RevertOverride reverts an active override
func (o *DecisionOverride) RevertOverride(ctx context.Context, overrideID, userID string) error {
	logger := klog.FromContext(ctx)

	o.overridesMutex.Lock()
	defer o.overridesMutex.Unlock()

	override, exists := o.activeOverrides[overrideID]
	if !exists {
		return fmt.Errorf("override not found: %s", overrideID)
	}

	if override.Status != OverrideStatusActive {
		return fmt.Errorf("override %s is not active", overrideID)
	}

	// Update override status
	override.Status = OverrideStatusReverted

	// Generate event
	o.eventRecorder.Eventf(override.Placement, corev1.EventTypeNormal, "OverrideReverted",
		"Override %s reverted by %s", overrideID, userID)

	// Record in history
	o.recordOverrideEvent(override, "reverted", "success", userID, "api", nil)

	logger.Info("Reverted placement override",
		"override", overrideID,
		"user", userID,
	)

	return nil
}

// GetActiveOverrides returns all currently active overrides
func (o *DecisionOverride) GetActiveOverrides() []*ActiveOverride {
	o.overridesMutex.RLock()
	defer o.overridesMutex.RUnlock()

	var active []*ActiveOverride
	for _, override := range o.activeOverrides {
		if override.Status == OverrideStatusActive {
			active = append(active, override)
		}
	}
	return active
}

// GetOverride retrieves an override by ID
func (o *DecisionOverride) GetOverride(overrideID string) (*ActiveOverride, error) {
	o.overridesMutex.RLock()
	defer o.overridesMutex.RUnlock()

	override, exists := o.activeOverrides[overrideID]
	if !exists {
		return nil, fmt.Errorf("override not found: %s", overrideID)
	}

	return override, nil
}

// CleanupExpiredOverrides removes expired overrides
func (o *DecisionOverride) CleanupExpiredOverrides(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	now := time.Now()
	expired := 0

	o.overridesMutex.Lock()
	defer o.overridesMutex.Unlock()

	for id, override := range o.activeOverrides {
		if override.ExpiresAt != nil && override.ExpiresAt.Before(now) {
			override.Status = OverrideStatusExpired
			
			// Send notification
			if o.notifier != nil {
				if err := o.notifier.NotifyOverrideExpired(ctx, override); err != nil {
					logger.Error(err, "Failed to send override expiry notification")
				}
			}
			
			// Generate event
			o.eventRecorder.Eventf(override.Placement, corev1.EventTypeNormal, "OverrideExpired",
				"Override %s expired", id)
			
			// Record in history
			o.recordOverrideEvent(override, "expired", "success", "system", "cleanup", nil)
			
			expired++
		}
	}

	if expired > 0 {
		logger.Info("Cleaned up expired overrides", "count", expired)
	}

	return nil
}

// Helper methods

func (o *DecisionOverride) findMatchingPolicies(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, spec *OverrideSpec) []*OverridePolicy {
	o.policiesMutex.RLock()
	defer o.policiesMutex.RUnlock()

	var matching []*OverridePolicy
	
	for _, policy := range o.overridePolicies {
		if !policy.Enabled {
			continue
		}

		// Check selector match
		if policy.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.Selector)
			if err != nil {
				continue
			}
			if !selector.Matches(labels.Set(placement.Labels)) {
				continue
			}
		}

		// Check workspace match
		if len(policy.Workspaces) > 0 {
			found := false
			for _, ws := range policy.Workspaces {
				if ws == placement.Namespace {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check emergency requirements
		if policy.EmergencyOnly && !spec.Emergency {
			continue
		}

		matching = append(matching, policy)
	}

	return matching
}

func (o *DecisionOverride) applyOverrideRules(
	ctx context.Context,
	decision *placementv1alpha1.PlacementDecision,
	policy *OverridePolicy,
	spec *OverrideSpec,
) (*placementv1alpha1.PlacementDecision, error) {
	
	// Start with a copy of the original decision
	modified := decision.DeepCopy()

	// Apply each rule in priority order
	sort.Slice(policy.Rules, func(i, j int) bool {
		return policy.Rules[i].Priority > policy.Rules[j].Priority
	})

	for _, rule := range policy.Rules {
		if err := o.applyRule(ctx, modified, &rule, spec); err != nil {
			return nil, fmt.Errorf("failed to apply rule %s: %w", rule.Name, err)
		}
	}

	return modified, nil
}

func (o *DecisionOverride) applyRule(
	ctx context.Context,
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	switch rule.Type {
	case OverrideRuleTypeClusterSelection:
		return o.applyClusterSelectionRule(decision, rule, spec)
	case OverrideRuleTypeResourceLimit:
		return o.applyResourceLimitRule(decision, rule, spec)
	case OverrideRuleTypeSchedulingPolicy:
		return o.applySchedulingPolicyRule(decision, rule, spec)
	case OverrideRuleTypeEmergencyDrain:
		return o.applyEmergencyDrainRule(decision, rule, spec)
	case OverrideRuleTypePriority:
		return o.applyPriorityRule(decision, rule, spec)
	default:
		return fmt.Errorf("unknown rule type: %s", rule.Type)
	}
}

func (o *DecisionOverride) applyClusterSelectionRule(
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	switch rule.Operation {
	case OverrideOperationReplace:
		if clusters, ok := rule.Value.([]string); ok {
			decision.Spec.Clusters = nil
			for _, cluster := range clusters {
				decision.Spec.Clusters = append(decision.Spec.Clusters, placementv1alpha1.ClusterDecision{
					ClusterName: cluster,
					Weight:      100, // Default weight
				})
			}
		}
	case OverrideOperationAdd:
		if clusters, ok := rule.Value.([]string); ok {
			for _, cluster := range clusters {
				// Check if cluster already exists
				exists := false
				for _, existing := range decision.Spec.Clusters {
					if existing.ClusterName == cluster {
						exists = true
						break
					}
				}
				if !exists {
					decision.Spec.Clusters = append(decision.Spec.Clusters, placementv1alpha1.ClusterDecision{
						ClusterName: cluster,
						Weight:      50, // Lower weight for added clusters
					})
				}
			}
		}
	case OverrideOperationRemove:
		if clusters, ok := rule.Value.([]string); ok {
			var filtered []placementv1alpha1.ClusterDecision
			for _, existing := range decision.Spec.Clusters {
				shouldRemove := false
				for _, cluster := range clusters {
					if existing.ClusterName == cluster {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					filtered = append(filtered, existing)
				}
			}
			decision.Spec.Clusters = filtered
		}
	}
	return nil
}

func (o *DecisionOverride) applyResourceLimitRule(
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	// Implementation for resource limit overrides
	// This would modify resource constraints on the decision
	return nil
}

func (o *DecisionOverride) applySchedulingPolicyRule(
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	// Implementation for scheduling policy overrides
	// This would modify the scheduling strategy or constraints
	return nil
}

func (o *DecisionOverride) applyEmergencyDrainRule(
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	// Implementation for emergency drain rules
	// This would remove clusters that need to be drained
	if clusters, ok := rule.Value.([]string); ok {
		var filtered []placementv1alpha1.ClusterDecision
		for _, existing := range decision.Spec.Clusters {
			shouldDrain := false
			for _, cluster := range clusters {
				if existing.ClusterName == cluster {
					shouldDrain = true
					break
				}
			}
			if !shouldDrain {
				filtered = append(filtered, existing)
			}
		}
		decision.Spec.Clusters = filtered
	}
	return nil
}

func (o *DecisionOverride) applyPriorityRule(
	decision *placementv1alpha1.PlacementDecision,
	rule *OverrideRule,
	spec *OverrideSpec,
) error {
	// Implementation for priority overrides
	// This would adjust cluster weights based on priority
	return nil
}

func (o *DecisionOverride) storeActiveOverride(override *ActiveOverride) {
	o.overridesMutex.Lock()
	defer o.overridesMutex.Unlock()
	
	o.activeOverrides[override.ID] = override
}

func (o *DecisionOverride) recordOverrideEvent(
	override *ActiveOverride,
	action, result, userID, source string,
	metadata map[string]interface{},
) {
	record := &OverrideRecord{
		Override:  override,
		Timestamp: time.Now(),
		Action:    action,
		Result:    result,
		UserID:    userID,
		Source:    source,
		Metadata:  metadata,
	}

	o.historyMutex.Lock()
	defer o.historyMutex.Unlock()

	o.overrideHistory = append(o.overrideHistory, record)
	
	// Trim history if too large
	if len(o.overrideHistory) > o.maxHistorySize {
		o.overrideHistory = o.overrideHistory[len(o.overrideHistory)-o.maxHistorySize:]
	}
}

func (o *DecisionOverride) generateOverrideID() string {
	return fmt.Sprintf("override-%d", time.Now().UnixNano())
}