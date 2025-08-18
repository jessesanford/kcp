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

// SelectedTarget represents a cluster selected for placement
type SelectedTarget struct {
	Target      *CandidateTarget    `json:"target"`
	Weight      int32               `json:"weight"`
	Reason      string              `json:"reason"`
	Score       float64             `json:"score"`
	Rank        int32               `json:"rank"`
	Resources   *ResourceAllocation `json:"resources,omitempty"`
}

// RejectedTarget represents a cluster that was considered but not selected
type RejectedTarget struct {
	Target    *CandidateTarget  `json:"target"`
	Reason    string            `json:"reason"`
	Score     float64           `json:"score,omitempty"`
	Violations []PolicyViolation `json:"violations,omitempty"`
}

// ResourceAllocation represents the resource allocation for a placement
type ResourceAllocation struct {
	Requested   corev1.ResourceList `json:"requested"`
	Allocated   corev1.ResourceList `json:"allocated"`
	Available   corev1.ResourceList `json:"available"`
	Utilization map[string]float64  `json:"utilization"`
}

// DecisionMetrics provides metrics about the decision process
type DecisionMetrics struct {
	// Candidate analysis
	TotalCandidates     int32             `json:"totalCandidates"`
	FeasibleCandidates  int32             `json:"feasibleCandidates"`
	SelectedCandidates  int32             `json:"selectedCandidates"`
	RejectedCandidates  int32             `json:"rejectedCandidates"`
	
	// Scoring metrics
	AverageScore        float64           `json:"averageScore"`
	HighestScore        float64           `json:"highestScore"`
	LowestScore         float64           `json:"lowestScore"`
	ScoreStandardDev    float64           `json:"scoreStandardDev"`
	
	// Policy evaluation
	PoliciesEvaluated   int32             `json:"policiesEvaluated"`
	PolicyViolations    int32             `json:"policyViolations"`
	
	// Performance metrics
	FilteringTime       time.Duration     `json:"filteringTime"`
	ScoringTime         time.Duration     `json:"scoringTime"`
	SelectionTime       time.Duration     `json:"selectionTime"`
	ValidationTime      time.Duration     `json:"validationTime"`
	TotalTime           time.Duration     `json:"totalTime"`
	
	// Resource metrics
	MemoryUsage         int64             `json:"memoryUsage,omitempty"`
	CPUTime             time.Duration     `json:"cpuTime,omitempty"`
	
	// Error tracking
	Errors              []string          `json:"errors,omitempty"`
	Warnings            []string          `json:"warnings,omitempty"`
}

// Scorer defines the interface for scoring placement targets
type Scorer interface {
	// Score calculates a score for the given target
	Score(ctx context.Context, target *workloadv1alpha1.SyncTarget, placement *placementv1alpha1.WorkloadPlacement) (float64, error)
	
	// Priority returns the priority/weight of this scorer
	Priority() int
	
	// Name returns the name of this scorer for identification
	Name() string
}

// Filter defines the interface for filtering placement candidates
type Filter interface {
	// Filter determines if a target is feasible for placement
	Filter(ctx context.Context, target *CandidateTarget, placement *placementv1alpha1.WorkloadPlacement) (bool, string, error)
	
	// Name returns the name of this filter for identification
	Name() string
	
	// Priority returns the priority of this filter (lower runs first)
	Priority() int
}

// Strategy defines the interface for placement selection strategies
type Strategy interface {
	// Select chooses the final targets from scored candidates
	Select(ctx context.Context, candidates []*ScoredTarget, constraints *placementv1alpha1.SchedulingConstraint) ([]*SelectedTarget, error)
	
	// Name returns the name of this strategy
	Name() string
	
	// Description returns a description of what this strategy does
	Description() string
}

// ScoredTarget represents a target that has been scored
type ScoredTarget struct {
	Target    *CandidateTarget  `json:"target"`
	Score     float64           `json:"score"`
	Breakdown ScoreBreakdown    `json:"breakdown"`
	Rank      int32             `json:"rank"`
}

