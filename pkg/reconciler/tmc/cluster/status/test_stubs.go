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

package status

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// StatusManagerInterface defines the interface for managing TMC cluster status.
type StatusManagerInterface interface {
	UpdateStatus(ctx context.Context, obj StatusUpdater, conditions []conditionsapi.Condition) error
	RecordEvent(obj runtime.Object, eventType, reason, message string)
	ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition
	SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string)
}

// StatusUpdater interface defines methods for objects that can have their status updated.
type StatusUpdater interface {
	conditionsutil.Getter
	conditionsutil.Setter
	metav1.Object
	runtime.Object
}

// StubManager is a minimal stub implementation for testing.
type StubManager struct {
	eventRecorder record.EventRecorder
}

// NewManager creates a stub status manager for testing.
func NewManager(eventRecorder record.EventRecorder, logger klog.Logger) StatusManagerInterface {
	return &StubManager{eventRecorder: eventRecorder}
}

func (m *StubManager) UpdateStatus(ctx context.Context, obj StatusUpdater, conditions []conditionsapi.Condition) error {
	if obj == nil {
		return nil
	}
	obj.SetConditions(conditions)
	return nil
}

func (m *StubManager) RecordEvent(obj runtime.Object, eventType, reason, message string) {
	if m.eventRecorder != nil && obj != nil {
		m.eventRecorder.Event(obj, eventType, reason, message)
	}
}

func (m *StubManager) ComputeReadyCondition(conditions []conditionsapi.Condition) *conditionsapi.Condition {
	if len(conditions) == 0 {
		return &conditionsapi.Condition{
			Type:    conditionsapi.ReadyCondition,
			Status:  corev1.ConditionUnknown,
			Reason:  "NoConditions",
			Message: "No conditions available",
		}
	}
	
	return &conditionsapi.Condition{
		Type:    conditionsapi.ReadyCondition,
		Status:  corev1.ConditionTrue,
		Reason:  "ComponentsReady",
		Message: "All components are ready",
	}
}

func (m *StubManager) SetTransitioningCondition(obj StatusUpdater, conditionType conditionsapi.ConditionType, reason, message string) {
	if obj == nil {
		return
	}
	
	condition := &conditionsapi.Condition{
		Type:    conditionType,
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: message,
	}
	
	conditionsutil.Set(obj, condition)
}