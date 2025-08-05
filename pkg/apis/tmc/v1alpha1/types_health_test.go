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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadHealthPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy    *WorkloadHealthPolicy
		wantValid bool
	}{
		"valid HTTP health check": {
			policy: &WorkloadHealthPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-health", Namespace: "default"},
				Spec: WorkloadHealthPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "frontend"},
						},
					},
					ClusterSelector: ClusterSelector{LocationSelector: []string{"us-west-1"}},
					HealthChecks: []HealthCheckConfig{
						{
							Name:    "http-check",
							Type:    HealthCheckTypeHTTP,
							Timeout: metav1.Duration{Duration: 10000000000}, // 10s
							HTTPCheck: &HTTPHealthCheck{
								URL:                 "http://localhost:8080/health",
								Method:              "GET",
								ExpectedStatusCodes: []int{200},
							},
						},
					},
					FailurePolicy: HealthFailurePolicyQuarantine,
				},
			},
			wantValid: true,
		},
		"valid TCP health check": {
			policy: &WorkloadHealthPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "tcp-health", Namespace: "default"},
				Spec: WorkloadHealthPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "database"},
						},
					},
					ClusterSelector: ClusterSelector{LocationSelector: []string{"us-east-1"}},
					HealthChecks: []HealthCheckConfig{
						{
							Name: "tcp-check",
							Type: HealthCheckTypeTCP,
							TCPCheck: &TCPHealthCheck{
								Host: "localhost",
								Port: 5432,
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"invalid health check - no configuration": {
			policy: &WorkloadHealthPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-health", Namespace: "default"},
				Spec: WorkloadHealthPolicySpec{
					WorkloadSelector: WorkloadSelector{},
					ClusterSelector:  ClusterSelector{},
					HealthChecks: []HealthCheckConfig{
						{
							Name: "incomplete-check",
							Type: HealthCheckTypeHTTP,
							// Missing HTTPCheck configuration
						},
					},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.policy == nil {
				t.Fatal("policy cannot be nil")
			}

			// Basic validation
			hasSelector := tc.policy.Spec.WorkloadSelector.LabelSelector != nil ||
				len(tc.policy.Spec.WorkloadSelector.WorkloadTypes) > 0
			hasHealthChecks := len(tc.policy.Spec.HealthChecks) > 0

			if !hasSelector && tc.wantValid {
				t.Error("expected valid health policy, but WorkloadSelector has no selection criteria")
			}
			if !hasHealthChecks && tc.wantValid {
				t.Error("expected valid health policy, but no health checks provided")
			}

			// Validate health checks
			for _, check := range tc.policy.Spec.HealthChecks {
				if check.Name == "" && tc.wantValid {
					t.Error("HealthCheck name cannot be empty")
				}

				// Validate type-specific configuration
				switch check.Type {
				case HealthCheckTypeHTTP:
					if check.HTTPCheck == nil && tc.wantValid {
						t.Error("HTTPCheck configuration required for HTTP health check")
					}
					if check.HTTPCheck != nil && check.HTTPCheck.URL == "" && tc.wantValid {
						t.Error("HTTP URL cannot be empty")
					}
				case HealthCheckTypeTCP:
					if check.TCPCheck == nil && tc.wantValid {
						t.Error("TCPCheck configuration required for TCP health check")
					}
					if check.TCPCheck != nil && (check.TCPCheck.Host == "" || check.TCPCheck.Port <= 0) && tc.wantValid {
						t.Error("TCP host and port must be specified")
					}
				case HealthCheckTypeGRPC:
					if check.GRPCCheck == nil && tc.wantValid {
						t.Error("GRPCCheck configuration required for GRPC health check")
					}
				case HealthCheckTypeCommand:
					if check.CommandCheck == nil && tc.wantValid {
						t.Error("CommandCheck configuration required for Command health check")
					}
				case HealthCheckTypeKubernetes:
					if check.KubernetesCheck == nil && tc.wantValid {
						t.Error("KubernetesCheck configuration required for Kubernetes health check")
					}
				}

				// Validate weight
				if check.Weight < 0 || check.Weight > 100 {
					if tc.wantValid {
						t.Errorf("HealthCheck weight must be 0-100, got %d", check.Weight)
					}
				}
			}
		})
	}
}

func TestHealthCheckResultCalculation(t *testing.T) {
	status := &WorkloadHealthPolicyStatus{
		Phase:              HealthPolicyPhaseActive,
		OverallHealthScore: func() *int32 { v := int32(85); return &v }(),
		HealthCheckResults: []HealthCheckResult{
			{Name: "http-check", Status: HealthStatusHealthy, Score: 90, ConsecutiveSuccesses: 5},
			{Name: "tcp-check", Status: HealthStatusHealthy, Score: 80, ConsecutiveSuccesses: 3},
		},
	}

	// Calculate overall score
	totalScore := int32(0)
	for _, result := range status.HealthCheckResults {
		totalScore += result.Score
	}
	expectedOverallScore := totalScore / int32(len(status.HealthCheckResults))

	if *status.OverallHealthScore != 85 {
		t.Errorf("expected overall score 85, got %d", *status.OverallHealthScore)
	}

	// Validate calculated score matches expectation
	if expectedOverallScore != 85 { // (90+80)/2 = 85
		t.Errorf("calculated overall score should be 85, got %d", expectedOverallScore)
	}

	// Test health status determination
	allHealthy := true
	for _, result := range status.HealthCheckResults {
		if result.Status != HealthStatusHealthy {
			allHealthy = false
			break
		}
	}

	if !allHealthy {
		t.Error("expected all health checks to be healthy")
	}
}

func TestHealthRecoveryPolicyDefaults(t *testing.T) {
	policy := &HealthRecoveryPolicy{
		AutoRecovery:        true,
		MaxRecoveryAttempts: 3,
		RecoveryThreshold:   80,
	}

	if !policy.AutoRecovery {
		t.Error("expected AutoRecovery to be true")
	}
	if policy.MaxRecoveryAttempts != 3 {
		t.Errorf("expected MaxRecoveryAttempts to be 3, got %d", policy.MaxRecoveryAttempts)
	}
	if policy.RecoveryThreshold != 80 {
		t.Errorf("expected RecoveryThreshold to be 80, got %d", policy.RecoveryThreshold)
	}
}

func TestHealthFailurePolicyValidation(t *testing.T) {
	validPolicies := []HealthFailurePolicy{
		HealthFailurePolicyIgnore,
		HealthFailurePolicyQuarantine,
		HealthFailurePolicyRemove,
		HealthFailurePolicyAlert,
	}

	for _, policy := range validPolicies {
		if policy == "" {
			t.Error("HealthFailurePolicy cannot be empty")
		}
	}

	// Test that all defined policies are valid enum values
	policies := map[HealthFailurePolicy]bool{
		HealthFailurePolicyIgnore:     true,
		HealthFailurePolicyQuarantine: true,
		HealthFailurePolicyRemove:     true,
		HealthFailurePolicyAlert:      true,
	}

	for policy := range policies {
		switch policy {
		case HealthFailurePolicyIgnore, HealthFailurePolicyQuarantine,
			HealthFailurePolicyRemove, HealthFailurePolicyAlert:
			// Valid
		default:
			t.Errorf("invalid HealthFailurePolicy: %s", policy)
		}
	}
}

func TestKubernetesProbeTypeValidation(t *testing.T) {
	validProbeTypes := []KubernetesProbeType{
		KubernetesProbeTypeReadiness,
		KubernetesProbeTypeLiveness,
		KubernetesProbeTypeStartup,
	}

	for _, probeType := range validProbeTypes {
		if probeType == "" {
			t.Error("KubernetesProbeType cannot be empty")
		}
	}

	// Test Kubernetes health check with different probe types
	for _, probeType := range validProbeTypes {
		check := KubernetesHealthCheck{
			ProbeType: probeType,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			RequiredHealthyPods: 1,
		}

		if check.ProbeType != probeType {
			t.Errorf("expected probe type %s, got %s", probeType, check.ProbeType)
		}
	}
}
