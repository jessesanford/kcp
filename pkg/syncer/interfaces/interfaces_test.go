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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/syncer/interfaces"
)

func TestSyncDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction interfaces.SyncDirection
		expected  string
	}{
		{
			name:      "upstream direction",
			direction: interfaces.SyncDirectionUpstream,
			expected:  "upstream",
		},
		{
			name:      "downstream direction",
			direction: interfaces.SyncDirectionDownstream,
			expected:  "downstream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.direction))
		})
	}
}

func TestSyncOperation(t *testing.T) {
	op := interfaces.SyncOperation{
		ID:            "test-op-1",
		Direction:     interfaces.SyncDirectionDownstream,
		SourceCluster: logicalcluster.Name("root:org:ws"),
		TargetCluster: logicalcluster.Name("root:org:target"),
		GVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Namespace: "default",
		Name:      "test-deployment",
		Priority:  10,
	}

	assert.Equal(t, "test-op-1", op.ID)
	assert.Equal(t, interfaces.SyncDirectionDownstream, op.Direction)
	assert.Equal(t, logicalcluster.Name("root:org:ws"), op.SourceCluster)
	assert.Equal(t, "default", op.Namespace)
	assert.Equal(t, int32(10), op.Priority)
}

func TestSyncStatus(t *testing.T) {
	retryDuration := 5 * time.Second
	status := interfaces.SyncStatus{
		Result:       interfaces.SyncResultRetry,
		Message:      "temporary failure",
		RetryAfter:   &retryDuration,
		ConflictType: interfaces.ConflictTypeResourceVersion,
	}

	assert.Equal(t, interfaces.SyncResultRetry, status.Result)
	assert.Equal(t, "temporary failure", status.Message)
	assert.NotNil(t, status.RetryAfter)
	assert.Equal(t, 5*time.Second, *status.RetryAfter)
}

func TestConflictTypes(t *testing.T) {
	conflicts := []interfaces.ConflictType{
		interfaces.ConflictTypeResourceVersion,
		interfaces.ConflictTypeOwnership,
		interfaces.ConflictTypeFieldManager,
		interfaces.ConflictTypeAnnotation,
	}

	expectedStrings := []string{
		"resource-version",
		"ownership",
		"field-manager",
		"annotation",
	}

	for i, conflict := range conflicts {
		assert.Equal(t, expectedStrings[i], string(conflict))
	}
}

func TestSyncMetrics(t *testing.T) {
	metrics := interfaces.SyncMetrics{
		TotalOperations:       100,
		SuccessfulOperations:  85,
		FailedOperations:      10,
		ConflictedOperations:  5,
		AverageProcessingTime: 2 * time.Second,
	}

	assert.Equal(t, int64(100), metrics.TotalOperations)
	assert.Equal(t, int64(85), metrics.SuccessfulOperations)
	assert.Equal(t, int64(10), metrics.FailedOperations)
	assert.Equal(t, int64(5), metrics.ConflictedOperations)
	assert.Equal(t, 2*time.Second, metrics.AverageProcessingTime)

	// Verify metrics consistency
	totalCalculated := metrics.SuccessfulOperations + metrics.FailedOperations + metrics.ConflictedOperations
	assert.Equal(t, metrics.TotalOperations, totalCalculated)
}

func TestTransformationContext(t *testing.T) {
	ctx := interfaces.TransformationContext{
		SourceWorkspace: logicalcluster.Name("root:org:source"),
		TargetWorkspace: logicalcluster.Name("root:org:target"),
		Direction:       interfaces.SyncDirectionDownstream,
		PlacementName:   "test-placement",
		SyncTargetName:  "cluster-1",
		Annotations: map[string]string{
			"tmc.kcp.io/placement":   "test-placement",
			"tmc.kcp.io/sync-target": "cluster-1",
		},
	}

	assert.Equal(t, logicalcluster.Name("root:org:source"), ctx.SourceWorkspace)
	assert.Equal(t, logicalcluster.Name("root:org:target"), ctx.TargetWorkspace)
	assert.Equal(t, interfaces.SyncDirectionDownstream, ctx.Direction)
	assert.Equal(t, "test-placement", ctx.PlacementName)
	assert.Equal(t, "cluster-1", ctx.SyncTargetName)
	assert.Contains(t, ctx.Annotations, "tmc.kcp.io/placement")
}

func TestSyncConflict(t *testing.T) {
	source := &unstructured.Unstructured{}
	source.SetName("test-resource")
	source.SetNamespace("default")

	target := &unstructured.Unstructured{}
	target.SetName("test-resource")
	target.SetNamespace("default")
	target.SetResourceVersion("123")

	conflict := interfaces.SyncConflict{
		Operation: interfaces.SyncOperation{
			ID:        "op-1",
			Direction: interfaces.SyncDirectionDownstream,
		},
		ConflictType:   interfaces.ConflictTypeResourceVersion,
		SourceResource: source,
		TargetResource: target,
		ConflictDetails: map[string]interface{}{
			"sourceVersion": "",
			"targetVersion": "123",
		},
		DetectedAt: time.Now(),
	}

	assert.Equal(t, interfaces.ConflictTypeResourceVersion, conflict.ConflictType)
	assert.NotNil(t, conflict.SourceResource)
	assert.NotNil(t, conflict.TargetResource)
	assert.Contains(t, conflict.ConflictDetails, "targetVersion")
}

func TestConflictResolution(t *testing.T) {
	retryDuration := 10 * time.Second
	resolution := interfaces.ConflictResolution{
		Resolved:   true,
		Resolution: &unstructured.Unstructured{},
		Strategy:   "server-side-apply",
		Message:    "Resolved using server-side apply",
		Retry:      false,
		RetryAfter: &retryDuration,
	}

	assert.True(t, resolution.Resolved)
	assert.NotNil(t, resolution.Resolution)
	assert.Equal(t, "server-side-apply", resolution.Strategy)
	assert.Equal(t, "Resolved using server-side apply", resolution.Message)
	assert.False(t, resolution.Retry)
	assert.Equal(t, 10*time.Second, *resolution.RetryAfter)
}

func TestSyncEngineConfig(t *testing.T) {
	config := interfaces.SyncEngineConfig{
		WorkerCount: 10,
		QueueDepth:  100,
		Workspace:   logicalcluster.Name("root:org:ws"),
		SupportedGVRs: []schema.GroupVersionResource{
			{Group: "apps", Version: "v1", Resource: "deployments"},
			{Group: "", Version: "v1", Resource: "services"},
		},
	}

	assert.Equal(t, 10, config.WorkerCount)
	assert.Equal(t, 100, config.QueueDepth)
	assert.Equal(t, logicalcluster.Name("root:org:ws"), config.Workspace)
	assert.Len(t, config.SupportedGVRs, 2)
	assert.Equal(t, "deployments", config.SupportedGVRs[0].Resource)
}
