package conflict

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolver_ResolveConflict(t *testing.T) {
	tests := map[string]struct {
		kcp            *unstructured.Unstructured
		downstream     *unstructured.Unstructured
		defaultStrategy ResolutionStrategy
		wantResolved   bool
		wantError      bool
	}{
		"no conflict": {
			kcp:            createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:     createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			defaultStrategy: KCPWins,
			wantResolved:   true,
		},
		"version conflict with KCP wins": {
			kcp:            createTestResourceWithGeneration("test", "v2", 2, map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:     createTestResourceWithGeneration("test", "v1", 1, map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			defaultStrategy: KCPWins,
			wantResolved:   true,
		},
		"deleted conflict": {
			kcp:            createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:     nil,
			defaultStrategy: KCPWins,
			wantResolved:   true,
		},
		"both nil resources": {
			kcp:            nil,
			downstream:     nil,
			defaultStrategy: KCPWins,
			wantError:      true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := NewResolver(tc.defaultStrategy, nil)
			
			
			result, err := resolver.ResolveConflict(context.Background(), tc.kcp, tc.downstream)

			if tc.wantError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Resolved != tc.wantResolved {
				t.Errorf("expected resolved=%v, got %v (strategy: %s)", tc.wantResolved, result.Resolved, result.Strategy)
			}
		})
	}
}

func TestConflictDetector_DetectConflict(t *testing.T) {
	detector := NewConflictDetector()

	tests := map[string]struct {
		kcp           *unstructured.Unstructured
		downstream    *unstructured.Unstructured
		wantConflict  bool
		wantType      ConflictType
	}{
		"identical resources": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			wantConflict: false,
		},
		"version difference": {
			kcp:          createTestResourceWithGeneration("test", "v2", 2, map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   createTestResourceWithGeneration("test", "v1", 1, map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			wantConflict: true,
			wantType:     VersionConflict,
		},
		"deletion conflict": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   nil,
			wantConflict: true,
			wantType:     DeletedConflict,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conflict := detector.DetectConflict(tc.kcp, tc.downstream)

			if tc.wantConflict {
				if conflict == nil {
					t.Errorf("expected conflict but got none")
					return
				}
				if conflict.Type != tc.wantType {
					t.Errorf("expected conflict type=%v, got %v", tc.wantType, conflict.Type)
				}
			} else {
				if conflict != nil {
					t.Errorf("expected no conflict but got: %+v", conflict)
				}
			}
		})
	}
}

// Helper function to create test resources
func createTestResource(name, resourceVersion string, spec map[string]interface{}) *unstructured.Unstructured {
	return createTestResourceWithGeneration(name, resourceVersion, 1, spec)
}

func createTestResourceWithGeneration(name, resourceVersion string, generation int64, spec map[string]interface{}) *unstructured.Unstructured {
	resource := &unstructured.Unstructured{}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	resource.SetName(name)
	resource.SetNamespace("default")
	resource.SetResourceVersion(resourceVersion)
	resource.SetGeneration(generation)

	unstructured.SetNestedMap(resource.Object, spec, "spec")

	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})

	return resource
}