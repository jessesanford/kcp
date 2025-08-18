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

package health

import (
	"strings"
	"testing"
	"time"
)

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   HealthStatus
		contains []string
	}{
		{
			name: "healthy status",
			status: HealthStatus{
				Healthy:   true,
				Message:   "all good",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"HEALTHY", "all good", "2024-01-01T12:00:00Z"},
		},
		{
			name: "unhealthy status",
			status: HealthStatus{
				Healthy:   false,
				Message:   "something wrong",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"UNHEALTHY", "something wrong", "2024-01-01T12:00:00Z"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("HealthStatus.String() = %q, expected to contain %q", result, expected)
				}
			}
		})
	}
}

func TestSystemHealthStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   SystemHealthStatus
		contains []string
	}{
		{
			name: "healthy system",
			status: SystemHealthStatus{
				Healthy:      true,
				Message:      "system is healthy",
				HealthyCount: 3,
				TotalCount:   3,
				Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"HEALTHY", "system is healthy", "3/3", "2024-01-01T12:00:00Z"},
		},
		{
			name: "unhealthy system",
			status: SystemHealthStatus{
				Healthy:      false,
				Message:      "some components unhealthy",
				HealthyCount: 2,
				TotalCount:   3,
				Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			contains: []string{"UNHEALTHY", "some components unhealthy", "2/3", "2024-01-01T12:00:00Z"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.String()
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("SystemHealthStatus.String() = %q, expected to contain %q", result, expected)
				}
			}
		})
	}
}

func TestDefaultHealthConfiguration(t *testing.T) {
	config := DefaultHealthConfiguration()
	
	if config.CheckTimeout <= 0 {
		t.Errorf("DefaultHealthConfiguration() CheckTimeout = %v, expected > 0", config.CheckTimeout)
	}
	
	if config.CheckInterval <= 0 {
		t.Errorf("DefaultHealthConfiguration() CheckInterval = %v, expected > 0", config.CheckInterval)
	}
	
	if config.MaxRetries < 0 {
		t.Errorf("DefaultHealthConfiguration() MaxRetries = %v, expected >= 0", config.MaxRetries)
	}
	
	if config.FailureThreshold <= 0 {
		t.Errorf("DefaultHealthConfiguration() FailureThreshold = %v, expected > 0", config.FailureThreshold)
	}
}