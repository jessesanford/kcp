package decision

import (
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DecisionMaker is the main interface for placement decision making
type DecisionMaker interface {
	// MakeDecision creates a placement decision for the given placement request
	MakeDecision(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, candidates []*CandidateTarget) (*placementv1alpha1.PlacementDecision, error)
	
	// ValidateDecision validates a placement decision against policies and constraints
	ValidateDecision(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, decision *placementv1alpha1.PlacementDecision) error
	
	// RecordDecision records a decision for audit and analysis purposes
	RecordDecision(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, decision *placementv1alpha1.PlacementDecision, duration time.Duration) error
	
	// ApplyOverride applies an override to a placement decision
	ApplyOverride(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, decision *placementv1alpha1.PlacementDecision, override *OverrideSpec, userID string) (*placementv1alpha1.PlacementDecision, error)
}

// DecisionContext provides context information for decision making
type DecisionContext struct {
	// Request metadata
	RequestID   string                `json:"requestId,omitempty"`
	UserID      string                `json:"userId,omitempty"`
	UserAgent   string                `json:"userAgent,omitempty"`
	Timestamp   time.Time             `json:"timestamp"`
	Workspace   string                `json:"workspace"`
	
	// Decision parameters
	Strategy    placementv1alpha1.PlacementStrategy `json:"strategy"`
	Constraints *placementv1alpha1.SchedulingConstraint `json:"constraints,omitempty"`
	Policies    []string              `json:"policies,omitempty"`
	
	// Environment context
	Emergency   bool                  `json:"emergency,omitempty"`
	Priority    *int32                `json:"priority,omitempty"`
	Deadline    *time.Time            `json:"deadline,omitempty"`
	
	// Metrics and tracing
	TraceID     string                `json:"traceId,omitempty"`
	SpanID      string                `json:"spanId,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DecisionResult represents the outcome of a placement decision
type DecisionResult struct {
	// Decision information
	Decision    *placementv1alpha1.PlacementDecision `json:"decision"`
	Status      DecisionResultStatus `json:"status"`
	Error       error               `json:"error,omitempty"`
	
	// Decision analysis
	Candidates  []*CandidateTarget  `json:"candidates"`
	Selected    []*SelectedTarget   `json:"selected"`
	Rejected    []*RejectedTarget   `json:"rejected"`
	
	// Scoring information
	Scores      map[string]float64  `json:"scores"`
	Reasons     []DecisionReason    `json:"reasons"`
	Violations  []PolicyViolation   `json:"violations,omitempty"`
	
	// Performance metrics
	Duration    time.Duration       `json:"duration"`
	Metrics     DecisionMetrics     `json:"metrics"`
	
	// Audit information
	RecordID    string              `json:"recordId,omitempty"`
	OverrideID  string              `json:"overrideId,omitempty"`
}

// DecisionResultStatus indicates the outcome of a decision attempt
type DecisionResultStatus string

const (
	DecisionResultStatusSuccess   DecisionResultStatus = "Success"
	DecisionResultStatusFailed    DecisionResultStatus = "Failed"
	DecisionResultStatusRejected  DecisionResultStatus = "Rejected"
	DecisionResultStatusOverridden DecisionResultStatus = "Overridden"
	DecisionResultStatusPending   DecisionResultStatus = "Pending"
)

// DecisionRecord represents a recorded placement decision
type DecisionRecord struct {
	// Decision metadata
	ID        string                                `json:"id"`
	Timestamp time.Time                           `json:"timestamp"`
	Placement *placementv1alpha1.WorkloadPlacement `json:"placement"`
	Decision  *placementv1alpha1.PlacementDecision `json:"decision"`

	// Decision context
	Candidates []*CandidateTarget                `json:"candidates"`
	Scores     map[string]float64               `json:"scores"`
	Reasons    []DecisionReason                 `json:"reasons"`
	Constraints *placementv1alpha1.SchedulingConstraint `json:"constraints,omitempty"`

	// Decision outcome
	Status     DecisionStatus                   `json:"status"`
	Error      string                          `json:"error,omitempty"`
	Duration   time.Duration                   `json:"duration"`
	Violations []PolicyViolation               `json:"violations,omitempty"`

	// Metadata for analysis
	SchedulerVersion string                      `json:"schedulerVersion"`
	UserAgent       string                      `json:"userAgent"`
	RequestID       string                      `json:"requestId,omitempty"`
}

// DecisionReason explains why a decision was made
type DecisionReason struct {
	Type        string                 `json:"type"`
	Message     string                 `json:"message"`
	SyncTarget  string                 `json:"syncTarget,omitempty"`
	Score       float64                `json:"score,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DecisionStatus represents the status of a decision
type DecisionStatus string

const (
	DecisionStatusPending   DecisionStatus = "Pending"
	DecisionStatusScheduled DecisionStatus = "Scheduled"
	DecisionStatusFailed    DecisionStatus = "Failed"
	DecisionStatusRejected  DecisionStatus = "Rejected"
	DecisionStatusExpired   DecisionStatus = "Expired"
)

// PolicyViolation represents a policy constraint violation
type PolicyViolation struct {
	Policy     string                 `json:"policy"`
	Rule       string                 `json:"rule"`
	Message    string                 `json:"message"`
	Severity   ViolationSeverity      `json:"severity"`
	SyncTarget string                 `json:"syncTarget,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ViolationSeverity indicates how severe a policy violation is
type ViolationSeverity string

const (
	ViolationSeverityError   ViolationSeverity = "Error"
	ViolationSeverityWarning ViolationSeverity = "Warning"
	ViolationSeverityInfo    ViolationSeverity = "Info"
)

// CandidateTarget represents a potential placement target
type CandidateTarget struct {
	SyncTarget *workloadv1alpha1.SyncTarget
	Workspace  string
	Score      float64
	Reasons    []string
	Violations []PolicyViolation
}

// MetricsCollector collects placement decision metrics
type MetricsCollector interface {
	RecordDecision(ctx context.Context, record *DecisionRecord)
	RecordDecisionLatency(duration time.Duration)
	RecordDecisionError(errorType string)
	RecordPolicyViolation(violation PolicyViolation)
}

// DecisionValidator validates placement decisions
type DecisionValidator interface {
	ValidateDecision(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, decision *placementv1alpha1.PlacementDecision) error
	ValidateConstraints(ctx context.Context, constraints *placementv1alpha1.SchedulingConstraint) error
}

// DecisionStorage provides persistent storage for decisions
type DecisionStorage interface {
	StoreDecision(ctx context.Context, record *DecisionRecord) error
	GetDecision(ctx context.Context, id string) (*DecisionRecord, error)
	ListDecisions(ctx context.Context, placement string) ([]*DecisionRecord, error)
	DeleteDecision(ctx context.Context, id string) error
	PurgeOldDecisions(ctx context.Context, before time.Time) error
}

// OverrideSpec represents the specification for creating an override
type OverrideSpec struct {
	Reason    string                 `json:"reason"`
	Emergency bool                   `json:"emergency"`
	Source    string                 `json:"source"`
	TTL       *time.Duration         `json:"ttl,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// User represents a user making placement requests
type User struct {
	Name      string   `json:"name"`
	Groups    []string `json:"groups,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
	Extra     map[string][]string `json:"extra,omitempty"`
}