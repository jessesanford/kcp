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

package synctarget

import (
	"context"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SyncTarget represents a sync target resource (foundation stub)
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncTargetSpec   `json:"spec,omitempty"`
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget
type SyncTargetSpec struct {
	// TODO: Add spec fields when workload APIs are available
}

// SyncTargetStatus defines the observed state of a SyncTarget
type SyncTargetStatus struct {
	Replicas          int32              `json:"replicas,omitempty"`
	ReadyReplicas     int32              `json:"readyReplicas,omitempty"`
	AvailableReplicas int32              `json:"availableReplicas,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyObject returns a generically typed copy of an object
func (in *SyncTarget) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of the SyncTarget
func (in *SyncTarget) DeepCopy() *SyncTarget {
	if in == nil {
		return nil
	}
	out := new(SyncTarget)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another SyncTarget
func (in *SyncTarget) DeepCopyInto(out *SyncTarget) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyInto copies spec properties
func (in *SyncTargetSpec) DeepCopyInto(out *SyncTargetSpec) {
	*out = *in
}

// DeepCopy returns a deep copy of the SyncTargetSpec
func (in *SyncTargetSpec) DeepCopy() *SyncTargetSpec {
	if in == nil {
		return nil
	}
	out := new(SyncTargetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies status properties
func (in *SyncTargetStatus) DeepCopyInto(out *SyncTargetStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a deep copy of the SyncTargetStatus
func (in *SyncTargetStatus) DeepCopy() *SyncTargetStatus {
	if in == nil {
		return nil
	}
	out := new(SyncTargetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeploymentManager abstracts syncer deployment operations
type DeploymentManager interface {
	// EnsureDeployment ensures a syncer deployment exists and is up-to-date
	EnsureDeployment(ctx context.Context, cluster logicalcluster.Path, target *SyncTarget) error

	// DeleteDeployment removes the syncer deployment
	DeleteDeployment(ctx context.Context, target *SyncTarget) error

	// GetDeploymentStatus retrieves the current deployment status
	GetDeploymentStatus(ctx context.Context, target *SyncTarget) (*DeploymentStatus, error)
}

// StatusUpdater abstracts status update operations
type StatusUpdater interface {
	// UpdateStatus updates the SyncTarget status based on deployment state
	UpdateStatus(ctx context.Context, target *SyncTarget, status *DeploymentStatus) error
}

// DeploymentStatus represents the state of a syncer deployment
type DeploymentStatus struct {
	Ready             bool
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	Condition         metav1.Condition
}

// ReconcileResult represents the outcome of a reconciliation
type ReconcileResult struct {
	Requeue      bool
	RequeueAfter time.Duration
	Error        error
}