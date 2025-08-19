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

package syncer

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

func TestNewSyncerClient_Basic(t *testing.T) {
	tests := map[string]struct {
		config      *rest.Config
		virtualURL  string
		expectError bool
		errorMsg    string
	}{
		"valid configuration": {
			config: &rest.Config{
				Host: "https://kcp-server.example.com",
			},
			virtualURL:  "https://kcp-syncer.example.com",
			expectError: false,
		},
		"nil config": {
			config:      nil,
			virtualURL:  "https://kcp-syncer.example.com",
			expectError: true,
			errorMsg:    "rest config is required",
		},
		"empty virtual URL": {
			config: &rest.Config{
				Host: "https://kcp-server.example.com",
			},
			virtualURL:  "",
			expectError: true,
			errorMsg:    "virtual URL is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client, err := NewSyncerClient(tc.config, tc.virtualURL)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tc.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if client == nil {
				t.Error("expected non-nil client")
				return
			}

			if client.virtualURL != tc.virtualURL {
				t.Errorf("expected virtualURL=%q, got %q", tc.virtualURL, client.virtualURL)
			}

			if client.resyncPeriod != 30*time.Second {
				t.Errorf("expected default resyncPeriod=30s, got %v", client.resyncPeriod)
			}
		})
	}
}

func TestSyncerClient_SetResyncPeriod_Basic(t *testing.T) {
	config := &rest.Config{Host: "https://test.example.com"}
	client, err := NewSyncerClient(config, "https://virtual.example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := map[string]struct {
		period   time.Duration
		expected time.Duration
	}{
		"normal period": {
			period:   time.Minute,
			expected: time.Minute,
		},
		"too short period - gets clamped": {
			period:   time.Second,
			expected: 10 * time.Second,
		},
		"minimum period": {
			period:   10 * time.Second,
			expected: 10 * time.Second,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client.SetResyncPeriod(tc.period)
			
			if client.resyncPeriod != tc.expected {
				t.Errorf("expected period=%v, got %v", tc.expected, client.resyncPeriod)
			}
		})
	}
}

func TestSyncerClient_checkTargetHealth_Basic(t *testing.T) {
	config := &rest.Config{Host: "https://test.example.com"}
	client, err := NewSyncerClient(config, "https://virtual.example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := map[string]struct {
		target          *workloadv1alpha1.SyncTarget
		expectHealthy   bool
		expectError     bool
	}{
		"healthy target with recent heartbeat": {
			target: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "healthy"},
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services", "configmaps", "secrets"},
				},
				Status: workloadv1alpha1.SyncTargetStatus{
					LastHeartbeat: &metav1.Time{Time: time.Now().Add(-30 * time.Second)},
				},
			},
			expectHealthy: true,
		},
		"unhealthy target with stale heartbeat": {
			target: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "stale"},
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services", "configmaps", "secrets"},
				},
				Status: workloadv1alpha1.SyncTargetStatus{
					LastHeartbeat: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
			},
			expectHealthy: false,
		},
		"unhealthy target with no heartbeat": {
			target: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "no-heartbeat"},
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services", "configmaps", "secrets"},
				},
			},
			expectHealthy: false,
		},
		"unhealthy target with missing required resource types": {
			target: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "incomplete"},
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods"}, // Missing other required types
				},
				Status: workloadv1alpha1.SyncTargetStatus{
					LastHeartbeat: &metav1.Time{Time: time.Now().Add(-30 * time.Second)},
				},
			},
			expectHealthy: false,
			expectError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			healthy, err := client.checkTargetHealth(ctx, tc.target)

			if tc.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if healthy != tc.expectHealthy {
				t.Errorf("expected healthy=%v, got %v", tc.expectHealthy, healthy)
			}
		})
	}
}

func TestSyncerClient_validateTargetConfiguration_Basic(t *testing.T) {
	config := &rest.Config{Host: "https://test.example.com"}
	client, err := NewSyncerClient(config, "https://virtual.example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	tests := map[string]struct {
		target      *workloadv1alpha1.SyncTarget
		expectError bool
		errorMsg    string
	}{
		"valid configuration": {
			target: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services", "configmaps", "secrets", "deployments"},
				},
			},
			expectError: false,
		},
		"missing required resource type": {
			target: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services"}, // Missing configmaps and secrets
				},
			},
			expectError: true,
			errorMsg:    "missing required resource types",
		},
		"no supported resource types": {
			target: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{},
			},
			expectError: true,
			errorMsg:    "no supported resource types",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := client.validateTargetConfiguration(tc.target)

			if tc.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tc.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSyncerClient_GetSyncTargets_Basic(t *testing.T) {
	config := &rest.Config{Host: "https://test.example.com"}
	client, err := NewSyncerClient(config, "https://virtual.example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	syncTargets := client.GetSyncTargets()
	if syncTargets == nil {
		t.Error("expected non-nil SyncTargets interface")
	}
}