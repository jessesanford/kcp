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
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TestSyncTargetBasicTypes tests the basic type structure
func TestSyncTargetBasicTypes(t *testing.T) {
	tests := []struct {
		name     string
		validate func(*testing.T)
	}{
		{
			name: "SyncTarget should have required TypeMeta fields",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					TypeMeta: metav1.TypeMeta{
						Kind:       "SyncTarget",
						APIVersion: "workload.kcp.io/v1alpha1",
					},
				}
				if st.Kind != "SyncTarget" {
					t.Errorf("Expected Kind to be SyncTarget, got %s", st.Kind)
				}
				if st.APIVersion != "workload.kcp.io/v1alpha1" {
					t.Errorf("Expected APIVersion to be workload.kcp.io/v1alpha1, got %s", st.APIVersion)
				}
			},
		},
		{
			name: "SyncTarget should have ObjectMeta",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-target",
						Namespace: "", // Cluster-scoped
					},
				}
				if st.Name != "test-target" {
					t.Errorf("Expected Name to be test-target, got %s", st.Name)
				}
				if st.Namespace != "" {
					t.Errorf("Expected empty Namespace for cluster-scoped resource, got %s", st.Namespace)
				}
			},
		},
		{
			name: "SyncTarget should have spec and status",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					Spec:   SyncTargetSpec{},
					Status: SyncTargetStatus{},
				}
				// These should be accessible without nil pointer panics
				_ = st.Spec
				_ = st.Status
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.validate)
	}
}

