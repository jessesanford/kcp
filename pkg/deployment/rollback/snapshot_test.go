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

package rollback

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestSnapshotManager_GenerateSnapshotID(t *testing.T) {
	sm := createTestSnapshotManager(t)

	deploymentRef := corev1.ObjectReference{
		Name:      "test-deployment",
		Namespace: "default",
	}

	id1 := sm.generateSnapshotID(deploymentRef, "v1.0.0")
	id2 := sm.generateSnapshotID(deploymentRef, "v1.0.0")

	// IDs should be different due to timestamp
	if id1 == id2 {
		t.Error("expected different snapshot IDs for same deployment")
	}

	// IDs should have proper prefix
	if len(id1) < 5 || id1[:5] != "snap-" {
		t.Errorf("expected snapshot ID to start with 'snap-', got %s", id1)
	}
}

func TestSnapshotManager_ExtractConfiguration(t *testing.T) {
	sm := createTestSnapshotManager(t)

	// Create test deployment resource
	deployment := map[string]interface{}{
		"kind":       "Deployment",
		"apiVersion": "apps/v1",
		"metadata": map[string]interface{}{
			"name":      "test-deployment",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx:1.20",
						},
					},
				},
			},
		},
	}

	// Create test service resource
	service := map[string]interface{}{
		"kind":       "Service",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name":      "test-service",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"type": "ClusterIP",
		},
	}

	// Create test configmap resource
	configmap := map[string]interface{}{
		"kind":       "ConfigMap",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name":      "test-config",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"config.yaml": "test: value",
			"app.conf":    "setting=true",
		},
	}

	resources := []runtime.RawExtension{
		{Object: deployment},
		{Object: service},
		{Object: configmap},
	}

	config := sm.extractConfiguration(resources)

	// Check deployment configuration
	if config["deployment.replicas"] != "3" {
		t.Errorf("expected deployment.replicas=3, got %s", config["deployment.replicas"])
	}

	if config["deployment.image"] != "nginx:1.20" {
		t.Errorf("expected deployment.image=nginx:1.20, got %s", config["deployment.image"])
	}

	// Check service configuration
	if config["service.test-service.type"] != "ClusterIP" {
		t.Errorf("expected service.test-service.type=ClusterIP, got %s", config["service.test-service.type"])
	}

	// Check configmap configuration
	if config["configmap.test-config.config.yaml"] != "present" {
		t.Error("expected configmap key to be marked as present")
	}

	if config["configmap.test-config.app.conf"] != "present" {
		t.Error("expected configmap key to be marked as present")
	}
}

func TestSnapshotManager_CalculateConfigHash(t *testing.T) {
	sm := createTestSnapshotManager(t)

	config1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	config2 := map[string]string{
		"key2": "value2",
		"key1": "value1",
	}

	config3 := map[string]string{
		"key1": "value1",
		"key2": "different",
	}

	hash1 := sm.calculateConfigHash(config1)
	hash2 := sm.calculateConfigHash(config2)
	hash3 := sm.calculateConfigHash(config3)

	// Same config should produce same hash regardless of key order
	if hash1 != hash2 {
		t.Error("expected same hash for equivalent configurations")
	}

	// Different config should produce different hash
	if hash1 == hash3 {
		t.Error("expected different hash for different configurations")
	}

	// Hash should not be empty
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}
}

