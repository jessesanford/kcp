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
					ScalingBehavior: &ScalingBehavior{
						ScaleUp: &ScalingDirection{
							StabilizationWindowSeconds: &[]int32{30}[0],
							Policies: []ScalingPolicy{
								{
									Type:          PodsScalingPolicy,
									Value:         2,
									PeriodSeconds: 60,
								},
							},
						},
						ScaleDown: &ScalingDirection{
							StabilizationWindowSeconds: &[]int32{300}[0],
							Policies: []ScalingPolicy{
								{
									Type:          PercentScalingPolicy,
									Value:         10,
									PeriodSeconds: 60,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom metric scaling with distribution",
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
					ClusterDistribution: &ClusterDistributionPolicy{
						Strategy: WeightedDistribution,
						Preferences: []ClusterPreference{
							{ClusterName: "cluster-a", Weight: 3},
							{ClusterName: "cluster-b", Weight: 1},
						},
						MinReplicasPerCluster: &[]int32{1}[0],
						MaxReplicasPerCluster: &[]int32{30}[0],
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			if tt.policy.Spec.MinReplicas < 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid policy but MinReplicas is negative")
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
			name: "valid RPS metric",
			metric: ScalingMetric{
				Type:        RequestsPerSecondMetric,
				TargetValue: intstr.FromInt(100),
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
				if tt.metric.MetricSelector == nil || tt.metric.MetricSelector.MetricName == "" {
					if !tt.wantErr {
						t.Errorf("Expected valid custom metric but MetricSelector is missing")
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

func TestClusterDistributionPolicyValidation(t *testing.T) {
	tests := []struct {
		name   string
		policy ClusterDistributionPolicy
		valid  bool
	}{
		{
			name: "valid even distribution",
			policy: ClusterDistributionPolicy{
				Strategy: EvenDistribution,
			},
			valid: true,
		},
		{
			name: "valid weighted distribution",
			policy: ClusterDistributionPolicy{
				Strategy: WeightedDistribution,
				Preferences: []ClusterPreference{
					{ClusterName: "cluster-a", Weight: 2},
					{ClusterName: "cluster-b", Weight: 1},
				},
			},
			valid: true,
		},
		{
			name: "valid preferred distribution with limits",
			policy: ClusterDistributionPolicy{
				Strategy: PreferredDistribution,
				Preferences: []ClusterPreference{
					{ClusterName: "primary", Weight: 10},
					{ClusterName: "secondary", Weight: 5},
				},
				MinReplicasPerCluster: &[]int32{1}[0],
				MaxReplicasPerCluster: &[]int32{10}[0],
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate strategy
			validStrategies := []DistributionStrategy{
				EvenDistribution, WeightedDistribution, PreferredDistribution,
			}
			found := false
			for _, strategy := range validStrategies {
				if tt.policy.Strategy == strategy {
					found = true
					break
				}
			}
			if !found {
				if tt.valid {
					t.Errorf("Expected valid policy but strategy %s is invalid", tt.policy.Strategy)
				}
				return
			}

			// Weighted and preferred distribution should have preferences
			if tt.policy.Strategy == WeightedDistribution || tt.policy.Strategy == PreferredDistribution {
				if len(tt.policy.Preferences) == 0 {
					if tt.valid {
						t.Errorf("Expected valid policy but %s strategy has no preferences", tt.policy.Strategy)
					}
					return
				}
			}

			// Validate replica limits
			if tt.policy.MinReplicasPerCluster != nil && tt.policy.MaxReplicasPerCluster != nil {
				if *tt.policy.MinReplicasPerCluster > *tt.policy.MaxReplicasPerCluster {
					if tt.valid {
						t.Errorf("Expected valid policy but MinReplicasPerCluster > MaxReplicasPerCluster")
					}
					return
				}
			}

			if !tt.valid {
				t.Errorf("Expected invalid policy but it passed validation")
			}
		})
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
		if metric.Type == CPUUtilizationMetric {
			// 75% > 70% target, should scale up
			scaleUpNeeded = true
		}
		if metric.Type == RequestsPerSecondMetric {
			// 120 > 100 target, should scale up
			scaleUpNeeded = true
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
}

func TestScalingPolicyTypeValidation(t *testing.T) {
	// Test policy type constants
	if PodsScalingPolicy != "Pods" {
		t.Errorf("PodsScalingPolicy constant has wrong value: %s", PodsScalingPolicy)
	}
	if PercentScalingPolicy != "Percent" {
		t.Errorf("PercentScalingPolicy constant has wrong value: %s", PercentScalingPolicy)
	}

	// Test policy select constants
	if MaxPolicySelect != "Max" {
		t.Errorf("MaxPolicySelect constant has wrong value: %s", MaxPolicySelect)
	}
	if MinPolicySelect != "Min" {
		t.Errorf("MinPolicySelect constant has wrong value: %s", MinPolicySelect)
	}
	if DisabledPolicySelect != "Disabled" {
		t.Errorf("DisabledPolicySelect constant has wrong value: %s", DisabledPolicySelect)
	}
}

func TestScalingBehaviorValidation(t *testing.T) {
	behavior := ScalingBehavior{
		ScaleUp: &ScalingDirection{
			StabilizationWindowSeconds: &[]int32{60}[0],
			SelectPolicy:               &[]ScalingPolicySelect{MaxPolicySelect}[0],
			Policies: []ScalingPolicy{
				{
					Type:          PodsScalingPolicy,
					Value:         4,
					PeriodSeconds: 60,
				},
				{
					Type:          PercentScalingPolicy,
					Value:         100,
					PeriodSeconds: 60,
				},
			},
		},
		ScaleDown: &ScalingDirection{
			StabilizationWindowSeconds: &[]int32{300}[0],
			SelectPolicy:               &[]ScalingPolicySelect{MinPolicySelect}[0],
			Policies: []ScalingPolicy{
				{
					Type:          PercentScalingPolicy,
					Value:         10,
					PeriodSeconds: 60,
				},
			},
		},
	}

	// Validate scale up policies
	if behavior.ScaleUp != nil {
		if len(behavior.ScaleUp.Policies) == 0 {
			t.Errorf("ScaleUp has no policies defined")
		}
		for _, policy := range behavior.ScaleUp.Policies {
			if policy.Value <= 0 {
				t.Errorf("ScaleUp policy has invalid value: %d", policy.Value)
			}
			if policy.PeriodSeconds <= 0 {
				t.Errorf("ScaleUp policy has invalid period: %d", policy.PeriodSeconds)
			}
		}
	}

	// Validate scale down policies
	if behavior.ScaleDown != nil {
		if len(behavior.ScaleDown.Policies) == 0 {
			t.Errorf("ScaleDown has no policies defined")
		}
		for _, policy := range behavior.ScaleDown.Policies {
			if policy.Value <= 0 {
				t.Errorf("ScaleDown policy has invalid value: %d", policy.Value)
			}
			if policy.PeriodSeconds <= 0 {
				t.Errorf("ScaleDown policy has invalid period: %d", policy.PeriodSeconds)
			}
		}
	}
}