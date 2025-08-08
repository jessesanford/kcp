/*
Copyright 2025 The KCP Authors.

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

package storage

import (
	"context"
	"testing"
	"time"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

func TestMemoryStorage(t *testing.T) {
	// Enable the feature gate for testing
	err := utilfeature.DefaultMutableFeatureGate.Set("TMCMetricsStorage=true")
	if err != nil {
		t.Fatalf("Failed to enable feature gate: %v", err)
	}
	defer func() {
		_ = utilfeature.DefaultMutableFeatureGate.Set("TMCMetricsStorage=false")
	}()

	tests := map[string]struct {
		setupFunc    func(*testing.T) MetricsStorage
		testFunc     func(*testing.T, MetricsStorage)
		expectError  bool
	}{
		"write and query single point": {
			setupFunc: func(t *testing.T) MetricsStorage {
				return createTestStorage(t)
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				now := time.Now()
				
				point := MetricPoint{
					Timestamp: now,
					Value:     42.0,
					Labels:    map[string]string{"host": "server1"},
				}

				err := storage.WriteMetricPoint(ctx, "cpu.usage", point)
				if err != nil {
					t.Fatalf("Failed to write point: %v", err)
				}

				series, err := storage.QueryMetrics(ctx, []string{"cpu.usage"}, QueryOptions{})
				if err != nil {
					t.Fatalf("Failed to query metrics: %v", err)
				}

				if len(series) != 1 {
					t.Fatalf("Expected 1 series, got %d", len(series))
				}
				if len(series[0].Points) != 1 {
					t.Fatalf("Expected 1 point, got %d", len(series[0].Points))
				}
				if series[0].Points[0].Value != 42.0 {
					t.Errorf("Expected value 42.0, got %f", series[0].Points[0].Value)
				}
			},
		},
		"write and query metric series": {
			setupFunc: func(t *testing.T) MetricsStorage {
				return createTestStorage(t)
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				now := time.Now()
				
				series := MetricSeries{
					Name:        "memory.usage",
					Description: "Memory usage in bytes",
					Unit:        "bytes",
					Points: []MetricPoint{
						{Timestamp: now.Add(-2 * time.Hour), Value: 100.0},
						{Timestamp: now.Add(-1 * time.Hour), Value: 200.0},
						{Timestamp: now, Value: 300.0},
					},
					CommonLabels: map[string]string{"region": "us-west-2"},
				}

				err := storage.WriteMetricSeries(ctx, series)
				if err != nil {
					t.Fatalf("Failed to write series: %v", err)
				}

				queriedSeries, err := storage.QueryMetrics(ctx, []string{"memory.usage"}, QueryOptions{})
				if err != nil {
					t.Fatalf("Failed to query metrics: %v", err)
				}

				if len(queriedSeries) != 1 {
					t.Fatalf("Expected 1 series, got %d", len(queriedSeries))
				}
				s := queriedSeries[0]
				if s.Name != "memory.usage" {
					t.Errorf("Expected name 'memory.usage', got %s", s.Name)
				}
				if s.Description != "Memory usage in bytes" {
					t.Errorf("Expected description 'Memory usage in bytes', got %s", s.Description)
				}
				if len(s.Points) != 3 {
					t.Fatalf("Expected 3 points, got %d", len(s.Points))
				}
			},
		},
		"query with time range": {
			setupFunc: func(t *testing.T) MetricsStorage {
				storage := createTestStorage(t)
				setupTestData(t, storage)
				return storage
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				now := time.Now()
				startTime := now.Add(-90 * time.Minute)
				endTime := now.Add(-30 * time.Minute)

				series, err := storage.QueryMetrics(ctx, []string{"cpu.usage"}, QueryOptions{
					StartTime: &startTime,
					EndTime:   &endTime,
				})
				if err != nil {
					t.Fatalf("Failed to query metrics: %v", err)
				}

				if len(series) != 1 {
					t.Fatalf("Expected 1 series, got %d", len(series))
				}
				
				// Should have points within the time range
				if len(series[0].Points) == 0 {
					t.Error("Expected some points within time range")
				}
			},
		},
		"list metric names": {
			setupFunc: func(t *testing.T) MetricsStorage {
				storage := createTestStorage(t)
				setupTestData(t, storage)
				return storage
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				names, err := storage.ListMetricNames(ctx, nil)
				if err != nil {
					t.Fatalf("Failed to list metric names: %v", err)
				}

				expectedNames := []string{"cpu.usage", "memory.usage"}
				if len(names) != len(expectedNames) {
					t.Fatalf("Expected %d names, got %d", len(expectedNames), len(names))
				}
			},
		},
		"apply retention policy": {
			setupFunc: func(t *testing.T) MetricsStorage {
				storage := createTestStorage(t)
				setupOldTestData(t, storage)
				return storage
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				
				policy := RetentionPolicy{
					MaxAge:    1 * time.Hour,
					MaxPoints: 5,
				}

				err := storage.ApplyRetentionPolicy(ctx, policy)
				if err != nil {
					t.Fatalf("Failed to apply retention policy: %v", err)
				}

				stats, err := storage.GetStats(ctx)
				if err != nil {
					t.Fatalf("Failed to get stats: %v", err)
				}

				if stats.TotalPoints > 10 {
					t.Errorf("Expected retention to limit points, got %d", stats.TotalPoints)
				}
			},
		},
		"get storage stats": {
			setupFunc: func(t *testing.T) MetricsStorage {
				storage := createTestStorage(t)
				setupTestData(t, storage)
				return storage
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				stats, err := storage.GetStats(ctx)
				if err != nil {
					t.Fatalf("Failed to get stats: %v", err)
				}

				if stats.TotalMetrics == 0 {
					t.Error("Expected non-zero total metrics")
				}
				if stats.TotalPoints == 0 {
					t.Error("Expected non-zero total points")
				}
			},
		},
		"close storage": {
			setupFunc: func(t *testing.T) MetricsStorage {
				return createTestStorage(t)
			},
			testFunc: func(t *testing.T, storage MetricsStorage) {
				ctx := context.Background()
				
				err := storage.Close()
				if err != nil {
					t.Fatalf("Failed to close storage: %v", err)
				}

				// Operations after close should fail
				point := MetricPoint{Timestamp: time.Now(), Value: 1.0}
				err = storage.WriteMetricPoint(ctx, "test", point)
				if err == nil {
					t.Error("Expected error writing to closed storage")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := tc.setupFunc(t)
			defer storage.Close()
			
			tc.testFunc(t, storage)
		})
	}
}

func TestNewMemoryStorage(t *testing.T) {
	// Enable the feature gate for testing
	err := utilfeature.DefaultMutableFeatureGate.Set("TMCMetricsStorage=true")
	if err != nil {
		t.Fatalf("Failed to enable feature gate: %v", err)
	}
	defer func() {
		_ = utilfeature.DefaultMutableFeatureGate.Set("TMCMetricsStorage=false")
	}()

	config := StorageConfig{
		RetentionPolicy: RetentionPolicy{
			MaxAge:    24 * time.Hour,
			MaxPoints: 1000,
		},
	}
	
	storage, err := NewMemoryStorage(config)
	if err != nil {
		t.Fatalf("Failed to create memory storage: %v", err)
	}
	
	if storage == nil {
		t.Fatal("Expected storage instance but got nil")
	}
	
	defer storage.Close()
}

// Helper functions

func createTestStorage(t *testing.T) MetricsStorage {
	config := StorageConfig{
		RetentionPolicy: RetentionPolicy{
			MaxAge:    24 * time.Hour,
			MaxPoints: 10000,
		},
	}
	
	storage, err := NewMemoryStorage(config)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	
	return storage
}

func setupTestData(t *testing.T, storage MetricsStorage) {
	ctx := context.Background()
	now := time.Now()
	
	// Add CPU usage data
	cpuSeries := MetricSeries{
		Name: "cpu.usage",
		Description: "CPU usage percentage",
		Unit: "percent",
		Points: []MetricPoint{
			{Timestamp: now.Add(-2 * time.Hour), Value: 25.0},
			{Timestamp: now.Add(-1 * time.Hour), Value: 50.0},
			{Timestamp: now, Value: 75.0},
		},
		CommonLabels: map[string]string{"host": "server1"},
	}
	
	// Add memory usage data
	memSeries := MetricSeries{
		Name: "memory.usage",
		Description: "Memory usage in bytes",
		Unit: "bytes",
		Points: []MetricPoint{
			{Timestamp: now.Add(-2 * time.Hour), Value: 1000000000},
			{Timestamp: now.Add(-1 * time.Hour), Value: 1500000000},
			{Timestamp: now, Value: 2000000000},
		},
		CommonLabels: map[string]string{"host": "server1"},
	}
	
	if err := storage.WriteMetricSeries(ctx, cpuSeries); err != nil {
		t.Fatalf("Failed to write CPU series: %v", err)
	}
	if err := storage.WriteMetricSeries(ctx, memSeries); err != nil {
		t.Fatalf("Failed to write memory series: %v", err)
	}
}

func setupOldTestData(t *testing.T, storage MetricsStorage) {
	ctx := context.Background()
	now := time.Now()
	
	// Add data points that exceed retention limits
	series := MetricSeries{
		Name: "old.metric",
		Points: make([]MetricPoint, 0, 20),
	}
	
	for i := 0; i < 20; i++ {
		series.Points = append(series.Points, MetricPoint{
			Timestamp: now.Add(time.Duration(-3-i) * time.Hour),
			Value:     float64(i),
		})
	}
	
	if err := storage.WriteMetricSeries(ctx, series); err != nil {
		t.Fatalf("Failed to write old test data: %v", err)
	}
}