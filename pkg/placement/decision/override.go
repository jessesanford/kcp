package decision

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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

// OverridePolicy defines rules for when and how to override decisions
type OverridePolicy struct {
	// Policy metadata
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Priority    int32                  `json:"priority"`
	Enabled     bool                   `json:"enabled"`
	Created     time.Time              `json:"created"`
	LastUpdated time.Time              `json:"lastUpdated"`
	Version     string                 `json:"version"`

	// Policy scope and targeting
	Selector    *metav1.LabelSelector  `json:"selector,omitempty"`
	Workspaces  []string               `json:"workspaces,omitempty"`
	Clusters    []string               `json:"clusters,omitempty"`
	Resources   []string               `json:"resources,omitempty"`

	// Override rules
	Rules       []OverrideRule         `json:"rules"`
	Conditions  []OverrideCondition    `json:"conditions,omitempty"`
	Actions     []OverrideAction       `json:"actions"`

	// Policy constraints
	MaxOverrides    *int32             `json:"maxOverrides,omitempty"`
	TTL             *time.Duration     `json:"ttl,omitempty"`
	RequiredRoles   []string           `json:"requiredRoles,omitempty"`
	ApprovalRequired bool              `json:"approvalRequired,omitempty"`

	// Emergency settings
	EmergencyOnly   bool               `json:"emergencyOnly,omitempty"`
	SeverityLevel   SeverityLevel      `json:"severityLevel,omitempty"`
	AutoRevert      bool               `json:"autoRevert,omitempty"`
	RevertDelay     *time.Duration     `json:"revertDelay,omitempty"`
}

// OverrideRule defines specific override behavior
type OverrideRule struct {
	Name          string                    `json:"name"`
	Description   string                    `json:"description,omitempty"`
	Type          OverrideRuleType          `json:"type"`
	Target        OverrideTarget            `json:"target"`
	Operation     OverrideOperation         `json:"operation"`
	Value         interface{}               `json:"value,omitempty"`
	Conditions    []string                  `json:"conditions,omitempty"`
	Priority      int32                     `json:"priority"`
	Temporary     bool                      `json:"temporary"`
	Duration      *time.Duration            `json:"duration,omitempty"`
}

// OverrideCondition defines when an override should be applied
type OverrideCondition struct {
	Type      ConditionType         `json:"type"`
	Field     string                `json:"field"`
	Operator  ConditionOperator     `json:"operator"`
	Value     interface{}           `json:"value"`
	Threshold *float64              `json:"threshold,omitempty"`
	Duration  *time.Duration        `json:"duration,omitempty"`
}

