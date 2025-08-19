// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TestObject represents a minimal test object for TMC testing
type TestObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	
	Spec   TestObjectSpec   `json:"spec,omitempty"`
	Status TestObjectStatus `json:"status,omitempty"`
}

// TestObjectSpec defines the desired state of TestObject
type TestObjectSpec struct {
	Message string `json:"message,omitempty"`
}

// TestObjectStatus defines the observed state of TestObject
type TestObjectStatus struct {
	Phase   string             `json:"phase,omitempty"`
	Message string             `json:"message,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// TestObjectList contains a list of TestObject
type TestObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestObject `json:"items"`
}

// DeepCopyObject returns a deep copy of the TestObject
func (in *TestObject) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(TestObject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another TestObject
func (in *TestObject) DeepCopyInto(out *TestObject) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyInto copies all properties into another TestObjectStatus
func (in *TestObjectStatus) DeepCopyInto(out *TestObjectStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

// NewTestObject creates a test object for TMC testing
func NewTestObject(name, workspace string) *TestObject {
	return &TestObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.tmc.kcp.io/v1alpha1",
			Kind:       "TestObject",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"kcp.io/cluster": workspace,
			},
		},
		Spec: TestObjectSpec{
			Message: fmt.Sprintf("test object %s in %s", name, workspace),
		},
	}
}

// MakeTMCKey creates a KCP-compatible key for TMC objects
func MakeTMCKey(workspace logicalcluster.Name, name string) string {
	return fmt.Sprintf("%s|%s", workspace, name)
}

// ParseTMCKey parses a TMC key into workspace and name
func ParseTMCKey(key string) (logicalcluster.Name, string, error) {
	// Simple parsing - in production this would use cache.SplitMetaNamespaceKey
	// but adapted for KCP's logical cluster format
	for i, r := range key {
		if r == '|' {
			return logicalcluster.Name(key[:i]), key[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid TMC key format: %s", key)
}

// SetCondition sets or updates a condition on a test object
func SetCondition(obj *TestObject, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.NewTime(time.Now())
	
	// Find existing condition
	for i, existing := range obj.Status.Conditions {
		if existing.Type == conditionType {
			obj.Status.Conditions[i].Status = status
			obj.Status.Conditions[i].Reason = reason
			obj.Status.Conditions[i].Message = message
			obj.Status.Conditions[i].LastTransitionTime = now
			return
		}
	}
	
	// Add new condition
	obj.Status.Conditions = append(obj.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// GetCondition retrieves a condition from a test object
func GetCondition(obj *TestObject, conditionType string) *metav1.Condition {
	for _, cond := range obj.Status.Conditions {
		if cond.Type == conditionType {
			return &cond
		}
	}
	return nil
}