// TestSyncTargetSpecValidation tests comprehensive spec validation
func TestSyncTargetSpecValidation(t *testing.T) {
	tests := []struct {
		name        string
		target      *SyncTarget
		wantErrs    bool
		expectedErr string
	}{
		{
			name: "valid minimal SyncTarget",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
				},
			},
			wantErrs: false,
		},
		{
			name: "empty cells should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{},
				},
			},
			wantErrs:    true,
			expectedErr: "at least one cell is required",
		},
		{
			name: "nil cells should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: nil,
				},
			},
			wantErrs:    true,
			expectedErr: "at least one cell is required",
		},
		{
			name: "duplicate cell names should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{
						{Name: "cell1"},
						{Name: "cell1"}, // Duplicate
					},
				},
			},
			wantErrs:    true,
			expectedErr: "Duplicate value",
		},
		{
			name: "invalid cell name should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "INVALID_CELL_NAME!"}},
				},
			},
			wantErrs:    true,
			expectedErr: "cell name must be a valid DNS label",
		},
		{
			name: "empty cell name should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: ""}},
				},
			},
			wantErrs:    true,
			expectedErr: "cell name is required",
		},
		{
			name: "negative evictAfter should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells:      []Cell{{Name: "test-cell"}},
					EvictAfter: &metav1.Duration{Duration: -time.Hour},
				},
			},
			wantErrs:    true,
			expectedErr: "evictAfter duration must be non-negative",
		},
		{
			name: "valid connection",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Connection: &SyncTargetConnection{
						URL:        "https://cluster1.example.com:6443",
						ServerName: "cluster1.example.com",
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "invalid connection URL should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Connection: &SyncTargetConnection{
						URL: "not-a-valid-url",
					},
				},
			},
			wantErrs:    true,
			expectedErr: "URL must have scheme and host",
		},
		{
			name: "empty connection URL should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Connection: &SyncTargetConnection{
						URL: "",
					},
				},
			},
			wantErrs:    true,
			expectedErr: "connection URL is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateSyncTarget(test.target)
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestCellValidation tests cell-specific validation
func TestCellValidation(t *testing.T) {
	tests := []struct {
		name        string
		cell        Cell
		wantErrs    bool
		expectedErr string
	}{
		{
			name: "valid cell",
			cell: Cell{
				Name: "test-cell",
				Labels: map[string]string{
					"zone": "us-west-1a",
				},
			},
			wantErrs: false,
		},
		{
			name: "cell with taints",
			cell: Cell{
				Name: "tainted-cell",
				Taints: []Taint{
					{
						Key:    "node-role",
						Value:  "master",
						Effect: TaintEffectNoSchedule,
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "invalid taint key",
			cell: Cell{
				Name: "test-cell",
				Taints: []Taint{
					{
						Key:    "invalid@key!",
						Effect: TaintEffectNoSchedule,
					},
				},
			},
			wantErrs:    true,
			expectedErr: "taint key is invalid",
		},
		{
			name: "empty taint key",
			cell: Cell{
				Name: "test-cell",
				Taints: []Taint{
					{
						Key:    "",
						Effect: TaintEffectNoSchedule,
					},
				},
			},
			wantErrs:    true,
			expectedErr: "taint key is required",
		},
		{
			name: "invalid taint effect",
			cell: Cell{
				Name: "test-cell",
				Taints: []Taint{
					{
						Key:    "test-key",
						Effect: TaintEffect("InvalidEffect"),
					},
				},
			},
			wantErrs:    true,
			expectedErr: "Unsupported value",
		},
		{
			name: "empty taint effect",
			cell: Cell{
				Name: "test-cell",
				Taints: []Taint{
					{
						Key:    "test-key",
						Effect: "",
					},
				},
			},
			wantErrs:    true,
			expectedErr: "taint effect is required",
		},
		{
			name: "invalid label key",
			cell: Cell{
				Name: "test-cell",
				Labels: map[string]string{
					"invalid@label!": "value",
				},
			},
			wantErrs:    true,
			expectedErr: "label key is invalid",
		},
		{
			name: "invalid label value",
			cell: Cell{
				Name: "test-cell",
				Labels: map[string]string{
					"valid-key": strings.Repeat("x", 64), // Too long
				},
			},
			wantErrs:    true,
			expectedErr: "label value is invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{test.cell},
				},
			}

			errs := ValidateSyncTarget(target)
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestTaintEffects tests all taint effects
func TestTaintEffects(t *testing.T) {
	effects := []TaintEffect{
		TaintEffectNoSchedule,
		TaintEffectPreferNoSchedule,
		TaintEffectNoExecute,
	}

	for _, effect := range effects {
		t.Run(string(effect), func(t *testing.T) {
			cell := Cell{
				Name: "test-cell",
				Taints: []Taint{
					{
						Key:    "test-key",
						Value:  "test-value",
						Effect: effect,
					},
				},
			}

			target := &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{cell},
				},
			}

			errs := ValidateSyncTarget(target)
			if len(errs) > 0 {
				t.Errorf("Valid taint effect %s should not produce errors: %v", effect, errs)
			}
		})
	}
}

// TestCredentialsValidation tests credentials validation
func TestCredentialsValidation(t *testing.T) {
	tests := []struct {
		name        string
		credentials *SyncTargetCredentials
		wantErrs    bool
		expectedErr string
	}{
		{
			name: "valid token credentials",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				Token: &TokenCredentials{
					Value: "test-token-value",
				},
			},
			wantErrs: false,
		},
		{
			name: "valid certificate credentials",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				Certificate: &CertificateCredentials{
					ClientCert: []byte("cert-data"),
					ClientKey:  []byte("key-data"),
				},
			},
			wantErrs: false,
		},
		{
			name: "valid service account credentials",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				ServiceAccount: &ServiceAccountCredentials{
					Namespace: "test-namespace",
					Name:      "test-service-account",
				},
			},
			wantErrs: false,
		},
		{
			name: "empty auth type",
			credentials: &SyncTargetCredentials{
				Type: "",
			},
			wantErrs:    true,
			expectedErr: "authentication type is required",
		},
		{
			name: "invalid auth type",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthType("invalid"),
			},
			wantErrs:    true,
			expectedErr: "Unsupported value",
		},
		{
			name: "token auth without token credentials",
			credentials: &SyncTargetCredentials{
				Type:  SyncTargetAuthTypeToken,
				Token: nil,
			},
			wantErrs:    true,
			expectedErr: "token credentials are required for token auth",
		},
		{
			name: "empty token value",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				Token: &TokenCredentials{
					Value: "",
				},
			},
			wantErrs:    true,
			expectedErr: "token value is required",
		},
		{
			name: "certificate auth without certificate credentials",
			credentials: &SyncTargetCredentials{
				Type:        SyncTargetAuthTypeCertificate,
				Certificate: nil,
			},
			wantErrs:    true,
			expectedErr: "certificate credentials are required for certificate auth",
		},
		{
			name: "empty client cert",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				Certificate: &CertificateCredentials{
					ClientCert: nil,
					ClientKey:  []byte("key-data"),
				},
			},
			wantErrs:    true,
			expectedErr: "client certificate is required",
		},
		{
			name: "empty client key",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				Certificate: &CertificateCredentials{
					ClientCert: []byte("cert-data"),
					ClientKey:  nil,
				},
			},
			wantErrs:    true,
			expectedErr: "client key is required",
		},
		{
			name: "service account auth without service account credentials",
			credentials: &SyncTargetCredentials{
				Type:           SyncTargetAuthTypeServiceAccount,
				ServiceAccount: nil,
			},
			wantErrs:    true,
			expectedErr: "service account credentials are required for service account auth",
		},
		{
			name: "empty service account namespace",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				ServiceAccount: &ServiceAccountCredentials{
					Namespace: "",
					Name:      "test-sa",
				},
			},
			wantErrs:    true,
			expectedErr: "service account namespace is required",
		},
		{
			name: "empty service account name",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				ServiceAccount: &ServiceAccountCredentials{
					Namespace: "test-ns",
					Name:      "",
				},
			},
			wantErrs:    true,
			expectedErr: "service account name is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells:       []Cell{{Name: "test-cell"}},
					Credentials: test.credentials,
				},
			}

			errs := ValidateSyncTarget(target)
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestCapabilitiesValidation tests capabilities validation
func TestCapabilitiesValidation(t *testing.T) {
	tests := []struct {
		name         string
		capabilities *SyncTargetCapabilities
		wantErrs     bool
		expectedErr  string
	}{
		{
			name: "valid capabilities",
			capabilities: &SyncTargetCapabilities{
				MaxWorkloads: &[]int32{100}[0],
				Features:     []string{"feature1", "feature2"},
				SupportedResourceTypes: []ResourceTypeSupport{
					{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Supported: true,
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "negative max workloads",
			capabilities: &SyncTargetCapabilities{
				MaxWorkloads: &[]int32{-1}[0],
			},
			wantErrs:    true,
			expectedErr: "maxWorkloads must be non-negative",
		},
		{
			name: "empty feature name",
			capabilities: &SyncTargetCapabilities{
				Features: []string{"valid-feature", ""},
			},
			wantErrs:    true,
			expectedErr: "feature name cannot be empty",
		},
		{
			name: "empty resource version",
			capabilities: &SyncTargetCapabilities{
				SupportedResourceTypes: []ResourceTypeSupport{
					{
						Group:   "apps",
						Version: "",
						Kind:    "Deployment",
					},
				},
			},
			wantErrs:    true,
			expectedErr: "version is required",
		},
		{
			name: "empty resource kind",
			capabilities: &SyncTargetCapabilities{
				SupportedResourceTypes: []ResourceTypeSupport{
					{
						Group:   "apps",
						Version: "v1",
						Kind:    "",
					},
				},
			},
			wantErrs:    true,
			expectedErr: "kind is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells:        []Cell{{Name: "test-cell"}},
					Capabilities: test.capabilities,
				},
			}

			errs := ValidateSyncTarget(target)
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestAPIExportReferenceValidation tests API export reference validation
func TestAPIExportReferenceValidation(t *testing.T) {
	tests := []struct {
		name        string
		apiExport   APIExportReference
		wantErrs    bool
		expectedErr string
	}{
		{
			name: "valid api export reference",
			apiExport: APIExportReference{
				Workspace: "root:org:workspace",
				Name:      "test-export",
			},
			wantErrs: false,
		},
		{
			name: "empty workspace",
			apiExport: APIExportReference{
				Workspace: "",
				Name:      "test-export",
			},
			wantErrs:    true,
			expectedErr: "workspace is required",
		},
		{
			name: "invalid workspace",
			apiExport: APIExportReference{
				Workspace: "invalid@workspace!",
				Name:      "test-export",
			},
			wantErrs:    true,
			expectedErr: "workspace must be a valid logical cluster path",
		},
		{
			name: "empty name",
			apiExport: APIExportReference{
				Workspace: "root:org:workspace",
				Name:      "",
			},
			wantErrs:    true,
			expectedErr: "APIExport name is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells:               []Cell{{Name: "test-cell"}},
					SupportedAPIExports: []APIExportReference{test.apiExport},
				},
			}

			errs := ValidateSyncTarget(target)
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestStatusValidation tests status validation
func TestStatusValidation(t *testing.T) {
	tests := []struct {
		name        string
		status      SyncTargetStatus
		wantErrs    bool
		expectedErr string
	}{
		{
			name: "valid status with virtual workspace",
			status: SyncTargetStatus{
				VirtualWorkspaces: []VirtualWorkspace{
					{URL: "https://proxy.example.com/clusters/test-target"},
				},
				SyncerIdentity: "test-syncer",
			},
			wantErrs: false,
		},
		{
			name: "empty virtual workspace URL",
			status: SyncTargetStatus{
				VirtualWorkspaces: []VirtualWorkspace{
					{URL: ""},
				},
			},
			wantErrs:    true,
			expectedErr: "virtual workspace URL is required",
		},
		{
			name: "invalid virtual workspace URL",
			status: SyncTargetStatus{
				VirtualWorkspaces: []VirtualWorkspace{
					{URL: "not-a-valid-url"},
				},
			},
			wantErrs:    true,
			expectedErr: "virtual workspace URL is invalid",
		},
		{
			name: "invalid syncer identity",
			status: SyncTargetStatus{
				SyncerIdentity: "INVALID_SYNCER!",
			},
			wantErrs:    true,
			expectedErr: "syncer identity must be a valid DNS subdomain",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateSyncTargetStatus(&test.status, field.NewPath("status"))
			hasErrs := len(errs) > 0

			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
				return
			}

			if test.wantErrs && test.expectedErr != "" {
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), test.expectedErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got errors: %v", test.expectedErr, errs)
				}
			}
		})
	}
}

// TestConditionHelpers tests all condition helper methods
func TestConditionHelpers(t *testing.T) {
	t.Run("SetReadyCondition", func(t *testing.T) {
		st := &SyncTarget{}
		st.SetReadyCondition(metav1.ConditionTrue, SyncTargetReasonReady, "Target is ready")

		if !st.IsReady() {
			t.Error("Expected IsReady to return true after setting Ready condition to True")
		}

		condition := st.GetCondition(SyncTargetConditionReady)
		if condition == nil {
			t.Fatal("Expected to find Ready condition")
		}

		if condition.Status != metav1.ConditionTrue {
			t.Errorf("Expected condition status to be True, got %s", condition.Status)
		}
		if condition.Reason != SyncTargetReasonReady {
			t.Errorf("Expected condition reason to be %s, got %s", SyncTargetReasonReady, condition.Reason)
		}
		if condition.Message != "Target is ready" {
			t.Errorf("Expected condition message to be 'Target is ready', got %s", condition.Message)
		}
		if condition.Type != SyncTargetConditionReady {
			t.Errorf("Expected condition type to be %s, got %s", SyncTargetConditionReady, condition.Type)
		}
	})

	t.Run("SetHeartbeatCondition", func(t *testing.T) {
		st := &SyncTarget{}
		st.SetHeartbeatCondition(metav1.ConditionTrue, SyncTargetReasonSyncerConnected, "Syncer connected")

		if !st.HasHeartbeat() {
			t.Error("Expected HasHeartbeat to return true after setting Heartbeat condition to True")
		}

		condition := st.GetCondition(SyncTargetConditionHeartbeat)
		if condition == nil {
			t.Fatal("Expected to find Heartbeat condition")
		}

		if condition.Status != metav1.ConditionTrue {
			t.Errorf("Expected condition status to be True, got %s", condition.Status)
		}
	})

	t.Run("SetSyncerReadyCondition", func(t *testing.T) {
		st := &SyncTarget{}
		st.SetSyncerReadyCondition(metav1.ConditionFalse, SyncTargetReasonSyncerDisconnected, "Syncer disconnected")

		condition := st.GetCondition(SyncTargetConditionSyncerReady)
		if condition == nil {
			t.Fatal("Expected to find SyncerReady condition")
		}

		if condition.Status != metav1.ConditionFalse {
			t.Errorf("Expected condition status to be False, got %s", condition.Status)
		}
		if condition.Reason != SyncTargetReasonSyncerDisconnected {
			t.Errorf("Expected condition reason to be %s, got %s", SyncTargetReasonSyncerDisconnected, condition.Reason)
		}
	})

	t.Run("ConditionTransitions", func(t *testing.T) {
		st := &SyncTarget{}
		
		// Set initial condition
		st.SetReadyCondition(metav1.ConditionFalse, "NotReady", "Initial state")
		firstTransitionTime := st.GetCondition(SyncTargetConditionReady).LastTransitionTime

		// Wait a small amount to ensure timestamp difference
		time.Sleep(1 * time.Millisecond)

		// Update with same status - transition time should not change
		st.SetReadyCondition(metav1.ConditionFalse, "StillNotReady", "Still not ready")
		secondTransitionTime := st.GetCondition(SyncTargetConditionReady).LastTransitionTime

		if !firstTransitionTime.Equal(&secondTransitionTime) {
			t.Error("Expected transition time to remain same when status doesn't change")
		}

		// Update with different status - transition time should change
		st.SetReadyCondition(metav1.ConditionTrue, SyncTargetReasonReady, "Now ready")
		thirdTransitionTime := st.GetCondition(SyncTargetConditionReady).LastTransitionTime

		if firstTransitionTime.Equal(&thirdTransitionTime) {
			t.Error("Expected transition time to change when status changes")
		}
	})

	t.Run("IsSchedulable", func(t *testing.T) {
		tests := []struct {
			name         string
			unschedulable bool
			ready        bool
			expected     bool
		}{
			{"schedulable and ready", false, true, true},
			{"schedulable but not ready", false, false, false},
			{"unschedulable but ready", true, true, false},
			{"unschedulable and not ready", true, false, false},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				st := &SyncTarget{
					Spec: SyncTargetSpec{
						Unschedulable: test.unschedulable,
					},
				}

				if test.ready {
					st.SetReadyCondition(metav1.ConditionTrue, SyncTargetReasonReady, "Ready")
				} else {
					st.SetReadyCondition(metav1.ConditionFalse, "NotReady", "Not ready")
				}

				result := st.IsSchedulable()
				if result != test.expected {
					t.Errorf("Expected IsSchedulable to return %v, got %v", test.expected, result)
				}
			})
		}
	})
}

