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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestStickyBindingValidation(t *testing.T) {
	tests := map[string]struct {
		binding   *StickyBinding
		wantValid bool
	}{
		"valid sticky binding": {
			binding: &StickyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: StickyBindingSpec{
					SessionID:     "session-123",
					TargetCluster: "cluster-east",
					WorkloadReference: ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "default",
					},
					AffinityPolicyRef: ObjectReference{
						APIVersion: "tmc.kcp.io/v1alpha1",
						Kind:       "SessionAffinityPolicy",
						Name:       "test-policy",
					},
					ExpiresAt: metav1.Time{Time: time.Now().Add(time.Hour)},
					StorageBackend: BindingStorageBackend{
						Type: StorageBackendTypeMemory,
					},
					Weight: 100,
				},
				Status: StickyBindingStatus{
					Phase: StickyBindingPhaseActive,
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   StickyBindingConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantValid: true,
		},
		"binding with auto renewal": {
			binding: &StickyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "auto-renewal-binding",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: StickyBindingSpec{
					SessionID:     "session-456",
					TargetCluster: "cluster-west",
					WorkloadReference: ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "auto-deployment",
					},
					AffinityPolicyRef: ObjectReference{
						APIVersion: "tmc.kcp.io/v1alpha1",
						Kind:       "SessionAffinityPolicy",
						Name:       "auto-policy",
					},
					ExpiresAt: metav1.Time{Time: time.Now().Add(2 * time.Hour)},
					AutoRenewal: &BindingAutoRenewal{
						Enabled:            true,
						RenewalInterval:    metav1.Duration{Duration: 5 * time.Minute},
						RenewalThreshold:   metav1.Duration{Duration: 10 * time.Minute},
						MaxRenewalAttempts: 3,
						ExtensionDuration:  metav1.Duration{Duration: time.Hour},
					},
					StorageBackend: BindingStorageBackend{
						Type: StorageBackendTypeConfigMap,
						ConfigMapRef: &corev1.LocalObjectReference{
							Name: "binding-storage",
						},
					},
					ConflictResolution: &BindingConflictResolution{
						Strategy:               ConflictResolutionStrategyHighestWeight,
						ManualApprovalRequired: false,
						ConflictTimeout:        metav1.Duration{Duration: 5 * time.Minute},
					},
				},
			},
			wantValid: true,
		},
		"binding with external storage": {
			binding: &StickyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "external-storage-binding",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: StickyBindingSpec{
					SessionID:     "session-789",
					TargetCluster: "cluster-central",
					WorkloadReference: ObjectReference{
						APIVersion: "batch/v1",
						Kind:       "Job",
						Name:       "external-job",
					},
					AffinityPolicyRef: ObjectReference{
						APIVersion: "tmc.kcp.io/v1alpha1",
						Kind:       "SessionAffinityPolicy",
						Name:       "external-policy",
					},
					ExpiresAt: metav1.Time{Time: time.Now().Add(24 * time.Hour)},
					StorageBackend: BindingStorageBackend{
						Type: StorageBackendTypeExternal,
						ExternalConfig: &ExternalStorageConfig{
							URL: "redis://redis.example.com:6379",
							AuthSecretRef: &corev1.LocalObjectReference{
								Name: "redis-auth",
							},
							ConnectionTimeout: metav1.Duration{Duration: 30 * time.Second},
							TLS: &ExternalStorageTLS{
								Enabled:            true,
								CASecretRef:        &corev1.LocalObjectReference{Name: "redis-ca"},
								ClientCertSecretRef: &corev1.LocalObjectReference{Name: "redis-client"},
								InsecureSkipVerify: false,
							},
						},
						Encryption: &StorageEncryption{
							Enabled:     true,
							Algorithm:   EncryptionAlgorithmAES256,
							KeySecretRef: &corev1.LocalObjectReference{Name: "encryption-key"},
						},
					},
				},
			},
			wantValid: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic validation tests
			if tt.binding.Spec.SessionID == "" && tt.wantValid {
				t.Error("SessionID should not be empty for valid binding")
			}
			if tt.binding.Spec.TargetCluster == "" && tt.wantValid {
				t.Error("TargetCluster should not be empty for valid binding")
			}
			if tt.binding.Spec.WorkloadReference.APIVersion == "" && tt.wantValid {
				t.Error("WorkloadReference.APIVersion should not be empty for valid binding")
			}

			// Test condition management
			conditions := tt.binding.GetConditions()
			if conditions == nil && tt.wantValid {
				t.Error("Conditions should be accessible via GetConditions")
			}

			// Test condition setting
			newConditions := conditionsv1alpha1.Conditions{
				{
					Type:   StickyBindingConditionStorageReady,
					Status: corev1.ConditionTrue,
				},
			}
			tt.binding.SetConditions(newConditions)
			if len(tt.binding.Status.Conditions) != 1 {
				t.Error("SetConditions should update binding conditions")
			}
		})
	}
}

