package conflict

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Conflict represents a synchronization conflict between KCP and downstream resources
type Conflict struct {
	GVR               schema.GroupVersionResource
	Namespace, Name   string
	Type              ConflictType
	Severity          ConflictSeverity
	KCPVersion        string
	DownstreamVersion string
	Fields            []FieldConflict
	DetectedAt        metav1.Time
}

// ConflictType categorizes the type of conflict encountered
type ConflictType string

const (
	VersionConflict   ConflictType = "version"
	SemanticConflict  ConflictType = "semantic"
	DeletedConflict   ConflictType = "deleted"
	OwnershipConflict ConflictType = "ownership"
)

// ConflictSeverity indicates the severity and impact of a conflict
type ConflictSeverity int

const (
	LowSeverity      ConflictSeverity = 1
	MediumSeverity   ConflictSeverity = 2
	HighSeverity     ConflictSeverity = 3
	CriticalSeverity ConflictSeverity = 4
)

// String returns the string representation of the severity
func (s ConflictSeverity) String() string {
	switch s {
	case LowSeverity:
		return "low"
	case MediumSeverity:
		return "medium"
	case HighSeverity:
		return "high"
	case CriticalSeverity:
		return "critical"
	default:
		return "unknown"
	}
}

// ResolutionStrategy defines different approaches to resolve conflicts
type ResolutionStrategy string

const (
	KCPWins        ResolutionStrategy = "kcp-wins"
	DownstreamWins ResolutionStrategy = "downstream-wins"
	Merge          ResolutionStrategy = "merge"
	Manual         ResolutionStrategy = "manual"
)

// ResolutionResult contains the outcome of conflict resolution
type ResolutionResult struct {
	Resolved  bool
	Strategy  ResolutionStrategy
	Merged    *unstructured.Unstructured
	Error     error
	Conflicts []FieldConflict
}

// FieldConflict represents a field-level conflict between versions
type FieldConflict struct {
	Path             string
	KCPValue         interface{}
	DownstreamValue  interface{}
	Resolution       string
}