// TestConnectionHelpers tests connection-related helper methods
func TestConnectionHelpers(t *testing.T) {
	t.Run("ConnectionState", func(t *testing.T) {
		st := &SyncTarget{}

		// Default state
		if st.GetConnectionState() != ConnectionStateDisconnected {
			t.Error("Expected default connection state to be Disconnected")
		}
		if st.IsConnected() {
			t.Error("Expected IsConnected to return false for default state")
		}

		// Set connected state
		st.SetConnectionState(ConnectionStateConnected)
		if st.GetConnectionState() != ConnectionStateConnected {
			t.Error("Expected connection state to be Connected after setting")
		}
		if !st.IsConnected() {
			t.Error("Expected IsConnected to return true after setting to Connected")
		}

		// Set error state
		st.SetConnectionState(ConnectionStateError)
		if st.IsConnected() {
			t.Error("Expected IsConnected to return false for Error state")
		}
	})

	t.Run("ValidateConnection", func(t *testing.T) {
		tests := []struct {
			name       string
			connection *SyncTargetConnection
			wantErrs   int
		}{
			{
				name:       "nil connection",
				connection: nil,
				wantErrs:   0,
			},
			{
				name: "valid connection",
				connection: &SyncTargetConnection{
					URL: "https://cluster.example.com:6443",
				},
				wantErrs: 0,
			},
			{
				name: "empty URL",
				connection: &SyncTargetConnection{
					URL: "",
				},
				wantErrs: 1,
			},
			{
				name: "invalid URL",
				connection: &SyncTargetConnection{
					URL: "not a url",
				},
				wantErrs: 1,
			},
			{
				name: "URL without scheme",
				connection: &SyncTargetConnection{
					URL: "cluster.example.com",
				},
				wantErrs: 1,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				st := &SyncTarget{
					Spec: SyncTargetSpec{
						Connection: test.connection,
					},
				}

				errs := st.ValidateConnection()
				if len(errs) != test.wantErrs {
					t.Errorf("Expected %d errors, got %d (%v)", test.wantErrs, len(errs), errs)
				}
			})
		}
	})

	t.Run("SupportsAuthType", func(t *testing.T) {
		tests := []struct {
			authType string
			expected bool
		}{
			{string(SyncTargetAuthTypeToken), true},
			{string(SyncTargetAuthTypeCertificate), true},
			{string(SyncTargetAuthTypeServiceAccount), true},
			{"invalid", false},
			{"", false},
		}

		st := &SyncTarget{
			Spec: SyncTargetSpec{
				Credentials: &SyncTargetCredentials{},
			},
		}

		for _, test := range tests {
			t.Run(test.authType, func(t *testing.T) {
				result := st.SupportsAuthType(test.authType)
				if result != test.expected {
					t.Errorf("Expected SupportsAuthType(%s) to return %v, got %v", test.authType, test.expected, result)
				}
			})
		}

		// Test with nil credentials
		stNoCredentials := &SyncTarget{}
		if stNoCredentials.SupportsAuthType(string(SyncTargetAuthTypeToken)) {
			t.Error("Expected SupportsAuthType to return false when credentials are nil")
		}
	})
}

