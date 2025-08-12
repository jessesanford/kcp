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

package collector

import (
	"context"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewCollector(t *testing.T) {
	tests := map[string]struct {
		options CollectorOptions
		wantErr bool
	}{
		"default options": {
			options: CollectorOptions{},
			wantErr: false,
		},
		"custom options": {
			options: CollectorOptions{
				CollectionInterval: 1 * time.Minute,
				MaxDataPoints:      500,
				EnableMetrics:      false,
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			collector, err := NewCollector(tc.options)
			if tc.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if !tc.wantErr && collector == nil {
				t.Error("Expected collector, got nil")
			}
		})
	}
}

func TestCollectPlacementData(t *testing.T) {
	collector, err := NewCollector(CollectorOptions{})
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	ctx := context.Background()
	clusterName := logicalcluster.Name("test-cluster")
	workspaceName := "test-workspace"
	placementName := "test-placement"
	placementNamespace := "test-namespace"

	err = collector.CollectPlacementData(ctx, clusterName, workspaceName, placementName, placementNamespace)
	if err != nil {
		t.Errorf("Failed to collect placement data: %v", err)
	}

	// Verify data was stored
	if collector.dataStore.Size() != 1 {
		t.Errorf("Expected 1 data point, got %d", collector.dataStore.Size())
	}
}

func TestDataStoreAdd(t *testing.T) {
	ds := &DataStore{
		data:    make([]PlacementData, 0),
		maxSize: 3,
	}

	// Add data points
	for i := 0; i < 5; i++ {
		data := PlacementData{
			Timestamp:     time.Now().Add(time.Duration(i) * time.Second),
			ClusterName:   logicalcluster.Name("cluster"),
			WorkspaceName: "workspace",
			PlacementName: "placement",
		}
		ds.Add(data)
	}

	// Should only keep the last 3 due to maxSize
	if ds.Size() != 3 {
		t.Errorf("Expected size 3, got %d", ds.Size())
	}
}

func TestMetricsCollector(t *testing.T) {
	mc, err := NewMetricsCollector("test_namespace")
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	data := PlacementData{
		Timestamp:     time.Now(),
		ClusterName:   logicalcluster.Name("test-cluster"),
		WorkspaceName: "test-workspace",
		PlacementName: "test-placement",
		HealthStatus:  "Healthy",
	}

	// Should not panic
	mc.RecordPlacementData(data)
	mc.RecordCollectionDuration("collect", 0.5)
	mc.RecordCollectionError("collect", "timeout")
	mc.UpdateDataStoreSize(10)

	err = mc.Close()
	if err != nil {
		t.Errorf("Failed to close metrics collector: %v", err)
	}
}