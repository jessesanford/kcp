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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// MockStatusUpdater implements StatusUpdater interface for testing.
type MockStatusUpdater struct {
	metav1.ObjectMeta
	conditions conditionsapi.Conditions
}

func (m *MockStatusUpdater) GetConditions() conditionsapi.Conditions {
	return m.conditions
}

func (m *MockStatusUpdater) SetConditions(conditions conditionsapi.Conditions) {
	m.conditions = conditions
}

func (m *MockStatusUpdater) DeepCopyObject() runtime.Object {
	copy := &MockStatusUpdater{
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
		conditions: make(conditionsapi.Conditions, len(m.conditions)),
	}
	for i, condition := range m.conditions {
		copy.conditions[i] = *condition.DeepCopy()
	}
	return copy
}

func (m *MockStatusUpdater) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// MockEventRecorder implements record.EventRecorder for testing.
type MockEventRecorder struct {
	events []MockEvent
}

type MockEvent struct {
	Object    interface{}
	EventType string
	Reason    string
	Message   string
}

func (m *MockEventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	m.events = append(m.events, MockEvent{
		Object:    object,
		EventType: eventType,
		Reason:    reason,
		Message:   message,
	})
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (m *MockEventRecorder) GetEvents() []MockEvent {
	return m.events
}

func (m *MockEventRecorder) Reset() {
	m.events = nil
}

// NewTestStatusUpdater creates a mock status updater for testing.
func NewTestStatusUpdater(name, namespace string) *MockStatusUpdater {
	return &MockStatusUpdater{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// NewTestEventRecorder creates a mock event recorder for testing.
func NewTestEventRecorder() *MockEventRecorder {
	return &MockEventRecorder{}
}