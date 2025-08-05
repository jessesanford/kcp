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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/placement/engine"
)

// Mock implementations for testing

type mockWorkloadPlacementLister struct {
	placements map[string]*tmcv1alpha1.WorkloadPlacement
}

func (m *mockWorkloadPlacementLister) Get(name string) (*tmcv1alpha1.WorkloadPlacement, error) {
	placement, exists := m.placements[name]
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{Resource: "workloadplacements"}, name)
	}
	return placement, nil
}

func (m *mockWorkloadPlacementLister) List() ([]*tmcv1alpha1.WorkloadPlacement, error) {
	var placements []*tmcv1alpha1.WorkloadPlacement
	for _, placement := range m.placements {
		placements = append(placements, placement)
	}
	return placements, nil
}

type mockClusterRegistrationLister struct {
	clusters map[string]*tmcv1alpha1.ClusterRegistration
}

func (m *mockClusterRegistrationLister) Get(name string) (*tmcv1alpha1.ClusterRegistration, error) {
	cluster, exists := m.clusters[name]
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{Resource: "clusterregistrations"}, name)
	}
	return cluster, nil
}

func (m *mockClusterRegistrationLister) List() ([]*tmcv1alpha1.ClusterRegistration, error) {
	var clusters []*tmcv1alpha1.ClusterRegistration
	for _, cluster := range m.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

type mockEventRecorder struct {
	events []string
}

func (m *mockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.events = append(m.events, eventtype+":"+reason+":"+message)
}

func (m *mockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	// Simplified implementation for testing
	m.events = append(m.events, eventtype+":"+reason+":formatted")
}

type mockClusterProvider struct {
	clusters []*engine.ClusterInfo
}

func (m *mockClusterProvider) GetAvailableClusters(ctx context.Context) ([]*engine.ClusterInfo, error) {
	return m.clusters, nil
}

// Test helper functions

func createTestWorkloadPlacement(name string, policy tmcv1alpha1.PlacementPolicy) *tmcv1alpha1.WorkloadPlacement {
	return &tmcv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: tmcv1alpha1.WorkloadPlacementSpec{
			WorkloadSelector: tmcv1alpha1.WorkloadSelector{
				WorkloadTypes: []tmcv1alpha1.WorkloadType{
					{APIVersion: "apps/v1", Kind: "Deployment"},
				},
			},
			ClusterSelector: tmcv1alpha1.ClusterSelector{},
			PlacementPolicy: policy,
		},
	}
}

func createTestClusterRegistration(name, location string, workloadCount int32) *tmcv1alpha1.ClusterRegistration {
	return &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: location,
		},
		Status: tmcv1alpha1.ClusterRegistrationStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
			WorkloadCount: workloadCount,
			LastHeartbeatTime: &metav1.Time{Time: time.Now()},
		},
	}
}

func TestNewWorkloadPlacementController(t *testing.T) {
	placementLister := &mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)}
	clusterLister := &mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)}
	clusterProvider := &mockClusterProvider{}
	eventRecorder := &mockEventRecorder{}

	controller, err := NewWorkloadPlacementController(placementLister, clusterLister, clusterProvider, eventRecorder)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	if controller == nil {
		t.Fatal("Expected non-nil controller")
	}

	if controller.GetName() != WorkloadPlacementControllerName {
		t.Errorf("Expected controller name %s, got %s", WorkloadPlacementControllerName, controller.GetName())
	}
}

func TestSyncWorkloadPlacement_NotFound(t *testing.T) {
	placementLister := &mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)}
	clusterLister := &mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)}
	clusterProvider := &mockClusterProvider{}
	eventRecorder := &mockEventRecorder{}

	controller, err := NewWorkloadPlacementController(placementLister, clusterLister, clusterProvider, eventRecorder)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Test with non-existent placement
	err = controller.syncWorkloadPlacement(context.Background(), "default/non-existent")
	if err != nil {
		t.Errorf("Expected no error for non-existent placement, got: %v", err)
	}
}

