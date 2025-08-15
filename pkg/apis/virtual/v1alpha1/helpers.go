package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsReady returns true if the APIResource is ready
func (ar *APIResource) IsReady() bool {
	return ar.Status.Phase == APIResourcePhaseReady
}

// GetCondition returns the condition with the given type for APIResource
func (ar *APIResource) GetCondition(conditionType string) *metav1.Condition {
	for i := range ar.Status.Conditions {
		if ar.Status.Conditions[i].Type == conditionType {
			return &ar.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the APIResource status
func (ar *APIResource) SetCondition(condition metav1.Condition) {
	for i, existing := range ar.Status.Conditions {
		if existing.Type == condition.Type {
			ar.Status.Conditions[i] = condition
			return
		}
	}
	ar.Status.Conditions = append(ar.Status.Conditions, condition)
}

// IsReady returns true if the VirtualWorkspace is ready
func (vw *VirtualWorkspace) IsReady() bool {
	return vw.Status.Phase == VirtualWorkspacePhaseReady
}

// GetCondition returns the condition with the given type for VirtualWorkspace
func (vw *VirtualWorkspace) GetCondition(conditionType string) *metav1.Condition {
	for i := range vw.Status.Conditions {
		if vw.Status.Conditions[i].Type == conditionType {
			return &vw.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the VirtualWorkspace status
func (vw *VirtualWorkspace) SetCondition(condition metav1.Condition) {
	for i, existing := range vw.Status.Conditions {
		if existing.Type == condition.Type {
			vw.Status.Conditions[i] = condition
			return
		}
	}
	vw.Status.Conditions = append(vw.Status.Conditions, condition)
}