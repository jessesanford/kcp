/*
Copyright 2025 The KCP Authors.

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

package downstream

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSyncResult(t *testing.T) {
	tests := map[string]struct {
		result *SyncResult
		want   *SyncResult
	}{
		"basic sync result": {
			result: &SyncResult{
				Operation: "create",
				Success:   true,
				Error:     nil,
			},
			want: &SyncResult{
				Operation: "create",
				Success:   true,
				Error:     nil,
			},
		},
		"sync result with retry": {
			result: &SyncResult{
				Operation:  "update",
				Success:    false,
				RetryAfter: &[]time.Duration{time.Second * 5}[0],
				Conflicts:  []string{"resource version conflict"},
			},
			want: &SyncResult{
				Operation:  "update",
				Success:    false,
				RetryAfter: &[]time.Duration{time.Second * 5}[0],
				Conflicts:  []string{"resource version conflict"},
			},
		},
		"sync result with changed fields": {
			result: &SyncResult{
				Operation:     "update",
				Success:       true,
				ChangedFields: []string{"spec.replicas", "metadata.labels"},
			},
			want: &SyncResult{
				Operation:     "update",
				Success:       true,
				ChangedFields: []string{"spec.replicas", "metadata.labels"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.result.Operation != tc.want.Operation {
				t.Errorf("Operation: got %s, want %s", tc.result.Operation, tc.want.Operation)
			}
			if tc.result.Success != tc.want.Success {
				t.Errorf("Success: got %v, want %v", tc.result.Success, tc.want.Success)
			}
			if len(tc.result.Conflicts) != len(tc.want.Conflicts) {
				t.Errorf("Conflicts length: got %d, want %d", len(tc.result.Conflicts), len(tc.want.Conflicts))
			}
		})
	}
}

func TestDownstreamConfig(t *testing.T) {
	tests := map[string]struct {
		config *DownstreamConfig
		want   *DownstreamConfig
	}{
		"custom config": {
			config: &DownstreamConfig{
				ConflictRetries:      5,
				UpdateStrategy:       "merge",
				PreserveFields:       []string{"status", "metadata.resourceVersion"},
				IgnoreFields:         []string{"metadata.managedFields", "metadata.generation"},
				DeletionPropagation:  metav1.DeletePropagationForeground,
				ConflictRetryDelay:   time.Second * 10,
			},
			want: &DownstreamConfig{
				ConflictRetries:      5,
				UpdateStrategy:       "merge",
				PreserveFields:       []string{"status", "metadata.resourceVersion"},
				IgnoreFields:         []string{"metadata.managedFields", "metadata.generation"},
				DeletionPropagation:  metav1.DeletePropagationForeground,
				ConflictRetryDelay:   time.Second * 10,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.config.ConflictRetries != tc.want.ConflictRetries {
				t.Errorf("ConflictRetries: got %d, want %d", tc.config.ConflictRetries, tc.want.ConflictRetries)
			}
			if tc.config.UpdateStrategy != tc.want.UpdateStrategy {
				t.Errorf("UpdateStrategy: got %s, want %s", tc.config.UpdateStrategy, tc.want.UpdateStrategy)
			}
			if tc.config.DeletionPropagation != tc.want.DeletionPropagation {
				t.Errorf("DeletionPropagation: got %s, want %s", tc.config.DeletionPropagation, tc.want.DeletionPropagation)
			}
		})
	}
}

func TestDefaultDownstreamConfig(t *testing.T) {
	config := DefaultDownstreamConfig()

	if config == nil {
		t.Fatal("DefaultDownstreamConfig() returned nil")
	}

	expectedDefaults := map[string]interface{}{
		"ConflictRetries":      3,
		"UpdateStrategy":       "strategic-merge",
		"DeletionPropagation":  metav1.DeletePropagationBackground,
		"ConflictRetryDelay":   time.Second * 5,
	}

	if config.ConflictRetries != expectedDefaults["ConflictRetries"].(int) {
		t.Errorf("Default ConflictRetries: got %d, want %d", config.ConflictRetries, expectedDefaults["ConflictRetries"].(int))
	}

	if config.UpdateStrategy != expectedDefaults["UpdateStrategy"].(string) {
		t.Errorf("Default UpdateStrategy: got %s, want %s", config.UpdateStrategy, expectedDefaults["UpdateStrategy"].(string))
	}

	if config.DeletionPropagation != expectedDefaults["DeletionPropagation"].(metav1.DeletionPropagation) {
		t.Errorf("Default DeletionPropagation: got %s, want %s", config.DeletionPropagation, expectedDefaults["DeletionPropagation"].(metav1.DeletionPropagation))
	}

	if config.ConflictRetryDelay != expectedDefaults["ConflictRetryDelay"].(time.Duration) {
		t.Errorf("Default ConflictRetryDelay: got %v, want %v", config.ConflictRetryDelay, expectedDefaults["ConflictRetryDelay"].(time.Duration))
	}

	// Check default preserve fields
	expectedPreserveFields := []string{"status", "metadata.resourceVersion", "metadata.uid", "metadata.creationTimestamp"}
	if len(config.PreserveFields) != len(expectedPreserveFields) {
		t.Errorf("Default PreserveFields length: got %d, want %d", len(config.PreserveFields), len(expectedPreserveFields))
	}

	// Check default ignore fields
	expectedIgnoreFields := []string{"metadata.managedFields"}
	if len(config.IgnoreFields) != len(expectedIgnoreFields) {
		t.Errorf("Default IgnoreFields length: got %d, want %d", len(config.IgnoreFields), len(expectedIgnoreFields))
	}
}

func TestResourceState(t *testing.T) {
	now := metav1.Now()
	conflictTime := metav1.Now()

	tests := map[string]struct {
		state *ResourceState
		want  *ResourceState
	}{
		"complete resource state": {
			state: &ResourceState{
				GVR:              schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespace:        "default",
				Name:             "test-deployment",
				ResourceVersion:  "12345",
				Generation:       1,
				LastSyncTime:     now,
				Hash:             "abc123def456",
				ConflictCount:    2,
				LastConflictTime: &conflictTime,
			},
			want: &ResourceState{
				GVR:              schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespace:        "default",
				Name:             "test-deployment",
				ResourceVersion:  "12345",
				Generation:       1,
				LastSyncTime:     now,
				Hash:             "abc123def456",
				ConflictCount:    2,
				LastConflictTime: &conflictTime,
			},
		},
		"minimal resource state": {
			state: &ResourceState{
				GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Namespace: "kube-system",
				Name:      "test-pod",
			},
			want: &ResourceState{
				GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				Namespace: "kube-system",
				Name:      "test-pod",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.state.GVR != tc.want.GVR {
				t.Errorf("GVR: got %v, want %v", tc.state.GVR, tc.want.GVR)
			}
			if tc.state.Namespace != tc.want.Namespace {
				t.Errorf("Namespace: got %s, want %s", tc.state.Namespace, tc.want.Namespace)
			}
			if tc.state.Name != tc.want.Name {
				t.Errorf("Name: got %s, want %s", tc.state.Name, tc.want.Name)
			}
			if tc.state.ConflictCount != tc.want.ConflictCount {
				t.Errorf("ConflictCount: got %d, want %d", tc.state.ConflictCount, tc.want.ConflictCount)
			}
		})
	}
}

func TestConflictType(t *testing.T) {
	tests := map[string]struct {
		conflictType ConflictType
		want         string
	}{
		"resource version conflict": {
			conflictType: ConflictTypeResourceVersion,
			want:         "ResourceVersion",
		},
		"generation conflict": {
			conflictType: ConflictTypeGeneration,
			want:         "Generation",
		},
		"field conflict": {
			conflictType: ConflictTypeFieldConflict,
			want:         "FieldConflict",
		},
		"deletion conflict": {
			conflictType: ConflictTypeDeletion,
			want:         "Deletion",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if string(tc.conflictType) != tc.want {
				t.Errorf("ConflictType: got %s, want %s", string(tc.conflictType), tc.want)
			}
		})
	}
}

func TestSyncConflict(t *testing.T) {
	tests := map[string]struct {
		conflict *SyncConflict
		want     *SyncConflict
	}{
		"resource version conflict": {
			conflict: &SyncConflict{
				Type:            ConflictTypeResourceVersion,
				Field:           "metadata.resourceVersion",
				UpstreamValue:   "12345",
				DownstreamValue: "12346",
				Resolvable:      true,
				Resolution:      "use-downstream",
			},
			want: &SyncConflict{
				Type:            ConflictTypeResourceVersion,
				Field:           "metadata.resourceVersion",
				UpstreamValue:   "12345",
				DownstreamValue: "12346",
				Resolvable:      true,
				Resolution:      "use-downstream",
			},
		},
		"field conflict": {
			conflict: &SyncConflict{
				Type:            ConflictTypeFieldConflict,
				Field:           "spec.replicas",
				UpstreamValue:   int64(3),
				DownstreamValue: int64(5),
				Resolvable:      false,
				Resolution:      "",
			},
			want: &SyncConflict{
				Type:            ConflictTypeFieldConflict,
				Field:           "spec.replicas",
				UpstreamValue:   int64(3),
				DownstreamValue: int64(5),
				Resolvable:      false,
				Resolution:      "",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.conflict.Type != tc.want.Type {
				t.Errorf("Type: got %s, want %s", tc.conflict.Type, tc.want.Type)
			}
			if tc.conflict.Field != tc.want.Field {
				t.Errorf("Field: got %s, want %s", tc.conflict.Field, tc.want.Field)
			}
			if tc.conflict.Resolvable != tc.want.Resolvable {
				t.Errorf("Resolvable: got %v, want %v", tc.conflict.Resolvable, tc.want.Resolvable)
			}
		})
	}
}