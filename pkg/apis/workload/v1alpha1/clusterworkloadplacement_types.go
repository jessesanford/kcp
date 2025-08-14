/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// These are valid conditions of ClusterWorkloadPlacement.
const (
	// PlacementReady indicates that the placement policy is ready and active.
	PlacementReady conditionsv1alpha1.ConditionType = "Ready"

	// PlacementSyncing indicates that workloads are being synced based on this placement.
	PlacementSyncing conditionsv1alpha1.ConditionType = "Syncing"

	// Common condition reasons for ClusterWorkloadPlacement
	InvalidSelectorReason      = "InvalidSelector"
	NoTargetsAvailableReason   = "NoTargetsAvailable"
	PlacementSuccessReason     = "PlacementSuccess"
)

// ClusterWorkloadPlacement defines a policy for placing workloads across
// multiple sync targets based on selector criteria and placement constraints.
// It enables declarative workload distribution with automatic target selection.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Targets",type="integer",JSONPath=".status.selectedTargets"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterWorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired placement policy configuration.
	Spec ClusterWorkloadPlacementSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the placement policy.
	Status ClusterWorkloadPlacementStatus `json:"status,omitempty"`
}

// ClusterWorkloadPlacementSpec defines the desired placement policy for workloads.
type ClusterWorkloadPlacementSpec struct {
	// namespaceSelector selects workloads based on their namespace labels.
	// Only workloads in namespaces matching this selector will be placed.
	//
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// locationSelector defines location-based placement constraints.
	// This allows targeting specific geographical or logical locations.
	//
	// +optional
	LocationSelector *LocationSelector `json:"locationSelector,omitempty"`

	// resourceRequirements specifies resource constraints for target selection.
	// Only targets meeting these requirements will be considered for placement.
	//
	// TODO: Implement comprehensive resource evaluation in follow-up PR
	// +optional
	ResourceRequirements *ResourceRequirements `json:"resourceRequirements,omitempty"`

	// maxReplicas limits the maximum number of targets for workload distribution.
	// If unspecified, workloads can be placed on any number of matching targets.
	//
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// minReplicas ensures minimum redundancy by requiring placement on at least
	// this many targets. Placement will fail if insufficient targets are available.
	//
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
}

// LocationSelector defines location-based placement constraints.
type LocationSelector struct {
	// requiredLocations is a list of location names that targets must match.
	// At least one of these locations must be satisfied for target eligibility.
	//
	// +optional
	// +listType=set
	RequiredLocations []string `json:"requiredLocations,omitempty"`

	// preferredLocations is a list of preferred location names for target ranking.
	// Targets matching preferred locations receive higher placement scores.
	//
	// TODO: Implement location scoring in follow-up PR
	// +optional
	// +listType=atomic
	PreferredLocations []string `json:"preferredLocations,omitempty"`
}

// ResourceRequirements defines resource constraints for target selection.
type ResourceRequirements struct {
	// minCPU specifies the minimum CPU capacity required on target clusters.
	//
	// TODO: Implement resource evaluation in follow-up PR
	// +optional
	MinCPU string `json:"minCpu,omitempty"`

	// minMemory specifies the minimum memory capacity required on target clusters.
	//
	// TODO: Implement resource evaluation in follow-up PR
	// +optional
	MinMemory string `json:"minMemory,omitempty"`
}

