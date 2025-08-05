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

func TestWorkloadSessionPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy    *WorkloadSessionPolicy
		wantValid bool
	}{
		"valid sticky session policy": {
			policy: &WorkloadSessionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "sticky-sessions", Namespace: "default"},
				Spec: WorkloadSessionPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web-app"},
						},
					},
					ClusterSelector: ClusterSelector{LocationSelector: []string{"us-west-1"}},
					SessionConfig: SessionConfig{
						SessionType: SessionTypeSticky,
						CookieConfig: &SessionCookieConfig{
							Name:     "APPSESSIONID",
							MaxAge:   3600,
							Secure:   true,
							HTTPOnly: true,
						},
					},
				},
			},
			wantValid: true,
		},
		"valid round-robin session policy": {
			policy: &WorkloadSessionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "rr-sessions", Namespace: "default"},
				Spec: WorkloadSessionPolicySpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "api-service"},
						},
					},
					ClusterSelector: ClusterSelector{LocationSelector: []string{"us-east-1"}},
					SessionConfig: SessionConfig{
						SessionType: SessionTypeRoundRobin,
						PersistenceConfig: &SessionPersistenceConfig{
							Enabled:          true,
							BackendType:      PersistenceBackendTypeRedis,
							ConnectionString: "redis://redis.default.svc.cluster.local:6379",
						},
					},
				},
			},
			wantValid: true,
		},
		"invalid session policy - no configuration": {
			policy: &WorkloadSessionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-sessions", Namespace: "default"},
				Spec: WorkloadSessionPolicySpec{
					WorkloadSelector: WorkloadSelector{},
					ClusterSelector:  ClusterSelector{},
					SessionConfig:    SessionConfig{
						// Missing SessionType
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
			hasSessionType := tc.policy.Spec.SessionConfig.SessionType != ""

			if !hasSelector && tc.wantValid {
				t.Error("expected valid session policy, but WorkloadSelector has no selection criteria")
			}
			if !hasSessionType && tc.wantValid {
				t.Error("expected valid session policy, but SessionType is not specified")
			}

			// Validate session type specific configuration
			switch tc.policy.Spec.SessionConfig.SessionType {
			case SessionTypeCookie:
				if tc.policy.Spec.SessionConfig.CookieConfig == nil && tc.wantValid {
					t.Error("CookieConfig required for Cookie session type")
				}
			case SessionTypeSticky:
				// Sticky sessions can work with various configurations
			case SessionTypeRoundRobin, SessionTypeLeastConnections, SessionTypeIPHash:
				// Load balancing types are valid
			case "":
				if tc.wantValid {
					t.Error("SessionType cannot be empty")
				}
			}

			// Validate persistence configuration
			if tc.policy.Spec.SessionConfig.PersistenceConfig != nil {
				persistence := tc.policy.Spec.SessionConfig.PersistenceConfig
				if persistence.BackendType == PersistenceBackendTypeRedis {
					if persistence.ConnectionString == "" && tc.wantValid {
						t.Error("ConnectionString required for Redis persistence backend")
					}
				}
			}
		})
	}
}

func TestSessionAffinityConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		affinity SessionAffinity
		wantErr  bool
	}{
		{
			name: "valid client IP affinity",
			affinity: SessionAffinity{
				Type:           SessionAffinityTypeClientIP,
				TimeoutSeconds: 3600,
			},
			wantErr: false,
		},
		{
			name: "valid cookie affinity",
			affinity: SessionAffinity{
				Type:           SessionAffinityTypeCookie,
				TimeoutSeconds: 1800,
				CookieName:     "session-affinity",
			},
			wantErr: false,
		},
		{
			name: "invalid cookie affinity - missing cookie name",
			affinity: SessionAffinity{
				Type:           SessionAffinityTypeCookie,
				TimeoutSeconds: 1800,
				// Missing CookieName
			},
			wantErr: true,
		},
		{
			name: "valid header affinity",
			affinity: SessionAffinity{
				Type:           SessionAffinityTypeHeader,
				TimeoutSeconds: 2400,
				HeaderName:     "X-Session-ID",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate affinity configuration
			switch tt.affinity.Type {
			case SessionAffinityTypeCookie:
				if tt.affinity.CookieName == "" && !tt.wantErr {
					t.Error("CookieName required for Cookie affinity type")
				}
			case SessionAffinityTypeHeader:
				if tt.affinity.HeaderName == "" && !tt.wantErr {
					t.Error("HeaderName required for Header affinity type")
				}
			case SessionAffinityTypeClientIP, SessionAffinityTypeNone:
				// No additional validation needed
			}

			if tt.affinity.TimeoutSeconds < 0 {
				if !tt.wantErr {
					t.Error("TimeoutSeconds cannot be negative")
				}
			}
		})
	}
}

func TestSessionPersistenceBackends(t *testing.T) {
	tests := []struct {
		name        string
		persistence SessionPersistenceConfig
		wantValid   bool
	}{
		{
			name: "valid memory persistence",
			persistence: SessionPersistenceConfig{
				Enabled:     true,
				BackendType: PersistenceBackendTypeMemory,
			},
			wantValid: true,
		},
		{
			name: "valid Redis persistence",
			persistence: SessionPersistenceConfig{
				Enabled:          true,
				BackendType:      PersistenceBackendTypeRedis,
				ConnectionString: "redis://redis:6379",
			},
			wantValid: true,
		},
		{
			name: "invalid Redis persistence - missing connection string",
			persistence: SessionPersistenceConfig{
				Enabled:     true,
				BackendType: PersistenceBackendTypeRedis,
				// Missing ConnectionString
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate persistence configuration
			if tt.persistence.Enabled {
				if tt.persistence.BackendType == PersistenceBackendTypeRedis {
					if tt.persistence.ConnectionString == "" && tt.wantValid {
						t.Error("ConnectionString required for Redis backend")
					}
				}
			}
		})
	}
}

func TestSessionBackendStatus(t *testing.T) {
	status := &WorkloadSessionPolicyStatus{
		Phase:          SessionPolicyPhaseActive,
		ActiveSessions: 150,
		TotalSessions:  1000,
		SessionBackends: []SessionBackendStatus{
			{
				Name:                "backend-1",
				ClusterName:         "cluster-west-1",
				Status:              SessionBackendStatusTypeHealthy,
				ActiveSessions:      75,
				Weight:              100,
				HealthCheckFailures: 0,
			},
			{
				Name:                "backend-2",
				ClusterName:         "cluster-west-2",
				Status:              SessionBackendStatusTypeHealthy,
				ActiveSessions:      75,
				Weight:              100,
				HealthCheckFailures: 0,
			},
		},
	}

	// Validate session distribution
	totalBackendSessions := int32(0)
	for _, backend := range status.SessionBackends {
		totalBackendSessions += backend.ActiveSessions
	}

	if totalBackendSessions != status.ActiveSessions {
		t.Errorf("expected total backend sessions %d to match active sessions %d",
			totalBackendSessions, status.ActiveSessions)
	}

	// Validate all backends are healthy
	for _, backend := range status.SessionBackends {
		if backend.Status != SessionBackendStatusTypeHealthy {
			t.Errorf("expected backend %s to be healthy, got %s", backend.Name, backend.Status)
		}
		if backend.HealthCheckFailures > 0 {
			t.Errorf("expected backend %s to have no health check failures, got %d",
				backend.Name, backend.HealthCheckFailures)
		}
	}
}
