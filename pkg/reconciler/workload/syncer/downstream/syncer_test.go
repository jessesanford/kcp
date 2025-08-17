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
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestNewSyncer(t *testing.T) {
	tests := map[string]struct {
		workspace        logicalcluster.Name
		kcpClient        kcpclientset.ClusterInterface
		downstreamClient dynamic.Interface
		syncTarget       *SyncTarget
		config           *DownstreamConfig
		wantError        bool
		wantConfig       *DownstreamConfig
	}{
		"valid syncer creation": {
			workspace:        logicalcluster.Name("root:test"),
			kcpClient:        nil, // Mock client
			downstreamClient: fake.NewSimpleDynamicClient(runtime.NewScheme()),
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			config:     nil, // Should use default config
			wantError:  false,
			wantConfig: DefaultDownstreamConfig(),
		},
		"syncer with custom config": {
			workspace:        logicalcluster.Name("root:test"),
			kcpClient:        nil,
			downstreamClient: fake.NewSimpleDynamicClient(runtime.NewScheme()),
			syncTarget: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			config: &DownstreamConfig{
				ConflictRetries:     5,
				UpdateStrategy:      "merge",
				ConflictRetryDelay:  time.Second * 10,
			},
			wantError: false,
			wantConfig: &DownstreamConfig{
				ConflictRetries:     5,
				UpdateStrategy:      "merge",
				ConflictRetryDelay:  time.Second * 10,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncer, err := NewSyncer(tc.workspace, tc.kcpClient, tc.downstreamClient, tc.syncTarget, tc.config)
			
			if tc.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.wantError {
				return
			}

			// Verify syncer fields
			if syncer.workspace != tc.workspace {
				t.Errorf("workspace = %v, want %v", syncer.workspace, tc.workspace)
			}
			if syncer.downstreamClient != tc.downstreamClient {
				t.Error("downstreamClient not set correctly")
			}
			if syncer.syncTarget != tc.syncTarget {
				t.Error("syncTarget not set correctly")
			}

			// Verify config
			if tc.config == nil {
				// Should use default config
				if syncer.config.ConflictRetries != DefaultDownstreamConfig().ConflictRetries {
					t.Errorf("Default config not used correctly")
				}
			} else {
				if syncer.config.ConflictRetries != tc.config.ConflictRetries {
					t.Errorf("Custom config not used correctly")
				}
			}

			// Verify state cache is initialized
			if syncer.stateCache == nil {
				t.Error("stateCache not initialized")
			}
		})
	}
}

func TestApplyToDownstream(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	
	tests := map[string]struct {
		obj               *unstructured.Unstructured
		existingObj       *unstructured.Unstructured
		setupReactors     func(*fake.FakeDynamicClient)
		wantOperation     string
		wantSuccess       bool
		wantError         bool
	}{
		"create new object": {
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			existingObj: nil, // Object doesn't exist downstream
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewNotFound(gvr.GroupResource(), "test-deployment")
				})
				client.PrependReactor("create", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					createAction := action.(ktesting.CreateAction)
					obj := createAction.GetObject().(*unstructured.Unstructured)
					obj.SetResourceVersion("1")
					obj.SetUID("new-uid")
					return true, obj, nil
				})
			},
			wantOperation: "create",
			wantSuccess:   true,
			wantError:     false,
		},
		"update existing object": {
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
			existingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":            "test-deployment",
						"namespace":       "default",
						"resourceVersion": "1",
						"uid":             "existing-uid",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name":            "test-deployment",
								"namespace":       "default",
								"resourceVersion": "1",
								"uid":             "existing-uid",
							},
							"spec": map[string]interface{}{
								"replicas": int64(3),
							},
						},
					}, nil
				})
				client.PrependReactor("update", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					updateAction := action.(ktesting.UpdateAction)
					obj := updateAction.GetObject().(*unstructured.Unstructured)
					obj.SetResourceVersion("2")
					return true, obj, nil
				})
			},
			wantOperation: "update",
			wantSuccess:   true,
			wantError:     false,
		},
		"get error": {
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("get error")
				})
			},
			wantOperation: "noop",
			wantSuccess:   false,
			wantError:     true,
		},
		"create error": {
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewNotFound(gvr.GroupResource(), "test-deployment")
				})
				client.PrependReactor("create", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("create error")
				})
			},
			wantOperation: "create",
			wantSuccess:   false,
			wantError:     true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClient(scheme)
			if tc.setupReactors != nil {
				tc.setupReactors(client)
			}

			syncer := &Syncer{
				workspace:        logicalcluster.Name("root:test"),
				downstreamClient: client,
				syncTarget: &SyncTarget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
				},
				config: DefaultDownstreamConfig(),
				stateCache: make(map[string]*ResourceState),
				transformer: NewPipeline(logicalcluster.Name("root:test")),
			}

			result, err := syncer.ApplyToDownstream(context.Background(), gvr, tc.obj)

			// Verify error expectations
			if tc.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify result
			if result.Operation != tc.wantOperation {
				t.Errorf("Operation = %s, want %s", result.Operation, tc.wantOperation)
			}
			if result.Success != tc.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tc.wantSuccess)
			}
		})
	}
}