// TestSyncStateHelpers tests sync state helper methods
func TestSyncStateHelpers(t *testing.T) {
	st := &SyncTarget{}

	// Default state
	if st.GetSyncState() != SyncStateNotReady {
		t.Error("Expected default sync state to be NotReady")
	}
	if st.IsSyncReady() {
		t.Error("Expected IsSyncReady to return false for default state")
	}

	// Set ready state
	st.SetSyncState(SyncStateReady)
	if st.GetSyncState() != SyncStateReady {
		t.Error("Expected sync state to be Ready after setting")
	}
	if !st.IsSyncReady() {
		t.Error("Expected IsSyncReady to return true after setting to Ready")
	}

	// Set error state
	st.SetSyncState(SyncStateError)
	if st.IsSyncReady() {
		t.Error("Expected IsSyncReady to return false for Error state")
	}
}

// TestSyncedResourceManagement tests synced resource management
func TestSyncedResourceManagement(t *testing.T) {
	st := &SyncTarget{}

	resource1 := SyncedResourceStatus{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-deployment",
		SyncState: SyncStateReady,
	}

	resource2 := SyncedResourceStatus{
		Group:     "v1",
		Version:   "v1",
		Kind:      "Service",
		Namespace: "default",
		Name:      "test-service",
		SyncState: SyncStateReady,
	}

	t.Run("AddSyncedResource", func(t *testing.T) {
		// Add first resource
		st.AddSyncedResource(resource1)
		if len(st.Status.SyncedResources) != 1 {
			t.Errorf("Expected 1 synced resource, got %d", len(st.Status.SyncedResources))
		}

		// Add second resource
		st.AddSyncedResource(resource2)
		if len(st.Status.SyncedResources) != 2 {
			t.Errorf("Expected 2 synced resources, got %d", len(st.Status.SyncedResources))
		}
	})

	t.Run("GetSyncedResource", func(t *testing.T) {
		found := st.GetSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
		if found == nil {
			t.Fatal("Expected to find synced resource")
		}
		if found.SyncState != SyncStateReady {
			t.Errorf("Expected sync state to be Ready, got %v", found.SyncState)
		}

		// Non-existent resource
		notFound := st.GetSyncedResource("apps", "v1", "Deployment", "default", "non-existent")
		if notFound != nil {
			t.Error("Expected not to find non-existent resource")
		}
	})

	t.Run("UpdateSyncedResource", func(t *testing.T) {
		// Update existing resource
		updatedResource := resource1
		updatedResource.SyncState = SyncStateError
		updatedResource.Error = "Sync failed"
		st.AddSyncedResource(updatedResource)

		if len(st.Status.SyncedResources) != 2 {
			t.Errorf("Expected 2 synced resources after update, got %d", len(st.Status.SyncedResources))
		}

		found := st.GetSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
		if found == nil {
			t.Fatal("Expected to find updated resource")
		}
		if found.SyncState != SyncStateError {
			t.Errorf("Expected sync state to be Error after update, got %v", found.SyncState)
		}
		if found.Error != "Sync failed" {
			t.Errorf("Expected error message to be 'Sync failed', got %s", found.Error)
		}
	})

	t.Run("RemoveSyncedResource", func(t *testing.T) {
		// Remove first resource
		st.RemoveSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
		if len(st.Status.SyncedResources) != 1 {
			t.Errorf("Expected 1 synced resource after removal, got %d", len(st.Status.SyncedResources))
		}

		// Verify correct resource was removed
		found := st.GetSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
		if found != nil {
			t.Error("Expected removed resource not to be found")
		}

		remaining := st.GetSyncedResource("v1", "v1", "Service", "default", "test-service")
		if remaining == nil {
			t.Error("Expected other resource to remain")
		}

		// Remove non-existent resource (should be no-op)
		st.RemoveSyncedResource("apps", "v1", "Deployment", "default", "non-existent")
		if len(st.Status.SyncedResources) != 1 {
			t.Errorf("Expected 1 synced resource after removing non-existent, got %d", len(st.Status.SyncedResources))
		}
	})
}

