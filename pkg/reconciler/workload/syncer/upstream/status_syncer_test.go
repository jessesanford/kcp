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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestStatusSyncer_extractStatus(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected interface{}
		wantErr  bool
	}{
		{
			name: "extract basic status",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-ns",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			expected: map[string]interface{}{
				"phase": "Running",
			},
			wantErr: false,
		},
		{
			name: "no status field",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "test-ns",
					},
				},
			},
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := syncer.extractStatus(tt.obj)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expected == nil && result != nil {
				t.Errorf("expected nil result but got %v", result)
			}
		})
	}
}

func TestStatusSyncer_reverseNamespaceTransform(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	tests := []struct {
		name                 string
		downstreamNamespace  string
		expectedNamespace    string
	}{
		{
			name:                 "transform prefixed namespace",
			downstreamNamespace:  "kcp-root:test-default",
			expectedNamespace:    "default",
		},
		{
			name:                 "no prefix",
			downstreamNamespace:  "default",
			expectedNamespace:    "default",
		},
		{
			name:                 "empty namespace",
			downstreamNamespace:  "",
			expectedNamespace:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.reverseNamespaceTransform(tt.downstreamNamespace)
			if result != tt.expectedNamespace {
				t.Errorf("expected %s but got %s", tt.expectedNamespace, result)
			}
		})
	}
}

func TestStatusSyncer_getGVR(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedGVR schema.GroupVersionResource
	}{
		{
			name: "v1 Pod",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			name: "apps/v1 Deployment",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.getGVR(tt.obj)
			if result != tt.expectedGVR {
				t.Errorf("expected %v but got %v", tt.expectedGVR, result)
			}
		})
	}
}

func TestStatusSyncer_isStatusUnchanged(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	status1 := map[string]interface{}{
		"phase": "Running",
	}
	status2 := map[string]interface{}{
		"phase": "Pending",
	}

	tests := []struct {
		name           string
		cacheKey       string
		cachedStatus   interface{}
		newStatus      interface{}
		expectedResult bool
	}{
		{
			name:           "cache miss",
			cacheKey:       "test-key",
			cachedStatus:   nil,
			newStatus:      status1,
			expectedResult: false,
		},
		{
			name:           "same status",
			cacheKey:       "test-key",
			cachedStatus:   status1,
			newStatus:      status1,
			expectedResult: true,
		},
		{
			name:           "different status",
			cacheKey:       "test-key",
			cachedStatus:   status1,
			newStatus:      status2,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up cache
			syncer.ClearCache()
			if tt.cachedStatus != nil {
				syncer.updateStatusCache(tt.cacheKey, tt.cachedStatus)
			}

			result := syncer.isStatusUnchanged(tt.cacheKey, tt.newStatus)
			if result != tt.expectedResult {
				t.Errorf("expected %v but got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestStatusSyncer_RegisterExtractor(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	extractor := &DefaultStatusExtractor{}

	syncer.RegisterExtractor(gvr, extractor)

	// Verify extractor was registered
	if registeredExtractor, exists := syncer.extractors[gvr]; !exists {
		t.Error("extractor was not registered")
	} else if registeredExtractor != extractor {
		t.Error("wrong extractor was registered")
	}
}

func TestStatusSyncer_ClearCache(t *testing.T) {
	syncer := NewStatusSyncer(nil, nil, "test-target", logicalcluster.New("root:test"))

	// Add something to cache
	syncer.updateStatusCache("test-key", map[string]interface{}{"test": "data"})

	if syncer.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1 but got %d", syncer.GetCacheSize())
	}

	// Clear cache
	syncer.ClearCache()

	if syncer.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0 after clear but got %d", syncer.GetCacheSize())
	}
}

func TestDefaultStatusExtractor(t *testing.T) {
	extractor := &DefaultStatusExtractor{}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	result, err := extractor.ExtractStatus(obj)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}

	// Test ShouldExtract
	if !extractor.ShouldExtract(obj) {
		t.Error("expected ShouldExtract to return true")
	}
}