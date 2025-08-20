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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Simple unit tests for core functionality
func TestSyncResultBasics(t *testing.T) {
	result := &SyncResult{
		Operation: "create",
		Success:   true,
	}
	
	if result.Operation != "create" {
		t.Errorf("Expected operation 'create', got %s", result.Operation)
	}
	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultDownstreamConfig()
	
	if config == nil {
		t.Fatal("DefaultDownstreamConfig returned nil")
	}
	
	if config.ConflictRetries != 3 {
		t.Errorf("Expected ConflictRetries=3, got %d", config.ConflictRetries)
	}
	
	if config.UpdateStrategy != "strategic-merge" {
		t.Errorf("Expected UpdateStrategy='strategic-merge', got %s", config.UpdateStrategy)
	}
	
	if config.DeletionPropagation != metav1.DeletePropagationBackground {
		t.Errorf("Expected DeletionPropagation=Background, got %s", config.DeletionPropagation)
	}
	
	if config.ConflictRetryDelay != time.Second*5 {
		t.Errorf("Expected ConflictRetryDelay=5s, got %v", config.ConflictRetryDelay)
	}
}

func TestResourceState(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	state := &ResourceState{
		GVR:       gvr,
		Namespace: "default",
		Name:      "test",
	}
	
	if state.GVR != gvr {
		t.Errorf("GVR mismatch")
	}
	if state.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got %s", state.Namespace)
	}
	if state.Name != "test" {
		t.Errorf("Expected name 'test', got %s", state.Name)
	}
}

func TestConflictTypes(t *testing.T) {
	tests := []ConflictType{
		ConflictTypeResourceVersion,
		ConflictTypeGeneration,
		ConflictTypeFieldConflict,
		ConflictTypeDeletion,
	}
	
	for _, ct := range tests {
		if string(ct) == "" {
			t.Errorf("ConflictType %v should not be empty string", ct)
		}
	}
}

func TestSyncConflict(t *testing.T) {
	conflict := &SyncConflict{
		Type:      ConflictTypeResourceVersion,
		Field:     "metadata.resourceVersion",
		Resolvable: true,
	}
	
	if conflict.Type != ConflictTypeResourceVersion {
		t.Errorf("Expected conflict type ResourceVersion")
	}
	if conflict.Field != "metadata.resourceVersion" {
		t.Errorf("Expected field 'metadata.resourceVersion', got %s", conflict.Field)
	}
	if !conflict.Resolvable {
		t.Error("Expected conflict to be resolvable")
	}
}

func TestTransformationPipeline(t *testing.T) {
	pipeline := NewPipeline("root:test")
	if pipeline == nil {
		t.Fatal("NewPipeline returned nil")
	}
	
	if pipeline.workspace != "root:test" {
		t.Errorf("Expected workspace 'root:test', got %s", pipeline.workspace)
	}
}