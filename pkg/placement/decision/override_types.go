package decision

import (
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// Override type enumerations

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

// Interface definitions for override functionality

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