/*
Copyright 2023 The KCP Authors.

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

package health

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestMonitor_HealthAggregation(t *testing.T) {
	tests := []struct {
		name       string
		components map[string]*ComponentHealth
		expected   HealthStatus
	}{
		{
			name:       "no components",
			components: map[string]*ComponentHealth{},
			expected:   HealthStatusUnknown,
		},
		{
			name: "all healthy",
			components: map[string]*ComponentHealth{
				"comp1": {Status: HealthStatusHealthy},
				"comp2": {Status: HealthStatusHealthy},
			},
			expected: HealthStatusHealthy,
		},
		{
			name: "one unhealthy",
			components: map[string]*ComponentHealth{
				"comp1": {Status: HealthStatusHealthy},
				"comp2": {Status: HealthStatusUnhealthy},
			},
			expected: HealthStatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewMonitor(
				fake.NewSimpleClientset(),
				logicalcluster.Name("root:test"),
				"default",
				"test-syncer",
			)

			result := monitor.aggregateStatus(tt.components)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHealthChecks(t *testing.T) {
	t.Run("ConnectionHealthCheck", func(t *testing.T) {
		checker := NewConnectionHealthCheck("test-connection")
		
		health := checker.Check(context.Background())
		if health.Status != HealthStatusUnhealthy {
			t.Errorf("expected unhealthy, got %v", health.Status)
		}
		
		checker.SetConnected(true)
		health = checker.Check(context.Background())
		if health.Status != HealthStatusHealthy {
			t.Errorf("expected healthy, got %v", health.Status)
		}
	})

	t.Run("ResourceHealthCheck", func(t *testing.T) {
		checker := NewResourceHealthCheck("test-resources")
		health := checker.Check(context.Background())
		
		if health.Status == HealthStatusUnknown {
			t.Errorf("expected non-unknown status, got %v", health.Status)
		}
	})
}

func TestMetrics(t *testing.T) {
	monitor := NewMonitor(
		fake.NewSimpleClientset(),
		logicalcluster.Name("root:test"),
		"default",
		"test-syncer",
	)
	
	monitor.RecordSync(100 * time.Millisecond)
	monitor.RecordSync(200 * time.Millisecond)
	
	rate := monitor.getSyncRate()
	if rate <= 0 {
		t.Errorf("expected positive sync rate, got %f", rate)
	}
	
	monitor.RecordError()
	errorRate := monitor.getErrorRate()
	expectedRate := 1.0 / 3.0 // 1 error out of 3 total operations
	if errorRate != expectedRate {
		t.Errorf("expected error rate %f, got %f", expectedRate, errorRate)
	}
}