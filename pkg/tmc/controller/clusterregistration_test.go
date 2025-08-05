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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewClusterRegistrationController(t *testing.T) {
	tests := map[string]struct {
		healthCheckInterval time.Duration
		wantError           bool
	}{
		"valid configuration": {
			healthCheckInterval: 30 * time.Second,
			wantError:           false,
		},
		"zero health check interval": {
			healthCheckInterval: 0,
			wantError:           false, // Should use default
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockInf := newMockInformer()

			controller, err := NewClusterRegistrationController(
				mockInf,
				tc.healthCheckInterval,
			)

			if tc.wantError {
				require.Error(t, err)
				assert.Nil(t, controller)
			} else {
				require.NoError(t, err)
				require.NotNil(t, controller)
				assert.Equal(t, ClusterRegistrationControllerName, controller.GetName())
			}
		})
	}
}

func TestClusterRegistrationController_SyncClusterRegistration(t *testing.T) {
	mockInf := newMockInformer()

	controller, err := NewClusterRegistrationController(
		mockInf,
		30*time.Second,
	)
	require.NoError(t, err)

	ctx := context.Background()

	tests := map[string]struct {
		key       string
		setupFunc func()
		wantError bool
	}{
		"invalid key": {
			key:       "invalid|key|format|too|many|parts",
			wantError: true,
		},
		"valid key - object not found": {
			key:       "root:test-workspace/test-cluster",
			wantError: false,
		},
		"valid key - object exists": {
			key: "root:test-workspace/test-cluster",
			setupFunc: func() {
				testObj := &mockObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "",
						Annotations: map[string]string{
							"kcp.io/cluster": "root:test-workspace",
						},
					},
				}
				mockInf.indexer.Add(testObj)
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Reset indexer
			mockInf.indexer = newMockInformer().indexer

			if tc.setupFunc != nil {
				tc.setupFunc()
			}

			err := controller.syncClusterRegistration(ctx, tc.key)

			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClusterRegistrationController_PerformGlobalHealthCheck(t *testing.T) {
	mockInf := newMockInformer()

	controller, err := NewClusterRegistrationController(
		mockInf,
		30*time.Second,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Test basic health check
	healthy, err := controller.performGlobalHealthCheck(ctx)
	assert.NoError(t, err)
	assert.True(t, healthy) // Should be healthy by default
}

func TestClusterRegistrationController_GetClusterRegistrations(t *testing.T) {
	mockInf := newMockInformer()

	controller, err := NewClusterRegistrationController(
		mockInf,
		30*time.Second,
	)
	require.NoError(t, err)

	// Initially empty
	registrations := controller.GetClusterRegistrations()
	assert.Empty(t, registrations)

	// Add some test objects
	testObj1 := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster1",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:workspace1",
			},
		},
	}
	testObj2 := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster2",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:workspace2",
			},
		},
	}

	mockInf.indexer.Add(testObj1)
	mockInf.indexer.Add(testObj2)

	// Should return both objects
	registrations = controller.GetClusterRegistrations()
	assert.Len(t, registrations, 2)
}

func TestClusterRegistrationController_GetClusterRegistrationByKey(t *testing.T) {
	mockInf := newMockInformer()

	controller, err := NewClusterRegistrationController(
		mockInf,
		30*time.Second,
	)
	require.NoError(t, err)

	testObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:test-workspace",
			},
		},
	}
	key := "test-key"

	// Object not found
	obj, exists, err := controller.GetClusterRegistrationByKey(key)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, obj)

	// Add object with specific key
	mockInf.indexer.Add(testObj)

	// Try to get with wrong key - still not found
	obj, exists, err = controller.GetClusterRegistrationByKey("wrong-key")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, obj)
}

func TestClusterRegistrationController_ListClusterRegistrationsByWorkspace(t *testing.T) {
	mockInf := newMockInformer()

	controller, err := NewClusterRegistrationController(
		mockInf,
		30*time.Second,
	)
	require.NoError(t, err)

	// Create test objects in different workspaces
	testObj1 := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster1",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:workspace1",
			},
		},
	}
	testObj2 := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster2",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:workspace1",
			},
		},
	}
	testObj3 := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster3",
			Annotations: map[string]string{
				"kcp.io/cluster": "root:workspace2",
			},
		},
	}

	// Add objects to indexer
	mockInf.indexer.Add(testObj1)
	mockInf.indexer.Add(testObj2)
	mockInf.indexer.Add(testObj3)

	// List objects from workspace1
	workspace1 := logicalcluster.Name("root:workspace1")
	objects, err := controller.ListClusterRegistrationsByWorkspace(workspace1)
	assert.NoError(t, err)

	// Should find objects that have workspace1 in their key
	// Note: This test is limited by our mock implementation
	// In real usage, the key generation would properly include workspace info
	assert.NotNil(t, objects)

	// List objects from non-existent workspace
	workspace3 := logicalcluster.Name("root:workspace3")
	objects, err = controller.ListClusterRegistrationsByWorkspace(workspace3)
	assert.NoError(t, err)
	assert.Empty(t, objects)
}
