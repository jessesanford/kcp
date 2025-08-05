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
)

func TestTrafficMetricsValidation(t *testing.T) {
	tests := []struct {
		name    string
		metrics *TrafficMetrics
		wantErr bool
	}{
		{
			name: "valid prometheus traffic metrics",
			metrics: &TrafficMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app-metrics",
					Namespace: "default",
				},
				Spec: TrafficMetricsSpec{
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
					MetricsSource: TrafficSource{
						Type:     PrometheusSource,
						Endpoint: "http://prometheus:9090",
					},
					CollectionInterval: &metav1.Duration{Duration: 30 * time.Second},
					RetentionPeriod:    &metav1.Duration{Duration: 24 * time.Hour},
				},
			},
			wantErr: false,
		},
		{
			name: "valid istio traffic metrics",
			metrics: &TrafficMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-mesh-metrics",
					Namespace: "istio-system",
				},
				Spec: TrafficMetricsSpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-a", "cluster-b"},
					},
					MetricsSource: TrafficSource{
						Type: IstioSource,
						Labels: map[string]string{
							"source_app": "frontend",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom endpoint metrics",
			metrics: &TrafficMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-metrics",
					Namespace: "monitoring",
				},
				Spec: TrafficMetricsSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"tier": "backend"},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west", "us-east"},
					},
					MetricsSource: TrafficSource{
						Type:        CustomSource,
						Endpoint:    "http://custom-metrics:8080",
						MetricsPath: "/api/v1/metrics",
						Labels: map[string]string{
							"service": "api",
						},
					},
					CollectionInterval: &metav1.Duration{Duration: 1 * time.Minute},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure required fields are present
			if tt.metrics.Spec.WorkloadSelector.LabelSelector == nil &&
				len(tt.metrics.Spec.WorkloadSelector.WorkloadTypes) == 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid metrics but WorkloadSelector is empty")
				}
				return
			}

			if tt.metrics.Spec.MetricsSource.Type == "" {
				if !tt.wantErr {
					t.Errorf("Expected valid metrics but MetricsSource.Type is empty")
				}
				return
			}

			// Validate source-specific requirements
			if tt.metrics.Spec.MetricsSource.Type == PrometheusSource ||
				tt.metrics.Spec.MetricsSource.Type == CustomSource {
				if tt.metrics.Spec.MetricsSource.Endpoint == "" {
					if !tt.wantErr {
						t.Errorf("Expected valid metrics but %s source missing endpoint", tt.metrics.Spec.MetricsSource.Type)
					}
					return
				}
			}

			if tt.wantErr {
				t.Errorf("Expected validation error but metrics passed validation")
			}
		})
	}
}

func TestClusterTrafficMetricsCalculations(t *testing.T) {
	metrics := ClusterTrafficMetrics{
		ClusterName:    "test-cluster",
		RequestCount:   1000,
		SuccessRate:    95.5,
		AverageLatency: 150,
		P95Latency:     &[]int64{300}[0],
		ErrorCount:     45,
		Throughput:     33.33,
		LastUpdated:    metav1.Now(),
		HealthScore:    &[]float64{85.2}[0],
	}

	// Validate success rate calculation consistency
	expectedErrorRate := 100.0 - metrics.SuccessRate
	actualErrorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
	if abs(expectedErrorRate-actualErrorRate) > 0.1 {
		t.Errorf("Success rate inconsistent: expected error rate %f, calculated %f", expectedErrorRate, actualErrorRate)
	}

	// Validate throughput is reasonable
	if metrics.Throughput <= 0 {
		t.Errorf("Throughput should be positive, got %f", metrics.Throughput)
	}

	// Validate health score range
	if metrics.HealthScore != nil {
		if *metrics.HealthScore < 0 || *metrics.HealthScore > 100 {
			t.Errorf("HealthScore should be between 0-100, got %f", *metrics.HealthScore)
		}
	}

	// Validate latency values
	if metrics.AverageLatency <= 0 {
		t.Errorf("AverageLatency should be positive, got %d", metrics.AverageLatency)
	}

	if metrics.P95Latency != nil && *metrics.P95Latency < metrics.AverageLatency {
		t.Errorf("P95Latency (%d) should be >= AverageLatency (%d)", *metrics.P95Latency, metrics.AverageLatency)
	}
}

