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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupName is the group name used in this package.
const GroupName = "workload.kcp.io"

// GroupVersion is the group version used to register these objects.
var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// SchemeGroupVersion is the group version used to register these objects.
var SchemeGroupVersion = GroupVersion

var (
	// SchemeBuilder points to a list of functions added to Scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme applies all the stored functions to the scheme. A non-nil error
	// indicates that one function failed and the attempt was abandoned.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&SyncTarget{},
		&SyncTargetList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// SyncTargetPhase represents the phase of a SyncTarget.
type SyncTargetPhase string

const (
	// SyncTargetPhasePending indicates that the SyncTarget is pending initial setup.
	SyncTargetPhasePending SyncTargetPhase = "Pending"
	// SyncTargetPhaseReady indicates that the SyncTarget is ready to sync workloads.
	SyncTargetPhaseReady SyncTargetPhase = "Ready"
	// SyncTargetPhaseNotReady indicates that the SyncTarget is not ready to sync workloads.
	SyncTargetPhaseNotReady SyncTargetPhase = "NotReady"
	// SyncTargetPhaseTerminating indicates that the SyncTarget is being terminated.
	SyncTargetPhaseTerminating SyncTargetPhase = "Terminating"
)

// SyncTargetConditionType represents a condition type for a SyncTarget.
type SyncTargetConditionType string

const (
	// SyncTargetReady indicates whether the SyncTarget is ready to sync workloads.
	SyncTargetReady SyncTargetConditionType = "Ready"
	// SyncTargetHealthy indicates whether the SyncTarget cluster is healthy.
	SyncTargetHealthy SyncTargetConditionType = "Healthy"
)

// SyncTargetSpec defines the desired state of a SyncTarget.
type SyncTargetSpec struct {
	// SupportedResourceTypes is a list of resource types that this SyncTarget can handle.
	// +optional
	SupportedResourceTypes []string `json:"supportedResourceTypes,omitempty"`

	// Location is the physical location of the target cluster.
	// +optional
	Location string `json:"location,omitempty"`
}

// SyncTargetStatus defines the observed state of a SyncTarget.
type SyncTargetStatus struct {
	// Phase represents the current phase of the SyncTarget.
	// +optional
	Phase SyncTargetPhase `json:"phase,omitempty"`

	// Conditions is a list of conditions that apply to the SyncTarget.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastHeartbeat is the timestamp of the last heartbeat from the syncer.
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
}

// SetCondition sets a condition on the SyncTarget.
func (s *SyncTarget) SetCondition(condition metav1.Condition) {
	if s.Status.Conditions == nil {
		s.Status.Conditions = []metav1.Condition{}
	}

	// Find existing condition with same type
	for i, existingCondition := range s.Status.Conditions {
		if existingCondition.Type == condition.Type {
			s.Status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	s.Status.Conditions = append(s.Status.Conditions, condition)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTarget represents a physical cluster that can receive workloads from KCP.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SyncTarget.
	Spec SyncTargetSpec `json:"spec,omitempty"`

	// Status defines the observed state of the SyncTarget.
	// +optional
	Status SyncTargetStatus `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object interface
func (s *SyncTarget) DeepCopyObject() runtime.Object {
	return s.DeepCopy()
}

// DeepCopy creates a deep copy of the SyncTarget
func (s *SyncTarget) DeepCopy() *SyncTarget {
	if s == nil {
		return nil
	}
	out := new(SyncTarget)
	s.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is passed as a pointer.
func (s *SyncTarget) DeepCopyInto(out *SyncTarget) {
	*out = *s
	out.TypeMeta = s.TypeMeta
	s.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	
	// Deep copy spec
	out.Spec = s.Spec
	if s.Spec.SupportedResourceTypes != nil {
		out.Spec.SupportedResourceTypes = make([]string, len(s.Spec.SupportedResourceTypes))
		copy(out.Spec.SupportedResourceTypes, s.Spec.SupportedResourceTypes)
	}
	
	// Deep copy status
	out.Status = s.Status
	if s.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(s.Status.Conditions))
		for i := range s.Status.Conditions {
			s.Status.Conditions[i].DeepCopyInto(&out.Status.Conditions[i])
		}
	}
	if s.Status.LastHeartbeat != nil {
		out.Status.LastHeartbeat = s.Status.LastHeartbeat.DeepCopy()
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetList contains a list of SyncTargets.
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}
// DeepCopyObject implements runtime.Object interface
func (s *SyncTargetList) DeepCopyObject() runtime.Object {
	return s.DeepCopy()
}

// DeepCopy creates a deep copy of the SyncTargetList
func (s *SyncTargetList) DeepCopy() *SyncTargetList {
	if s == nil {
		return nil
	}
	out := new(SyncTargetList)
	s.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the
// same type that is passed as a pointer.
func (s *SyncTargetList) DeepCopyInto(out *SyncTargetList) {
	*out = *s
	out.TypeMeta = s.TypeMeta
	s.ListMeta.DeepCopyInto(&out.ListMeta)
	
	if s.Items != nil {
		out.Items = make([]SyncTarget, len(s.Items))
		for i := range s.Items {
			s.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