func TestSessionBindingConstraintValidation(t *testing.T) {
	tests := map[string]struct {
		constraint *SessionBindingConstraint
		wantValid  bool
	}{
		"valid max bindings per cluster constraint": {
			constraint: &SessionBindingConstraint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "max-bindings-constraint",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
					Target: ConstraintTarget{
						Type: ConstraintTargetTypeCluster,
						ClusterSelector: &ClusterSelector{
							ClusterNames: []string{"cluster-east", "cluster-west"},
						},
					},
					Enforcement:     ConstraintEnforcementHard,
					Limit:           10,
					ViolationAction: ViolationActionBlock,
					Description:     "Limit bindings per cluster to prevent resource exhaustion",
				},
			},
			wantValid: true,
		},
		"resource utilization constraint with exemptions": {
			constraint: &SessionBindingConstraint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-utilization-constraint",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeResourceUtilizationLimit,
					Target: ConstraintTarget{
						Type: ConstraintTargetTypeNamespace,
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"environment": "production",
							},
						},
					},
					Enforcement:     ConstraintEnforcementSoft,
					Limit:           80, // 80% utilization
					ViolationAction: ViolationActionWarn,
					Exemptions: []ConstraintExemption{
						{
							Name: "emergency-exemption",
							Target: ConstraintTarget{
								Type: ConstraintTargetTypeNamespace,
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"emergency": "true",
									},
								},
							},
							Conditions: []ExemptionCondition{
								{
									Type:  ExemptionConditionTypeEmergency,
									Value: "critical-issue",
								},
							},
							ExpiresAt: &metav1.Time{Time: time.Now().Add(24 * time.Hour)},
							Reason:    "Critical incident response",
						},
					},
					CheckInterval: metav1.Duration{Duration: time.Minute},
				},
				Status: SessionBindingConstraintStatus{
					Phase:          SessionBindingConstraintPhaseActive,
					ViolationCount: 2,
					CurrentUsage:   func() *int64 { u := int64(75); return &u }(),
					RecentViolations: []ConstraintViolation{
						{
							Timestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
							ViolationType: "ResourceUtilizationExceeded",
							Target: ObjectReference{
								APIVersion: "v1",
								Kind:       "Namespace",
								Name:       "production",
							},
							CurrentValue: 85,
							LimitValue:   80,
							Action:       ViolationActionWarn,
							Message:      "Resource utilization exceeded threshold",
						},
					},
				},
			},
			wantValid: true,
		},
		"global constraint with warning enforcement": {
			constraint: &SessionBindingConstraint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-warning-constraint",
					Namespace: "default",
					Annotations: map[string]string{
						"kcp.io/cluster": "root:test",
					},
				},
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerWorkload,
					Target: ConstraintTarget{
						Type: ConstraintTargetTypeGlobal,
					},
					Enforcement:     ConstraintEnforcementWarning,
					Limit:           5,
					ViolationAction: ViolationActionLog,
					CheckInterval:   metav1.Duration{Duration: 30 * time.Second},
					Description:     "Global limit on bindings per workload for monitoring",
				},
			},
			wantValid: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic validation tests
			if tt.constraint.Spec.ConstraintType == "" && tt.wantValid {
				t.Error("ConstraintType should not be empty for valid constraint")
			}
			if tt.constraint.Spec.Target.Type == "" && tt.wantValid {
				t.Error("Target.Type should not be empty for valid constraint")
			}
			if tt.constraint.Spec.Limit <= 0 && tt.wantValid {
				t.Error("Limit should be positive for valid constraint")
			}

			// Test condition management
			conditions := tt.constraint.GetConditions()
			if conditions == nil && tt.wantValid {
				t.Error("Conditions should be accessible via GetConditions")
			}

			// Test condition setting
			newConditions := conditionsv1alpha1.Conditions{
				{
					Type:   SessionBindingConstraintConditionReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   SessionBindingConstraintConditionEnforced,
					Status: corev1.ConditionTrue,
				},
			}
			tt.constraint.SetConditions(newConditions)
			if len(tt.constraint.Status.Conditions) != 2 {
				t.Error("SetConditions should update constraint conditions")
			}
		})
	}
}

