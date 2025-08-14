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

package upstream

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	kcpfakedynamic "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	workloadv1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"
)

func TestNewController(t *testing.T) {
	tests := map[string]struct {
		kcpClient     interface{}
		syncInterval  time.Duration
		numWorkers    int
		expectedError bool
	}{
		"valid configuration": {
			kcpClient:     kcpfakedynamic.NewSimpleClientset(),
			syncInterval:  30 * time.Second,
			numWorkers:    2,
			expectedError: false,
		},
		"nil kcp client": {
			kcpClient:     nil,
			syncInterval:  30 * time.Second,
			numWorkers:    2,
			expectedError: true,
		},
		"zero sync interval uses default": {
			kcpClient:     kcpfakedynamic.NewSimpleClientset(),
			syncInterval:  0,
			numWorkers:    2,
			expectedError: false,
		},
		"zero workers uses default": {
			kcpClient:     kcpfakedynamic.NewSimpleClientset(),
			syncInterval:  30 * time.Second,
			numWorkers:    0,
			expectedError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a mock informer
			informer := &mockSyncTargetInformer{}

			var kcpClient interface{}
			if tc.kcpClient != nil {
				kcpClient = tc.kcpClient
			}

			controller, err := NewController(
				kcpClient,
				informer,
				tc.syncInterval,
				tc.numWorkers,
			)

			if tc.expectedError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if controller == nil {
				t.Fatal("controller should not be nil")
			}

			// Verify default values are applied
			if tc.syncInterval <= 0 && controller.syncInterval != DefaultSyncInterval {
				t.Errorf("expected default sync interval %v, got %v", DefaultSyncInterval, controller.syncInterval)
			}

			if tc.numWorkers <= 0 && controller.numWorkers != DefaultNumWorkers {
				t.Errorf("expected default num workers %v, got %v", DefaultNumWorkers, controller.numWorkers)
			}
		})
	}
}

func TestUpstreamSyncController_Start(t *testing.T) {
	informer := &mockSyncTargetInformer{}
	kcpClient := kcpfakedynamic.NewSimpleClientset()

	controller, err := NewController(kcpClient, informer, 100*time.Millisecond, 1)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	// Test controller starts and stops cleanly
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.Start(ctx)
	}()

	// Wait for context timeout
	select {
	case <-done:
		// Controller stopped cleanly
	case <-time.After(300 * time.Millisecond):
		t.Error("controller did not stop within timeout")
	}
}

func TestUpstreamSyncController_EnqueueSyncTarget(t *testing.T) {
	informer := &mockSyncTargetInformer{}
	kcpClient := kcpfakedynamic.NewSimpleClientset()

	controller, err := NewController(kcpClient, informer, DefaultSyncInterval, 1)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
		},
	}

	// Test enqueuing a sync target
	controller.enqueueSyncTarget(syncTarget)

	// Verify work item was added to queue
	if controller.queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", controller.queue.Len())
	}

	// Get and verify the work item
	item, quit := controller.queue.Get()
	if quit {
		t.Fatal("queue should not be shut down")
	}
	defer controller.queue.Done(item)

	workItem, ok := item.(*WorkItem)
	if !ok {
		t.Fatalf("expected WorkItem, got %T", item)
	}

	if workItem.Action != ActionSync {
		t.Errorf("expected action %v, got %v", ActionSync, workItem.Action)
	}

	if workItem.Key.Name != "test-sync-target" {
		t.Errorf("expected name %v, got %v", "test-sync-target", workItem.Key.Name)
	}
}

func TestUpstreamSyncController_EnqueueSyncTargetDelete(t *testing.T) {
	informer := &mockSyncTargetInformer{}
	kcpClient := kcpfakedynamic.NewSimpleClientset()

	controller, err := NewController(kcpClient, informer, DefaultSyncInterval, 1)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sync-target",
		},
	}

	// Test enqueuing a sync target for deletion
	controller.enqueueSyncTargetDelete(syncTarget)

	// Verify work item was added to queue
	if controller.queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", controller.queue.Len())
	}

	// Get and verify the work item
	item, quit := controller.queue.Get()
	if quit {
		t.Fatal("queue should not be shut down")
	}
	defer controller.queue.Done(item)

	workItem, ok := item.(*WorkItem)
	if !ok {
		t.Fatalf("expected WorkItem, got %T", item)
	}

	if workItem.Action != ActionDelete {
		t.Errorf("expected action %v, got %v", ActionDelete, workItem.Action)
	}
}

