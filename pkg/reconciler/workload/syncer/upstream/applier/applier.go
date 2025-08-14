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

package applier

import (
	"context"
	"fmt"
	"sync/atomic"
	
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer/upstream"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// applierImpl implements UpdateApplier
type applierImpl struct {
	client       dynamic.Interface
	dryRun       bool
	appliedCount int64
}

// NewApplier creates a new update applier
func NewApplier(client dynamic.Interface) upstream.UpdateApplier {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
		return &noopApplier{}
	}
	
	return &applierImpl{
		client: client,
	}
}

// Apply applies a single update to KCP
func (a *applierImpl) Apply(ctx context.Context, update *upstream.Update) error {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
		return nil
	}
	
	klog.V(4).Infof("Applying %s update for %s/%s", 
		update.Type, update.Resource.GetNamespace(), update.Resource.GetName())
	
	// Get resource interface
	gvr := update.Resource.GroupVersionKind().GroupVersion().WithResource(
		update.Resource.GetKind() + "s")
	
	var resourceClient dynamic.ResourceInterface
	if update.Resource.GetNamespace() != "" {
		resourceClient = a.client.Resource(gvr).Namespace(update.Resource.GetNamespace())
	} else {
		resourceClient = a.client.Resource(gvr)
	}
	
	// Apply based on type
	var err error
	switch update.Type {
	case upstream.UpdateTypeCreate:
		_, err = resourceClient.Create(ctx, update.Resource, metav1.CreateOptions{})
	case upstream.UpdateTypeUpdate:
		_, err = resourceClient.Update(ctx, update.Resource, metav1.UpdateOptions{})
	case upstream.UpdateTypeDelete:
		err = resourceClient.Delete(ctx, update.Resource.GetName(), metav1.DeleteOptions{})
	case upstream.UpdateTypeStatus:
		_, err = resourceClient.UpdateStatus(ctx, update.Resource, metav1.UpdateOptions{})
	default:
		err = fmt.Errorf("unknown update type: %s", update.Type)
	}
	
	if err != nil {
		return err
	}
	
	atomic.AddInt64(&a.appliedCount, 1)
	return nil
}

// ApplyBatch applies multiple updates
func (a *applierImpl) ApplyBatch(ctx context.Context, updates []*upstream.Update) error {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
		return nil
	}
	
	for _, update := range updates {
		if err := a.Apply(ctx, update); err != nil {
			return err
		}
	}
	
	return nil
}

// SetDryRun enables or disables dry-run mode
func (a *applierImpl) SetDryRun(enabled bool) {
	a.dryRun = enabled
}

// GetAppliedCount returns the number of successful applies
func (a *applierImpl) GetAppliedCount() int64 {
	return atomic.LoadInt64(&a.appliedCount)
}

// noopApplier when feature is disabled
type noopApplier struct{}

func (n *noopApplier) Apply(ctx context.Context, update *upstream.Update) error { return nil }
func (n *noopApplier) ApplyBatch(ctx context.Context, updates []*upstream.Update) error { return nil }
func (n *noopApplier) SetDryRun(enabled bool) {}
func (n *noopApplier) GetAppliedCount() int64 { return 0 }