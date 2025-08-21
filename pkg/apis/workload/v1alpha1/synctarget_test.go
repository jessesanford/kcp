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

// TestSyncTargetTypes tests the SyncTarget types implementation
func TestSyncTargetTypes(t *testing.T) {
	tests := []struct {
		name     string
		validate func(*testing.T)
	}{
		{
			name: "SyncTarget should have connection details in spec",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					Spec: SyncTargetSpec{
						Cells: []Cell{{Name: "test-cell"}},
						Connection: &SyncTargetConnection{
							URL: "https://cluster1.example.com",
						},
					},
				}
				if st.Spec.Connection == nil {
					t.Errorf("Expected connection to be present")
				}
				if st.Spec.Connection.URL != "https://cluster1.example.com" {
					t.Errorf("Expected URL to be set correctly")
				}
			},
		},
		{
			name: "SyncTarget should have credentials support",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					Spec: SyncTargetSpec{
						Cells: []Cell{{Name: "test-cell"}},
						Credentials: &SyncTargetCredentials{
							Type: SyncTargetAuthTypeToken,
							Token: &TokenCredentials{
								Value: "test-token",
							},
						},
					},
				}
				if st.Spec.Credentials == nil {
					t.Errorf("Expected credentials to be present")
				}
				if st.Spec.Credentials.Type != SyncTargetAuthTypeToken {
					t.Errorf("Expected auth type to be token")
				}
			},
		},
		{
			name: "SyncTarget should have capabilities support",
			validate: func(t *testing.T) {
				st := &SyncTarget{
					Spec: SyncTargetSpec{
						Cells: []Cell{{Name: "test-cell"}},
						Capabilities: &SyncTargetCapabilities{
							MaxWorkloads: &[]int32{100}[0],
							Features:     []string{"feature1", "feature2"},
						},
					},
				}
				if st.Spec.Capabilities == nil {
					t.Errorf("Expected capabilities to be present")
				}
				if *st.Spec.Capabilities.MaxWorkloads != 100 {
					t.Errorf("Expected max workloads to be 100")
				}
				if len(st.Spec.Capabilities.Features) != 2 {
					t.Errorf("Expected 2 features")
				}
			},
		},
		{
			name: "SyncTarget should track connection state in status",
			validate: func(t *testing.T) {
				st := &SyncTarget{}
				st.SetConnectionState(ConnectionStateConnected)
				
				if st.Status.ConnectionState != ConnectionStateConnected {
					t.Errorf("Expected connection state to be Connected")
				}
				if !st.IsConnected() {
					t.Errorf("Expected IsConnected to return true")
				}
			},
		},
		{
			name: "SyncTarget should track sync state in status",
			validate: func(t *testing.T) {
				st := &SyncTarget{}
				st.SetSyncState(SyncStateReady)
				
				if st.Status.SyncState != SyncStateReady {
					t.Errorf("Expected sync state to be Ready")
				}
				if !st.IsSyncReady() {
					t.Errorf("Expected IsSyncReady to return true")
				}
			},
		},
		{
			name: "SyncTarget should track health status",
			validate: func(t *testing.T) {
				st := &SyncTarget{}
				st.SetHealthStatus(HealthStatusHealthy, "All checks passing")
				
				if st.GetHealthStatus() != HealthStatusHealthy {
					t.Errorf("Expected health status to be Healthy")
				}
				if !st.IsHealthy() {
					t.Errorf("Expected IsHealthy to return true")
				}
				if st.Status.Health.Message != "All checks passing" {
					t.Errorf("Expected health message to be set")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.validate)
	}
}

