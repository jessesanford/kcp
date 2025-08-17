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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// Condition types for SyncTarget
const (
	// SyncTargetConditionReady indicates the SyncTarget is ready to accept workloads
	SyncTargetConditionReady conditionsv1alpha1.ConditionType = "Ready"

	// SyncTargetConditionHeartbeat indicates syncer heartbeat status
	SyncTargetConditionHeartbeat conditionsv1alpha1.ConditionType = "Heartbeat"

	// SyncTargetConditionSyncerReady indicates the syncer is ready and operational
	SyncTargetConditionSyncerReady conditionsv1alpha1.ConditionType = "SyncerReady"
)

// Condition reasons for SyncTarget
const (
	// SyncTargetReasonSyncerConnected indicates the syncer is connected and operational
	SyncTargetReasonSyncerConnected = "SyncerConnected"

	// SyncTargetReasonSyncerDisconnected indicates the syncer is disconnected
	SyncTargetReasonSyncerDisconnected = "SyncerDisconnected"

	// SyncTargetReasonHeartbeatMissing indicates heartbeats are missing
	SyncTargetReasonHeartbeatMissing = "HeartbeatMissing"

	// SyncTargetReasonUnschedulable indicates the target is marked unschedulable
	SyncTargetReasonUnschedulable = "Unschedulable"

	// SyncTargetReasonReady indicates the target is ready for workloads
	SyncTargetReasonReady = "Ready"
)

// IsReady returns true if the SyncTarget is ready to accept workloads
func (s *SyncTarget) IsReady() bool {
	return conditions.IsTrue(s, SyncTargetConditionReady)
}

// SetCondition sets a condition on the SyncTarget
func (s *SyncTarget) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(s, condition)
}

// GetCondition gets a condition from the SyncTarget by type
func (s *SyncTarget) GetCondition(conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	return conditions.Get(s, conditionType)
}

// SetReadyCondition sets the Ready condition on the SyncTarget
func (s *SyncTarget) SetReadyCondition(status corev1.ConditionStatus, reason, message string) {
	s.SetCondition(conditionsv1alpha1.Condition{
		Type:               SyncTargetConditionReady,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// SetHeartbeatCondition sets the Heartbeat condition on the SyncTarget
func (s *SyncTarget) SetHeartbeatCondition(status corev1.ConditionStatus, reason, message string) {
	s.SetCondition(conditionsv1alpha1.Condition{
		Type:               SyncTargetConditionHeartbeat,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// SetSyncerReadyCondition sets the SyncerReady condition on the SyncTarget
func (s *SyncTarget) SetSyncerReadyCondition(status corev1.ConditionStatus, reason, message string) {
	s.SetCondition(conditionsv1alpha1.Condition{
		Type:               SyncTargetConditionSyncerReady,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// IsSchedulable returns true if the SyncTarget can accept new workloads
func (s *SyncTarget) IsSchedulable() bool {
	return !s.Spec.Unschedulable && s.IsReady()
}

// HasHeartbeat returns true if the SyncTarget has received a recent heartbeat
func (s *SyncTarget) HasHeartbeat() bool {
	condition := s.GetCondition(SyncTargetConditionHeartbeat)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetCellByName returns the cell with the given name, or nil if not found
func (s *SyncTarget) GetCellByName(name string) *Cell {
	for i := range s.Spec.Cells {
		if s.Spec.Cells[i].Name == name {
			return &s.Spec.Cells[i]
		}
	}
	return nil
}

// HasTaint returns true if the cell has the specified taint
func (c *Cell) HasTaint(key string, effect TaintEffect) bool {
	for _, taint := range c.Taints {
		if taint.Key == key && taint.Effect == effect {
			return true
		}
	}
	return false
}

// GetTaint returns the taint with the specified key and effect, or nil if not found
func (c *Cell) GetTaint(key string, effect TaintEffect) *Taint {
	for i := range c.Taints {
		if c.Taints[i].Key == key && c.Taints[i].Effect == effect {
			return &c.Taints[i]
		}
	}
	return nil
}
