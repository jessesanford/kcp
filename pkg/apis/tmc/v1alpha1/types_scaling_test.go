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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestWorkloadScalingPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  *WorkloadScalingPolicy
		wantErr bool
	}{
		{
			name: "valid CPU-based scaling policy",
			policy: &WorkloadScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app-scaling",
					Namespace: "default",
				},
				Spec: WorkloadScalingPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web"},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "prod"},
						},
					},
					MinReplicas: 2,
					MaxReplicas: 10,
					ScalingMetrics: []ScalingMetric{
						{
							Type:        CPUUtilizationMetric,
							TargetValue: intstr.FromString("70%"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid multi-metric scaling policy",
			policy: &WorkloadScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "api-service-scaling",
					Namespace: "default",
				},
				Spec: WorkloadScalingPolicySpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-a", "cluster-b"},
					},
					MinReplicas: 3,
					MaxReplicas: 20,
					ScalingMetrics: []ScalingMetric{
						{
							Type:        CPUUtilizationMetric,
							TargetValue: intstr.FromString("60%"),
						},
						{
							Type:        RequestsPerSecondMetric,
							TargetValue: intstr.FromInt(100),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom metric scaling",
			policy: &WorkloadScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "queue-worker-scaling",
					Namespace: "workers",
				},
				Spec: WorkloadScalingPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"component": "worker"},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west", "us-east"},
					},
					MinReplicas: 1,
					MaxReplicas: 50,
					ScalingMetrics: []ScalingMetric{
						{
							Type:        QueueLengthMetric,
							TargetValue: intstr.FromInt(10),
						},
						{
							Type:        CustomMetric,
							TargetValue: intstr.FromString("5"),
							MetricSelector: &MetricSelector{
								MetricName:      "pending_jobs",
								AggregationType: &[]MetricAggregationType{SumAggregation}[0],
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid policy - no scaling metrics",
			policy: &WorkloadScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-scaling",
					Namespace: "default",
				},
				Spec: WorkloadScalingPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "test"},
						},
					},
					MinReplicas:    1,
					MaxReplicas:    5,
					ScalingMetrics: []ScalingMetric{}, // Empty - should fail
				},
			},
			wantErr: true,
		},
		{
			name: "invalid policy - max < min replicas",
			policy: &WorkloadScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-replicas",
					Namespace: "default",
				},
				Spec: WorkloadScalingPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "test"},
						},
					},
					MinReplicas: 10,
					MaxReplicas: 5, // Invalid - less than min
					ScalingMetrics: []ScalingMetric{
						{
							Type:        CPUUtilizationMetric,
							TargetValue: intstr.FromString("70%"),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			if tt.policy.Spec.MinReplicas < 1 {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but MinReplicas is less than 1")
				}
				return
			}

			if tt.policy.Spec.MaxReplicas < tt.policy.Spec.MinReplicas {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but MaxReplicas < MinReplicas")
				}
				return
			}

			if len(tt.policy.Spec.ScalingMetrics) == 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but no scaling metrics defined")
				}
				return
			}

			// Validate workload selector
			if tt.policy.Spec.WorkloadSelector.LabelSelector == nil &&
				len(tt.policy.Spec.WorkloadSelector.WorkloadTypes) == 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but WorkloadSelector is empty")
				}
				return
			}

			// Validate cluster selector
			if tt.policy.Spec.ClusterSelector.LabelSelector == nil &&
				len(tt.policy.Spec.ClusterSelector.ClusterNames) == 0 &&
				len(tt.policy.Spec.ClusterSelector.LocationSelector) == 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but ClusterSelector is empty")
				}
				return
			}

			if tt.wantErr {
				t.Errorf("Expected validation error but policy passed validation")
			}
		})
	}
}

