/*
Copyright The KCP Authors.

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

package aggregator

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/unstructured"

	"github.com/kcp-dev/kcp/pkg/status/interfaces"
)

func TestAggregator_LatestWins(t *testing.T) {
	tests := map[string]struct {
		updates []*interfaces.StatusUpdate
		want    string
	}{
		"single update": {
			updates: []*interfaces.StatusUpdate{
				{
					Source:    "source1",
					Timestamp: time.Now(),
					Status: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Running",
							},
						},
					},
				},
			},
			want: "Running",
		},
		"latest wins": {
			updates: []*interfaces.StatusUpdate{
				{
					Source:    "source1",
					Timestamp: time.Now().Add(-time.Minute),
					Status: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Pending",
							},
						},
					},
				},
				{
					Source:    "source2",
					Timestamp: time.Now(),
					Status: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Running",
							},
						},
					},
				},
			},
			want: "Running",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			aggregator := NewAggregator(AggregatorConfig{})
			
			result, err := aggregator.AggregateStatus(context.TODO(), tc.updates, interfaces.AggregationStrategyLatestWins)
			if err != nil {
				t.Fatalf("AggregateStatus failed: %v", err)
			}

			phase, found, err := unstructured.NestedString(result.Status.Object, "status", "phase")
			if err != nil {
				t.Fatalf("Failed to get phase: %v", err)
			}
			if !found {
				t.Fatal("Phase not found in result")
			}
			if phase != tc.want {
				t.Errorf("Expected phase %q, got %q", tc.want, phase)
			}
		})
	}
}

func TestAggregator_SourcePriority(t *testing.T) {
	config := AggregatorConfig{
		SourcePriorities: map[string]int{
			"high-priority": 100,
			"low-priority":  10,
		},
	}
	aggregator := NewAggregator(config)

	updates := []*interfaces.StatusUpdate{
		{
			Source:    "low-priority",
			Timestamp: time.Now(),
			Status: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "LowPriority",
					},
				},
			},
		},
		{
			Source:    "high-priority",
			Timestamp: time.Now().Add(-time.Minute), // Older timestamp
			Status: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "HighPriority",
					},
				},
			},
		},
	}

	result, err := aggregator.AggregateStatus(context.TODO(), updates, interfaces.AggregationStrategySourcePriority)
	if err != nil {
		t.Fatalf("AggregateStatus failed: %v", err)
	}

	phase, found, err := unstructured.NestedString(result.Status.Object, "status", "phase")
	if err != nil {
		t.Fatalf("Failed to get phase: %v", err)
	}
	if !found {
		t.Fatal("Phase not found in result")
	}
	if phase != "HighPriority" {
		t.Errorf("Expected phase 'HighPriority', got %q", phase)
	}
}

func TestAggregator_DefaultStrategy(t *testing.T) {
	aggregator := NewAggregator(AggregatorConfig{})

	gvr := schema.GroupVersionResource{Group: "test", Version: "v1", Resource: "testresources"}
	
	// Test default strategy (should be LatestWins)
	defaultStrategy := aggregator.GetDefaultStrategy(gvr)
	if defaultStrategy != interfaces.AggregationStrategyLatestWins {
		t.Errorf("Expected default strategy %q, got %q", interfaces.AggregationStrategyLatestWins, defaultStrategy)
	}

	// Set custom strategy
	aggregator.SetDefaultStrategy(gvr, interfaces.AggregationStrategySourcePriority)
	
	customStrategy := aggregator.GetDefaultStrategy(gvr)
	if customStrategy != interfaces.AggregationStrategySourcePriority {
		t.Errorf("Expected custom strategy %q, got %q", interfaces.AggregationStrategySourcePriority, customStrategy)
	}
}