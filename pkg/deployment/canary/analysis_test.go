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

package canary

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/metrics"
)

func TestMetricsAnalyzer_AnalyzeMetrics(t *testing.T) {
	registry := metrics.NewMetricsRegistry(true)
	config := &CanaryConfiguration{
		MetricsRegistry:         registry,
		DefaultAnalysisInterval: time.Minute,
		DefaultSuccessThreshold: 95,
	}

	analyzer, err := NewMetricsAnalyzer(config)
	if err != nil {
		t.Fatalf("Failed to create metrics analyzer: %v", err)
	}

	tests := map[string]struct {
		canary          *deploymentv1alpha1.CanaryDeployment
		expectResults   int
		expectAllPassed bool
	}{
		"canary with metric queries": {
			canary: &deploymentv1alpha1.CanaryDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-canary",
					Namespace: "default",
				},
				Spec: deploymentv1alpha1.CanaryDeploymentSpec{
					CanaryVersion: "v2.0.0",
					StableVersion: "v1.0.0",
					Analysis: deploymentv1alpha1.CanaryAnalysis{
						MetricQueries: []deploymentv1alpha1.MetricQuery{
							{
								Name:          "error_rate",
								Query:         "error_rate",
								ThresholdType: deploymentv1alpha1.ThresholdTypeLessThan,
								Threshold:     5.0,
								Weight:        ptr.To(20),
							},
							{
								Name:          "latency_p99",
								Query:         "latency_p99",
								ThresholdType: deploymentv1alpha1.ThresholdTypeLessThan,
								Threshold:     200.0,
								Weight:        ptr.To(15),
							},
						},
					},
				},
			},
			expectResults:   2,
			expectAllPassed: true,
		},
		"canary without metric queries": {
			canary: &deploymentv1alpha1.CanaryDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-canary-no-queries",
					Namespace: "default",
				},
				Spec: deploymentv1alpha1.CanaryDeploymentSpec{
					CanaryVersion: "v2.0.0",
					StableVersion: "v1.0.0",
					Analysis: deploymentv1alpha1.CanaryAnalysis{
						MetricQueries: []deploymentv1alpha1.MetricQuery{},
					},
				},
			},
			expectResults:   3, // Default metrics
			expectAllPassed: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.TODO()
			results, err := analyzer.AnalyzeMetrics(ctx, tc.canary)
			if err != nil {
				t.Errorf("AnalyzeMetrics failed: %v", err)
				return
			}

			if len(results) != tc.expectResults {
				t.Errorf("Expected %d results, got %d", tc.expectResults, len(results))
			}

			if tc.expectAllPassed {
				for _, result := range results {
					if !result.Passed {
						t.Errorf("Expected result %s to pass, but it failed", result.MetricName)
					}
				}
			}

			// Verify result structure
			for _, result := range results {
				if result.MetricName == "" {
					t.Error("Result missing metric name")
				}
				if result.Weight <= 0 {
					t.Error("Result has invalid weight")
				}
				if result.Timestamp.IsZero() {
					t.Error("Result missing timestamp")
				}
			}
		})
	}
}