// TestHealthStatusManagement tests health status management
func TestHealthStatusManagement(t *testing.T) {
	t.Run("SetHealthStatus", func(t *testing.T) {
		st := &SyncTarget{}

		// Default state
		if st.GetHealthStatus() != HealthStatusUnknown {
			t.Error("Expected default health status to be Unknown")
		}
		if st.IsHealthy() {
			t.Error("Expected IsHealthy to return false for default state")
		}

		// Set healthy status
		st.SetHealthStatus(HealthStatusHealthy, "All systems operational")
		if st.GetHealthStatus() != HealthStatusHealthy {
			t.Error("Expected health status to be Healthy after setting")
		}
		if !st.IsHealthy() {
			t.Error("Expected IsHealthy to return true after setting to Healthy")
		}
		if st.Status.Health.Message != "All systems operational" {
			t.Errorf("Expected health message to be set correctly, got %s", st.Status.Health.Message)
		}
		if st.Status.Health.LastChecked == nil {
			t.Error("Expected LastChecked to be set")
		}

		// Set degraded status
		st.SetHealthStatus(HealthStatusDegraded, "Some issues detected")
		if st.GetHealthStatus() != HealthStatusDegraded {
			t.Error("Expected health status to be Degraded after setting")
		}
		if st.IsHealthy() {
			t.Error("Expected IsHealthy to return false for Degraded state")
		}
	})

	t.Run("AddHealthCheck", func(t *testing.T) {
		st := &SyncTarget{}

		check1 := HealthCheck{
			Name:    "connectivity",
			Status:  HealthCheckStatusPassed,
			Message: "Connection successful",
		}

		check2 := HealthCheck{
			Name:    "auth",
			Status:  HealthCheckStatusPassed,
			Message: "Authentication successful",
		}

		// Add first check
		st.AddHealthCheck(check1)
		if st.Status.Health == nil {
			t.Fatal("Expected health status to be initialized")
		}
		if len(st.Status.Health.Checks) != 1 {
			t.Errorf("Expected 1 health check, got %d", len(st.Status.Health.Checks))
		}

		// Add second check
		st.AddHealthCheck(check2)
		if len(st.Status.Health.Checks) != 2 {
			t.Errorf("Expected 2 health checks, got %d", len(st.Status.Health.Checks))
		}

		// Update existing check
		updatedCheck := check1
		updatedCheck.Status = HealthCheckStatusFailed
		updatedCheck.Message = "Connection failed"
		st.AddHealthCheck(updatedCheck)

		if len(st.Status.Health.Checks) != 2 {
			t.Errorf("Expected 2 health checks after update, got %d", len(st.Status.Health.Checks))
		}

		// Find and verify updated check
		var found *HealthCheck
		for i := range st.Status.Health.Checks {
			if st.Status.Health.Checks[i].Name == "connectivity" {
				found = &st.Status.Health.Checks[i]
				break
			}
		}
		if found == nil {
			t.Fatal("Expected to find updated health check")
		}
		if found.Status != HealthCheckStatusFailed {
			t.Errorf("Expected health check status to be Failed after update, got %v", found.Status)
		}
		if found.Message != "Connection failed" {
			t.Errorf("Expected health check message to be updated, got %s", found.Message)
		}
	})

	t.Run("HealthCheckStatus", func(t *testing.T) {
		statuses := []HealthCheckStatus{
			HealthCheckStatusPassed,
			HealthCheckStatusFailed,
			HealthCheckStatusUnknown,
		}

		for _, status := range statuses {
			t.Run(string(status), func(t *testing.T) {
				check := HealthCheck{
					Name:   "test-check",
					Status: status,
				}

				st := &SyncTarget{}
				st.AddHealthCheck(check)

				if len(st.Status.Health.Checks) != 1 {
					t.Error("Expected health check to be added")
				}
				if st.Status.Health.Checks[0].Status != status {
					t.Errorf("Expected health check status %v, got %v", status, st.Status.Health.Checks[0].Status)
				}
			})
		}
	})
}

