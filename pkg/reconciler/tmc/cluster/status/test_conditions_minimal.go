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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	conditionsutil "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// Essential TMC condition types for testing
const (
	ClusterConnectionCondition   conditionsapi.ConditionType = "ClusterConnection"
	ClusterRegistrationCondition conditionsapi.ConditionType = "ClusterRegistration"
	PlacementAvailableCondition  conditionsapi.ConditionType = "PlacementAvailable"
	HeartbeatCondition          conditionsapi.ConditionType = "Heartbeat"
	ResourcesAvailableCondition conditionsapi.ConditionType = "ResourcesAvailable"
)

// Essential condition reasons
const (
	ClusterConnectedReason      = "ClusterConnected"
	ClusterDisconnectedReason  = "ClusterDisconnected"
	ConnectionTimeoutReason     = "ConnectionTimeout"
	ClusterRegisteredReason     = "ClusterRegistered"
	HeartbeatHealthyReason      = "HeartbeatHealthy"
	HeartbeatMissedReason       = "HeartbeatMissed"
	InsufficientResourcesReason = "InsufficientResources"
	PlacementUnavailableReason  = "PlacementUnavailable"
)

// ConditionBuilder for creating test conditions
type ConditionBuilder struct {
	condition conditionsapi.Condition
}

func NewConditionBuilder(conditionType conditionsapi.ConditionType) *ConditionBuilder {
	return &ConditionBuilder{
		condition: conditionsapi.Condition{
			Type:               conditionType,
			LastTransitionTime: metav1.NewTime(metav1.Now().Truncate(1 * 1000000000)),
		},
	}
}

func (b *ConditionBuilder) WithStatus(status corev1.ConditionStatus) *ConditionBuilder {
	b.condition.Status = status
	return b
}

func (b *ConditionBuilder) WithReason(reason string) *ConditionBuilder {
	b.condition.Reason = reason
	return b
}

func (b *ConditionBuilder) WithMessage(message string) *ConditionBuilder {
	b.condition.Message = message
	return b
}

func (b *ConditionBuilder) WithSeverity(severity conditionsapi.ConditionSeverity) *ConditionBuilder {
	b.condition.Severity = severity
	return b
}

func (b *ConditionBuilder) Build() *conditionsapi.Condition {
	return &b.condition
}

// Essential condition constructors for testing
func ClusterConnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(ClusterConnectedReason).
		WithMessage(message).
		Build()
}

func ClusterDisconnectedCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(ClusterDisconnectedReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityError).
		Build()
}

func ClusterConnectionTimeoutCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterConnectionCondition).
		WithStatus(corev1.ConditionUnknown).
		WithReason(ConnectionTimeoutReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

func ClusterRegisteredCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ClusterRegistrationCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(ClusterRegisteredReason).
		WithMessage(message).
		Build()
}

func HeartbeatHealthyCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(HeartbeatCondition).
		WithStatus(corev1.ConditionTrue).
		WithReason(HeartbeatHealthyReason).
		WithMessage(message).
		Build()
}

func HeartbeatUnhealthyCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == HeartbeatMissedReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return NewConditionBuilder(HeartbeatCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

func ResourcesInsufficientCondition(message string) *conditionsapi.Condition {
	return NewConditionBuilder(ResourcesAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(InsufficientResourcesReason).
		WithMessage(message).
		WithSeverity(conditionsapi.ConditionSeverityWarning).
		Build()
}

func PlacementUnavailableCondition(reason, message string) *conditionsapi.Condition {
	severity := conditionsapi.ConditionSeverityWarning
	if reason == InsufficientResourcesReason {
		severity = conditionsapi.ConditionSeverityError
	}
	
	return NewConditionBuilder(PlacementAvailableCondition).
		WithStatus(corev1.ConditionFalse).
		WithReason(reason).
		WithMessage(message).
		WithSeverity(severity).
		Build()
}

// Essential condition helper functions for testing
func IsConditionTrue(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsConditionFalse(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition != nil && condition.Status == corev1.ConditionFalse
}

func IsConditionUnknown(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) bool {
	condition := conditionsutil.Get(obj, conditionType)
	return condition == nil || condition.Status == corev1.ConditionUnknown
}

func GetConditionReason(obj conditionsutil.Getter, conditionType conditionsapi.ConditionType) string {
	condition := conditionsutil.Get(obj, conditionType)
	if condition != nil {
		return condition.Reason
	}
	return ""
}

func HasCriticalConditionError(conditions []conditionsapi.Condition) bool {
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			return true
		}
	}
	return false
}

func GetCriticalConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var critical []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityError {
			critical = append(critical, condition)
		}
	}
	return critical
}

func GetWarningConditions(conditions []conditionsapi.Condition) []conditionsapi.Condition {
	var warnings []conditionsapi.Condition
	for _, condition := range conditions {
		if condition.Status == corev1.ConditionFalse && condition.Severity == conditionsapi.ConditionSeverityWarning {
			warnings = append(warnings, condition)
		}
	}
	return warnings
}