// ScoreBreakdown provides detailed scoring information
type ScoreBreakdown struct {
	ScorerScores map[string]float64 `json:"scorerScores"`
	TotalScore   float64            `json:"totalScore"`
	MaxScore     float64            `json:"maxScore"`
	Normalized   float64            `json:"normalized"`
	Factors      []ScoreFactor      `json:"factors,omitempty"`
}

// ScoreFactor represents an individual scoring factor
type ScoreFactor struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description,omitempty"`
}

// PolicyEvaluator defines the interface for evaluating placement policies
type PolicyEvaluator interface {
	// EvaluatePolicy evaluates a policy against a target
	EvaluatePolicy(ctx context.Context, policy *placementv1alpha1.PlacementPolicy, target *CandidateTarget, placement *placementv1alpha1.WorkloadPlacement) (bool, []PolicyViolation, error)
	
	// ListPolicies returns applicable policies for a placement
	ListPolicies(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement) ([]*placementv1alpha1.PlacementPolicy, error)
	
	// ValidateConstraints validates scheduling constraints
	ValidateConstraints(ctx context.Context, constraints *placementv1alpha1.SchedulingConstraint, targets []*CandidateTarget) error
}

// HealthChecker defines the interface for checking target health
type HealthChecker interface {
	// CheckHealth checks the health of a sync target
	CheckHealth(ctx context.Context, target *workloadv1alpha1.SyncTarget) (*HealthStatus, error)
	
	// IsHealthy returns a simple boolean health status
	IsHealthy(ctx context.Context, target *workloadv1alpha1.SyncTarget) (bool, error)
	
	// GetHealthHistory returns recent health history for a target
	GetHealthHistory(ctx context.Context, target *workloadv1alpha1.SyncTarget, duration time.Duration) ([]*HealthRecord, error)
}