func TestUpstreamSyncController_IsSyncTargetReady(t *testing.T) {
	tests := map[string]struct {
		syncTarget    *workloadv1alpha1.SyncTarget
		expectedReady bool
	}{
		"ready sync target": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetReady,
							Status: "True",
						},
					},
				},
			},
			expectedReady: true,
		},
		"not ready sync target": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetReady,
							Status: "False",
						},
					},
				},
			},
			expectedReady: false,
		},
		"no conditions": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{},
				},
			},
			expectedReady: false,
		},
		"wrong condition type": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetSyncerReady,
							Status: "True",
						},
					},
				},
			},
			expectedReady: false,
		},
	}

	informer := &mockSyncTargetInformer{}
	kcpClient := kcpfakedynamic.NewSimpleClientset()

	controller, err := NewController(kcpClient, informer, DefaultSyncInterval, 1)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ready := controller.isSyncTargetReady(tc.syncTarget)

			if ready != tc.expectedReady {
				t.Errorf("expected ready=%v, got ready=%v", tc.expectedReady, ready)
			}
		})
	}
}

func TestUpstreamSyncController_UpdateSyncTargetStatus(t *testing.T) {
	informer := &mockSyncTargetInformer{}
	kcpClient := kcpfakedynamic.NewSimpleClientset()

	controller, err := NewController(kcpClient, informer, DefaultSyncInterval, 1)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	key := SyncTargetKey{
		Cluster: logicalcluster.Name("test-cluster"),
		Name:    "test-sync-target",
	}

	// Test successful sync result
	successResult := &SyncResult{
		Success:   true,
		Timestamp: time.Now(),
	}

	controller.updateSyncTargetStatus(key, successResult)

	status, exists := controller.GetSyncTargetStatus(key)
	if !exists {
		t.Fatal("expected status to exist")
	}

	if status.SyncCount != 1 {
		t.Errorf("expected sync count 1, got %d", status.SyncCount)
	}

	if status.ErrorCount != 0 {
		t.Errorf("expected error count 0, got %d", status.ErrorCount)
	}

	if status.LastSync.Success != true {
		t.Errorf("expected last sync success true, got %v", status.LastSync.Success)
	}

	// Test error result
	errorResult := &SyncResult{
		Success:   false,
		Error:     fmt.Errorf("test error"),
		Timestamp: time.Now(),
	}

	controller.updateSyncTargetStatus(key, errorResult)

	status, exists = controller.GetSyncTargetStatus(key)
	if !exists {
		t.Fatal("expected status to exist")
	}

	if status.SyncCount != 2 {
		t.Errorf("expected sync count 2, got %d", status.SyncCount)
	}

	if status.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", status.ErrorCount)
	}

	if status.LastErrorTime == nil {
		t.Error("expected last error time to be set")
	}
}

func TestSyncTargetKey_String(t *testing.T) {
	key := SyncTargetKey{
		Cluster: logicalcluster.Name("test-cluster"),
		Name:    "test-sync-target",
	}

	expected := "test-cluster/test-sync-target"
	if key.String() != expected {
		t.Errorf("expected %v, got %v", expected, key.String())
	}
}

// Mock implementations for testing

type mockSyncTargetInformer struct{}

func (m *mockSyncTargetInformer) Informer() cache.SharedIndexInformer {
	return &mockSharedIndexInformer{}
}

func (m *mockSyncTargetInformer) Lister() workloadv1alpha1listers.SyncTargetClusterLister {
	return &mockSyncTargetLister{}
}

type mockSharedIndexInformer struct{}

func (m *mockSharedIndexInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (m *mockSharedIndexInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (m *mockSharedIndexInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (m *mockSharedIndexInformer) GetStore() cache.Store {
	return nil
}

func (m *mockSharedIndexInformer) GetController() cache.Controller {
	return nil
}

func (m *mockSharedIndexInformer) Run(stopCh <-chan struct{}) {}

func (m *mockSharedIndexInformer) HasSynced() bool {
	return true
}

func (m *mockSharedIndexInformer) LastSyncResourceVersion() string {
	return ""
}

func (m *mockSharedIndexInformer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error {
	return nil
}

func (m *mockSharedIndexInformer) SetTransform(handler cache.TransformFunc) error {
	return nil
}

func (m *mockSharedIndexInformer) IsStopped() bool {
	return false
}

func (m *mockSharedIndexInformer) AddIndexers(indexers cache.Indexers) error {
	return nil
}

func (m *mockSharedIndexInformer) GetIndexer() cache.Indexer {
	return nil
}

type mockSyncTargetLister struct{}

func (m *mockSyncTargetLister) List(selector labels.Selector) ([]*workloadv1alpha1.SyncTarget, error) {
	return nil, nil
}

func (m *mockSyncTargetLister) Cluster(clusterName logicalcluster.Name) workloadv1alpha1listers.SyncTargetLister {
	return &mockSingleClusterSyncTargetLister{}
}

type mockSingleClusterSyncTargetLister struct{}

func (m *mockSingleClusterSyncTargetLister) List(selector labels.Selector) ([]*workloadv1alpha1.SyncTarget, error) {
	return nil, nil
}

func (m *mockSingleClusterSyncTargetLister) Get(name string) (*workloadv1alpha1.SyncTarget, error) {
	return nil, fmt.Errorf("not found")
}