func TestDeleteFromDownstream(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	
	tests := map[string]struct {
		namespace     string
		name          string
		existingObj   *unstructured.Unstructured
		setupReactors func(*fake.FakeDynamicClient)
		wantSuccess   bool
		wantError     bool
	}{
		"delete existing object": {
			namespace: "default",
			name:      "test-deployment",
			existingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name":      "test-deployment",
								"namespace": "default",
							},
						},
					}, nil
				})
				client.PrependReactor("delete", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, nil
				})
			},
			wantSuccess: true,
			wantError:   false,
		},
		"delete non-existent object": {
			namespace: "default",
			name:      "missing-deployment",
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewNotFound(gvr.GroupResource(), "missing-deployment")
				})
			},
			wantSuccess: true,
			wantError:   false,
		},
		"get error": {
			namespace: "default",
			name:      "test-deployment",
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("get error")
				})
			},
			wantSuccess: false,
			wantError:   true,
		},
		"delete error": {
			namespace: "default",
			name:      "test-deployment",
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.Unstructured{}, nil
				})
				client.PrependReactor("delete", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("delete error")
				})
			},
			wantSuccess: false,
			wantError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClient(scheme)
			if tc.setupReactors != nil {
				tc.setupReactors(client)
			}

			syncer := &Syncer{
				workspace:        logicalcluster.Name("root:test"),
				downstreamClient: client,
				config:           DefaultDownstreamConfig(),
				stateCache:       make(map[string]*ResourceState),
			}

			result, err := syncer.DeleteFromDownstream(context.Background(), gvr, tc.namespace, tc.name)

			// Verify error expectations
			if tc.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify result
			if result.Operation != "delete" {
				t.Errorf("Operation = %s, want delete", result.Operation)
			}
			if result.Success != tc.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tc.wantSuccess)
			}
		})
	}
}

