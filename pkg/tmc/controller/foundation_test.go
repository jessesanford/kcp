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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// mockInformer provides a minimal implementation of SharedIndexInformer for testing
type mockInformer struct {
	handlers []cache.ResourceEventHandler
	indexer  cache.Indexer
	synced   bool
}

func newMockInformer() *mockInformer {
	return &mockInformer{
		handlers: []cache.ResourceEventHandler{},
		indexer:  cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}),
		synced:   true,
	}
}

func (m *mockInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	m.handlers = append(m.handlers, handler)
	return nil, nil
}

func (m *mockInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return m.AddEventHandler(handler)
}

func (m *mockInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (m *mockInformer) GetStore() cache.Store {
	return m.indexer
}

func (m *mockInformer) GetController() cache.Controller {
	return nil
}

func (m *mockInformer) Run(stopCh <-chan struct{}) {
}

func (m *mockInformer) HasSynced() bool {
	return m.synced
}

func (m *mockInformer) LastSyncResourceVersion() string {
	return ""
}

func (m *mockInformer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error {
	return nil
}

func (m *mockInformer) SetTransform(handler cache.TransformFunc) error {
	return nil
}

func (m *mockInformer) IsStopped() bool {
	return false
}

func (m *mockInformer) GetIndexer() cache.Indexer {
	return m.indexer
}

func (m *mockInformer) AddIndexers(indexers cache.Indexers) error {
	return nil
}

// Trigger event handlers for testing
func (m *mockInformer) triggerAdd(obj interface{}) {
	for _, handler := range m.handlers {
		if funcs, ok := handler.(cache.ResourceEventHandlerFuncs); ok && funcs.AddFunc != nil {
			funcs.AddFunc(obj)
		}
	}
}

func (m *mockInformer) triggerUpdate(oldObj, newObj interface{}) {
	for _, handler := range m.handlers {
		if funcs, ok := handler.(cache.ResourceEventHandlerFuncs); ok && funcs.UpdateFunc != nil {
			funcs.UpdateFunc(oldObj, newObj)
		}
	}
}

func (m *mockInformer) triggerDelete(obj interface{}) {
	for _, handler := range m.handlers {
		if funcs, ok := handler.(cache.ResourceEventHandlerFuncs); ok && funcs.DeleteFunc != nil {
			funcs.DeleteFunc(obj)
		}
	}
}

// Mock object for testing
type mockObject struct {
	metav1.ObjectMeta
}

func (m *mockObject) DeepCopyObject() runtime.Object {
	return &mockObject{
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
	}
}

func (m *mockObject) GetObjectKind() schema.ObjectKind {
	return &metav1.TypeMeta{
		APIVersion: "test.example.com/v1",
		Kind:       "MockObject",
	}
}

func TestNewTMCController(t *testing.T) {
	tests := map[string]struct {
		opts        TMCControllerOptions
		wantError   bool
		errorString string
	}{
		"valid options": {
			opts: TMCControllerOptions{
				Name:     "test-controller",
				Informer: newMockInformer(),
				SyncHandler: func(ctx context.Context, key string) error {
					return nil
				},
			},
			wantError: false,
		},
		"missing name": {
			opts: TMCControllerOptions{
				Informer: newMockInformer(),
				SyncHandler: func(ctx context.Context, key string) error {
					return nil
				},
			},
			wantError:   true,
			errorString: "controller name is required",
		},
		"missing informer": {
			opts: TMCControllerOptions{
				Name: "test-controller",
				SyncHandler: func(ctx context.Context, key string) error {
					return nil
				},
			},
			wantError:   true,
			errorString: "informer is required",
		},
		"missing sync handler": {
			opts: TMCControllerOptions{
				Name:     "test-controller",
				Informer: newMockInformer(),
			},
			wantError:   true,
			errorString: "sync handler is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller, err := NewTMCController(tc.opts)

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorString)
				assert.Nil(t, controller)
			} else {
				require.NoError(t, err)
				require.NotNil(t, controller)
				assert.Equal(t, tc.opts.Name, controller.GetName())
			}
		})
	}
}

func TestTMCController_EventHandling(t *testing.T) {
	mockInf := newMockInformer()
	var processedKeys []string
	var mu sync.Mutex

	syncHandler := func(ctx context.Context, key string) error {
		mu.Lock()
		defer mu.Unlock()
		processedKeys = append(processedKeys, key)
		return nil
	}

	controller, err := NewTMCController(TMCControllerOptions{
		Name:        "test-controller",
		Informer:    mockInf,
		SyncHandler: syncHandler,
	})
	require.NoError(t, err)

	// Test that event handlers were registered
	assert.Len(t, mockInf.handlers, 1)

	// Create a test object
	testObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-object",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				"kcp.io/cluster": "test-cluster",
			},
		},
	}

	// Add object to indexer so it can be found by key
	err = mockInf.indexer.Add(testObj)
	require.NoError(t, err)

	// Start controller in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go controller.Start(ctx, 1)

	// Wait a moment for controller to start
	time.Sleep(100 * time.Millisecond)

	// Trigger events
	mockInf.triggerAdd(testObj)
	mockInf.triggerUpdate(testObj, testObj)
	mockInf.triggerDelete(testObj)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check that events were processed
	mu.Lock()
	defer mu.Unlock()
	
	// We should have at least some processed keys
	assert.Greater(t, len(processedKeys), 0)
}

func TestTMCController_SyncHandlerError(t *testing.T) {
	mockInf := newMockInformer()
	var callCount int
	var mu sync.Mutex

	syncHandler := func(ctx context.Context, key string) error {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		return fmt.Errorf("sync error")
	}

	controller, err := NewTMCController(TMCControllerOptions{
		Name:        "test-controller",
		Informer:    mockInf,
		SyncHandler: syncHandler,
	})
	require.NoError(t, err)

	// Start controller in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go controller.Start(ctx, 1)

	// Wait a moment for controller to start
	time.Sleep(100 * time.Millisecond)

	// Create a test object and trigger event
	testObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-object",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				"kcp.io/cluster": "test-cluster",
			},
		},
	}

	mockInf.triggerAdd(testObj)

	// Wait for processing and retries
	time.Sleep(500 * time.Millisecond)

	// Check that sync handler was called multiple times due to retries
	mu.Lock()
	defer mu.Unlock()
	assert.Greater(t, callCount, 1, "Expected multiple calls due to error retry")
}

func TestTMCController_QueueLength(t *testing.T) {
	mockInf := newMockInformer()
	
	// Create a sync handler that never completes to keep items in queue
	syncHandler := func(ctx context.Context, key string) error {
		time.Sleep(100 * time.Millisecond) // Slow processing
		return nil
	}

	controller, err := NewTMCController(TMCControllerOptions{
		Name:        "test-controller",
		Informer:    mockInf,
		SyncHandler: syncHandler,
	})
	require.NoError(t, err)

	// Initially queue should be empty
	assert.Equal(t, 0, controller.GetQueueLength())

	// Add some items to queue by triggering events
	testObj := &mockObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				"kcp.io/cluster": "test",
			},
		},
	}
	
	mockInf.triggerAdd(testObj)
	mockInf.triggerUpdate(testObj, testObj)
	
	// Queue length should increase
	assert.Greater(t, controller.GetQueueLength(), 0)
}