// TestCellHelpers tests cell helper methods
func TestCellHelpers(t *testing.T) {
	st := &SyncTarget{
		Spec: SyncTargetSpec{
			Cells: []Cell{
				{
					Name: "cell1",
					Labels: map[string]string{
						"zone": "us-west-1a",
						"type": "compute",
					},
					Taints: []Taint{
						{
							Key:    "node-role",
							Value:  "master",
							Effect: TaintEffectNoSchedule,
						},
						{
							Key:    "dedicated",
							Value:  "system",
							Effect: TaintEffectNoExecute,
						},
					},
				},
				{
					Name: "cell2",
					Labels: map[string]string{
						"zone": "us-west-1b",
						"type": "storage",
					},
				},
				{
					Name: "cell3",
				},
			},
		},
	}

	t.Run("GetCellByName", func(t *testing.T) {
		// Find existing cell
		cell1 := st.GetCellByName("cell1")
		if cell1 == nil {
			t.Fatal("Expected to find cell1")
		}
		if cell1.Name != "cell1" {
			t.Errorf("Expected cell name to be cell1, got %s", cell1.Name)
		}
		if len(cell1.Labels) != 2 {
			t.Errorf("Expected 2 labels, got %d", len(cell1.Labels))
		}
		if len(cell1.Taints) != 2 {
			t.Errorf("Expected 2 taints, got %d", len(cell1.Taints))
		}

		// Find cell with no labels/taints
		cell3 := st.GetCellByName("cell3")
		if cell3 == nil {
			t.Fatal("Expected to find cell3")
		}
		if len(cell3.Labels) != 0 {
			t.Errorf("Expected 0 labels, got %d", len(cell3.Labels))
		}
		if len(cell3.Taints) != 0 {
			t.Errorf("Expected 0 taints, got %d", len(cell3.Taints))
		}

		// Try to find non-existent cell
		nonExistent := st.GetCellByName("non-existent")
		if nonExistent != nil {
			t.Error("Expected not to find non-existent cell")
		}
	})

	t.Run("HasTaint", func(t *testing.T) {
		cell1 := st.GetCellByName("cell1")
		if cell1 == nil {
			t.Fatal("Expected to find cell1")
		}

		// Test existing taints
		if !cell1.HasTaint("node-role", TaintEffectNoSchedule) {
			t.Error("Expected cell1 to have node-role taint with NoSchedule effect")
		}
		if !cell1.HasTaint("dedicated", TaintEffectNoExecute) {
			t.Error("Expected cell1 to have dedicated taint with NoExecute effect")
		}

		// Test non-existent taints
		if cell1.HasTaint("non-existent", TaintEffectNoSchedule) {
			t.Error("Expected cell1 not to have non-existent taint")
		}
		if cell1.HasTaint("node-role", TaintEffectNoExecute) {
			t.Error("Expected cell1 not to have node-role taint with NoExecute effect")
		}

		// Test cell without taints
		cell2 := st.GetCellByName("cell2")
		if cell2 == nil {
			t.Fatal("Expected to find cell2")
		}
		if cell2.HasTaint("any-key", TaintEffectNoSchedule) {
			t.Error("Expected cell2 not to have any taints")
		}
	})

	t.Run("GetTaint", func(t *testing.T) {
		cell1 := st.GetCellByName("cell1")
		if cell1 == nil {
			t.Fatal("Expected to find cell1")
		}

		// Test existing taint
		taint := cell1.GetTaint("node-role", TaintEffectNoSchedule)
		if taint == nil {
			t.Fatal("Expected to find node-role taint")
		}
		if taint.Key != "node-role" {
			t.Errorf("Expected taint key to be node-role, got %s", taint.Key)
		}
		if taint.Value != "master" {
			t.Errorf("Expected taint value to be master, got %s", taint.Value)
		}
		if taint.Effect != TaintEffectNoSchedule {
			t.Errorf("Expected taint effect to be NoSchedule, got %v", taint.Effect)
		}

		// Test non-existent taint
		nonExistent := cell1.GetTaint("non-existent", TaintEffectNoSchedule)
		if nonExistent != nil {
			t.Error("Expected not to find non-existent taint")
		}

		// Test taint with wrong effect
		wrongEffect := cell1.GetTaint("node-role", TaintEffectNoExecute)
		if wrongEffect != nil {
			t.Error("Expected not to find node-role taint with NoExecute effect")
		}
	})
}