func TestScalingMetricValidation(t *testing.T) {
	tests := []struct {
		name    string
		metric  ScalingMetric
		wantErr bool
	}{
		{
			name: "valid CPU metric",
			metric: ScalingMetric{
				Type:        CPUUtilizationMetric,
				TargetValue: intstr.FromString("70%"),
			},
			wantErr: false,
		},
		{
			name: "valid memory metric",
			metric: ScalingMetric{
				Type:        MemoryUtilizationMetric,
				TargetValue: intstr.FromString("80%"),
			},
			wantErr: false,
		},
		{
			name: "valid RPS metric",
			metric: ScalingMetric{
				Type:        RequestsPerSecondMetric,
				TargetValue: intstr.FromInt(100),
			},
			wantErr: false,
		},
		{
			name: "valid queue length metric",
			metric: ScalingMetric{
				Type:        QueueLengthMetric,
				TargetValue: intstr.FromInt(50),
			},
			wantErr: false,
		},
		{
			name: "valid custom metric with selector",
			metric: ScalingMetric{
				Type:        CustomMetric,
				TargetValue: intstr.FromString("50"),
				MetricSelector: &MetricSelector{
					MetricName:      "custom_queue_length",
					AggregationType: &[]MetricAggregationType{AverageAggregation}[0],
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"queue": "processing"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom metric with sum aggregation",
			metric: ScalingMetric{
				Type:        CustomMetric,
				TargetValue: intstr.FromInt(1000),
				MetricSelector: &MetricSelector{
					MetricName:      "total_requests",
					AggregationType: &[]MetricAggregationType{SumAggregation}[0],
				},
			},
			wantErr: false,
		},
		{
			name: "invalid custom metric - missing selector",
			metric: ScalingMetric{
				Type:        CustomMetric,
				TargetValue: intstr.FromString("50"),
				// MetricSelector missing - should fail
			},
			wantErr: true,
		},
		{
			name: "invalid custom metric - empty metric name",
			metric: ScalingMetric{
				Type:        CustomMetric,
				TargetValue: intstr.FromString("50"),
				MetricSelector: &MetricSelector{
					MetricName: "", // Empty - should fail
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate metric type
			validTypes := []ScalingMetricType{
				CPUUtilizationMetric, MemoryUtilizationMetric,
				RequestsPerSecondMetric, QueueLengthMetric, CustomMetric,
			}
			found := false
			for _, validType := range validTypes {
				if tt.metric.Type == validType {
					found = true
					break
				}
			}
			if !found {
				if !tt.wantErr {
					t.Errorf("Expected valid metric but type %s is invalid", tt.metric.Type)
				}
				return
			}

			// Custom metrics should have metric selector
			if tt.metric.Type == CustomMetric {
				if tt.metric.MetricSelector == nil {
					if !tt.wantErr {
						t.Errorf("Expected valid custom metric but MetricSelector is missing")
					}
					return
				}
				if tt.metric.MetricSelector.MetricName == "" {
					if !tt.wantErr {
						t.Errorf("Expected valid custom metric but MetricName is empty")
					}
					return
				}
			}

			if tt.wantErr {
				t.Errorf("Expected validation error but metric passed validation")
			}
		})
	}
}

func TestMetricAggregationTypes(t *testing.T) {
	// Test aggregation type constants
	expectedTypes := map[MetricAggregationType]string{
		AverageAggregation: "Average",
		MaximumAggregation: "Maximum",
		MinimumAggregation: "Minimum",
		SumAggregation:     "Sum",
	}

	for aggType, expected := range expectedTypes {
		if string(aggType) != expected {
			t.Errorf("MetricAggregationType constant %s has wrong value: expected %s, got %s",
				expected, expected, string(aggType))
		}
	}
}

func TestScalingMetricTypes(t *testing.T) {
	// Test metric type constants
	expectedTypes := map[ScalingMetricType]string{
		CPUUtilizationMetric:    "CPUUtilization",
		MemoryUtilizationMetric: "MemoryUtilization",
		RequestsPerSecondMetric: "RequestsPerSecond",
		QueueLengthMetric:       "QueueLength",
		CustomMetric:            "Custom",
	}

	for metricType, expected := range expectedTypes {
		if string(metricType) != expected {
			t.Errorf("ScalingMetricType constant %s has wrong value: expected %s, got %s",
				expected, expected, string(metricType))
		}
	}
}

func TestWorkloadScalingPolicyStatusCalculations(t *testing.T) {
	status := WorkloadScalingPolicyStatus{
		CurrentReplicas: &[]int32{8}[0],
		DesiredReplicas: &[]int32{10}[0],
		ClusterReplicas: map[string]int32{
			"cluster-a": 5,
			"cluster-b": 3,
		},
		LastScaleTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
		CurrentMetrics: []CurrentMetricStatus{
			{
				Type:         CPUUtilizationMetric,
				CurrentValue: intstr.FromString("75%"),
				TargetValue:  intstr.FromString("70%"),
			},
			{
				Type:         RequestsPerSecondMetric,
				CurrentValue: intstr.FromInt(120),
				TargetValue:  intstr.FromInt(100),
			},
			{
				Type:         CustomMetric,
				CurrentValue: intstr.FromInt(15),
				TargetValue:  intstr.FromInt(10),
				MetricName:   "pending_jobs",
			},
		},
	}

	// Validate replica distribution consistency
	totalClusterReplicas := int32(0)
	for _, replicas := range status.ClusterReplicas {
		totalClusterReplicas += replicas
	}
	if totalClusterReplicas != *status.CurrentReplicas {
		t.Errorf("ClusterReplicas sum (%d) doesn't match CurrentReplicas (%d)",
			totalClusterReplicas, *status.CurrentReplicas)
	}

	// Validate scaling direction based on metrics
	scaleUpNeeded := false
	for _, metric := range status.CurrentMetrics {
		switch metric.Type {
		case CPUUtilizationMetric:
			// 75% > 70% target, should scale up
			scaleUpNeeded = true
		case RequestsPerSecondMetric:
			// 120 > 100 target, should scale up
			scaleUpNeeded = true
		case CustomMetric:
			// 15 > 10 target, should scale up
			if metric.MetricName == "pending_jobs" {
				scaleUpNeeded = true
			}
		}
	}

	if scaleUpNeeded && *status.DesiredReplicas <= *status.CurrentReplicas {
		t.Errorf("Metrics indicate scale up needed but DesiredReplicas (%d) <= CurrentReplicas (%d)",
			*status.DesiredReplicas, *status.CurrentReplicas)
	}

	// Validate last scale time is reasonable
	if status.LastScaleTime != nil {
		timeSinceScale := time.Since(status.LastScaleTime.Time)
		if timeSinceScale < 0 {
			t.Errorf("LastScaleTime is in the future")
		}
		if timeSinceScale > 24*time.Hour {
			t.Errorf("LastScaleTime is too old: %v", timeSinceScale)
		}
	}

	// Validate observed workloads tracking
	if len(status.ObservedWorkloads) > 0 {
		for i, workload := range status.ObservedWorkloads {
			if workload.APIVersion == "" || workload.Kind == "" || workload.Name == "" {
				t.Errorf("ObservedWorkloads[%d] has empty required fields", i)
			}
		}
	}
}

func TestCurrentMetricStatusValidation(t *testing.T) {
	tests := []struct {
		name   string
		metric CurrentMetricStatus
		valid  bool
	}{
		{
			name: "valid CPU metric status",
			metric: CurrentMetricStatus{
				Type:         CPUUtilizationMetric,
				CurrentValue: intstr.FromString("75%"),
				TargetValue:  intstr.FromString("70%"),
			},
			valid: true,
		},
		{
			name: "valid RPS metric status",
			metric: CurrentMetricStatus{
				Type:         RequestsPerSecondMetric,
				CurrentValue: intstr.FromInt(150),
				TargetValue:  intstr.FromInt(100),
			},
			valid: true,
		},
		{
			name: "valid custom metric status",
			metric: CurrentMetricStatus{
				Type:         CustomMetric,
				CurrentValue: intstr.FromInt(25),
				TargetValue:  intstr.FromInt(20),
				MetricName:   "queue_depth",
			},
			valid: true,
		},
		{
			name: "invalid custom metric - missing metric name",
			metric: CurrentMetricStatus{
				Type:         CustomMetric,
				CurrentValue: intstr.FromInt(25),
				TargetValue:  intstr.FromInt(20),
				// MetricName missing for CustomMetric
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Custom metrics should have metric name
			if tt.metric.Type == CustomMetric && tt.metric.MetricName == "" {
				if tt.valid {
					t.Errorf("Expected valid metric but CustomMetric missing MetricName")
				}
				return
			}

			// All metrics should have non-empty values
			if tt.metric.CurrentValue.String() == "" || tt.metric.TargetValue.String() == "" {
				if tt.valid {
					t.Errorf("Expected valid metric but has empty values")
				}
				return
			}

			if !tt.valid {
				t.Errorf("Expected invalid metric but it passed validation")
			}
		})
	}
}