func TestUpdateWithConflictResolution(t *testing.T) {
	scheme := runtime.NewScheme()
	
	tests := map[string]struct {
		config        *DownstreamConfig
		setupReactors func(*fake.FakeDynamicClient)
		wantSuccess   bool
		wantRetries   bool
		wantError     bool
	}{
		"successful update": {
			config: &DownstreamConfig{ConflictRetries: 3, ConflictRetryDelay: time.Millisecond * 10},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("update", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					updateAction := action.(ktesting.UpdateAction)
					obj := updateAction.GetObject().(*unstructured.Unstructured)
					obj.SetResourceVersion("2")
					return true, obj, nil
				})
			},
			wantSuccess: true,
			wantError:   false,
		},
		"conflict resolution with retry": {
			config: &DownstreamConfig{ConflictRetries: 3, ConflictRetryDelay: time.Millisecond * 10},
			setupReactors: func(client *fake.FakeDynamicClient) {
				attempts := 0
				client.PrependReactor("update", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					attempts++
					if attempts < 2 {
						// First attempt fails with conflict
						return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test", errors.New("conflict"))
					}
					// Second attempt succeeds
					updateAction := action.(ktesting.UpdateAction)
					obj := updateAction.GetObject().(*unstructured.Unstructured)
					obj.SetResourceVersion("3")
					return true, obj, nil
				})
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"name":            "test",
								"resourceVersion": "2",
							},
						},
					}, nil
				})
			},
			wantSuccess: true,
			wantRetries: true,
			wantError:   false,
		},
		"exceeded conflict retries": {
			config: &DownstreamConfig{ConflictRetries: 1, ConflictRetryDelay: time.Millisecond * 10},
			setupReactors: func(client *fake.FakeDynamicClient) {
				client.PrependReactor("update", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "test", errors.New("conflict"))
				})
				client.PrependReactor("get", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"name":            "test",
								"resourceVersion": "2",
							},
						},
					}, nil
				})
			},
			wantSuccess: false,
			wantError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleDynamicClient(scheme)
			if tc.setupReactors != nil {
				tc.setupReactors(client)
			}

			syncer := &Syncer{
				config: tc.config,
			}

			desired := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			}

			existing := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":            "test",
						"resourceVersion": "1",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			}

			downstreamResource := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"})
			resourceClient := downstreamResource.Namespace("default")

			_, result, err := syncer.updateWithConflictResolution(context.Background(), resourceClient, desired, existing)

			// Verify error expectations
			if tc.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify result
			if result.Success != tc.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tc.wantSuccess)
			}

			if tc.wantRetries && len(result.Conflicts) == 0 {
				t.Error("Expected conflicts to be recorded during retries")
			}
		})
	}
}

func TestGenerateStateKey(t *testing.T) {
	tests := map[string]struct {
		gvr       schema.GroupVersionResource
		namespace string
		name      string
		want      string
	}{
		"namespaced resource": {
			gvr:       schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			namespace: "default",
			name:      "test-deployment",
			want:      "apps/v1, Resource=deployments/default/test-deployment",
		},
		"cluster-scoped resource": {
			gvr:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			namespace: "",
			name:      "test-node",
			want:      "/v1, Resource=nodes//test-node",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncer := &Syncer{}
			result := syncer.generateStateKey(tc.gvr, tc.namespace, tc.name)
			if result != tc.want {
				t.Errorf("generateStateKey() = %s, want %s", result, tc.want)
			}
		})
	}
}