// ClusterWorkloadPlacementStatus defines the observed state of a placement policy.
type ClusterWorkloadPlacementStatus struct {
	// conditions is a list of conditions that apply to the placement policy.
	//
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// selectedTargets is the number of sync targets currently selected
	// by this placement policy.
	//
	// +optional
	SelectedTargets int32 `json:"selectedTargets,omitempty"`

	// targetSelections contains detailed information about target selection
	// and placement decisions made by this policy.
	//
	// +optional
	// +listType=atomic
	TargetSelections []TargetSelection `json:"targetSelections,omitempty"`

	// lastPlacementTime is the last time placement evaluation was performed.
	//
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`
}

// TargetSelection represents the selection of a specific sync target
// and the reasoning behind the placement decision.
type TargetSelection struct {
	// targetName is the name of the selected sync target.
	//
	// +required
	TargetName string `json:"targetName"`

	// workspace is the logical cluster workspace where the target exists.
	//
	// +optional
	Workspace string `json:"workspace,omitempty"`

	// selected indicates whether this target was selected for placement.
	//
	// +required
	Selected bool `json:"selected"`

	// reason provides explanation for the selection decision.
	//
	// +optional
	Reason string `json:"reason,omitempty"`

	// score represents the placement score for this target (higher is better).
	// Used for ranking when multiple targets are available.
	//
	// TODO: Implement scoring logic in follow-up PR
	// +optional
	Score int32 `json:"score,omitempty"`

	// lastEvaluationTime is when this target was last evaluated for placement.
	//
	// +optional
	LastEvaluationTime *metav1.Time `json:"lastEvaluationTime,omitempty"`
}

// EvaluatePlacement checks if a target matches placement rules and returns
// whether the target should be selected along with a reason for the decision.
// This is the core placement evaluation method.
func (cwp *ClusterWorkloadPlacement) EvaluatePlacement(target *SyncTarget) (bool, string) {
	if target == nil {
		return false, "target is nil"
	}

	// Check namespace selector constraints
	if cwp.Spec.NamespaceSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(cwp.Spec.NamespaceSelector)
		if err != nil {
			return false, fmt.Sprintf("invalid namespace selector: %v", err)
		}
		
		// In a real scenario, this would check against namespace labels
		// For now, we check target labels as a placeholder
		if !selector.Matches(labels.Set(target.Labels)) {
			return false, "namespace selector does not match target labels"
		}
	}

	// Check location selector constraints
	if cwp.Spec.LocationSelector != nil {
		if !cwp.evaluateLocationSelector(target) {
			return false, "location requirements not met"
		}
	}

	// Check resource requirements (basic implementation)
	if cwp.Spec.ResourceRequirements != nil {
		if !cwp.evaluateResourceRequirements(target) {
			return false, "insufficient resources"
		}
	}

	return true, "target meets all placement criteria"
}

// evaluateLocationSelector checks location constraints against a target.
func (cwp *ClusterWorkloadPlacement) evaluateLocationSelector(target *SyncTarget) bool {
	if cwp.Spec.LocationSelector == nil {
		return true
	}

	// Check required locations
	if len(cwp.Spec.LocationSelector.RequiredLocations) > 0 {
		targetLocation := target.Spec.Location
		if targetLocation == "" {
			return false // No location specified on target
		}

		locationMatches := false
		for _, required := range cwp.Spec.LocationSelector.RequiredLocations {
			if targetLocation == required {
				locationMatches = true
				break
			}
		}

		if !locationMatches {
			return false
		}
	}

	// Preferred locations affect scoring but not filtering
	// TODO: Implement scoring logic in follow-up PR
	
	return true
}

// evaluateResourceRequirements checks resource constraints against a target.
// This is a simplified implementation - comprehensive resource evaluation 
// will be added in follow-up PR.
func (cwp *ClusterWorkloadPlacement) evaluateResourceRequirements(target *SyncTarget) bool {
	if cwp.Spec.ResourceRequirements == nil {
		return true
	}

	// TODO: Implement proper resource capacity checking
	// For now, return true if target has any capacity information
	if target.Status.Allocatable.CPU != nil || target.Status.Allocatable.Memory != nil {
		return true
	}

	// If no resource information is available, assume target is capable
	return true
}

// GetConditions returns the conditions of the ClusterWorkloadPlacement.
func (cwp *ClusterWorkloadPlacement) GetConditions() conditionsv1alpha1.Conditions {
	return cwp.Status.Conditions
}

// SetConditions sets the conditions of the ClusterWorkloadPlacement.
func (cwp *ClusterWorkloadPlacement) SetConditions(conditions conditionsv1alpha1.Conditions) {
	cwp.Status.Conditions = conditions
}

// ClusterWorkloadPlacementList contains a list of ClusterWorkloadPlacement resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterWorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterWorkloadPlacement `json:"items"`
}