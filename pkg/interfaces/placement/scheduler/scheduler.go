package scheduler

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kcp-dev/kcp/pkg/interfaces/placement"
)

// Scheduler schedules workloads to locations
type Scheduler interface {
	// Schedule computes placement for workload
	Schedule(
		ctx context.Context,
		workload *unstructured.Unstructured,
		policy *placement.PlacementPolicy,
	) (*placement.PlacementDecision, error)

	// Reschedule updates existing placement
	Reschedule(
		ctx context.Context,
		workload *unstructured.Unstructured,
		currentPlacement *placement.PlacementDecision,
	) (*placement.PlacementDecision, error)

	// Preempt makes room for higher priority workload
	Preempt(
		ctx context.Context,
		workload *unstructured.Unstructured,
		targets []*placement.SyncTarget,
	) (*PreemptionResult, error)
}

// PreemptionResult contains preemption outcome
type PreemptionResult struct {
	// Preempted workloads
	PreemptedWorkloads []WorkloadRef

	// FreedResources
	FreedResources placement.ResourceList

	// NewPlacement for the workload
	NewPlacement *placement.PlacementDecision
}

// WorkloadRef references a workload
type WorkloadRef struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// SchedulingQueue manages workloads waiting for placement
type SchedulingQueue interface {
	// Add adds workload to queue
	Add(workload *unstructured.Unstructured) error

	// Pop gets next workload to schedule
	Pop() (*unstructured.Unstructured, error)

	// Update updates workload in queue
	Update(workload *unstructured.Unstructured) error

	// Delete removes workload from queue
	Delete(workload *unstructured.Unstructured) error

	// Len returns queue length
	Len() int
}

// SchedulerFramework provides scheduling framework
type SchedulerFramework interface {
	// RunFilterPlugins runs filter plugins
	RunFilterPlugins(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *placement.SyncTarget,
	) error

	// RunScorePlugins runs scoring plugins
	RunScorePlugins(
		ctx context.Context,
		workload *unstructured.Unstructured,
		targets []*placement.SyncTarget,
	) ([]float64, error)

	// RunBindPlugin binds workload to target
	RunBindPlugin(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *placement.SyncTarget,
	) error
}

// SchedulerPlugin is a scheduling plugin
type SchedulerPlugin interface {
	// Name returns plugin name
	Name() string

	// Filter filters unsuitable targets
	Filter(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *placement.SyncTarget,
	) error

	// Score scores a target
	Score(
		ctx context.Context,
		workload *unstructured.Unstructured,
		target *placement.SyncTarget,
	) (float64, error)
}

// PriorityQueue manages workloads by priority
type PriorityQueue interface {
	SchedulingQueue

	// AddWithPriority adds workload with priority
	AddWithPriority(workload *unstructured.Unstructured, priority int32) error

	// UpdatePriority updates workload priority
	UpdatePriority(workload *unstructured.Unstructured, priority int32) error

	// PopHighest gets highest priority workload
	PopHighest() (*unstructured.Unstructured, error)
}

// SchedulerConfig configures the scheduler behavior
type SchedulerConfig struct {
	// MaxConcurrentScheduling limits concurrent scheduling operations
	MaxConcurrentScheduling int

	// SchedulingTimeout for scheduling operations
	SchedulingTimeout int64

	// RetryBackoff configuration
	RetryBackoff BackoffConfig

	// Plugins to use in the scheduler
	Plugins []PluginConfig
}

// BackoffConfig defines retry backoff behavior
type BackoffConfig struct {
	// InitialInterval for first retry
	InitialInterval int64

	// MaxInterval for retries
	MaxInterval int64

	// Multiplier for backoff calculation
	Multiplier float64

	// MaxRetries before giving up
	MaxRetries int
}

// PluginConfig configures a scheduler plugin
type PluginConfig struct {
	// Name of the plugin
	Name string

	// Weight of the plugin for scoring
	Weight int32

	// Args for plugin configuration
	Args map[string]interface{}
}

// SchedulerExtension allows extending scheduler functionality
type SchedulerExtension interface {
	// PreSchedule runs before scheduling starts
	PreSchedule(
		ctx context.Context,
		workload *unstructured.Unstructured,
	) error

	// PostSchedule runs after scheduling completes
	PostSchedule(
		ctx context.Context,
		workload *unstructured.Unstructured,
		decision *placement.PlacementDecision,
	) error

	// OnSchedulingFailure handles scheduling failures
	OnSchedulingFailure(
		ctx context.Context,
		workload *unstructured.Unstructured,
		err error,
	) error
}

// SchedulingContext provides context for scheduling operations
type SchedulingContext struct {
	// Workload being scheduled
	Workload *unstructured.Unstructured

	// Policy governing the scheduling
	Policy *placement.PlacementPolicy

	// AvailableTargets for placement
	AvailableTargets []*placement.SyncTarget

	// FilteredTargets after filtering
	FilteredTargets []*placement.SyncTarget

	// ScoredTargets with their scores
	ScoredTargets []ScoredTarget

	// SelectedTarget for placement
	SelectedTarget *placement.SyncTarget

	// SchedulingPhase current phase
	SchedulingPhase SchedulingPhase
}

// ScoredTarget represents a target with its score
type ScoredTarget struct {
	// Target being scored
	Target *placement.SyncTarget

	// Score assigned to the target
	Score float64

	// Breakdown of score components
	ScoreBreakdown map[string]float64
}

// SchedulingPhase represents the current scheduling phase
type SchedulingPhase string

const (
	SchedulingPhaseQueued    SchedulingPhase = "Queued"
	SchedulingPhaseFiltering SchedulingPhase = "Filtering"
	SchedulingPhaseScoring   SchedulingPhase = "Scoring"
	SchedulingPhaseBinding   SchedulingPhase = "Binding"
	SchedulingPhaseComplete  SchedulingPhase = "Complete"
	SchedulingPhaseFailed    SchedulingPhase = "Failed"
)