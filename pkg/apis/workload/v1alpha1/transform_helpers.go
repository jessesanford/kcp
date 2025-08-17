package v1alpha1

import (
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// TransformConditionActive indicates transform is active
	TransformConditionActive = "Active"

	// TransformConditionApplied indicates successful application
	TransformConditionApplied = "Applied"
)

// IsActive returns true if transform is active
func (t *WorkloadTransform) IsActive() bool {
	return !t.Spec.Paused && conditions.IsTrue(t, TransformConditionActive)
}

// IsApplied returns true if transform is successfully applied
func (t *WorkloadTransform) IsApplied() bool {
	return conditions.IsTrue(t, TransformConditionApplied)
}

// SetCondition sets a condition on the WorkloadTransform
func (t *WorkloadTransform) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(t, condition)
}

// GetApplicationForLocation returns application status for a location
func (t *WorkloadTransform) GetApplicationForLocation(locationName string) *TransformApplication {
	for i := range t.Status.TransformApplications {
		if t.Status.TransformApplications[i].LocationName == locationName {
			return &t.Status.TransformApplications[i]
		}
	}
	return nil
}

// ApplyTransform applies the transform to an object
func (t *WorkloadTransform) ApplyTransform(obj *unstructured.Unstructured) error {
	for _, transform := range t.Spec.Transforms {
		if err := applyTransformOperation(obj, transform); err != nil {
			return err
		}
	}
	return nil
}

func applyTransformOperation(obj *unstructured.Unstructured, transform Transform) error {
	switch transform.Type {
	case TransformTypeAnnotate:
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		for k, v := range transform.Annotations {
			annotations[k] = v
		}
		obj.SetAnnotations(annotations)

	case TransformTypeLabel:
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range transform.Labels {
			labels[k] = v
		}
		obj.SetLabels(labels)

	// Other transform types would be implemented here
	}

	return nil
}

// MatchesTarget checks if an object matches the transform target
func (t *WorkloadTransform) MatchesTarget(obj *unstructured.Unstructured) bool {
	if obj.GetAPIVersion() != t.Spec.TargetRef.APIVersion {
		return false
	}
	if obj.GetKind() != t.Spec.TargetRef.Kind {
		return false
	}
	if t.Spec.TargetRef.Name != "" && obj.GetName() != t.Spec.TargetRef.Name {
		return false
	}
	// LabelSelector matching would be implemented here
	return true
}