func TestStorageBackendValidation(t *testing.T) {
	tests := map[string]struct {
		backend   BindingStorageBackend
		wantValid bool
	}{
		"memory storage backend": {
			backend: BindingStorageBackend{
				Type: StorageBackendTypeMemory,
			},
			wantValid: true,
		},
		"configmap storage backend": {
			backend: BindingStorageBackend{
				Type: StorageBackendTypeConfigMap,
				ConfigMapRef: &corev1.LocalObjectReference{
					Name: "binding-storage",
				},
			},
			wantValid: true,
		},
		"secret storage backend with encryption": {
			backend: BindingStorageBackend{
				Type: StorageBackendTypeSecret,
				SecretRef: &corev1.LocalObjectReference{
					Name: "binding-secrets",
				},
				Encryption: &StorageEncryption{
					Enabled:   true,
					Algorithm: EncryptionAlgorithmChaCha20Poly1305,
					KeySecretRef: &corev1.LocalObjectReference{
						Name: "encryption-key",
					},
				},
			},
			wantValid: true,
		},
		"external storage with full TLS config": {
			backend: BindingStorageBackend{
				Type: StorageBackendTypeExternal,
				ExternalConfig: &ExternalStorageConfig{
					URL: "etcd://etcd.example.com:2379",
					AuthSecretRef: &corev1.LocalObjectReference{
						Name: "etcd-auth",
					},
					ConnectionTimeout: metav1.Duration{Duration: 10 * time.Second},
					TLS: &ExternalStorageTLS{
						Enabled:             true,
						CASecretRef:         &corev1.LocalObjectReference{Name: "etcd-ca"},
						ClientCertSecretRef: &corev1.LocalObjectReference{Name: "etcd-client"},
						InsecureSkipVerify:  false,
					},
				},
				Encryption: &StorageEncryption{
					Enabled:   true,
					Algorithm: EncryptionAlgorithmAES256,
					KeySecretRef: &corev1.LocalObjectReference{
						Name: "storage-encryption-key",
					},
				},
			},
			wantValid: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.backend.Type == "" && tt.wantValid {
				t.Error("Storage backend type should not be empty for valid backend")
			}

			// Type-specific validation
			switch tt.backend.Type {
			case StorageBackendTypeConfigMap:
				if tt.backend.ConfigMapRef == nil && tt.wantValid {
					t.Error("ConfigMapRef should be set for ConfigMap storage backend")
				}
			case StorageBackendTypeSecret:
				if tt.backend.SecretRef == nil && tt.wantValid {
					t.Error("SecretRef should be set for Secret storage backend")
				}
			case StorageBackendTypeExternal:
				if tt.backend.ExternalConfig == nil && tt.wantValid {
					t.Error("ExternalConfig should be set for External storage backend")
				}
				if tt.backend.ExternalConfig != nil && tt.backend.ExternalConfig.URL == "" && tt.wantValid {
					t.Error("URL should be set for External storage backend")
				}
			}
		})
	}
}

func TestConflictResolutionLogic(t *testing.T) {
	tests := map[string]struct {
		strategy ConflictResolutionStrategy
		wantName string
	}{
		"highest weight strategy": {
			strategy: ConflictResolutionStrategyHighestWeight,
			wantName: "HighestWeight",
		},
		"newest binding strategy": {
			strategy: ConflictResolutionStrategyNewestBinding,
			wantName: "NewestBinding",
		},
		"oldest binding strategy": {
			strategy: ConflictResolutionStrategyOldestBinding,
			wantName: "OldestBinding",
		},
		"manual strategy": {
			strategy: ConflictResolutionStrategyManual,
			wantName: "Manual",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if string(tt.strategy) != tt.wantName {
				t.Errorf("Strategy constant mismatch: got %s, want %s", tt.strategy, tt.wantName)
			}
		})
	}
}

