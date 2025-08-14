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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// GetCondition returns the condition with the given type if it exists,
// otherwise returns nil.
func (s *SyncTarget) GetCondition(conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == conditionType {
			return &s.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets the condition with the given type to the given status.
// If the condition already exists, it updates the existing condition.
// If the condition does not exist, it adds a new condition.
func (s *SyncTarget) SetCondition(conditionType conditionsv1alpha1.ConditionType, status corev1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := conditionsv1alpha1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}

	// Find existing condition
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == conditionType {
			// Update existing condition if status or reason changed
			if s.Status.Conditions[i].Status != status || s.Status.Conditions[i].Reason != reason {
				s.Status.Conditions[i].Status = status
				s.Status.Conditions[i].Reason = reason
				s.Status.Conditions[i].Message = message
				s.Status.Conditions[i].LastTransitionTime = now
			} else {
				// Only update the message if status and reason are the same
				s.Status.Conditions[i].Message = message
			}
			return
		}
	}

	// Add new condition
	s.Status.Conditions = append(s.Status.Conditions, condition)
}

// IsReady returns true if the SyncTarget is ready to accept workloads.
// A SyncTarget is considered ready when the Ready condition is True.
func (s *SyncTarget) IsReady() bool {
	condition := s.GetCondition(SyncTargetReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsSyncerReady returns true if the syncer component is connected and operational.
func (s *SyncTarget) IsSyncerReady() bool {
	condition := s.GetCondition(SyncTargetSyncerReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsClusterReady returns true if the target cluster is reachable and healthy.
func (s *SyncTarget) IsClusterReady() bool {
	condition := s.GetCondition(SyncTargetClusterReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetAvailableCapacity returns the available capacity by subtracting allocated
// from allocatable resources. Returns nil if allocatable is not set.
func (s *SyncTarget) GetAvailableCapacity() *ResourceCapacity {
	if s.Status.Allocatable.IsZero() {
		return nil
	}

	available := &ResourceCapacity{
		Custom: make(map[string]resource.Quantity),
	}

	// Calculate available CPU
	if s.Status.Allocatable.CPU != nil {
		cpu := s.Status.Allocatable.CPU.DeepCopy()
		available.CPU = &cpu
		if s.Status.Allocated.CPU != nil {
			available.CPU.Sub(*s.Status.Allocated.CPU)
		}
	}

	// Calculate available Memory
	if s.Status.Allocatable.Memory != nil {
		memory := s.Status.Allocatable.Memory.DeepCopy()
		available.Memory = &memory
		if s.Status.Allocated.Memory != nil {
			available.Memory.Sub(*s.Status.Allocated.Memory)
		}
	}

	// Calculate available Storage
	if s.Status.Allocatable.Storage != nil {
		storage := s.Status.Allocatable.Storage.DeepCopy()
		available.Storage = &storage
		if s.Status.Allocated.Storage != nil {
			available.Storage.Sub(*s.Status.Allocated.Storage)
		}
	}

	// Calculate available Pods
	if s.Status.Allocatable.Pods != nil {
		pods := s.Status.Allocatable.Pods.DeepCopy()
		available.Pods = &pods
		if s.Status.Allocated.Pods != nil {
			available.Pods.Sub(*s.Status.Allocated.Pods)
		}
	}

	// Calculate available custom resources
	for name, allocatable := range s.Status.Allocatable.Custom {
		available.Custom[name] = allocatable.DeepCopy()
		if allocated, exists := s.Status.Allocated.Custom[name]; exists {
			temp := available.Custom[name]
			temp.Sub(allocated)
			available.Custom[name] = temp
		}
	}

	return available
}

// HasSufficientCapacity returns true if the SyncTarget has sufficient capacity
// to accommodate the requested resources.
func (s *SyncTarget) HasSufficientCapacity(requested *ResourceCapacity) bool {
	if requested == nil {
		return true
	}

	available := s.GetAvailableCapacity()
	if available == nil {
		// If no allocatable capacity is set, assume unlimited capacity
		return true
	}

	// Check CPU
	if requested.CPU != nil && available.CPU != nil {
		if requested.CPU.Cmp(*available.CPU) > 0 {
			return false
		}
	}

	// Check Memory
	if requested.Memory != nil && available.Memory != nil {
		if requested.Memory.Cmp(*available.Memory) > 0 {
			return false
		}
	}

	// Check Storage
	if requested.Storage != nil && available.Storage != nil {
		if requested.Storage.Cmp(*available.Storage) > 0 {
			return false
		}
	}

	// Check Pods
	if requested.Pods != nil && available.Pods != nil {
		if requested.Pods.Cmp(*available.Pods) > 0 {
			return false
		}
	}

	// Check custom resources
	for name, requestedQty := range requested.Custom {
		if availableQty, exists := available.Custom[name]; exists {
			if requestedQty.Cmp(availableQty) > 0 {
				return false
			}
		}
	}

	return true
}

// MatchesSelector returns true if the given labels and location match the
// SyncTarget's workload selector criteria.
func (s *SyncTarget) MatchesSelector(labels map[string]string, location string) bool {
	if s.Spec.Selector == nil {
		return true // No selector means all workloads match
	}

	selector := s.Spec.Selector

	// Check location match
	if len(selector.Locations) > 0 {
		locationMatches := false
		for _, selectorLocation := range selector.Locations {
			if selectorLocation == location {
				locationMatches = true
				break
			}
		}
		if !locationMatches {
			return false
		}
	}

	// Check label match
	return matchesLabelSelector(labels, selector.MatchLabels, selector.MatchExpressions)
}

// IsLastSyncRecent returns true if the last sync time is within the given duration.
func (s *SyncTarget) IsLastSyncRecent(duration time.Duration) bool {
	if s.Status.LastSyncTime == nil {
		return false
	}
	return time.Since(s.Status.LastSyncTime.Time) <= duration
}

// GetSyncerConfigDefaults returns a SyncerConfig with default values applied.
func GetSyncerConfigDefaults() *SyncerConfig {
	return &SyncerConfig{
		SyncMode:     "push",
		SyncInterval: "30s",
		RetryBackoff: &RetryBackoffConfig{
			InitialInterval: "1s",
			MaxInterval:     "5m",
			Multiplier:      2.0,
		},
	}
}

// IsZero returns true if the ResourceCapacity has no resources defined.
func (rc *ResourceCapacity) IsZero() bool {
	return (rc.CPU == nil || rc.CPU.IsZero()) &&
		(rc.Memory == nil || rc.Memory.IsZero()) &&
		(rc.Storage == nil || rc.Storage.IsZero()) &&
		(rc.Pods == nil || rc.Pods.IsZero()) &&
		len(rc.Custom) == 0
}

// matchesLabelSelector checks if labels match the given label selector criteria.
func matchesLabelSelector(labels map[string]string, matchLabels map[string]string, matchExpressions []metav1.LabelSelectorRequirement) bool {
	// Check matchLabels
	for key, expectedValue := range matchLabels {
		if actualValue, exists := labels[key]; !exists || actualValue != expectedValue {
			return false
		}
	}

	// Check matchExpressions
	for _, expr := range matchExpressions {
		switch expr.Operator {
		case metav1.LabelSelectorOpIn:
			if value, exists := labels[expr.Key]; !exists {
				return false
			} else {
				found := false
				for _, allowedValue := range expr.Values {
					if value == allowedValue {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
		case metav1.LabelSelectorOpNotIn:
			if value, exists := labels[expr.Key]; exists {
				for _, disallowedValue := range expr.Values {
					if value == disallowedValue {
						return false
					}
				}
			}
		case metav1.LabelSelectorOpExists:
			if _, exists := labels[expr.Key]; !exists {
				return false
			}
		case metav1.LabelSelectorOpDoesNotExist:
			if _, exists := labels[expr.Key]; exists {
				return false
			}
		}
	}

	return true
}