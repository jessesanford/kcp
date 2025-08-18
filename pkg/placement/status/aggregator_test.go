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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockStatusCollector implements StatusCollector for testing
type mockStatusCollector struct {
	statuses map[string]TargetStatus
}

func (m *mockStatusCollector) CollectStatus(ctx context.Context, target SyncTarget) (TargetStatus, error) {
	if status, exists := m.statuses[target.Name]; exists {
		return status, nil
	}
	
	// Return default status for unknown targets
	return TargetStatus{
		Target:         target,
		Health:         HealthStatusUnknown,
		ResourceCount:  0,
		ReadyResources: 0,
		LastUpdated:    metav1.Now(),
	}, nil
}

// mockHealthCalculator implements HealthCalculator for testing
type mockHealthCalculator struct {
	overallHealth HealthStatus
}

func (m *mockHealthCalculator) CalculateOverallHealth(statuses []TargetStatus) HealthStatus {
	return m.overallHealth
}

// mockMetricsRecorder implements MetricsRecorder for testing
type mockMetricsRecorder struct {
	recordedStatuses []*AggregatedStatus
}

func (m *mockMetricsRecorder) RecordAggregatedStatus(status *AggregatedStatus) error {
	m.recordedStatuses = append(m.recordedStatuses, status)
	return nil
}

func TestStatusAggregator(t *testing.T) {
	tests := map[string]struct {
		placement      *WorkloadPlacement
		targetStatuses map[string]TargetStatus
		expectedHealth HealthStatus
		expectedCount  int
	}{
		"healthy placement with ready targets": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-placement",
					Namespace: "test-ns",
				},
				Spec: WorkloadPlacementSpec{
					LocationResource: &LocationResourceReference{
						Name:      "target-1",
						Workspace: "test-workspace",
					},
				},
				Status: WorkloadPlacementStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Placed",
							Status: metav1.ConditionTrue,
							Reason: "target-2",
						},
					},
				},
			},
			targetStatuses: map[string]TargetStatus{
				"target-1": {
					Health:         HealthStatusHealthy,
					ResourceCount:  10,
					ReadyResources: 10,
				},
				"target-2": {
					Health:         HealthStatusHealthy,
					ResourceCount:  5,
					ReadyResources: 5,
				},
			},
			expectedHealth: HealthStatusHealthy,
			expectedCount:  2,
		},
		"degraded placement with mixed target health": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-placement",
					Namespace: "test-ns",
				},
				Spec: WorkloadPlacementSpec{
					LocationResource: &LocationResourceReference{
						Name:      "target-1",
						Workspace: "test-workspace",
					},
				},
			},
			targetStatuses: map[string]TargetStatus{
				"target-1": {
					Health:         HealthStatusHealthy,
					ResourceCount:  10,
					ReadyResources: 8,
				},
			},
			expectedHealth: HealthStatusDegraded,
			expectedCount:  1,
		},
		"no targets placement": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-placement",
					Namespace: "test-ns",
				},
				Spec: WorkloadPlacementSpec{},
			},
			targetStatuses: map[string]TargetStatus{},
			expectedHealth: HealthStatusUnknown,
			expectedCount:  0,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock dependencies
			collector := &mockStatusCollector{statuses: tc.targetStatuses}
			health := &mockHealthCalculator{overallHealth: tc.expectedHealth}
			metrics := &mockMetricsRecorder{}
			
			// Create aggregator
			aggregator := NewStatusAggregator(collector, health, metrics)
			
			// Test aggregation
			ctx := context.Background()
			result, err := aggregator.AggregateStatus(ctx, tc.placement)
			
			// Verify results
			if err != nil {
				t.Fatalf("AggregateStatus failed: %v", err)
			}
			
			if result.OverallHealth != tc.expectedHealth {
				t.Errorf("Expected health %v, got %v", tc.expectedHealth, result.OverallHealth)
			}
			
			if result.TotalTargets != tc.expectedCount {
				t.Errorf("Expected %d targets, got %d", tc.expectedCount, result.TotalTargets)
			}
			
			// Verify metrics were recorded (should always be 1 for any successful aggregation)
			if len(metrics.recordedStatuses) != 1 {
				t.Errorf("Expected 1 recorded status, got %d", len(metrics.recordedStatuses))
			}
			
			// Verify the recorded status matches our expectation
			if metrics.recordedStatuses[0].OverallHealth != tc.expectedHealth {
				t.Errorf("Recorded status health %v, expected %v", 
					metrics.recordedStatuses[0].OverallHealth, tc.expectedHealth)
			}
		})
	}
}

func TestHealthStatusMethods(t *testing.T) {
	tests := map[string]struct {
		health   HealthStatus
		isHealthy bool
		isDegraded bool
		isUnhealthy bool
		isUnknown bool
	}{
		"healthy status": {
			health:      HealthStatusHealthy,
			isHealthy:   true,
			isDegraded:  false,
			isUnhealthy: false,
			isUnknown:   false,
		},
		"degraded status": {
			health:      HealthStatusDegraded,
			isHealthy:   false,
			isDegraded:  true,
			isUnhealthy: false,
			isUnknown:   false,
		},
		"unhealthy status": {
			health:      HealthStatusUnhealthy,
			isHealthy:   false,
			isDegraded:  false,
			isUnhealthy: true,
			isUnknown:   false,
		},
		"unknown status": {
			health:      HealthStatusUnknown,
			isHealthy:   false,
			isDegraded:  false,
			isUnhealthy: false,
			isUnknown:   true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.health.IsHealthy() != tc.isHealthy {
				t.Errorf("IsHealthy() = %v, want %v", tc.health.IsHealthy(), tc.isHealthy)
			}
			if tc.health.IsDegraded() != tc.isDegraded {
				t.Errorf("IsDegraded() = %v, want %v", tc.health.IsDegraded(), tc.isDegraded)
			}
			if tc.health.IsUnhealthy() != tc.isUnhealthy {
				t.Errorf("IsUnhealthy() = %v, want %v", tc.health.IsUnhealthy(), tc.isUnhealthy)
			}
			if tc.health.IsUnknown() != tc.isUnknown {
				t.Errorf("IsUnknown() = %v, want %v", tc.health.IsUnknown(), tc.isUnknown)
			}
		})
	}
}