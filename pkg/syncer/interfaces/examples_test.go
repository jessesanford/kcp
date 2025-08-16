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

package interfaces_test

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

// ExampleSyncOperation demonstrates creating a sync operation.
func ExampleSyncOperation() {
	op := interfaces.SyncOperation{
		ID:            "sync-deployment-12345",
		Direction:     interfaces.SyncDirectionDownstream,
		SourceCluster: logicalcluster.Name("root:org:workspace"),
		TargetCluster: logicalcluster.Name("root:org:cluster-1"),
		GVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Namespace: "production",
		Name:      "web-app",
		Priority:  10,
		Timestamp: metav1.Now(),
	}

	fmt.Printf("Sync operation %s: %s/%s from %s to %s\n",
		op.ID, op.Namespace, op.Name, op.SourceCluster, op.TargetCluster)
	// Output: Sync operation sync-deployment-12345: production/web-app from root:org:workspace to root:org:cluster-1
}

// ExampleSyncEngine demonstrates using the sync engine interface.
func ExampleSyncEngine() {
	// This would normally be a real implementation
	var engine interfaces.SyncEngine

	ctx := context.Background()

	// Start the sync engine
	if err := engine.Start(ctx); err != nil {
		fmt.Printf("Failed to start engine: %v\n", err)
		return
	}

	// Enqueue a sync operation
	op := interfaces.SyncOperation{
		ID:        "op-1",
		Direction: interfaces.SyncDirectionDownstream,
		// ... other fields
	}

	if err := engine.EnqueueSyncOperation(op); err != nil {
		fmt.Printf("Failed to enqueue: %v\n", err)
		return
	}

	// Check status
	if status, found := engine.GetSyncStatus("op-1"); found {
		fmt.Printf("Operation status: %s\n", status.Result)
	}

	// Stop the engine
	if err := engine.Stop(ctx); err != nil {
		fmt.Printf("Failed to stop engine: %v\n", err)
	}
}

// ExampleResourceTransformer demonstrates resource transformation.
func ExampleResourceTransformer() {
	// This would normally be a real implementation
	var transformer interfaces.ResourceTransformer

	ctx := context.Background()
	resource := &unstructured.Unstructured{}
	resource.SetName("my-deployment")
	resource.SetNamespace("default")

	// Transform for downstream sync
	transformed, err := transformer.TransformForDownstream(
		ctx,
		logicalcluster.Name("root:org:workspace"),
		logicalcluster.Name("root:org:cluster-1"),
		resource,
	)

	if err != nil {
		fmt.Printf("Transformation failed: %v\n", err)
		return
	}

	// Check if transformation added required annotations
	annotations := transformed.GetAnnotations()
	if source, ok := annotations[interfaces.SyncSourceAnnotation]; ok {
		fmt.Printf("Sync source: %s\n", source)
	}
}

// ExampleConflictResolver demonstrates conflict resolution.
func ExampleConflictResolver() {
	// This would normally be a real implementation
	var resolver interfaces.ConflictResolver

	ctx := context.Background()

	conflict := interfaces.SyncConflict{
		Operation: interfaces.SyncOperation{
			ID:        "op-1",
			Direction: interfaces.SyncDirectionDownstream,
		},
		ConflictType:   interfaces.ConflictTypeResourceVersion,
		SourceResource: &unstructured.Unstructured{},
		TargetResource: &unstructured.Unstructured{},
		DetectedAt:     time.Now(),
	}

	// Attempt to resolve the conflict
	resolution, err := resolver.ResolveConflict(ctx, conflict)
	if err != nil {
		fmt.Printf("Failed to resolve conflict: %v\n", err)
		return
	}

	if resolution.Resolved {
		fmt.Printf("Conflict resolved using strategy: %s\n", resolution.Strategy)
	} else if resolution.Retry {
		fmt.Printf("Retry after %v\n", *resolution.RetryAfter)
	}
}

// ExampleStatusCollector demonstrates status collection.
func ExampleStatusCollector() {
	// This would normally be a real implementation
	var collector interfaces.StatusCollector

	ctx := context.Background()

	// Record a sync status
	status := interfaces.SyncStatus{
		Operation: interfaces.SyncOperation{
			ID: "op-1",
		},
		Result:         interfaces.SyncResultSuccess,
		Message:        "Resource synchronized successfully",
		ProcessingTime: 150 * time.Millisecond,
		Timestamp:      metav1.Now(),
	}

	if err := collector.RecordSyncStatus(ctx, status); err != nil {
		fmt.Printf("Failed to record status: %v\n", err)
		return
	}

	// Get metrics for a workspace
	workspace := logicalcluster.Name("root:org:workspace")
	since := time.Now().Add(-1 * time.Hour)
	metrics := collector.GetWorkspaceMetrics(workspace, &since)

	fmt.Printf("Workspace metrics: %d successful, %d failed\n",
		metrics.SuccessfulOperations, metrics.FailedOperations)
}

// ExampleTransformationContext demonstrates creating a transformation context.
func ExampleTransformationContext() {
	ctx := interfaces.TransformationContext{
		SourceWorkspace: logicalcluster.Name("root:org:source"),
		TargetWorkspace: logicalcluster.Name("root:org:target"),
		Direction:       interfaces.SyncDirectionDownstream,
		PlacementName:   "production-placement",
		SyncTargetName:  "us-west-cluster",
		Annotations: map[string]string{
			interfaces.PlacementAnnotation:  "production-placement",
			interfaces.SyncTargetAnnotation: "us-west-cluster",
			"tmc.kcp.io/region":             "us-west",
		},
	}

	fmt.Printf("Transforming from %s to %s for placement %s\n",
		ctx.SourceWorkspace, ctx.TargetWorkspace, ctx.PlacementName)
	// Output: Transforming from root:org:source to root:org:target for placement production-placement
}