// OverrideAction defines what to do when an override is triggered
type OverrideAction struct {
	Type        ActionType            `json:"type"`
	Target      string                `json:"target"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Notification *NotificationConfig   `json:"notification,omitempty"`
	Rollback    *RollbackConfig       `json:"rollback,omitempty"`
}

// ActiveOverride represents a currently active override
type ActiveOverride struct {
	// Override identification
	ID          string                `json:"id"`
	PolicyName  string                `json:"policyName"`
	RuleName    string                `json:"ruleName"`
	TargetName  string                `json:"targetName"`
	Created     time.Time             `json:"created"`
	ExpiresAt   *time.Time            `json:"expiresAt,omitempty"`

	// Override context
	Placement   *placementv1alpha1.WorkloadPlacement `json:"placement"`
	Original    *placementv1alpha1.PlacementDecision `json:"original"`
	Override    *placementv1alpha1.PlacementDecision `json:"override"`
	Reason      string                `json:"reason"`
	Emergency   bool                  `json:"emergency"`
	UserID      string                `json:"userId"`
	Source      string                `json:"source"`

	// Override state
	Status      OverrideStatus        `json:"status"`
	Metrics     OverrideMetrics       `json:"metrics"`
	History     []OverrideEvent       `json:"history"`

	// Approval and authorization
	ApprovalRequired bool              `json:"approvalRequired"`
	ApprovedBy       []string          `json:"approvedBy,omitempty"`
	ApprovedAt       *time.Time        `json:"approvedAt,omitempty"`
	RejectedBy       []string          `json:"rejectedBy,omitempty"`
	RejectedAt       *time.Time        `json:"rejectedAt,omitempty"`
}

// OverrideRecord records the history of an override for audit purposes
type OverrideRecord struct {
	Override    *ActiveOverride       `json:"override"`
	Timestamp   time.Time             `json:"timestamp"`
	Action      string                `json:"action"`
	Result      string                `json:"result"`
	UserID      string                `json:"userId"`
	Source      string                `json:"source"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Various enums and types

type OverrideRuleType string
const (
	OverrideRuleTypeClusterSelection OverrideRuleType = "ClusterSelection"
	OverrideRuleTypeResourceLimit    OverrideRuleType = "ResourceLimit"
	OverrideRuleTypeSchedulingPolicy OverrideRuleType = "SchedulingPolicy"
	OverrideRuleTypeEmergencyDrain   OverrideRuleType = "EmergencyDrain"
	OverrideRuleTypePriority         OverrideRuleType = "Priority"
)

type OverrideTarget string
const (
	OverrideTargetClusters    OverrideTarget = "Clusters"
	OverrideTargetWorkspaces  OverrideTarget = "Workspaces"
	OverrideTargetResources   OverrideTarget = "Resources"
	OverrideTargetStrategy    OverrideTarget = "Strategy"
	OverrideTargetConstraints OverrideTarget = "Constraints"
)

type OverrideOperation string
const (
	OverrideOperationReplace  OverrideOperation = "Replace"
	OverrideOperationAdd      OverrideOperation = "Add"
	OverrideOperationRemove   OverrideOperation = "Remove"
	OverrideOperationScale    OverrideOperation = "Scale"
	OverrideOperationDrain    OverrideOperation = "Drain"
	OverrideOperationBlock    OverrideOperation = "Block"
)

type ConditionType string
const (
	ConditionTypeMetric       ConditionType = "Metric"
	ConditionTypeTime         ConditionType = "Time"
	ConditionTypeEvent        ConditionType = "Event"
	ConditionTypeHealth       ConditionType = "Health"
	ConditionTypeEmergency    ConditionType = "Emergency"
)

type ConditionOperator string
const (
	ConditionOperatorEqual        ConditionOperator = "Equal"
	ConditionOperatorNotEqual     ConditionOperator = "NotEqual"
	ConditionOperatorGreater      ConditionOperator = "Greater"
	ConditionOperatorLess         ConditionOperator = "Less"
	ConditionOperatorGreaterEqual ConditionOperator = "GreaterEqual"
	ConditionOperatorLessEqual    ConditionOperator = "LessEqual"
	ConditionOperatorContains     ConditionOperator = "Contains"
	ConditionOperatorMatches      ConditionOperator = "Matches"
)

type ActionType string
const (
	ActionTypeOverride    ActionType = "Override"
	ActionTypeNotify      ActionType = "Notify"
	ActionTypeBlock       ActionType = "Block"
	ActionTypeEscalate    ActionType = "Escalate"
	ActionTypeRollback    ActionType = "Rollback"
)

type OverrideStatus string
const (
	OverrideStatusPending     OverrideStatus = "Pending"
	OverrideStatusActive      OverrideStatus = "Active"
	OverrideStatusExpired     OverrideStatus = "Expired"
	OverrideStatusReverted    OverrideStatus = "Reverted"
	OverrideStatusRejected    OverrideStatus = "Rejected"
	OverrideStatusFailed      OverrideStatus = "Failed"
)

type SeverityLevel string
const (
	SeverityLevelLow       SeverityLevel = "Low"
	SeverityLevelMedium    SeverityLevel = "Medium"
	SeverityLevelHigh      SeverityLevel = "High"
	SeverityLevelCritical  SeverityLevel = "Critical"
	SeverityLevelEmergency SeverityLevel = "Emergency"
)

// Configuration structures

type NotificationConfig struct {
	Channels  []string               `json:"channels"`
	Template  string                 `json:"template,omitempty"`
	Recipients []string              `json:"recipients,omitempty"`
	Urgent    bool                   `json:"urgent"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type RollbackConfig struct {
	Enabled     bool           `json:"enabled"`
	Delay       time.Duration  `json:"delay"`
	Conditions  []string       `json:"conditions,omitempty"`
	AutoRevert  bool           `json:"autoRevert"`
	MaxRetries  int32          `json:"maxRetries"`
}

type OverrideMetrics struct {
	AppliedAt      time.Time     `json:"appliedAt"`
	Duration       time.Duration `json:"duration"`
	AffectedPods   int32         `json:"affectedPods"`
	AffectedClusters int32       `json:"affectedClusters"`
	SuccessRate    float64       `json:"successRate"`
	ErrorCount     int32         `json:"errorCount"`
	LastError      string        `json:"lastError,omitempty"`
}

type OverrideEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Interface definitions

type OverrideMetricsCollector interface {
	RecordOverride(ctx context.Context, override *ActiveOverride)
	RecordOverrideLatency(duration time.Duration)
	RecordOverrideError(errorType string)
	RecordEmergencyOverride(severity SeverityLevel)
}

type OverrideAuthChecker interface {
	CanCreateOverride(ctx context.Context, user string, policy *OverridePolicy) (bool, error)
	CanApproveOverride(ctx context.Context, user string, override *ActiveOverride) (bool, error)
	RequiresApproval(ctx context.Context, override *ActiveOverride) bool
}

type OverrideValidator interface {
	ValidateOverride(ctx context.Context, override *ActiveOverride) error
	ValidatePolicy(ctx context.Context, policy *OverridePolicy) error
}

type OverrideNotifier interface {
	NotifyOverrideCreated(ctx context.Context, override *ActiveOverride) error
	NotifyOverrideApproved(ctx context.Context, override *ActiveOverride) error
	NotifyOverrideExpired(ctx context.Context, override *ActiveOverride) error
	NotifyEmergencyOverride(ctx context.Context, override *ActiveOverride) error
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

// OverrideSpec represents the specification for creating an override
type OverrideSpec struct {
	Reason    string                 `json:"reason"`
	Emergency bool                   `json:"emergency"`
	Source    string                 `json:"source"`
	TTL       *time.Duration         `json:"ttl,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}