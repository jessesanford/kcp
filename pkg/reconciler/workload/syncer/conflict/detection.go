package conflict

import (
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConflictDetector detects conflicts between KCP and downstream resources
type ConflictDetector struct {
	ignoreFields []string
}

// NewConflictDetector creates a new conflict detector with default ignored fields
func NewConflictDetector() *ConflictDetector {
	return &ConflictDetector{
		ignoreFields: []string{
			"metadata.resourceVersion",
			"metadata.generation",
			"metadata.managedFields",
			"metadata",
			"status",
		},
	}
}

// DetectConflict compares KCP and downstream resources to identify conflicts
func (d *ConflictDetector) DetectConflict(kcp, downstream *unstructured.Unstructured) *Conflict {
	if kcp == nil || downstream == nil {
		return d.detectDeletionConflict(kcp, downstream)
	}

	gvk := kcp.GetObjectKind().GroupVersionKind()
	conflict := &Conflict{
		GVR: schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		},
		Namespace:         kcp.GetNamespace(),
		Name:              kcp.GetName(),
		KCPVersion:        kcp.GetResourceVersion(),
		DownstreamVersion: downstream.GetResourceVersion(),
		DetectedAt:        metav1.Now(),
	}

	// Check for version conflicts
	if kcp.GetGeneration() != downstream.GetGeneration() {
		conflict.Type = VersionConflict
		conflict.Severity = d.assessVersionConflictSeverity(kcp, downstream)
	}

	// Detect field-level conflicts (but only for meaningful fields, not metadata)
	fieldConflicts := d.detectFieldConflicts(kcp.Object, downstream.Object, "")
	if len(fieldConflicts) > 0 {
		conflict.Fields = fieldConflicts
		// Only set as semantic conflict if no other conflict type was detected
		if conflict.Type == "" {
			conflict.Type = SemanticConflict
			conflict.Severity = d.assessSemanticConflictSeverity(fieldConflicts)
		}
	}

	// Check for ownership conflicts
	if !reflect.DeepEqual(kcp.GetOwnerReferences(), downstream.GetOwnerReferences()) {
		conflict.Type = OwnershipConflict
		conflict.Severity = CriticalSeverity
	}

	// Return nil if no conflicts detected
	if conflict.Type == "" && len(conflict.Fields) == 0 {
		return nil
	}

	return conflict
}

// detectDeletionConflict handles cases where one resource is nil (deleted)
func (d *ConflictDetector) detectDeletionConflict(kcp, downstream *unstructured.Unstructured) *Conflict {
	if kcp == nil && downstream == nil {
		return nil
	}

	existing := kcp
	if kcp == nil {
		existing = downstream
	}

	gvk := existing.GetObjectKind().GroupVersionKind()
	return &Conflict{
		GVR: schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: strings.ToLower(gvk.Kind) + "s",
		},
		Namespace:  existing.GetNamespace(),
		Name:       existing.GetName(),
		Type:       DeletedConflict,
		Severity:   MediumSeverity,
		DetectedAt: metav1.Now(),
	}
}

// detectFieldConflicts recursively compares object fields
func (d *ConflictDetector) detectFieldConflicts(kcpObj, downstreamObj map[string]interface{}, path string) []FieldConflict {
	var conflicts []FieldConflict

	for key, kcpValue := range kcpObj {
		fieldPath := d.buildFieldPath(path, key)
		if d.shouldIgnoreField(fieldPath) {
			continue
		}

		downstreamValue, exists := downstreamObj[key]
		if !exists {
			conflicts = append(conflicts, FieldConflict{Path: fieldPath, KCPValue: kcpValue, Resolution: "missing_in_downstream"})
			continue
		}

		if !reflect.DeepEqual(kcpValue, downstreamValue) {
			conflicts = append(conflicts, FieldConflict{
				Path: fieldPath, KCPValue: kcpValue, DownstreamValue: downstreamValue, Resolution: "value_mismatch"})
		}
	}

	return conflicts
}

// assessVersionConflictSeverity determines severity of version conflicts
func (d *ConflictDetector) assessVersionConflictSeverity(kcp, downstream *unstructured.Unstructured) ConflictSeverity {
	diff := kcp.GetGeneration() - downstream.GetGeneration()
	if diff < 0 {
		diff = -diff
	}
	if diff > 10 {
		return HighSeverity
	} else if diff > 5 {
		return MediumSeverity
	}
	return LowSeverity
}

// assessSemanticConflictSeverity determines severity based on field conflicts
func (d *ConflictDetector) assessSemanticConflictSeverity(conflicts []FieldConflict) ConflictSeverity {
	criticalFields := 0
	for _, conflict := range conflicts {
		if strings.Contains(conflict.Path, "spec.selector") || strings.Contains(conflict.Path, "spec.replicas") {
			criticalFields++
		}
	}
	if criticalFields > 0 {
		return HighSeverity
	} else if len(conflicts) > 3 {
		return MediumSeverity
	}
	return LowSeverity
}

// shouldIgnoreField checks if a field should be ignored during conflict detection
func (d *ConflictDetector) shouldIgnoreField(path string) bool {
	for _, ignored := range d.ignoreFields {
		if strings.HasPrefix(path, ignored) {
			return true
		}
	}
	return false
}

// buildFieldPath constructs a dot-separated field path
func (d *ConflictDetector) buildFieldPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}