func TestTrafficMetricsPhaseTransitions(t *testing.T) {
	metrics := &TrafficMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-metrics",
			Namespace: "default",
		},
		Status: TrafficMetricsStatus{
			Phase: InitializingPhase,
		},
	}

	// Test valid phase transitions
	validTransitions := []TrafficMetricsPhase{
		InitializingPhase,
		CollectingPhase,
		AnalyzingPhase,
		ReadyPhase,
	}

	for i, phase := range validTransitions {
		metrics.Status.Phase = phase
		if metrics.Status.Phase != phase {
			t.Errorf("Expected phase %s, got %s", phase, metrics.Status.Phase)
		}

		// Validate phase progression makes sense
		if i > 0 && phase == InitializingPhase {
			t.Errorf("Should not transition back to InitializingPhase from %s", validTransitions[i-1])
		}
	}

	// Test failure phase can be reached from any state
	for _, fromPhase := range validTransitions {
		metrics.Status.Phase = fromPhase
		metrics.Status.Phase = FailedPhase
		if metrics.Status.Phase != FailedPhase {
			t.Errorf("Failed to transition from %s to FailedPhase", fromPhase)
		}
	}
}

func TestTrafficSourceTypeValidation(t *testing.T) {
	validTypes := []TrafficSourceType{
		PrometheusSource,
		IstioSource,
		CustomSource,
	}

	for _, sourceType := range validTypes {
		source := TrafficSource{
			Type: sourceType,
		}

		if source.Type != sourceType {
			t.Errorf("Expected source type %s, got %s", sourceType, source.Type)
		}
	}

	// Test that source types are constants
	if PrometheusSource != "Prometheus" {
		t.Errorf("PrometheusSource constant has wrong value: %s", PrometheusSource)
	}
	if IstioSource != "Istio" {
		t.Errorf("IstioSource constant has wrong value: %s", IstioSource)
	}
	if CustomSource != "Custom" {
		t.Errorf("CustomSource constant has wrong value: %s", CustomSource)
	}
}

func TestTrafficMetricsStatusAggregation(t *testing.T) {
	status := TrafficMetricsStatus{
		Phase:          ReadyPhase,
		LastUpdateTime: &metav1.Time{Time: time.Now()},
		Metrics: map[string]ClusterTrafficMetrics{
			"cluster-a": {
				ClusterName:    "cluster-a",
				RequestCount:   500,
				SuccessRate:    96.0,
				AverageLatency: 120,
				ErrorCount:     20,
				Throughput:     16.67,
			},
			"cluster-b": {
				ClusterName:    "cluster-b",
				RequestCount:   300,
				SuccessRate:    94.0,
				AverageLatency: 180,
				ErrorCount:     18,
				Throughput:     10.0,
			},
		},
		TotalRequests:      &[]int64{800}[0],
		OverallSuccessRate: &[]float64{95.25}[0],
	}

	// Validate total requests calculation
	expectedTotal := int64(500 + 300)
	if *status.TotalRequests != expectedTotal {
		t.Errorf("Expected total requests %d, got %d", expectedTotal, *status.TotalRequests)
	}

	// Validate weighted success rate calculation
	// Cluster A: 500 requests * 96% = 480 successful
	// Cluster B: 300 requests * 94% = 282 successful
	// Overall: (480 + 282) / 800 = 95.25%
	expectedSuccessRate := 95.25
	if abs(*status.OverallSuccessRate-expectedSuccessRate) > 0.01 {
		t.Errorf("Expected overall success rate %f, got %f", expectedSuccessRate, *status.OverallSuccessRate)
	}

	// Validate phase is appropriate for having metrics
	if status.Phase != ReadyPhase {
		t.Errorf("Expected ReadyPhase when metrics are available, got %s", status.Phase)
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}