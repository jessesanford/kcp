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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsapi "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestAggregator_AggregateClusterStatus(t *testing.T) {
	tests := map[string]struct {
		components     []ComponentStatus
		wantConditions int
		wantReady      corev1.ConditionStatus
		wantHealth     ClusterHealth
	}{
		"no components": {
			components:     []ComponentStatus{},
			wantConditions: 1, // Only Ready condition
			wantReady:      corev1.ConditionUnknown,
			wantHealth:     ClusterHealthUnknown, // Should be Unknown when no conditions
		},
		"all healthy components": {
			components: []ComponentStatus{
				{
					Name:           "connection",
					Critical:       true,
					LastUpdateTime: metav1.Now(),
					Conditions: []conditionsapi.Condition{
						{
							Type:   ClusterConnectionCondition,
							Status: corev1.ConditionTrue,
							Reason: "Connected",
						},
					},
				},
				{
					Name:           "registration",
					Critical:       true,
					LastUpdateTime: metav1.Now(),
					Conditions: []conditionsapi.Condition{
						{
							Type:   ClusterRegistrationCondition,
							Status: corev1.ConditionTrue,
							Reason: "Registered",
						},
					},
				},
			},
			wantConditions: 3, // Connection, Registration, Ready
			wantReady:      corev1.ConditionTrue,
			wantHealth:     ClusterHealthHealthy,
		},
		"critical component failed": {
			components: []ComponentStatus{
				{
					Name:           "connection",
					Critical:       true,
					LastUpdateTime: metav1.Now(),
					Conditions: []conditionsapi.Condition{
						{
							Type:     ClusterConnectionCondition,
							Status:   corev1.ConditionFalse,
							Reason:   "Disconnected",
							Severity: conditionsapi.ConditionSeverityError,
						},
					},
				},
			},
			wantConditions: 2, // Connection, Ready
			wantReady:      corev1.ConditionFalse,
			wantHealth:     ClusterHealthUnhealthy,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			aggregator := NewAggregator()

			conditions := aggregator.AggregateClusterStatus(tc.components)

			if len(conditions) != tc.wantConditions {
				t.Errorf("expected %d conditions, got %d", tc.wantConditions, len(conditions))
			}

			// Find Ready condition
			var readyCondition *conditionsapi.Condition
			for _, condition := range conditions {
				if condition.Type == conditionsapi.ReadyCondition {
					readyCondition = &condition
					break
				}
			}

			if readyCondition == nil {
				t.Error("Ready condition not found")
				return
			}

			if readyCondition.Status != tc.wantReady {
				t.Errorf("expected Ready status %s, got %s", tc.wantReady, readyCondition.Status)
			}

			health := aggregator.ComputeOverallHealth(conditions)
			if health != tc.wantHealth {
				t.Errorf("expected health %s, got %s", tc.wantHealth, health)
			}
		})
	}
}

func TestAggregator_FilterStaleConditions(t *testing.T) {
	aggregator := NewAggregator()
	
	now := time.Now()
	conditions := []conditionsapi.Condition{
		{
			Type:               "Fresh",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(now.Add(-1 * time.Minute)),
		},
		{
			Type:               "Stale",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(now.Add(-10 * time.Minute)),
		},
	}

	filtered := aggregator.FilterStaleConditions(conditions, 5*time.Minute)

	if len(filtered) != 1 {
		t.Errorf("expected 1 condition after filtering, got %d", len(filtered))
	}

	if filtered[0].Type != "Fresh" {
		t.Errorf("expected Fresh condition to remain, got %s", filtered[0].Type)
	}
}

func TestClusterHealthHelpers(t *testing.T) {
	tests := map[string]struct {
		health     ClusterHealth
		isHealthy  bool
		isDegraded bool
		isUnhealthy bool
		isUnknown  bool
	}{
		"healthy": {
			health:     ClusterHealthHealthy,
			isHealthy:  true,
			isDegraded: false,
			isUnhealthy: false,
			isUnknown:  false,
		},
		"degraded": {
			health:     ClusterHealthDegraded,
			isHealthy:  false,
			isDegraded: true,
			isUnhealthy: false,
			isUnknown:  false,
		},
		"unhealthy": {
			health:     ClusterHealthUnhealthy,
			isHealthy:  false,
			isDegraded: false,
			isUnhealthy: true,
			isUnknown:  false,
		},
		"unknown": {
			health:     ClusterHealthUnknown,
			isHealthy:  false,
			isDegraded: false,
			isUnhealthy: false,
			isUnknown:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if IsHealthy(tc.health) != tc.isHealthy {
				t.Errorf("IsHealthy(%s) = %v, want %v", tc.health, IsHealthy(tc.health), tc.isHealthy)
			}
			if IsDegraded(tc.health) != tc.isDegraded {
				t.Errorf("IsDegraded(%s) = %v, want %v", tc.health, IsDegraded(tc.health), tc.isDegraded)
			}
			if IsUnhealthy(tc.health) != tc.isUnhealthy {
				t.Errorf("IsUnhealthy(%s) = %v, want %v", tc.health, IsUnhealthy(tc.health), tc.isUnhealthy)
			}
			if IsUnknown(tc.health) != tc.isUnknown {
				t.Errorf("IsUnknown(%s) = %v, want %v", tc.health, IsUnknown(tc.health), tc.isUnknown)
			}
		})
	}
}