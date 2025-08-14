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
)

func TestNewResourceMapper(t *testing.T) {
	mapper := NewResourceMapper()

	if mapper == nil {
		t.Fatal("mapper should not be nil")
	}

	if len(mapper.rules) == 0 {
		t.Error("mapper should have default rules")
	}
}

func TestResourceMapper_MapResource(t *testing.T) {
	mapper := NewResourceMapper()

	tests := map[string]struct {
		gvr            schema.GroupVersionResource
		obj            *unstructured.Unstructured
		expectedGVR    schema.GroupVersionResource
		expectedMapped bool
	}{
		"pod mapping": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-pod",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			expectedGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			expectedMapped: true,
		},
		"service mapping": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-service",
					},
					"spec": map[string]interface{}{
						"clusterIP": "10.96.0.1",
					},
				},
			},
			expectedGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			expectedMapped: true,
		},
		"deployment mapping": {
			gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			obj: &unstructured.Unstructured{},
			expectedGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			expectedMapped: true,
		},
		"unknown resource": {
			gvr: schema.GroupVersionResource{Group: "custom.io", Version: "v1", Resource: "customresources"},
			obj: &unstructured.Unstructured{},
			expectedGVR:    schema.GroupVersionResource{Group: "custom.io", Version: "v1", Resource: "customresources"},
			expectedMapped: false,
		},
		"pod with failed phase should be filtered": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "failed-pod",
					},
					"status": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
			expectedGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			expectedMapped: true, // Rule found, but transformation may fail
		},
		"system pod should be filtered": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "kube-proxy-xyz",
					},
				},
			},
			expectedGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			expectedMapped: false, // Condition should filter this out
		},
		"system configmap should be filtered": {
			gvr: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "kube-root-ca.crt",
					},
				},
			},
			expectedGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			expectedMapped: false, // Condition should filter this out
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resultGVR, mapped := mapper.MapResource(tc.gvr, tc.obj)

			if mapped != tc.expectedMapped {
				t.Errorf("expected mapped=%v, got mapped=%v", tc.expectedMapped, mapped)
			}

			if resultGVR != tc.expectedGVR {
				t.Errorf("expected GVR %v, got %v", tc.expectedGVR, resultGVR)
			}

			// If it's a service mapping, verify clusterIP was removed
			if tc.gvr.Resource == "services" && mapped {
				if clusterIP, found, _ := unstructured.NestedString(tc.obj.Object, "spec", "clusterIP"); found {
					t.Errorf("clusterIP should be removed, found: %s", clusterIP)
				}
			}
		})
	}
}

func TestResourceMapper_AddRemoveMappingRule(t *testing.T) {
	mapper := NewResourceMapper()
	initialRuleCount := len(mapper.rules)

	// Add a new rule
	customGVR := schema.GroupVersionResource{Group: "custom.io", Version: "v1", Resource: "customs"}
	newRule := MappingRule{
		FromGVR: customGVR,
		ToGVR:   customGVR,
	}

	mapper.AddMappingRule(newRule)

	if len(mapper.rules) != initialRuleCount+1 {
		t.Errorf("expected %d rules after adding, got %d", initialRuleCount+1, len(mapper.rules))
	}

	// Test the new rule works
	obj := &unstructured.Unstructured{}
	resultGVR, mapped := mapper.MapResource(customGVR, obj)
	if !mapped {
		t.Error("new rule should be mapped")
	}
	if resultGVR != customGVR {
		t.Error("new rule should return correct GVR")
	}

	// Remove the rule
	mapper.RemoveMappingRule(customGVR)

	if len(mapper.rules) != initialRuleCount {
		t.Errorf("expected %d rules after removing, got %d", initialRuleCount, len(mapper.rules))
	}

	// Test the rule no longer works
	_, mapped = mapper.MapResource(customGVR, obj)
	if mapped {
		t.Error("removed rule should not be mapped")
	}
}