func TestUpdateStateCache(t *testing.T) {
	syncer := &Syncer{
		stateCache: make(map[string]*ResourceState),
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":            "test-deployment",
				"namespace":       "default",
				"resourceVersion": "123",
				"generation":      int64(2),
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	key := "test-key"
	syncer.updateStateCache(key, gvr, obj)

	// Verify state was cached
	state, exists := syncer.stateCache[key]
	if !exists {
		t.Fatal("State not cached")
	}

	if state.GVR != gvr {
		t.Errorf("GVR = %v, want %v", state.GVR, gvr)
	}
	if state.Name != "test-deployment" {
		t.Errorf("Name = %s, want test-deployment", state.Name)
	}
	if state.Namespace != "default" {
		t.Errorf("Namespace = %s, want default", state.Namespace)
	}
	if state.ResourceVersion != "123" {
		t.Errorf("ResourceVersion = %s, want 123", state.ResourceVersion)
	}
	if state.Generation != 2 {
		t.Errorf("Generation = %d, want 2", state.Generation)
	}
	if state.Hash == "" {
		t.Error("Hash should not be empty")
	}
}

func TestRemoveFromStateCache(t *testing.T) {
	syncer := &Syncer{
		stateCache: map[string]*ResourceState{
			"test-key": {
				Name: "test-deployment",
			},
		},
	}

	// Verify initial state
	if _, exists := syncer.stateCache["test-key"]; !exists {
		t.Fatal("Initial state not found")
	}

	syncer.removeFromStateCache("test-key")

	// Verify removal
	if _, exists := syncer.stateCache["test-key"]; exists {
		t.Error("State was not removed from cache")
	}
}

func TestGenerateObjectHash(t *testing.T) {
	tests := map[string]struct {
		obj1 *unstructured.Unstructured
		obj2 *unstructured.Unstructured
		want string
	}{
		"identical objects": {
			obj1: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			obj2: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			want: "same", // We expect same hash for identical content
		},
		"different objects": {
			obj1: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			obj2: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
			want: "different", // We expect different hash for different content
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncer := &Syncer{}
			
			hash1 := syncer.generateObjectHash(tc.obj1)
			hash2 := syncer.generateObjectHash(tc.obj2)

			if tc.want == "same" && hash1 != hash2 {
				t.Errorf("Expected same hash for identical objects: %s != %s", hash1, hash2)
			}
			if tc.want == "different" && hash1 == hash2 {
				t.Errorf("Expected different hash for different objects: %s == %s", hash1, hash2)
			}

			// Verify hash is consistent
			hash1Again := syncer.generateObjectHash(tc.obj1)
			if hash1 != hash1Again {
				t.Error("Hash generation is not consistent")
			}
		})
	}
}

func TestGenerateObjectHashIgnoresTransientFields(t *testing.T) {
	syncer := &Syncer{}

	obj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":            "test",
				"resourceVersion": "1",
				"generation":      int64(1),
				"managedFields":   []interface{}{},
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	obj2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":            "test",
				"resourceVersion": "2", // Different resource version
				"generation":      int64(2), // Different generation
				"managedFields":   []interface{}{map[string]interface{}{"manager": "test"}}, // Different managed fields
			},
			"spec": map[string]interface{}{
				"replicas": int64(3), // Same spec
			},
			"status": map[string]interface{}{
				"phase":    "Running",
				"replicas": int64(2), // Different status
			},
		},
	}

	hash1 := syncer.generateObjectHash(obj1)
	hash2 := syncer.generateObjectHash(obj2)

	// Hashes should be the same because we ignore transient fields
	if hash1 != hash2 {
		t.Errorf("Expected same hash when only transient fields differ: %s != %s", hash1, hash2)
	}
}

// Tests use the actual Pipeline implementation

// Additional tests for edge cases
func TestSyncerEdgeCases(t *testing.T) {
	t.Run("empty workspace name", func(t *testing.T) {
		syncer, err := NewSyncer(
			logicalcluster.Name(""),
			nil,
			fake.NewSimpleDynamicClient(runtime.NewScheme()),
			&SyncTarget{},
			nil,
		)
		
		if err != nil {
			t.Errorf("Unexpected error with empty workspace: %v", err)
		}
		if syncer.workspace != logicalcluster.Name("") {
			t.Error("Empty workspace not handled correctly")
		}
	})

	t.Run("nil sync target", func(t *testing.T) {
		syncer, err := NewSyncer(
			logicalcluster.Name("root:test"),
			nil,
			fake.NewSimpleDynamicClient(runtime.NewScheme()),
			nil,
			nil,
		)
		
		if err != nil {
			t.Errorf("Unexpected error with nil sync target: %v", err)
		}
		if syncer.syncTarget != nil {
			t.Error("Nil sync target not handled correctly")
		}
	})
}

func TestSyncerConcurrency(t *testing.T) {
	syncer := &Syncer{
		stateCache: make(map[string]*ResourceState),
	}

	// Test concurrent access to state cache
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
		},
	}

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			key := fmt.Sprintf("key-%d", index)
			syncer.updateStateCache(key, gvr, obj)
			syncer.removeFromStateCache(key)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or race
}