func TestSyncWorkloadPlacement_Success(t *testing.T) {
	placement := createTestWorkloadPlacement("test-placement", tmcv1alpha1.PlacementPolicyRoundRobin)
	// Pre-add finalizer so placement decision happens on first call
	placement.Finalizers = []string{"workloadplacement.tmc.kcp.io/controller"}
	placementLister := &mockWorkloadPlacementLister{
		placements: map[string]*tmcv1alpha1.WorkloadPlacement{
			"test-placement": placement,
		},
	}

	cluster := createTestClusterRegistration("cluster1", "us-west-1", 5)
	clusterLister := &mockClusterRegistrationLister{
		clusters: map[string]*tmcv1alpha1.ClusterRegistration{
			"cluster1": cluster,
		},
	}

	clusterProvider := &mockClusterProvider{
		clusters: []*engine.ClusterInfo{
			{
				Name:         "cluster1",
				Location:     "us-west-1",
				WorkloadCount: 5,
				Available:    true,
			},
		},
	}

	eventRecorder := &mockEventRecorder{}

	controller, err := NewWorkloadPlacementController(placementLister, clusterLister, clusterProvider, eventRecorder)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Test successful placement
	err = controller.syncWorkloadPlacement(context.Background(), "default/test-placement")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that success event was recorded
	found := false
	for _, event := range eventRecorder.events {
		if event == "Normal:PlacementSucceeded:Successfully determined cluster placement" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected success event to be recorded. Got events: %v", eventRecorder.events)
	}
}

func TestPerformPlacement(t *testing.T) {
	placement := createTestWorkloadPlacement("test-placement", tmcv1alpha1.PlacementPolicyRoundRobin)
	
	clusterProvider := &mockClusterProvider{
		clusters: []*engine.ClusterInfo{
			{Name: "cluster1", Location: "us-west-1", WorkloadCount: 3},
			{Name: "cluster2", Location: "us-west-2", WorkloadCount: 5},
		},
	}

	controller, err := NewWorkloadPlacementController(
		&mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)},
		&mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)},
		clusterProvider,
		&mockEventRecorder{},
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	err = controller.performPlacement(context.Background(), placement)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that clusters were selected
	if len(placement.Status.SelectedClusters) == 0 {
		t.Error("Expected clusters to be selected")
	}

	// Check that placement time was set
	if placement.Status.LastPlacementTime == nil {
		t.Error("Expected placement time to be set")
	}

	// Check that history was recorded
	if len(placement.Status.PlacementHistory) == 0 {
		t.Error("Expected placement history to be recorded")
	}
}

func TestPerformPlacement_NoAvailableClusters(t *testing.T) {
	placement := createTestWorkloadPlacement("test-placement", tmcv1alpha1.PlacementPolicyRoundRobin)
	
	clusterProvider := &mockClusterProvider{
		clusters: []*engine.ClusterInfo{}, // No clusters available
	}

	controller, err := NewWorkloadPlacementController(
		&mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)},
		&mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)},
		clusterProvider,
		&mockEventRecorder{},
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	err = controller.performPlacement(context.Background(), placement)
	if err == nil {
		t.Error("Expected error when no clusters are available")
	}
}

func TestFinalizerHandling(t *testing.T) {
	placement := createTestWorkloadPlacement("test-placement", tmcv1alpha1.PlacementPolicyRoundRobin)
	
	controller, err := NewWorkloadPlacementController(
		&mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)},
		&mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)},
		&mockClusterProvider{},
		&mockEventRecorder{},
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Initially no finalizer
	if controller.hasFinalizer(placement) {
		t.Error("Expected no finalizer initially")
	}

	// Add finalizer
	controller.addFinalizer(placement)
	if !controller.hasFinalizer(placement) {
		t.Error("Expected finalizer to be added")
	}

	// Remove finalizer
	controller.removeFinalizer(placement)
	if controller.hasFinalizer(placement) {
		t.Error("Expected finalizer to be removed")
	}
}

func TestPlacementHistoryLimit(t *testing.T) {
	placement := createTestWorkloadPlacement("test-placement", tmcv1alpha1.PlacementPolicyRoundRobin)
	
	// Add 12 history entries to test the limit
	for i := 0; i < 12; i++ {
		entry := tmcv1alpha1.PlacementHistoryEntry{
			Timestamp: metav1.Time{Time: time.Now().Add(time.Duration(i) * time.Minute)},
			Policy:    tmcv1alpha1.PlacementPolicyRoundRobin,
			SelectedClusters: []string{"cluster1"},
			Reason:    "test",
		}
		placement.Status.PlacementHistory = append(placement.Status.PlacementHistory, entry)
	}

	clusterProvider := &mockClusterProvider{
		clusters: []*engine.ClusterInfo{
			{Name: "cluster1", Location: "us-west-1"},
		},
	}

	controller, err := NewWorkloadPlacementController(
		&mockWorkloadPlacementLister{placements: make(map[string]*tmcv1alpha1.WorkloadPlacement)},
		&mockClusterRegistrationLister{clusters: make(map[string]*tmcv1alpha1.ClusterRegistration)},
		clusterProvider,
		&mockEventRecorder{},
	)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	err = controller.performPlacement(context.Background(), placement)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should be limited to 10 entries
	if len(placement.Status.PlacementHistory) != 10 {
		t.Errorf("Expected 10 history entries, got %d", len(placement.Status.PlacementHistory))
	}
}