func TestDefaultNamespaceMapper_ToLogical(t *testing.T) {
	mapper := NewDefaultNamespaceMapper("test-sync-target")

	tests := map[string]struct {
		physical       string
		syncTargetName string
		expected       string
	}{
		"regular namespace": {
			physical: "app-namespace",
			syncTargetName: "test-sync-target",
			expected: "kcp-test-sync-target-app-namespace",
		},
		"default namespace": {
			physical: "default",
			syncTargetName: "test-sync-target",
			expected: "kcp-test-sync-target-default",
		},
		"kube-system": {
			physical: "kube-system",
			syncTargetName: "test-sync-target",
			expected: "kcp-test-sync-target-system-system",
		},
		"kube-public": {
			physical: "kube-public",
			syncTargetName: "test-sync-target",
			expected: "kcp-test-sync-target-system-public",
		},
		"kube-node-lease": {
			physical: "kube-node-lease",
			syncTargetName: "test-sync-target",
			expected: "kcp-test-sync-target-system-node-lease",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := mapper.ToLogical(tc.physical, tc.syncTargetName)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestDefaultNamespaceMapper_ToPhysical(t *testing.T) {
	mapper := NewDefaultNamespaceMapper("test-sync-target")

	tests := map[string]struct {
		logical        string
		syncTargetName string
		expected       string
		expectError    bool
	}{
		"regular logical namespace": {
			logical: "kcp-test-sync-target-app-namespace",
			syncTargetName: "test-sync-target",
			expected: "app-namespace",
			expectError: false,
		},
		"default logical namespace": {
			logical: "kcp-test-sync-target-default",
			syncTargetName: "test-sync-target",
			expected: "default",
			expectError: false,
		},
		"system namespace": {
			logical: "kcp-test-sync-target-system-system",
			syncTargetName: "test-sync-target",
			expected: "kube-system",
			expectError: false,
		},
		"system public namespace": {
			logical: "kcp-test-sync-target-system-public",
			syncTargetName: "test-sync-target",
			expected: "kube-public",
			expectError: false,
		},
		"invalid prefix": {
			logical: "kcp-other-sync-target-namespace",
			syncTargetName: "test-sync-target",
			expected: "",
			expectError: true,
		},
		"missing prefix": {
			logical: "regular-namespace",
			syncTargetName: "test-sync-target",
			expected: "",
			expectError: true,
		},
		"empty after prefix removal": {
			logical: "kcp-test-sync-target-",
			syncTargetName: "test-sync-target",
			expected: "",
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := mapper.ToPhysical(tc.logical, tc.syncTargetName)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestDefaultNamespaceMapper_IsLogicalNamespace(t *testing.T) {
	mapper := NewDefaultNamespaceMapper("test-sync-target")

	tests := map[string]struct {
		namespace      string
		syncTargetName string
		expected       bool
	}{
		"valid logical namespace": {
			namespace: "kcp-test-sync-target-app-namespace",
			syncTargetName: "test-sync-target",
			expected: true,
		},
		"valid system logical namespace": {
			namespace: "kcp-test-sync-target-system-system",
			syncTargetName: "test-sync-target",
			expected: true,
		},
		"different sync target": {
			namespace: "kcp-other-sync-target-namespace",
			syncTargetName: "test-sync-target",
			expected: false,
		},
		"regular namespace": {
			namespace: "regular-namespace",
			syncTargetName: "test-sync-target",
			expected: false,
		},
		"kcp prefix but wrong format": {
			namespace: "kcp-namespace",
			syncTargetName: "test-sync-target",
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := mapper.IsLogicalNamespace(tc.namespace, tc.syncTargetName)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestMappingStats(t *testing.T) {
	stats := NewMappingStats()

	if stats == nil {
		t.Fatal("stats should not be nil")
	}

	if stats.TotalMapped != 0 {
		t.Error("initial total mapped should be 0")
	}

	if stats.FailedMappings != 0 {
		t.Error("initial failed mappings should be 0")
	}

	// Record some mappings
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	serviceGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}

	stats.RecordMapping(podGVR)
	stats.RecordMapping(podGVR)
	stats.RecordMapping(serviceGVR)
	stats.RecordFailure()

	if stats.TotalMapped != 3 {
		t.Errorf("expected total mapped 3, got %d", stats.TotalMapped)
	}

	if stats.FailedMappings != 1 {
		t.Errorf("expected failed mappings 1, got %d", stats.FailedMappings)
	}

	if stats.MappingsByGVR[podGVR] != 2 {
		t.Errorf("expected pod mappings 2, got %d", stats.MappingsByGVR[podGVR])
	}

	if stats.MappingsByGVR[serviceGVR] != 1 {
		t.Errorf("expected service mappings 1, got %d", stats.MappingsByGVR[serviceGVR])
	}

	// Get stats summary
	summary := stats.GetStats()
	if summary["total_mapped"] != int64(3) {
		t.Error("stats summary should include total mapped")
	}

	if summary["failed_mappings"] != int64(1) {
		t.Error("stats summary should include failed mappings")
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := map[string]struct {
		namespace   string
		expectError bool
	}{
		"valid namespace": {
			namespace:   "valid-namespace-123",
			expectError: false,
		},
		"empty namespace": {
			namespace:   "",
			expectError: true,
		},
		"too long namespace": {
			namespace:   string(make([]byte, 254)), // 254 characters
			expectError: true,
		},
		"invalid characters": {
			namespace:   "Invalid_Namespace",
			expectError: true,
		},
		"starts with hyphen": {
			namespace:   "-invalid",
			expectError: true,
		},
		"ends with hyphen": {
			namespace:   "invalid-",
			expectError: true,
		},
		"valid with hyphens": {
			namespace:   "valid-name-123",
			expectError: false,
		},
		"only numbers": {
			namespace:   "123456",
			expectError: false,
		},
		"only letters": {
			namespace:   "abcdef",
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateNamespace(tc.namespace)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBidirectionalNamespaceMapping(t *testing.T) {
	mapper := NewDefaultNamespaceMapper("test-sync-target")

	testNamespaces := []string{
		"default",
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"app-namespace",
		"my-app-123",
	}

	for _, physicalNS := range testNamespaces {
		t.Run(physicalNS, func(t *testing.T) {
			// Transform to logical
			logicalNS := mapper.ToLogical(physicalNS, "test-sync-target")

			// Transform back to physical
			resultPhysical, err := mapper.ToPhysical(logicalNS, "test-sync-target")
			if err != nil {
				t.Fatalf("failed to transform back to physical: %v", err)
			}

			if resultPhysical != physicalNS {
				t.Errorf("round trip failed: %s -> %s -> %s", physicalNS, logicalNS, resultPhysical)
			}

			// Verify logical namespace is recognized
			if !mapper.IsLogicalNamespace(logicalNS, "test-sync-target") {
				t.Errorf("logical namespace %s should be recognized", logicalNS)
			}

			// Verify original physical namespace is not recognized as logical
			if mapper.IsLogicalNamespace(physicalNS, "test-sync-target") {
				t.Errorf("physical namespace %s should not be recognized as logical", physicalNS)
			}
		})
	}
}