// TestSyncTargetUpdate tests update validation
func TestSyncTargetUpdate(t *testing.T) {
	oldTarget := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
		Spec: SyncTargetSpec{
			Cells: []Cell{{Name: "test-cell"}},
		},
	}

	t.Run("valid update", func(t *testing.T) {
		newTarget := oldTarget.DeepCopy()
		newTarget.Spec.Unschedulable = true

		errs := ValidateSyncTargetUpdate(newTarget, oldTarget)
		if len(errs) > 0 {
			t.Errorf("Expected no validation errors for valid update, got: %v", errs)
		}
	})

	t.Run("name change should fail", func(t *testing.T) {
		newTarget := oldTarget.DeepCopy()
		newTarget.Name = "different-name"

		errs := ValidateSyncTargetUpdate(newTarget, oldTarget)
		hasNameError := false
		for _, err := range errs {
			if strings.Contains(err.Error(), "name is immutable") {
				hasNameError = true
				break
			}
		}
		if !hasNameError {
			t.Errorf("Expected name immutability error, got: %v", errs)
		}
	})
}

// TestResourceList tests ResourceList type
func TestResourceList(t *testing.T) {
	t.Run("ResourceList creation", func(t *testing.T) {
		rl := ResourceList{
			"cpu":    resource.MustParse("2"),
			"memory": resource.MustParse("4Gi"),
		}

		if len(rl) != 2 {
			t.Errorf("Expected 2 resources, got %d", len(rl))
		}

		cpuQuantity, exists := rl["cpu"]
		if !exists {
			t.Error("Expected cpu resource to exist")
		}
		if !cpuQuantity.Equal(resource.MustParse("2")) {
			t.Errorf("Expected cpu quantity to be 2, got %v", cpuQuantity)
		}
	})

	t.Run("SyncTarget with resource lists", func(t *testing.T) {
		st := &SyncTarget{
			Status: SyncTargetStatus{
				Capacity: ResourceList{
					"cpu":    resource.MustParse("4"),
					"memory": resource.MustParse("8Gi"),
				},
				Allocatable: ResourceList{
					"cpu":    resource.MustParse("3.5"),
					"memory": resource.MustParse("7Gi"),
				},
			},
		}

		if len(st.Status.Capacity) != 2 {
			t.Errorf("Expected 2 capacity resources, got %d", len(st.Status.Capacity))
		}
		if len(st.Status.Allocatable) != 2 {
			t.Errorf("Expected 2 allocatable resources, got %d", len(st.Status.Allocatable))
		}
	})
}

// TestConcurrency tests concurrent access patterns
func TestConcurrency(t *testing.T) {
	t.Run("concurrent condition updates", func(t *testing.T) {
		st := &SyncTarget{}
		var wg sync.WaitGroup
		numGoroutines := 10

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					status := metav1.ConditionTrue
					if (id+j)%2 == 0 {
						status = metav1.ConditionFalse
					}
					st.SetReadyCondition(status, "TestReason", "Test message")
				}
			}(i)
		}

		wg.Wait()

		// Should have exactly one Ready condition
		condition := st.GetCondition(SyncTargetConditionReady)
		if condition == nil {
			t.Error("Expected Ready condition to exist after concurrent updates")
		}
	})

	t.Run("concurrent resource management", func(t *testing.T) {
		st := &SyncTarget{}
		var wg sync.WaitGroup
		numGoroutines := 5

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				resource := SyncedResourceStatus{
					Group:     "test",
					Version:   "v1",
					Kind:      "TestResource",
					Namespace: "default",
					Name:      fmt.Sprintf("resource-%d", id),
					SyncState: SyncStateReady,
				}
				st.AddSyncedResource(resource)
			}(i)
		}

		wg.Wait()

		// Should have all resources added
		if len(st.Status.SyncedResources) != numGoroutines {
			t.Errorf("Expected %d synced resources, got %d", numGoroutines, len(st.Status.SyncedResources))
		}
	})
}