func TestSnapshotManager_ValidateSnapshot(t *testing.T) {
	sm := createTestSnapshotManager(t)

	tests := map[string]struct {
		snapshot *DeploymentSnapshot
		wantErr  bool
		errMsg   string
	}{
		"valid snapshot": {
			snapshot: &DeploymentSnapshot{
				ID:      "snap-123",
				Version: "v1.0.0",
				Resources: []runtime.RawExtension{
					{Raw: []byte(`{"kind":"Deployment"}`)},
				},
				Configuration: map[string]string{"key": "value"},
				ConfigHash:    sm.calculateConfigHash(map[string]string{"key": "value"}),
			},
			wantErr: false,
		},
		"nil snapshot": {
			snapshot: nil,
			wantErr:  true,
			errMsg:   "snapshot is nil",
		},
		"empty ID": {
			snapshot: &DeploymentSnapshot{
				Version: "v1.0.0",
				Resources: []runtime.RawExtension{
					{Raw: []byte(`{"kind":"Deployment"}`)},
				},
			},
			wantErr: true,
			errMsg:  "snapshot ID is empty",
		},
		"no resources": {
			snapshot: &DeploymentSnapshot{
				ID:        "snap-123",
				Version:   "v1.0.0",
				Resources: []runtime.RawExtension{},
			},
			wantErr: true,
			errMsg:  "snapshot contains no resources",
		},
		"config hash mismatch": {
			snapshot: &DeploymentSnapshot{
				ID:      "snap-123",
				Version: "v1.0.0",
				Resources: []runtime.RawExtension{
					{Raw: []byte(`{"kind":"Deployment"}`)},
				},
				Configuration: map[string]string{"key": "value"},
				ConfigHash:    "wrong-hash",
			},
			wantErr: true,
			errMsg:  "snapshot config hash mismatch",
		},
		"nil raw data": {
			snapshot: &DeploymentSnapshot{
				ID:      "snap-123",
				Version: "v1.0.0",
				Resources: []runtime.RawExtension{
					{Raw: nil},
				},
				Configuration: map[string]string{"key": "value"},
				ConfigHash:    sm.calculateConfigHash(map[string]string{"key": "value"}),
			},
			wantErr: true,
			errMsg:  "resource 0 has nil raw data",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := sm.ValidateSnapshot(nil, tc.snapshot)

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tc.wantErr && err != nil {
				if tc.errMsg != "" && !contains(err.Error(), tc.errMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tc.errMsg, err.Error())
				}
			}
		})
	}
}

func TestSnapshotManager_CleanupExpiredSnapshots(t *testing.T) {
	config := &EngineConfig{
		MaxSnapshots:              3,
		SnapshotRetentionDuration: &metav1.Duration{Duration: 24 * time.Hour},
	}

	sm := createTestSnapshotManagerWithConfig(t, config)

	// This test would require mocking the storage backend
	// For now, just test that the method doesn't panic
	deploymentRef := corev1.ObjectReference{
		Name:      "test-deployment",
		Namespace: "default",
	}

	err := sm.CleanupExpiredSnapshots(nil, deploymentRef)
	if err != nil {
		// Expected since we don't have actual storage implementation
		t.Logf("Expected error from cleanup: %v", err)
	}
}

// Benchmark tests

func BenchmarkSnapshotManager_GenerateSnapshotID(b *testing.B) {
	sm := createTestSnapshotManager(b)
	deploymentRef := corev1.ObjectReference{
		Name:      "test-deployment",
		Namespace: "default",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.generateSnapshotID(deploymentRef, "v1.0.0")
	}
}

func BenchmarkSnapshotManager_CalculateConfigHash(b *testing.B) {
	sm := createTestSnapshotManager(b)
	config := map[string]string{
		"deployment.replicas": "3",
		"deployment.image":    "nginx:1.20",
		"service.type":        "ClusterIP",
		"configmap.key1":      "present",
		"configmap.key2":      "present",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.calculateConfigHash(config)
	}
}

func BenchmarkSnapshotManager_ExtractConfiguration(b *testing.B) {
	sm := createTestSnapshotManager(b)

	// Create test resources
	resources := make([]runtime.RawExtension, 10)
	for i := 0; i < 10; i++ {
		resource := map[string]interface{}{
			"kind":       "Deployment",
			"apiVersion": "apps/v1",
			"spec": map[string]interface{}{
				"replicas": float64(3),
			},
		}
		resources[i] = runtime.RawExtension{Object: resource}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.extractConfiguration(resources)
	}
}

// Helper functions

func createTestSnapshotManager(tb testing.TB) *SnapshotManager {
	tb.Helper()
	return createTestSnapshotManagerWithConfig(tb, &EngineConfig{})
}

func createTestSnapshotManagerWithConfig(tb testing.TB, config *EngineConfig) *SnapshotManager {
	tb.Helper()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	cluster := logicalcluster.Name("test")

	return NewSnapshotManager(client, cluster, config)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   (len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}