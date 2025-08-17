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

package transformation

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SyncTarget represents a sync target for transformation purposes.
// This is a placeholder until Phase 5 APIs are available.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	
	// Spec contains the sync target specification
	Spec SyncTargetSpec `json:"spec,omitempty"`
}

// SyncTargetSpec defines the desired state of a sync target
type SyncTargetSpec struct {
	// ClusterName is the name of the target cluster
	ClusterName string `json:"clusterName,omitempty"`
	
	// Namespace is the target namespace for transformations
	Namespace string `json:"namespace,omitempty"`
}

// ResourceTransformer defines the interface for transforming resources
// during synchronization between KCP and physical clusters.
type ResourceTransformer interface {
	// ShouldTransform returns true if this transformer should process the given object
	ShouldTransform(obj runtime.Object) bool
	
	// TransformForDownstream transforms a resource when syncing from KCP to a physical cluster
	TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error)
	
	// TransformForUpstream transforms a resource when syncing from a physical cluster back to KCP
	TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error)
	
	// Name returns a human-readable name for the transformer
	Name() string
}