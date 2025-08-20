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
	"fmt"
	"reflect"

	"github.com/kcp-dev/logicalcluster/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
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

// Pipeline coordinates resource transformations during synchronization.
// It applies transformations in a specific order and provides bi-directional
// transformation capabilities for KCP <-> physical cluster synchronization.
type Pipeline struct {
	transformers []ResourceTransformer
	workspace    logicalcluster.Name
}

// NewPipeline creates a new transformation pipeline with default transformers
// configured for the given logical cluster workspace.
func NewPipeline(workspace logicalcluster.Name) *Pipeline {
	pipeline := &Pipeline{
		workspace: workspace,
	}
	
	// Register default transformers in execution order
	pipeline.RegisterTransformer(NewNamespaceTransformer(workspace))
	pipeline.RegisterTransformer(NewMetadataTransformer())
	pipeline.RegisterTransformer(NewOwnerReferenceTransformer())
	pipeline.RegisterTransformer(NewSecretTransformer())
	
	return pipeline
}

// TransformForDownstream applies all registered transformations when syncing
// resources from KCP to a physical cluster. Transformers are applied in
// registration order.
func (p *Pipeline) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	// Create a deep copy to avoid modifying the original
	result := obj.DeepCopyObject()
	
	klog.V(4).InfoS("Starting downstream transformation pipeline",
		"workspace", p.workspace,
		"objectKind", getObjectKind(result),
		"targetCluster", target.Spec.ClusterName)
	
	for _, transformer := range p.transformers {
		if !transformer.ShouldTransform(result) {
			klog.V(6).InfoS("Skipping transformer for object",
				"transformer", transformer.Name(),
				"objectKind", getObjectKind(result))
			continue
		}
		
		klog.V(5).InfoS("Applying downstream transformer",
			"transformer", transformer.Name(),
			"objectKind", getObjectKind(result))
		
		var err error
		result, err = transformer.TransformForDownstream(ctx, result, target)
		if err != nil {
			return nil, fmt.Errorf("transformer %s failed for downstream sync: %w", transformer.Name(), err)
		}
		
		if result == nil {
			return nil, fmt.Errorf("transformer %s returned nil object", transformer.Name())
		}
	}
	
	klog.V(4).InfoS("Completed downstream transformation pipeline",
		"workspace", p.workspace,
		"objectKind", getObjectKind(result),
		"targetCluster", target.Spec.ClusterName)
	
	return result, nil
}

// TransformForUpstream applies all registered transformations when syncing
// resources from a physical cluster back to KCP. Transformers are applied
// in reverse order to undo downstream transformations.
func (p *Pipeline) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	// Create a deep copy to avoid modifying the original
	result := obj.DeepCopyObject()
	
	klog.V(4).InfoS("Starting upstream transformation pipeline",
		"workspace", p.workspace,
		"objectKind", getObjectKind(result),
		"sourceCluster", source.Spec.ClusterName)
	
	// Apply transformers in reverse order for upstream
	for i := len(p.transformers) - 1; i >= 0; i-- {
		transformer := p.transformers[i]
		
		if !transformer.ShouldTransform(result) {
			klog.V(6).InfoS("Skipping transformer for object",
				"transformer", transformer.Name(),
				"objectKind", getObjectKind(result))
			continue
		}
		
		klog.V(5).InfoS("Applying upstream transformer",
			"transformer", transformer.Name(),
			"objectKind", getObjectKind(result))
		
		var err error
		result, err = transformer.TransformForUpstream(ctx, result, source)
		if err != nil {
			return nil, fmt.Errorf("transformer %s failed for upstream sync: %w", transformer.Name(), err)
		}
		
		if result == nil {
			return nil, fmt.Errorf("transformer %s returned nil object", transformer.Name())
		}
	}
	
	klog.V(4).InfoS("Completed upstream transformation pipeline",
		"workspace", p.workspace,
		"objectKind", getObjectKind(result),
		"sourceCluster", source.Spec.ClusterName)
	
	return result, nil
}

// RegisterTransformer adds a custom transformer to the pipeline.
// Transformers are executed in the order they are registered for downstream,
// and in reverse order for upstream transformations.
func (p *Pipeline) RegisterTransformer(transformer ResourceTransformer) {
	if transformer == nil {
		klog.Warning("Attempted to register nil transformer")
		return
	}
	
	// Check for duplicate transformers by name
	for _, existing := range p.transformers {
		if existing.Name() == transformer.Name() {
			klog.V(2).InfoS("Replacing existing transformer", "name", transformer.Name())
			// Find and replace the existing transformer
			for i, t := range p.transformers {
				if t.Name() == transformer.Name() {
					p.transformers[i] = transformer
					return
				}
			}
		}
	}
	
	p.transformers = append(p.transformers, transformer)
	klog.V(2).InfoS("Registered transformer", "name", transformer.Name(), "total", len(p.transformers))
}

// RemoveTransformer removes a transformer from the pipeline by name.
func (p *Pipeline) RemoveTransformer(name string) {
	for i, transformer := range p.transformers {
		if transformer.Name() == name {
			p.transformers = append(p.transformers[:i], p.transformers[i+1:]...)
			klog.V(2).InfoS("Removed transformer", "name", name, "remaining", len(p.transformers))
			return
		}
	}
	klog.V(2).InfoS("Transformer not found for removal", "name", name)
}

// ListTransformers returns the names of all registered transformers in execution order.
func (p *Pipeline) ListTransformers() []string {
	names := make([]string, len(p.transformers))
	for i, transformer := range p.transformers {
		names[i] = transformer.Name()
	}
	return names
}

// getObjectKind returns a string representation of the object's kind for logging.
func getObjectKind(obj runtime.Object) string {
	if obj == nil {
		return "unknown"
	}
	
	if gvk := obj.GetObjectKind().GroupVersionKind(); !gvk.Empty() {
		return gvk.String()
	}
	
	return reflect.TypeOf(obj).String()
}