// TestSyncTargetValidation tests the validation functionality
func TestSyncTargetValidation(t *testing.T) {
	tests := []struct {
		name     string
		target   *SyncTarget
		wantErrs bool
	}{
		{
			name: "valid SyncTarget with connection should pass",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Connection: &SyncTargetConnection{
						URL: "https://cluster1.example.com",
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "SyncTarget with invalid URL should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Connection: &SyncTargetConnection{
						URL: "not-a-valid-url",
					},
				},
			},
			wantErrs: true,
		},
		{
			name: "SyncTarget without cells should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{}, // Empty cells
				},
			},
			wantErrs: true,
		},
		{
			name: "SyncTarget with invalid credentials should fail",
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{Name: "test-target"},
				Spec: SyncTargetSpec{
					Cells: []Cell{{Name: "test-cell"}},
					Credentials: &SyncTargetCredentials{
						Type: SyncTargetAuthTypeToken,
						// Missing token credentials
					},
				},
			},
			wantErrs: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateSyncTarget(test.target)
			hasErrs := len(errs) > 0
			
			if hasErrs != test.wantErrs {
				t.Errorf("Expected validation errors: %v, got errors: %v (%v)", test.wantErrs, hasErrs, errs)
			}
		})
	}
}

// TestAuthTypeSupport tests authentication type support
func TestAuthTypeSupport(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		expected bool
	}{
		{"token auth should be supported", "token", true},
		{"certificate auth should be supported", "certificate", true},
		{"serviceAccount auth should be supported", "serviceAccount", true},
		{"invalid auth should not be supported", "invalid", false},
	}

	st := &SyncTarget{
		Spec: SyncTargetSpec{
			Credentials: &SyncTargetCredentials{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := st.SupportsAuthType(test.authType)
			if result != test.expected {
				t.Errorf("Expected SupportsAuthType(%s) to return %v, got %v", test.authType, test.expected, result)
			}
		})
	}
}

