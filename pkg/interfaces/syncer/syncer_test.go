package syncer_test

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/kcp-dev/kcp/pkg/interfaces/syncer"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// Ensure interfaces can be implemented
type testSyncer struct{}

var _ syncer.Syncer = &testSyncer{}

func (t *testSyncer) Start(ctx context.Context) error { return nil }
func (t *testSyncer) Stop() error                     { return nil }
func (t *testSyncer) GetSyncTarget() *workloadv1alpha1.SyncTarget {
	return nil
}
func (t *testSyncer) GetCapabilities() syncer.Capabilities {
	return syncer.Capabilities{}
}
func (t *testSyncer) RegisterResource(gvr schema.GroupVersionResource) error   { return nil }
func (t *testSyncer) UnregisterResource(gvr schema.GroupVersionResource) error { return nil }
func (t *testSyncer) GetStatus() syncer.Status                                 { return syncer.Status{} }

type testResourceSyncer struct{}

var _ syncer.ResourceSyncer = &testResourceSyncer{}

func (t *testResourceSyncer) Sync(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) error {
	return nil
}
func (t *testResourceSyncer) Delete(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) error {
	return nil
}
func (t *testResourceSyncer) GetStatus(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return nil, nil
}
func (t *testResourceSyncer) List(ctx context.Context, workspace logicalcluster.Name, gvr schema.GroupVersionResource) ([]*unstructured.Unstructured, error) {
	return nil, nil
}

type testSyncerFactory struct{}

var _ syncer.SyncerFactory = &testSyncerFactory{}

func (t *testSyncerFactory) NewSyncer(
	target *workloadv1alpha1.SyncTarget,
	upstreamClient dynamic.ClusterInterface,
	downstreamClient dynamic.Interface,
) (syncer.Syncer, error) {
	return &testSyncer{}, nil
}
func (t *testSyncerFactory) ValidateConfiguration(config map[string]interface{}) error {
	return nil
}

func TestSyncerInterface(t *testing.T) {
	// Test that interface can be implemented
	var s syncer.Syncer = &testSyncer{}
	if s == nil {
		t.Fatal("Failed to implement Syncer interface")
	}
}

func TestResourceSyncerInterface(t *testing.T) {
	var rs syncer.ResourceSyncer = &testResourceSyncer{}
	if rs == nil {
		t.Fatal("Failed to implement ResourceSyncer interface")
	}
}

func TestSyncerFactoryInterface(t *testing.T) {
	var sf syncer.SyncerFactory = &testSyncerFactory{}
	if sf == nil {
		t.Fatal("Failed to implement SyncerFactory interface")
	}
}