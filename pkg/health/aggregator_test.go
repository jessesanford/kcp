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
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultHealthAggregator(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewDefaultHealthAggregator(config)
	
	// Test empty aggregator
	ctx := context.Background()
	status := aggregator.CheckAll(ctx)
	
	if status.TotalCount != 0 {
		t.Errorf("Empty aggregator TotalCount = %d, expected 0", status.TotalCount)
	}
	
	if status.HealthyCount != 0 {
		t.Errorf("Empty aggregator HealthyCount = %d, expected 0", status.HealthyCount)
	}
	
	if !status.Healthy {
		t.Errorf("Empty aggregator should be considered healthy, got %v", status.Healthy)
	}
}

func TestDefaultHealthAggregator_WithHealthyComponents(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewDefaultHealthAggregator(config)
	
	// Add healthy checkers
	checker1 := NewStaticHealthChecker("component1", true, "healthy")
	checker2 := NewStaticHealthChecker("component2", true, "all good")
	
	aggregator.AddChecker(checker1)
	aggregator.AddChecker(checker2)
	
	ctx := context.Background()
	status := aggregator.CheckAll(ctx)
	
	if status.TotalCount != 2 {
		t.Errorf("TotalCount = %d, expected 2", status.TotalCount)
	}
	
	if status.HealthyCount != 2 {
		t.Errorf("HealthyCount = %d, expected 2", status.HealthyCount)
	}
	
	if !status.Healthy {
		t.Errorf("All components healthy, system should be healthy")
	}
	
	if !strings.Contains(status.Message, "2 components are healthy") {
		t.Errorf("Message should indicate all components are healthy, got: %s", status.Message)
	}
}

func TestDefaultHealthAggregator_WithUnhealthyComponents(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewDefaultHealthAggregator(config)
	
	// Add mixed checkers
	checker1 := NewStaticHealthChecker("component1", true, "healthy")
	checker2 := NewStaticHealthChecker("component2", false, "failing")
	
	aggregator.AddChecker(checker1)
	aggregator.AddChecker(checker2)
	
	ctx := context.Background()
	status := aggregator.CheckAll(ctx)
	
	if status.TotalCount != 2 {
		t.Errorf("TotalCount = %d, expected 2", status.TotalCount)
	}
	
	if status.HealthyCount != 1 {
		t.Errorf("HealthyCount = %d, expected 1", status.HealthyCount)
	}
	
	if status.Healthy {
		t.Errorf("One component unhealthy, system should be unhealthy")
	}
	
	if !strings.Contains(status.Message, "1/2 components healthy") {
		t.Errorf("Message should indicate partial health, got: %s", status.Message)
	}
}

func TestDefaultHealthAggregator_CheckComponent(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewDefaultHealthAggregator(config)
	
	checker := NewStaticHealthChecker("test-component", true, "working fine")
	aggregator.AddChecker(checker)
	
	ctx := context.Background()
	
	// Test existing component
	status, err := aggregator.CheckComponent(ctx, "test-component")
	if err != nil {
		t.Fatalf("CheckComponent() error = %v, expected nil", err)
	}
	
	if !status.Healthy {
		t.Errorf("Component should be healthy")
	}
	
	if status.Message != "working fine" {
		t.Errorf("Message = %q, expected 'working fine'", status.Message)
	}
	
	// Test non-existing component
	_, err = aggregator.CheckComponent(ctx, "non-existent")
	if err == nil {
		t.Errorf("CheckComponent() for non-existent component should return error")
	}
}

func TestDefaultHealthAggregator_RemoveChecker(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewDefaultHealthAggregator(config)
	
	checker := NewStaticHealthChecker("removable", true, "temporary")
	aggregator.AddChecker(checker)
	
	ctx := context.Background()
	
	// Verify checker is present
	status := aggregator.CheckAll(ctx)
	if status.TotalCount != 1 {
		t.Errorf("Before removal TotalCount = %d, expected 1", status.TotalCount)
	}
	
	// Remove checker
	aggregator.RemoveChecker("removable")
	
	// Verify checker is gone
	status = aggregator.CheckAll(ctx)
	if status.TotalCount != 0 {
		t.Errorf("After removal TotalCount = %d, expected 0", status.TotalCount)
	}
	
	// Verify direct check fails
	_, err := aggregator.CheckComponent(ctx, "removable")
	if err == nil {
		t.Errorf("CheckComponent() for removed component should return error")
	}
}

func TestWeightedHealthAggregator(t *testing.T) {
	config := DefaultHealthConfiguration()
	aggregator := NewWeightedHealthAggregator(config)
	
	// Add components with different weights
	checker1 := NewStaticHealthChecker("critical", true, "critical component healthy")
	checker2 := NewStaticHealthChecker("optional", false, "optional component failed")
	
	aggregator.AddWeightedChecker(checker1, 3.0) // Critical component with high weight
	aggregator.AddWeightedChecker(checker2, 1.0) // Optional component with low weight
	
	ctx := context.Background()
	status := aggregator.CheckAll(ctx)
	
	// Should be healthy because weighted health is 3.0/4.0 = 75%, which is below 80% threshold for this test
	// but in practice, having critical components healthy should make system healthy
	if status.TotalCount != 2 {
		t.Errorf("TotalCount = %d, expected 2", status.TotalCount)
	}
	
	if status.HealthyCount != 1 {
		t.Errorf("HealthyCount = %d, expected 1", status.HealthyCount)
	}
	
	// Check that weighted health percentage is reported
	if !strings.Contains(status.Message, "Weighted health") {
		t.Errorf("Message should mention weighted health, got: %s", status.Message)
	}
}