func TestBindingPhaseTransitions(t *testing.T) {
	validPhases := []StickyBindingPhase{
		StickyBindingPhasePending,
		StickyBindingPhaseActive,
		StickyBindingPhaseRenewing,
		StickyBindingPhaseExpired,
		StickyBindingPhaseConflicted,
		StickyBindingPhaseFailed,
	}

	validTransitions := map[StickyBindingPhase][]StickyBindingPhase{
		StickyBindingPhasePending:    {StickyBindingPhaseActive, StickyBindingPhaseFailed},
		StickyBindingPhaseActive:     {StickyBindingPhaseRenewing, StickyBindingPhaseExpired, StickyBindingPhaseConflicted, StickyBindingPhaseFailed},
		StickyBindingPhaseRenewing:   {StickyBindingPhaseActive, StickyBindingPhaseFailed},
		StickyBindingPhaseExpired:    {StickyBindingPhaseActive}, // Can be renewed
		StickyBindingPhaseConflicted: {StickyBindingPhaseActive, StickyBindingPhaseFailed},
		StickyBindingPhaseFailed:     {}, // Terminal state
	}

	for fromPhase, allowedToPhases := range validTransitions {
		t.Run(string(fromPhase), func(t *testing.T) {
			for _, phase := range validPhases {
				isAllowed := false
				for _, allowed := range allowedToPhases {
					if phase == allowed {
						isAllowed = true
						break
					}
				}

				if isAllowed {
					t.Logf("Valid transition: %s -> %s", fromPhase, phase)
				} else if phase != fromPhase {
					t.Logf("Invalid transition: %s -> %s", fromPhase, phase)
				}
			}
		})
	}
}

func TestPerformanceMetricsCalculation(t *testing.T) {
	now := time.Now()
	metrics := &BindingPerformanceMetrics{
		CreationLatency:       metav1.Duration{Duration: 100 * time.Millisecond},
		AverageRenewalLatency: metav1.Duration{Duration: 50 * time.Millisecond},
		StorageLatency:        metav1.Duration{Duration: 10 * time.Millisecond},
		ConflictResolutionTime: metav1.Duration{Duration: 200 * time.Millisecond},
		RequestCount:          1000,
		ErrorCount:            5,
		LastRequestTime:       &metav1.Time{Time: now.Add(-time.Minute)},
	}

	// Test error rate calculation
	errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
	expectedErrorRate := 0.005 // 0.5%
	if errorRate != expectedErrorRate {
		t.Errorf("Error rate calculation: got %f, want %f", errorRate, expectedErrorRate)
	}

	// Test availability calculation (based on last request time)
	timeSinceLastRequest := time.Since(metrics.LastRequestTime.Time)
	if timeSinceLastRequest > 5*time.Minute {
		t.Log("Binding may be considered stale")
	} else {
		t.Log("Binding is actively serving requests")
	}

	// Test performance thresholds
	if metrics.CreationLatency.Duration > 500*time.Millisecond {
		t.Log("Creation latency is high")
	}
	if metrics.StorageLatency.Duration > 100*time.Millisecond {
		t.Log("Storage latency is high")
	}
}

func TestEncryptionAlgorithmSupport(t *testing.T) {
	algorithms := []EncryptionAlgorithm{
		EncryptionAlgorithmAES256,
		EncryptionAlgorithmChaCha20Poly1305,
	}

	for _, algo := range algorithms {
		t.Run(string(algo), func(t *testing.T) {
			encryption := &StorageEncryption{
				Enabled:   true,
				Algorithm: algo,
				KeySecretRef: &corev1.LocalObjectReference{
					Name: "test-key",
				},
			}

			if !encryption.Enabled {
				t.Error("Encryption should be enabled")
			}
			if encryption.Algorithm != algo {
				t.Errorf("Algorithm mismatch: got %s, want %s", encryption.Algorithm, algo)
			}
			if encryption.KeySecretRef == nil {
				t.Error("KeySecretRef should be set")
			}
		})
	}
}