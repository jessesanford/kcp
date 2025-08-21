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
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TestValidateSyncTargetConnection tests SyncTarget connection validation
func TestValidateSyncTargetConnection(t *testing.T) {
	tests := []struct {
		name       string
		connection *SyncTargetConnection
		wantErrs   int
	}{
		{
			name: "valid connection should pass",
			connection: &SyncTargetConnection{
				URL: "https://cluster1.example.com:6443",
			},
			wantErrs: 0,
		},
		{
			name: "invalid URL should fail",
			connection: &SyncTargetConnection{
				URL: "not-a-url",
			},
			wantErrs: 1,
		},
		{
			name: "empty URL should fail",
			connection: &SyncTargetConnection{
				URL: "",
			},
			wantErrs: 1,
		},
		{
			name: "valid URL with server name should pass",
			connection: &SyncTargetConnection{
				URL:        "https://cluster1.example.com:6443",
				ServerName: "api.cluster1.example.com",
			},
			wantErrs: 0,
		},
		{
			name: "invalid server name should fail",
			connection: &SyncTargetConnection{
				URL:        "https://cluster1.example.com:6443",
				ServerName: "invalid_server_name",
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateSyncTargetConnection(test.connection, field.NewPath("connection"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateSyncTargetCredentials tests credential validation
func TestValidateSyncTargetCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials *SyncTargetCredentials
		wantErrs    int
	}{
		{
			name: "valid token credentials should pass",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				Token: &TokenCredentials{
					Value: "valid-token",
				},
			},
			wantErrs: 0,
		},
		{
			name: "valid certificate credentials should pass",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				Certificate: &CertificateCredentials{
					ClientCert: []byte("cert-data"),
					ClientKey:  []byte("key-data"),
				},
			},
			wantErrs: 0,
		},
		{
			name: "valid service account credentials should pass",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				ServiceAccount: &ServiceAccountCredentials{
					Namespace: "default",
					Name:      "syncer",
				},
			},
			wantErrs: 0,
		},
		{
			name: "missing auth type should fail",
			credentials: &SyncTargetCredentials{
				Token: &TokenCredentials{
					Value: "valid-token",
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid auth type should fail",
			credentials: &SyncTargetCredentials{
				Type: "invalid",
			},
			wantErrs: 1,
		},
		{
			name: "token type without token credentials should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				// Missing Token field
			},
			wantErrs: 1,
		},
		{
			name: "certificate type without certificate credentials should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				// Missing Certificate field
			},
			wantErrs: 1,
		},
		{
			name: "service account type without service account credentials should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				// Missing ServiceAccount field
			},
			wantErrs: 1,
		},
		{
			name: "empty token value should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeToken,
				Token: &TokenCredentials{
					Value: "",
				},
			},
			wantErrs: 1,
		},
		{
			name: "empty certificate data should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeCertificate,
				Certificate: &CertificateCredentials{
					// Missing ClientCert and ClientKey
				},
			},
			wantErrs: 2,
		},
		{
			name: "invalid service account namespace should fail",
			credentials: &SyncTargetCredentials{
				Type: SyncTargetAuthTypeServiceAccount,
				ServiceAccount: &ServiceAccountCredentials{
					Namespace: "Invalid_Namespace",
					Name:      "syncer",
				},
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateSyncTargetCredentials(test.credentials, field.NewPath("credentials"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateSyncTargetCapabilities tests capability validation
func TestValidateSyncTargetCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		capabilities *SyncTargetCapabilities
		wantErrs     int
	}{
		{
			name: "valid capabilities should pass",
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
			wantErrs: 0,
		},
		{
			name: "negative max workloads should fail",
			capabilities: &SyncTargetCapabilities{
				MaxWorkloads: &[]int32{-1}[0],
			},
			wantErrs: 1,
		},
		{
			name: "empty feature name should fail",
			capabilities: &SyncTargetCapabilities{
				Features: []string{"valid-feature", ""},
			},
			wantErrs: 1,
		},
		{
			name: "invalid resource type should fail",
			capabilities: &SyncTargetCapabilities{
				SupportedResourceTypes: []ResourceTypeSupport{
					{
						// Missing Version and Kind
						Group: "apps",
					},
				},
			},
			wantErrs: 2, // Missing version and kind
		},
		{
			name: "invalid group in resource type should fail",
			capabilities: &SyncTargetCapabilities{
				SupportedResourceTypes: []ResourceTypeSupport{
					{
						Group:   "Invalid_Group",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateSyncTargetCapabilities(test.capabilities, field.NewPath("capabilities"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateCell tests cell validation
func TestValidateCell(t *testing.T) {
	tests := []struct {
		name     string
		cell     *Cell
		wantErrs int
	}{
		{
			name: "valid cell should pass",
			cell: &Cell{
				Name: "valid-cell",
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
			wantErrs: 0,
		},
		{
			name: "empty cell name should fail",
			cell: &Cell{
				Name: "",
			},
			wantErrs: 1,
		},
		{
			name: "invalid cell name should fail",
			cell: &Cell{
				Name: "Invalid_Cell_Name",
			},
			wantErrs: 1,
		},
		{
			name: "invalid label key should fail",
			cell: &Cell{
				Name: "valid-cell",
				Labels: map[string]string{
					"invalid key": "value",
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid label value should fail",
			cell: &Cell{
				Name: "valid-cell",
				Labels: map[string]string{
					"key": "invalid\nvalue",
				},
			},
			wantErrs: 1,
		},
		{
			name: "duplicate taint keys should fail",
			cell: &Cell{
				Name: "valid-cell",
				Taints: []Taint{
					{
						Key:    "duplicate-key",
						Effect: TaintEffectNoSchedule,
					},
					{
						Key:    "duplicate-key",
						Effect: TaintEffectPreferNoSchedule,
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid taint should fail",
			cell: &Cell{
				Name: "valid-cell",
				Taints: []Taint{
					{
						Key: "",
						// Empty key should fail
					},
				},
			},
			wantErrs: 2, // Empty key and empty effect
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateCell(test.cell, field.NewPath("cell"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateTaint tests taint validation
func TestValidateTaint(t *testing.T) {
	tests := []struct {
		name     string
		taint    *Taint
		wantErrs int
	}{
		{
			name: "valid taint should pass",
			taint: &Taint{
				Key:    "node-role",
				Value:  "master",
				Effect: TaintEffectNoSchedule,
			},
			wantErrs: 0,
		},
		{
			name: "taint without value should pass",
			taint: &Taint{
				Key:    "node-role",
				Effect: TaintEffectNoSchedule,
			},
			wantErrs: 0,
		},
		{
			name: "empty taint key should fail",
			taint: &Taint{
				Key:    "",
				Effect: TaintEffectNoSchedule,
			},
			wantErrs: 1,
		},
		{
			name: "invalid taint key should fail",
			taint: &Taint{
				Key:    "invalid key",
				Effect: TaintEffectNoSchedule,
			},
			wantErrs: 1,
		},
		{
			name: "invalid taint value should fail",
			taint: &Taint{
				Key:    "node-role",
				Value:  "invalid\nvalue",
				Effect: TaintEffectNoSchedule,
			},
			wantErrs: 1,
		},
		{
			name: "empty taint effect should fail",
			taint: &Taint{
				Key: "node-role",
				// Effect is required
			},
			wantErrs: 1,
		},
		{
			name: "invalid taint effect should fail",
			taint: &Taint{
				Key:    "node-role",
				Effect: "InvalidEffect",
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateTaint(test.taint, field.NewPath("taint"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateAPIExportReference tests APIExport reference validation
func TestValidateAPIExportReference(t *testing.T) {
	tests := []struct {
		name     string
		ref      *APIExportReference
		wantErrs int
	}{
		{
			name: "valid APIExport reference should pass",
			ref: &APIExportReference{
				Workspace: "root:org:team",
				Name:      "my-api-export",
			},
			wantErrs: 0,
		},
		{
			name: "empty workspace should fail",
			ref: &APIExportReference{
				Workspace: "",
				Name:      "my-api-export",
			},
			wantErrs: 1,
		},
		{
			name: "empty name should fail",
			ref: &APIExportReference{
				Workspace: "root:org:team",
				Name:      "",
			},
			wantErrs: 1,
		},
		{
			name: "invalid workspace path should fail",
			ref: &APIExportReference{
				Workspace: "root:",
				Name:      "my-api-export",
			},
			wantErrs: 1,
		},
		{
			name: "invalid name should fail",
			ref: &APIExportReference{
				Workspace: "root:org:team",
				Name:      "Invalid_Name",
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := validateAPIExportReference(test.ref, field.NewPath("apiExport"))
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestValidateSyncTargetUpdate tests update validation
func TestValidateSyncTargetUpdate(t *testing.T) {
	oldTarget := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
		Spec: SyncTargetSpec{
			Cells: []Cell{{Name: "cell1"}},
		},
	}

	tests := []struct {
		name      string
		newTarget *SyncTarget
		wantErrs  int
	}{
		{
			name: "valid update should pass",
			newTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "cell1"}},
					Connection: &SyncTargetConnection{
						URL: "https://cluster1.example.com",
					},
				},
			},
			wantErrs: 0,
		},
		{
			name: "changing name should fail",
			newTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "different-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "cell1"}},
				},
			},
			wantErrs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateSyncTargetUpdate(test.newTarget, oldTarget)
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d: %v", test.wantErrs, len(errs), errs)
			}
		})
	}
}