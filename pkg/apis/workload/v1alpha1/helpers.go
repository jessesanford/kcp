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
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for SyncTarget
const (
	// SyncTargetConditionReady indicates the SyncTarget is ready to accept workloads
	SyncTargetConditionReady = "Ready"

	// SyncTargetConditionHeartbeat indicates syncer heartbeat status
	SyncTargetConditionHeartbeat = "Heartbeat"

	// SyncTargetConditionSyncerReady indicates the syncer is ready and operational
	SyncTargetConditionSyncerReady = "SyncerReady"
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
	return s.getConditionStatus(SyncTargetConditionReady) == metav1.ConditionTrue
}

// SetCondition sets a condition on the SyncTarget
func (s *SyncTarget) SetCondition(condition metav1.Condition) {
	s.setCondition(condition)
}

// GetCondition gets a condition from the SyncTarget by type
func (s *SyncTarget) GetCondition(conditionType string) *metav1.Condition {
	return s.getCondition(conditionType)
}

// SetReadyCondition sets the Ready condition on the SyncTarget
func (s *SyncTarget) SetReadyCondition(status metav1.ConditionStatus, reason, message string) {
	s.SetCondition(metav1.Condition{
		Type:               SyncTargetConditionReady,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// SetHeartbeatCondition sets the Heartbeat condition on the SyncTarget
func (s *SyncTarget) SetHeartbeatCondition(status metav1.ConditionStatus, reason, message string) {
	s.SetCondition(metav1.Condition{
		Type:               SyncTargetConditionHeartbeat,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// SetSyncerReadyCondition sets the SyncerReady condition on the SyncTarget
func (s *SyncTarget) SetSyncerReadyCondition(status metav1.ConditionStatus, reason, message string) {
	s.SetCondition(metav1.Condition{
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
	return condition != nil && condition.Status == metav1.ConditionTrue
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

// Connection helper methods

// ValidateConnection performs validation on the connection configuration
func (s *SyncTarget) ValidateConnection() []string {
	var errors []string

	if s.Spec.Connection == nil {
		return errors // Connection is optional
	}

	conn := s.Spec.Connection
	if conn.URL == "" {
		errors = append(errors, "connection URL is required")
	} else {
		parsedURL, err := url.Parse(conn.URL)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid connection URL: %v", err))
		} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
			errors = append(errors, "connection URL must have scheme and host")
		}
	}

	return errors
}

// SupportsAuthType returns true if the SyncTarget supports the specified authentication type
func (s *SyncTarget) SupportsAuthType(authType string) bool {
	if s.Spec.Credentials == nil {
		return false
	}

	switch SyncTargetAuthType(authType) {
	case SyncTargetAuthTypeToken, SyncTargetAuthTypeCertificate, SyncTargetAuthTypeServiceAccount:
		return true
	default:
		return false
	}
}

// GetConnectionState returns the current connection state
func (s *SyncTarget) GetConnectionState() ConnectionState {
	if s.Status.ConnectionState == "" {
		return ConnectionStateDisconnected
	}
	return s.Status.ConnectionState
}

// IsConnected returns true if the target is currently connected
func (s *SyncTarget) IsConnected() bool {
	return s.GetConnectionState() == ConnectionStateConnected
}

// SetConnectionState sets the connection state
func (s *SyncTarget) SetConnectionState(state ConnectionState) {
	s.Status.ConnectionState = state
}

// Sync state helper methods

// SetSyncState sets the synchronization state
func (s *SyncTarget) SetSyncState(state SyncState) {
	s.Status.SyncState = state
}

// GetSyncState returns the current sync state
func (s *SyncTarget) GetSyncState() SyncState {
	if s.Status.SyncState == "" {
		return SyncStateNotReady
	}
	return s.Status.SyncState
}

// IsSyncReady returns true if synchronization is ready
func (s *SyncTarget) IsSyncReady() bool {
	return s.GetSyncState() == SyncStateReady
}

// AddSyncedResource adds a resource to the synced resources list
func (s *SyncTarget) AddSyncedResource(resource SyncedResourceStatus) {
	// Check if resource already exists and update it
	for i := range s.Status.SyncedResources {
		existing := &s.Status.SyncedResources[i]
		if existing.Group == resource.Group &&
			existing.Version == resource.Version &&
			existing.Kind == resource.Kind &&
			existing.Namespace == resource.Namespace &&
			existing.Name == resource.Name {
			s.Status.SyncedResources[i] = resource
			return
		}
	}
	
	// Add new resource
	s.Status.SyncedResources = append(s.Status.SyncedResources, resource)
}

// RemoveSyncedResource removes a resource from the synced resources list
func (s *SyncTarget) RemoveSyncedResource(group, version, kind, namespace, name string) {
	for i := range s.Status.SyncedResources {
		resource := &s.Status.SyncedResources[i]
		if resource.Group == group &&
			resource.Version == version &&
			resource.Kind == kind &&
			resource.Namespace == namespace &&
			resource.Name == name {
			s.Status.SyncedResources = append(s.Status.SyncedResources[:i], s.Status.SyncedResources[i+1:]...)
			return
		}
	}
}

// GetSyncedResource returns a synced resource by identifier
func (s *SyncTarget) GetSyncedResource(group, version, kind, namespace, name string) *SyncedResourceStatus {
	for i := range s.Status.SyncedResources {
		resource := &s.Status.SyncedResources[i]
		if resource.Group == group &&
			resource.Version == version &&
			resource.Kind == kind &&
			resource.Namespace == namespace &&
			resource.Name == name {
			return resource
		}
	}
	return nil
}

// Health helper methods

// SetHealthStatus sets the overall health status
func (s *SyncTarget) SetHealthStatus(status HealthStatusType, message string) {
	if s.Status.Health == nil {
		s.Status.Health = &HealthStatus{}
	}
	
	now := metav1.Now()
	s.Status.Health.Status = status
	s.Status.Health.Message = message
	s.Status.Health.LastChecked = &now
}

// GetHealthStatus returns the current health status
func (s *SyncTarget) GetHealthStatus() HealthStatusType {
	if s.Status.Health == nil {
		return HealthStatusUnknown
	}
	return s.Status.Health.Status
}

// IsHealthy returns true if the target is healthy
func (s *SyncTarget) IsHealthy() bool {
	return s.GetHealthStatus() == HealthStatusHealthy
}

// AddHealthCheck adds or updates a health check result
func (s *SyncTarget) AddHealthCheck(check HealthCheck) {
	if s.Status.Health == nil {
		s.Status.Health = &HealthStatus{}
	}
	
	// Check if health check already exists and update it
	for i := range s.Status.Health.Checks {
		if s.Status.Health.Checks[i].Name == check.Name {
			s.Status.Health.Checks[i] = check
			return
		}
	}
	
	// Add new health check
	s.Status.Health.Checks = append(s.Status.Health.Checks, check)
}

// Private helper methods for condition management

// getCondition returns the condition with the specified type
func (s *SyncTarget) getCondition(conditionType string) *metav1.Condition {
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == conditionType {
			return &s.Status.Conditions[i]
		}
	}
	return nil
}

// getConditionStatus returns the status of the specified condition
func (s *SyncTarget) getConditionStatus(conditionType string) metav1.ConditionStatus {
	condition := s.getCondition(conditionType)
	if condition == nil {
		return metav1.ConditionUnknown
	}
	return condition.Status
}

// setCondition sets or updates a condition
func (s *SyncTarget) setCondition(condition metav1.Condition) {
	// Check if condition already exists and update it
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == condition.Type {
			// Only update if status changed or transition time is not set
			existingCondition := &s.Status.Conditions[i]
			if existingCondition.Status != condition.Status {
				condition.LastTransitionTime = metav1.Now()
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			s.Status.Conditions[i] = condition
			return
		}
	}
	
	// Add new condition
	if condition.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = metav1.Now()
	}
	s.Status.Conditions = append(s.Status.Conditions, condition)
}
