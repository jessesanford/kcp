package decision

import (
	"context"
	"fmt"
	"sync"
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// DecisionRecorder records placement decisions and maintains history
type DecisionRecorder struct {
	// Client for interacting with KCP
	kcpClient kcpclientset.ClusterInterface

	// Event recorder for generating events
	eventRecorder record.EventRecorder

	// Decision history cache
	decisionHistory map[string][]*DecisionRecord
	historyMutex    sync.RWMutex

	// Metrics collector
	metricsCollector MetricsCollector

	// Configuration
	maxHistorySize int
	retentionTime  time.Duration

	// Decision validation
	validator DecisionValidator

	// Decision storage backend
	storage DecisionStorage
}

// NewDecisionRecorder creates a new decision recorder
func NewDecisionRecorder(
	kcpClient kcpclientset.ClusterInterface,
	eventRecorder record.EventRecorder,
	storage DecisionStorage,
	metricsCollector MetricsCollector,
) *DecisionRecorder {
	return &DecisionRecorder{
		kcpClient:        kcpClient,
		eventRecorder:    eventRecorder,
		storage:          storage,
		metricsCollector: metricsCollector,
		decisionHistory:  make(map[string][]*DecisionRecord),
		maxHistorySize:   100, // configurable
		retentionTime:    24 * time.Hour * 30, // 30 days
		validator:        NewDecisionValidator(),
	}
}

// RecordDecision records a placement decision with full context
func (r *DecisionRecorder) RecordDecision(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
	candidates []*CandidateTarget,
	duration time.Duration,
) error {
	logger := klog.FromContext(ctx)
	startTime := time.Now()

	// Create decision record
	record := &DecisionRecord{
		ID:               r.generateDecisionID(placement),
		Timestamp:        startTime,
		Placement:        placement.DeepCopy(),
		Decision:         decision.DeepCopy(),
		Candidates:       candidates,
		Scores:           r.extractScores(candidates),
		Duration:         duration,
		SchedulerVersion: r.getSchedulerVersion(),
		UserAgent:        r.extractUserAgent(ctx),
		RequestID:        r.extractRequestID(ctx),
		Status:           DecisionStatusScheduled,
	}

	// Generate decision reasons
	record.Reasons = r.generateDecisionReasons(ctx, placement, decision, candidates)

	// Validate decision
	if r.validator != nil {
		if err := r.validator.ValidateDecision(ctx, placement, decision); err != nil {
			record.Status = DecisionStatusFailed
			record.Error = err.Error()
			logger.Error(err, "Decision validation failed")
		}
	}

	// Check for policy violations
	violations := r.checkPolicyViolations(ctx, placement, decision, candidates)
	if len(violations) > 0 {
		record.Violations = violations
		for _, violation := range violations {
			if violation.Severity == ViolationSeverityError {
				record.Status = DecisionStatusRejected
				break
			}
		}
	}

	// Store decision persistently
	if r.storage != nil {
		if err := r.storage.StoreDecision(ctx, record); err != nil {
			logger.Error(err, "Failed to store decision persistently")
		}
	}

	// Update in-memory history
	r.updateDecisionHistory(placement.Name, record)

	// Record metrics
	if r.metricsCollector != nil {
		r.metricsCollector.RecordDecision(ctx, record)
		r.metricsCollector.RecordDecisionLatency(duration)

		for _, violation := range violations {
			r.metricsCollector.RecordPolicyViolation(violation)
		}
	}

	// Generate Kubernetes events
	r.generateEvents(ctx, placement, record)

	logger.Info("Recorded placement decision",
		"placement", placement.Name,
		"decision", record.ID,
		"status", record.Status,
		"candidates", len(candidates),
		"violations", len(violations),
		"duration", duration,
	)

	return nil
}

// RecordFailedDecision records a failed placement decision
func (r *DecisionRecorder) RecordFailedDecision(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	err error,
	candidates []*CandidateTarget,
	duration time.Duration,
) error {
	logger := klog.FromContext(ctx)

	record := &DecisionRecord{
		ID:               r.generateDecisionID(placement),
		Timestamp:        time.Now(),
		Placement:        placement.DeepCopy(),
		Candidates:       candidates,
		Status:           DecisionStatusFailed,
		Error:            err.Error(),
		Duration:         duration,
		SchedulerVersion: r.getSchedulerVersion(),
		UserAgent:        r.extractUserAgent(ctx),
		RequestID:        r.extractRequestID(ctx),
	}

	// Store failure
	if r.storage != nil {
		if storeErr := r.storage.StoreDecision(ctx, record); storeErr != nil {
			logger.Error(storeErr, "Failed to store failed decision")
		}
	}

	// Update history
	r.updateDecisionHistory(placement.Name, record)

	// Record error metrics
	if r.metricsCollector != nil {
		r.metricsCollector.RecordDecisionError(r.classifyError(err))
		r.metricsCollector.RecordDecisionLatency(duration)
	}

	// Generate error event
	r.eventRecorder.Eventf(placement, corev1.EventTypeWarning, "SchedulingFailed",
		"Failed to schedule workload placement: %v", err)

	logger.Error(err, "Recorded failed placement decision",
		"placement", placement.Name,
		"candidates", len(candidates),
		"duration", duration,
	)

	return nil
}

// GetDecisionHistory returns the decision history for a placement
func (r *DecisionRecorder) GetDecisionHistory(placementName string) []*DecisionRecord {
	r.historyMutex.RLock()
	defer r.historyMutex.RUnlock()

	history, exists := r.decisionHistory[placementName]
	if !exists {
		return []*DecisionRecord{}
	}

	// Return a copy to prevent modification
	result := make([]*DecisionRecord, len(history))
	copy(result, history)
	return result
}

// GetLatestDecision returns the most recent decision for a placement
func (r *DecisionRecorder) GetLatestDecision(placementName string) *DecisionRecord {
	r.historyMutex.RLock()
	defer r.historyMutex.RUnlock()

	history, exists := r.decisionHistory[placementName]
	if !exists || len(history) == 0 {
		return nil
	}

	return history[len(history)-1]
}

// GetDecisionByID retrieves a specific decision by ID
func (r *DecisionRecorder) GetDecisionByID(ctx context.Context, id string) (*DecisionRecord, error) {
	if r.storage != nil {
		return r.storage.GetDecision(ctx, id)
	}

	// Fallback to in-memory search
	r.historyMutex.RLock()
	defer r.historyMutex.RUnlock()

	for _, history := range r.decisionHistory {
		for _, record := range history {
			if record.ID == id {
				return record, nil
			}
		}
	}

	return nil, fmt.Errorf("decision not found: %s", id)
}

// PurgeOldDecisions removes decisions older than the retention time
func (r *DecisionRecorder) PurgeOldDecisions(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	cutoffTime := time.Now().Add(-r.retentionTime)

	// Purge from persistent storage
	if r.storage != nil {
		if err := r.storage.PurgeOldDecisions(ctx, cutoffTime); err != nil {
			logger.Error(err, "Failed to purge old decisions from storage")
		}
	}

	// Purge from in-memory cache
	r.historyMutex.Lock()
	defer r.historyMutex.Unlock()

	for placementName, history := range r.decisionHistory {
		var filtered []*DecisionRecord
		for _, record := range history {
			if record.Timestamp.After(cutoffTime) {
				filtered = append(filtered, record)
			}
		}
		r.decisionHistory[placementName] = filtered
	}

	logger.Info("Purged old decisions", "cutoffTime", cutoffTime)
	return nil
}

// updateDecisionHistory updates the in-memory decision history
func (r *DecisionRecorder) updateDecisionHistory(placementName string, record *DecisionRecord) {
	r.historyMutex.Lock()
	defer r.historyMutex.Unlock()

	history, exists := r.decisionHistory[placementName]
	if !exists {
		history = make([]*DecisionRecord, 0, r.maxHistorySize)
	}

	// Add new record
	history = append(history, record)

	// Trim to max size
	if len(history) > r.maxHistorySize {
		history = history[len(history)-r.maxHistorySize:]
	}

	r.decisionHistory[placementName] = history
}

// generateDecisionID creates a unique ID for a decision
func (r *DecisionRecorder) generateDecisionID(placement *placementv1alpha1.WorkloadPlacement) string {
	return fmt.Sprintf("%s-%s-%d",
		placement.Name,
		placement.Namespace,
		time.Now().UnixNano(),
	)
}

// extractScores extracts scores from candidate targets
func (r *DecisionRecorder) extractScores(candidates []*CandidateTarget) map[string]float64 {
	scores := make(map[string]float64)
	for _, candidate := range candidates {
		scores[candidate.SyncTarget.Name] = candidate.Score
	}
	return scores
}

// generateDecisionReasons generates human-readable reasons for the decision
func (r *DecisionRecorder) generateDecisionReasons(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
	candidates []*CandidateTarget,
) []DecisionReason {
	var reasons []DecisionReason

	// Add strategy reason
	reasons = append(reasons, DecisionReason{
		Type:    "Strategy",
		Message: fmt.Sprintf("Applied %s strategy", placement.Spec.Strategy),
	})

	// Add scoring reasons
	for _, cluster := range decision.Spec.Clusters {
		for _, candidate := range candidates {
			if candidate.SyncTarget.Name == cluster.ClusterName {
				reasons = append(reasons, DecisionReason{
					Type:       "Scoring",
					Message:    fmt.Sprintf("Selected cluster %s with score %.2f", cluster.ClusterName, candidate.Score),
					SyncTarget: cluster.ClusterName,
					Score:      candidate.Score,
				})
				break
			}
		}
	}

	// Add constraint reasons
	if placement.Spec.Constraints != nil {
		reasons = append(reasons, DecisionReason{
			Type:    "Constraints",
			Message: "Applied scheduling constraints",
			Metadata: map[string]interface{}{
				"maxClusters": placement.Spec.Constraints.MaxClusters,
				"minClusters": placement.Spec.Constraints.MinClusters,
			},
		})
	}

	return reasons
}

// checkPolicyViolations checks for any policy violations in the decision
func (r *DecisionRecorder) checkPolicyViolations(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
	candidates []*CandidateTarget,
) []PolicyViolation {
	var violations []PolicyViolation

	// Check for candidate violations
	for _, candidate := range candidates {
		violations = append(violations, candidate.Violations...)
	}

	// Check resource constraints
	if placement.Spec.ResourceRequirements != nil {
		for _, cluster := range decision.Spec.Clusters {
			if violation := r.checkResourceConstraints(cluster, placement.Spec.ResourceRequirements); violation != nil {
				violations = append(violations, *violation)
			}
		}
	}

	return violations
}

// checkResourceConstraints validates resource requirements against selected clusters
func (r *DecisionRecorder) checkResourceConstraints(
	cluster placementv1alpha1.ClusterDecision,
	requirements *corev1.ResourceRequirements,
) *PolicyViolation {
	// Implementation would check if cluster has enough resources
	// For now, return nil (no violation)
	return nil
}

// generateEvents generates Kubernetes events for the decision
func (r *DecisionRecorder) generateEvents(ctx context.Context, placement *placementv1alpha1.WorkloadPlacement, record *DecisionRecord) {
	switch record.Status {
	case DecisionStatusScheduled:
		r.eventRecorder.Eventf(placement, corev1.EventTypeNormal, "Scheduled",
			"Successfully scheduled to %d clusters in %v", len(record.Decision.Spec.Clusters), record.Duration)
	case DecisionStatusFailed:
		r.eventRecorder.Eventf(placement, corev1.EventTypeWarning, "SchedulingFailed",
			"Failed to schedule: %s", record.Error)
	case DecisionStatusRejected:
		r.eventRecorder.Eventf(placement, corev1.EventTypeWarning, "PolicyViolation",
			"Scheduling rejected due to %d policy violations", len(record.Violations))
	}
}

// classifyError classifies an error for metrics purposes
func (r *DecisionRecorder) classifyError(err error) string {
	// Simple classification logic
	errStr := err.Error()
	switch {
	case contains(errStr, "no candidates"):
		return "no_candidates"
	case contains(errStr, "no feasible"):
		return "no_feasible"
	case contains(errStr, "policy"):
		return "policy_violation"
	case contains(errStr, "resource"):
		return "insufficient_resources"
	default:
		return "unknown"
	}
}

// Helper functions

func (r *DecisionRecorder) getSchedulerVersion() string {
	return "v1.0.0" // Would be injected at build time
}

func (r *DecisionRecorder) extractUserAgent(ctx context.Context) string {
	if ua := ctx.Value(logging.UserAgentKey); ua != nil {
		return ua.(string)
	}
	return "unknown"
}

func (r *DecisionRecorder) extractRequestID(ctx context.Context) string {
	if rid := ctx.Value(logging.RequestIDKey); rid != nil {
		return rid.(string)
	}
	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// NewDecisionValidator creates a new decision validator
func NewDecisionValidator() DecisionValidator {
	return &defaultDecisionValidator{}
}

// defaultDecisionValidator provides basic decision validation
type defaultDecisionValidator struct{}

func (v *defaultDecisionValidator) ValidateDecision(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
) error {
	// Validate that decision matches placement requirements
	if decision.Spec.WorkloadPlacement != placement.Name {
		return fmt.Errorf("decision placement mismatch: expected %s, got %s",
			placement.Name, decision.Spec.WorkloadPlacement)
	}

	// Validate cluster count constraints
	if placement.Spec.Constraints != nil {
		clusterCount := len(decision.Spec.Clusters)
		if placement.Spec.Constraints.MinClusters != nil &&
			clusterCount < int(*placement.Spec.Constraints.MinClusters) {
			return fmt.Errorf("insufficient clusters: need at least %d, got %d",
				*placement.Spec.Constraints.MinClusters, clusterCount)
		}
		if placement.Spec.Constraints.MaxClusters != nil &&
			clusterCount > int(*placement.Spec.Constraints.MaxClusters) {
			return fmt.Errorf("too many clusters: need at most %d, got %d",
				*placement.Spec.Constraints.MaxClusters, clusterCount)
		}
	}

	return nil
}

func (v *defaultDecisionValidator) ValidateConstraints(
	ctx context.Context,
	constraints *placementv1alpha1.SchedulingConstraint,
) error {
	if constraints == nil {
		return nil
	}

	// Validate min/max cluster constraints
	if constraints.MinClusters != nil && constraints.MaxClusters != nil {
		if *constraints.MinClusters > *constraints.MaxClusters {
			return fmt.Errorf("minClusters (%d) cannot be greater than maxClusters (%d)",
				*constraints.MinClusters, *constraints.MaxClusters)
		}
	}

	return nil
}