// TestEdgeCases tests edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("empty SyncTarget", func(t *testing.T) {
		st := &SyncTarget{}

		// Test methods on empty SyncTarget
		if st.IsReady() {
			t.Error("Empty SyncTarget should not be ready")
		}
		if st.IsConnected() {
			t.Error("Empty SyncTarget should not be connected")
		}
		if st.IsSyncReady() {
			t.Error("Empty SyncTarget should not have sync ready")
		}
		if st.IsHealthy() {
			t.Error("Empty SyncTarget should not be healthy")
		}
		if st.IsSchedulable() {
			t.Error("Empty SyncTarget should not be schedulable")
		}
	})

	t.Run("nil pointer safety", func(t *testing.T) {
		st := &SyncTarget{}

		// Test methods that might access nil pointers
		if st.SupportsAuthType("token") {
			t.Error("SyncTarget with nil credentials should not support any auth type")
		}

		errs := st.ValidateConnection()
		if len(errs) != 0 {
			t.Errorf("ValidateConnection with nil connection should return no errors, got: %v", errs)
		}

		cell := st.GetCellByName("any-name")
		if cell != nil {
			t.Error("GetCellByName should return nil when no cells exist")
		}
	})

	t.Run("large data handling", func(t *testing.T) {
		st := &SyncTarget{
			ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
			Spec: SyncTargetSpec{
				Cells: make([]Cell, 100), // Large number of cells
			},
		}

		// Initialize cells
		for i := range st.Spec.Cells {
			st.Spec.Cells[i] = Cell{
				Name: fmt.Sprintf("cell-%d", i),
			}
		}

		// Test that operations work with large data
		cell50 := st.GetCellByName("cell-50")
		if cell50 == nil {
			t.Error("Expected to find cell-50 in large cell list")
		}

		// Add many synced resources
		for i := 0; i < 100; i++ {
			resource := SyncedResourceStatus{
				Group:     "test",
				Version:   "v1",
				Kind:      "TestResource",
				Namespace: "default",
				Name:      fmt.Sprintf("resource-%d", i),
				SyncState: SyncStateReady,
			}
			st.AddSyncedResource(resource)
		}

		if len(st.Status.SyncedResources) != 100 {
			t.Errorf("Expected 100 synced resources, got %d", len(st.Status.SyncedResources))
		}

		// Test finding specific resource in large list
		found := st.GetSyncedResource("test", "v1", "TestResource", "default", "resource-75")
		if found == nil {
			t.Error("Expected to find resource-75 in large resource list")
		}
	})
}

// TestDeepCopy tests deep copy functionality
func TestDeepCopy(t *testing.T) {
	original := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-target",
			Labels: map[string]string{
				"env": "test",
			},
		},
		Spec: SyncTargetSpec{
			Cells: []Cell{
				{
					Name: "cell1",
					Labels: map[string]string{
						"zone": "us-west-1a",
					},
					Taints: []Taint{
						{
							Key:    "node-role",
							Value:  "master",
							Effect: TaintEffectNoSchedule,
						},
					},
				},
			},
			Connection: &SyncTargetConnection{
				URL: "https://cluster.example.com",
			},
			Credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				Token: &TokenCredentials{
					Value: "test-token",
				},
			},
		},
		Status: SyncTargetStatus{
			ConnectionState: ConnectionStateConnected,
			SyncedResources: []SyncedResourceStatus{
				{
					Group:     "apps",
					Version:   "v1",
					Kind:      "Deployment",
					Name:      "test-deployment",
					SyncState: SyncStateReady,
				},
			},
		},
	}

	copy := original.DeepCopy()

	// Verify copy is not the same object
	if original == copy {
		t.Error("DeepCopy should return a different object")
	}

	// Verify deep copy worked correctly
	if !reflect.DeepEqual(original, copy) {
		t.Error("DeepCopy should create an exact copy")
	}

	// Verify modifying copy doesn't affect original
	copy.Name = "modified-name"
	if original.Name == "modified-name" {
		t.Error("Modifying copy should not affect original")
	}

	// Test modifying nested structures
	copy.Spec.Cells[0].Name = "modified-cell"
	if original.Spec.Cells[0].Name == "modified-cell" {
		t.Error("Modifying nested structure in copy should not affect original")
	}

	copy.Spec.Connection.URL = "https://modified.example.com"
	if original.Spec.Connection.URL == "https://modified.example.com" {
		t.Error("Modifying connection in copy should not affect original")
	}
}

// TestValidationComprehensive runs comprehensive validation tests
func TestValidationComprehensive(t *testing.T) {
	t.Run("comprehensive valid SyncTarget", func(t *testing.T) {
		target := &SyncTarget{
			TypeMeta: metav1.TypeMeta{
				Kind:       "SyncTarget",
				APIVersion: "workload.kcp.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "comprehensive-target",
				Labels: map[string]string{
					"environment": "test",
					"region":      "us-west-1",
				},
			},
			Spec: SyncTargetSpec{
				Cells: []Cell{
					{
						Name: "primary-cell",
						Labels: map[string]string{
							"zone": "us-west-1a",
							"type": "compute",
						},
						Taints: []Taint{
							{
								Key:    "dedicated",
								Value:  "system",
								Effect: TaintEffectNoSchedule,
							},
						},
					},
					{
						Name: "secondary-cell",
						Labels: map[string]string{
							"zone": "us-west-1b",
							"type": "storage",
						},
					},
				},
				Connection: &SyncTargetConnection{
					URL:        "https://cluster.example.com:6443",
					ServerName: "cluster.example.com",
					CABundle:   []byte("ca-bundle-data"),
				},
				Credentials: &SyncTargetCredentials{
					Type: SyncTargetAuthTypeToken,
					Token: &TokenCredentials{
						Value: "test-token-value",
					},
				},
				Capabilities: &SyncTargetCapabilities{
					MaxWorkloads: &[]int32{1000}[0],
					Features:     []string{"storage", "networking", "compute"},
					SupportedResourceTypes: []ResourceTypeSupport{
						{
							Group:     "apps",
							Version:   "v1",
							Kind:      "Deployment",
							Supported: true,
						},
						{
							Group:     "",
							Version:   "v1",
							Kind:      "Service",
							Supported: true,
						},
					},
				},
				SupportedAPIExports: []APIExportReference{
					{
						Workspace: "root:org:workspace",
						Name:      "kubernetes-api",
					},
				},
				EvictAfter: &metav1.Duration{Duration: 5 * time.Minute},
			},
		}

		errs := ValidateSyncTarget(target)
		if len(errs) > 0 {
			t.Errorf("Comprehensive valid SyncTarget should not have validation errors: %v", errs)
		}
	})
}