func TestMetricsAnalyzer_GetHealthScore(t *testing.T) {
	registry := metrics.NewMetricsRegistry(true)
	config := &CanaryConfiguration{
		MetricsRegistry: registry,
	}

	analyzer, err := NewMetricsAnalyzer(config)
	if err != nil {
		t.Fatalf("Failed to create metrics analyzer: %v", err)
	}

	tests := map[string]struct {
		canary      *deploymentv1alpha1.CanaryDeployment
		expectScore float64
		expectError bool
	}{
		"healthy canary": {
			canary: &deploymentv1alpha1.CanaryDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "healthy-canary",
					Namespace: "default",
				},
				Spec: deploymentv1alpha1.CanaryDeploymentSpec{
					CanaryVersion: "v2.0.0",
					StableVersion: "v1.0.0",
					Analysis: deploymentv1alpha1.CanaryAnalysis{
						MetricQueries: []deploymentv1alpha1.MetricQuery{
							{
								Name:          "error_rate",
								Query:         "error_rate",
								ThresholdType: deploymentv1alpha1.ThresholdTypeLessThan,
								Threshold:     5.0,
								Weight:        ptr.To(10),
							},
						},
					},
				},
			},
			expectScore: 100.0, // Should pass with simulated values
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.TODO()
			score, err := analyzer.GetHealthScore(ctx, tc.canary)

			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
				return
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tc.expectError {
				if score < 0 || score > 100 {
					t.Errorf("Invalid health score: %f (should be 0-100)", score)
				}
				if score != tc.expectScore {
					t.Errorf("Expected health score %f, got %f", tc.expectScore, score)
				}
			}
		})
	}
}

func TestEvaluateThreshold(t *testing.T) {
	analyzer := &metricsAnalyzer{}

	tests := map[string]struct {
		value         float64
		thresholdType deploymentv1alpha1.ThresholdType
		threshold     float64
		expectPassed  bool
	}{
		"less than - passes": {
			value:         3.0,
			thresholdType: deploymentv1alpha1.ThresholdTypeLessThan,
			threshold:     5.0,
			expectPassed:  true,
		},
		"less than - fails": {
			value:         7.0,
			thresholdType: deploymentv1alpha1.ThresholdTypeLessThan,
			threshold:     5.0,
			expectPassed:  false,
		},
		"greater than - passes": {
			value:         10.0,
			thresholdType: deploymentv1alpha1.ThresholdTypeGreaterThan,
			threshold:     5.0,
			expectPassed:  true,
		},
		"greater than - fails": {
			value:         3.0,
			thresholdType: deploymentv1alpha1.ThresholdTypeGreaterThan,
			threshold:     5.0,
			expectPassed:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := analyzer.evaluateThreshold(tc.value, tc.thresholdType, tc.threshold)
			if result != tc.expectPassed {
				t.Errorf("Expected threshold evaluation to be %t, got %t", tc.expectPassed, result)
			}
		})
	}
}

func TestGetMetricWeight(t *testing.T) {
	tests := map[string]struct {
		query        deploymentv1alpha1.MetricQuery
		expectWeight int
	}{
		"query with weight": {
			query: deploymentv1alpha1.MetricQuery{
				Name:   "test",
				Weight: ptr.To(25),
			},
			expectWeight: 25,
		},
		"query without weight": {
			query: deploymentv1alpha1.MetricQuery{
				Name: "test",
			},
			expectWeight: 10, // Default weight
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			weight := getMetricWeight(tc.query)
			if weight != tc.expectWeight {
				t.Errorf("Expected weight %d, got %d", tc.expectWeight, weight)
			}
		})
	}
}

func TestBuildQueryLabels(t *testing.T) {
	analyzer := &metricsAnalyzer{}
	canary := &deploymentv1alpha1.CanaryDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-canary",
			Namespace: "test-namespace",
		},
		Spec: deploymentv1alpha1.CanaryDeploymentSpec{
			CanaryVersion: "v2.0.0",
			StableVersion: "v1.0.0",
			TargetRef: corev1.ObjectReference{
				Name:      "test-deployment",
				Namespace: "test-namespace",
			},
		},
	}

	labels := analyzer.buildQueryLabels(canary)

	expectedLabels := map[string]string{
		"canary_name":    "test-canary",
		"canary_version": "v2.0.0",
		"stable_version": "v1.0.0",
		"deployment":     "test-deployment",
		"namespace":      "test-namespace",
	}

	for key, expectedValue := range expectedLabels {
		if value, exists := labels[key]; !exists {
			t.Errorf("Expected label %s not found", key)
		} else if value != expectedValue {
			t.Errorf("Expected label %s to have value %s, got %s", key, expectedValue, value)
		}
	}
}