// HealthStatus represents the health status of a sync target
type HealthStatus struct {
	Status      HealthStatusType    `json:"status"`
	Message     string              `json:"message,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
	Checks      []HealthCheck       `json:"checks"`
	Score       float64             `json:"score"`
	
	// Detailed metrics
	Latency     time.Duration       `json:"latency,omitempty"`
	Availability float64            `json:"availability,omitempty"`
	ErrorRate   float64             `json:"errorRate,omitempty"`
	
	// Resource health
	ResourceHealth map[string]float64 `json:"resourceHealth,omitempty"`
}

// HealthStatusType represents the type of health status
type HealthStatusType string

const (
	HealthStatusHealthy     HealthStatusType = "Healthy"
	HealthStatusDegraded    HealthStatusType = "Degraded"
	HealthStatusUnhealthy   HealthStatusType = "Unhealthy"
	HealthStatusUnknown     HealthStatusType = "Unknown"
	HealthStatusMaintenance HealthStatusType = "Maintenance"
)

// HealthCheck represents an individual health check
type HealthCheck struct {
	Name        string              `json:"name"`
	Status      HealthStatusType    `json:"status"`
	Message     string              `json:"message,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
	Duration    time.Duration       `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HealthRecord represents a historical health record
type HealthRecord struct {
	Timestamp   time.Time       `json:"timestamp"`
	Status      HealthStatusType `json:"status"`
	Score       float64         `json:"score"`
	Latency     time.Duration   `json:"latency,omitempty"`
	ErrorRate   float64         `json:"errorRate,omitempty"`
}

// ResourceMonitor defines the interface for monitoring resource usage
type ResourceMonitor interface {
	// GetResourceUsage returns current resource usage for a target
	GetResourceUsage(ctx context.Context, target *workloadv1alpha1.SyncTarget) (*ResourceUsage, error)
	
	// GetResourceHistory returns historical resource usage
	GetResourceHistory(ctx context.Context, target *workloadv1alpha1.SyncTarget, duration time.Duration) ([]*ResourceUsageRecord, error)
	
	// PredictResourceUsage predicts future resource usage
	PredictResourceUsage(ctx context.Context, target *workloadv1alpha1.SyncTarget, duration time.Duration) (*ResourceUsage, error)
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	Timestamp   time.Time           `json:"timestamp"`
	CPU         ResourceMetric      `json:"cpu"`
	Memory      ResourceMetric      `json:"memory"`
	Storage     ResourceMetric      `json:"storage,omitempty"`
	Network     *NetworkMetric      `json:"network,omitempty"`
	Custom      map[string]ResourceMetric `json:"custom,omitempty"`
	
	// Pod-level metrics
	PodCount    int32               `json:"podCount"`
	PodCapacity int32               `json:"podCapacity"`
	
	// Node-level metrics
	NodeCount   int32               `json:"nodeCount"`
	NodeMetrics map[string]*NodeMetric `json:"nodeMetrics,omitempty"`
}

// ResourceMetric represents a resource metric
type ResourceMetric struct {
	Used        int64   `json:"used"`
	Available   int64   `json:"available"`
	Total       int64   `json:"total"`
	Utilization float64 `json:"utilization"`
	Unit        string  `json:"unit"`
}

// NetworkMetric represents network usage metrics
type NetworkMetric struct {
	BytesIn     int64   `json:"bytesIn"`
	BytesOut    int64   `json:"bytesOut"`
	PacketsIn   int64   `json:"packetsIn"`
	PacketsOut  int64   `json:"packetsOut"`
	Bandwidth   int64   `json:"bandwidth"`
	Latency     time.Duration `json:"latency,omitempty"`
}

// NodeMetric represents per-node metrics
type NodeMetric struct {
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	CPU         ResourceMetric  `json:"cpu"`
	Memory      ResourceMetric  `json:"memory"`
	Storage     ResourceMetric  `json:"storage,omitempty"`
	PodCount    int32           `json:"podCount"`
	PodCapacity int32           `json:"podCapacity"`
}

// ResourceUsageRecord represents historical resource usage
type ResourceUsageRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	Usage     ResourceUsage `json:"usage"`
}

// AffinityEvaluator defines the interface for evaluating affinity and anti-affinity rules
type AffinityEvaluator interface {
	// EvaluateAffinity evaluates node affinity rules
	EvaluateAffinity(ctx context.Context, target *CandidateTarget, placement *placementv1alpha1.WorkloadPlacement) (float64, error)
	
	// EvaluateAntiAffinity evaluates anti-affinity rules
	EvaluateAntiAffinity(ctx context.Context, target *CandidateTarget, placement *placementv1alpha1.WorkloadPlacement) (float64, error)
	
	// CheckAffinityViolations checks for affinity rule violations
	CheckAffinityViolations(ctx context.Context, targets []*SelectedTarget, placement *placementv1alpha1.WorkloadPlacement) ([]PolicyViolation, error)
}

// LoadBalancer defines the interface for load balancing decisions
type LoadBalancer interface {
	// BalanceLoad distributes workload across selected targets
	BalanceLoad(ctx context.Context, targets []*SelectedTarget, placement *placementv1alpha1.WorkloadPlacement) ([]*SelectedTarget, error)
	
	// GetLoadDistribution returns the current load distribution
	GetLoadDistribution(ctx context.Context, workspace string) (map[string]float64, error)
	
	// OptimizeDistribution suggests optimizations for current distribution
	OptimizeDistribution(ctx context.Context, workspace string) ([]*LoadBalancingRecommendation, error)
}

// LoadBalancingRecommendation represents a load balancing recommendation
type LoadBalancingRecommendation struct {
	Type        RecommendationType  `json:"type"`
	Source      string              `json:"source"`
	Target      string              `json:"target"`
	Workload    string              `json:"workload,omitempty"`
	Reason      string              `json:"reason"`
	Impact      float64             `json:"impact"`
	Priority    RecommendationPriority `json:"priority"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RecommendationType represents the type of recommendation
type RecommendationType string

const (
	RecommendationTypeMigrate       RecommendationType = "Migrate"
	RecommendationTypeScale         RecommendationType = "Scale"
	RecommendationTypeRebalance     RecommendationType = "Rebalance"
	RecommendationTypeConsolidate   RecommendationType = "Consolidate"
	RecommendationTypeEvict         RecommendationType = "Evict"
)

// RecommendationPriority represents the priority of a recommendation
type RecommendationPriority string

const (
	RecommendationPriorityLow       RecommendationPriority = "Low"
	RecommendationPriorityMedium    RecommendationPriority = "Medium"
	RecommendationPriorityHigh      RecommendationPriority = "High"
	RecommendationPriorityCritical  RecommendationPriority = "Critical"
)

// CapacityPlanner defines the interface for capacity planning
type CapacityPlanner interface {
	// PlanCapacity plans capacity requirements for a placement
	PlanCapacity(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, targets []*CandidateTarget) (*CapacityPlan, error)
	
	// ValidateCapacity validates that targets have sufficient capacity
	ValidateCapacity(ctx context.Context, targets []*SelectedTarget, placement *placementv1alpha1.WorkloadPlacement) ([]CapacityViolation, error)
	
	// PredictCapacityNeeds predicts future capacity needs
	PredictCapacityNeeds(ctx context.Context, workspace string, horizon time.Duration) (*CapacityForecast, error)
}

// CapacityPlan represents a capacity planning result
type CapacityPlan struct {
	Placement       *placementv1alpha1.WorkloadPlacement `json:"placement"`
	TotalRequirements corev1.ResourceList           `json:"totalRequirements"`
	PerTargetAllocation map[string]corev1.ResourceList `json:"perTargetAllocation"`
	UtilizationImpact map[string]float64             `json:"utilizationImpact"`
	Recommendations []CapacityRecommendation         `json:"recommendations,omitempty"`
	Timestamp       time.Time                        `json:"timestamp"`
}

// CapacityViolation represents a capacity constraint violation
type CapacityViolation struct {
	Target      string              `json:"target"`
	Resource    string              `json:"resource"`
	Required    int64               `json:"required"`
	Available   int64               `json:"available"`
	Deficit     int64               `json:"deficit"`
	Severity    ViolationSeverity   `json:"severity"`
	Message     string              `json:"message"`
}

// CapacityRecommendation represents a capacity recommendation
type CapacityRecommendation struct {
	Type        RecommendationType  `json:"type"`
	Target      string              `json:"target"`
	Resource    string              `json:"resource,omitempty"`
	Change      int64               `json:"change,omitempty"`
	Reason      string              `json:"reason"`
	Priority    RecommendationPriority `json:"priority"`
	Impact      string              `json:"impact,omitempty"`
}

// CapacityForecast represents a capacity forecast
type CapacityForecast struct {
	Workspace       string                         `json:"workspace"`
	Horizon         time.Duration                  `json:"horizon"`
	CurrentCapacity corev1.ResourceList            `json:"currentCapacity"`
	PredictedUsage  corev1.ResourceList            `json:"predictedUsage"`
	RequiredCapacity corev1.ResourceList           `json:"requiredCapacity"`
	CapacityGap     corev1.ResourceList            `json:"capacityGap,omitempty"`
	Confidence      float64                        `json:"confidence"`
	Recommendations []CapacityRecommendation       `json:"recommendations,omitempty"`
	Timestamp       time.Time                      `json:"timestamp"`
}

// User represents a user making placement requests
type User struct {
	Name      string   `json:"name"`
	Groups    []string `json:"groups,omitempty"`
	UID       types.UID `json:"uid,omitempty"`
	Extra     map[string][]string `json:"extra,omitempty"`
}

// Constants for common scorer names
const (
	ScorerNameResource    = "resource"
	ScorerNameAffinity    = "affinity"
	ScorerNameLoad        = "load"
	ScorerNameLatency     = "latency"
	ScorerNameHealth      = "health"
	ScorerNameCost        = "cost"
	ScorerNameCompliance  = "compliance"
)

// Constants for common filter names
const (
	FilterNameResource     = "resource"
	FilterNameHealth       = "health"
	FilterNameCompliance   = "compliance"
	FilterNameMaintenance  = "maintenance"
	FilterNameCapacity     = "capacity"
	FilterNameAffinity     = "affinity"
	FilterNameTaint        = "taint"
)

// Constants for strategy names
const (
	StrategyNameBestFit     = "bestfit"
	StrategyNameSpread      = "spread"
	StrategyNameBinPack     = "binpack"
	StrategyNameLeastLoaded = "leastloaded"
	StrategyNameRoundRobin  = "roundrobin"
	StrategyNameWeighted    = "weighted"
	StrategyNameCustom      = "custom"
)