// TestConnectionValidation tests URL validation functionality
func TestConnectionValidation(t *testing.T) {
	tests := []struct {
		name      string
		target    *SyncTarget
		wantErrs  int
	}{
		{
			name: "valid URL should pass",
			target: &SyncTarget{
				Spec: SyncTargetSpec{
					Connection: &SyncTargetConnection{
						URL: "https://cluster1.example.com:6443",
					},
				},
			},
			wantErrs: 0,
		},
		{
			name: "invalid URL should fail",
			target: &SyncTarget{
				Spec: SyncTargetSpec{
					Connection: &SyncTargetConnection{
						URL: "not a valid url",
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "no connection should pass",
			target: &SyncTarget{
				Spec: SyncTargetSpec{
					Connection: nil,
				},
			},
			wantErrs: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := test.target.ValidateConnection()
			if len(errs) != test.wantErrs {
				t.Errorf("Expected %d validation errors, got %d (%v)", test.wantErrs, len(errs), errs)
			}
		})
	}
}

// TestConditionManagement tests condition setting and getting
func TestConditionManagement(t *testing.T) {
	st := &SyncTarget{}

	// Test setting and getting conditions
	st.SetReadyCondition(metav1.ConditionTrue, SyncTargetReasonReady, "Target is ready")

	if !st.IsReady() {
		t.Errorf("Expected SyncTarget to be ready after setting Ready condition to True")
	}

	condition := st.GetCondition(SyncTargetConditionReady)
	if condition == nil {
		t.Errorf("Expected to find Ready condition")
	} else {
		if condition.Status != metav1.ConditionTrue {
			t.Errorf("Expected Ready condition status to be True")
		}
		if condition.Reason != SyncTargetReasonReady {
			t.Errorf("Expected Ready condition reason to be %s, got %s", SyncTargetReasonReady, condition.Reason)
		}
	}

	// Test heartbeat condition
	st.SetHeartbeatCondition(metav1.ConditionTrue, SyncTargetReasonSyncerConnected, "Syncer connected")
	if !st.HasHeartbeat() {
		t.Errorf("Expected SyncTarget to have heartbeat after setting Heartbeat condition to True")
	}
}

// TestSyncedResourceManagement tests synced resource tracking
func TestSyncedResourceManagement(t *testing.T) {
	st := &SyncTarget{}

	resource := SyncedResourceStatus{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-deployment",
		SyncState: SyncStateReady,
	}

	// Add resource
	st.AddSyncedResource(resource)
	if len(st.Status.SyncedResources) != 1 {
		t.Errorf("Expected 1 synced resource, got %d", len(st.Status.SyncedResources))
	}

	// Get resource
	found := st.GetSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
	if found == nil {
		t.Errorf("Expected to find synced resource")
	} else if found.SyncState != SyncStateReady {
		t.Errorf("Expected sync state to be Ready")
	}

	// Update resource
	updatedResource := resource
	updatedResource.SyncState = SyncStateError
	updatedResource.Error = "Sync failed"
	st.AddSyncedResource(updatedResource)

	if len(st.Status.SyncedResources) != 1 {
		t.Errorf("Expected 1 synced resource after update, got %d", len(st.Status.SyncedResources))
	}

	found = st.GetSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
	if found.SyncState != SyncStateError {
		t.Errorf("Expected sync state to be Error after update")
	}

	// Remove resource
	st.RemoveSyncedResource("apps", "v1", "Deployment", "default", "test-deployment")
	if len(st.Status.SyncedResources) != 0 {
		t.Errorf("Expected 0 synced resources after removal, got %d", len(st.Status.SyncedResources))
	}
}

// TestHealthCheckManagement tests health check management
func TestHealthCheckManagement(t *testing.T) {
	st := &SyncTarget{}

	check := HealthCheck{
		Name:    "connectivity",
		Status:  HealthCheckStatusPassed,
		Message: "Connection successful",
	}

	// Add health check
	st.AddHealthCheck(check)
	if st.Status.Health == nil {
		t.Errorf("Expected health status to be initialized")
	}
	if len(st.Status.Health.Checks) != 1 {
		t.Errorf("Expected 1 health check, got %d", len(st.Status.Health.Checks))
	}

	// Update health check
	updatedCheck := check
	updatedCheck.Status = HealthCheckStatusFailed
	updatedCheck.Message = "Connection failed"
	st.AddHealthCheck(updatedCheck)

	if len(st.Status.Health.Checks) != 1 {
		t.Errorf("Expected 1 health check after update, got %d", len(st.Status.Health.Checks))
	}

	found := st.Status.Health.Checks[0]
	if found.Status != HealthCheckStatusFailed {
		t.Errorf("Expected health check status to be Failed after update")
	}
	if found.Message != "Connection failed" {
		t.Errorf("Expected health check message to be updated")
	}
}

// TestCellHelpers tests cell helper methods
func TestCellHelpers(t *testing.T) {
	st := &SyncTarget{
		Spec: SyncTargetSpec{
			Cells: []Cell{
				{
					Name: "cell1",
					Taints: []Taint{
						{
							Key:    "node-role",
							Value:  "master",
							Effect: TaintEffectNoSchedule,
						},
					},
				},
				{Name: "cell2"},
			},
		},
	}

	// Test GetCellByName
	cell1 := st.GetCellByName("cell1")
	if cell1 == nil {
		t.Errorf("Expected to find cell1")
	} else if cell1.Name != "cell1" {
		t.Errorf("Expected cell name to be cell1")
	}

	cell3 := st.GetCellByName("cell3")
	if cell3 != nil {
		t.Errorf("Expected not to find cell3")
	}

	// Test taint helpers
	if !cell1.HasTaint("node-role", TaintEffectNoSchedule) {
		t.Errorf("Expected cell1 to have node-role taint")
	}

	if cell1.HasTaint("nonexistent", TaintEffectNoSchedule) {
		t.Errorf("Expected cell1 not to have nonexistent taint")
	}

	taint := cell1.GetTaint("node-role", TaintEffectNoSchedule)
	if taint == nil {
		t.Errorf("Expected to find node-role taint")
	} else if taint.Value != "master" {
		t.Errorf("Expected taint value to be master")
	}
}