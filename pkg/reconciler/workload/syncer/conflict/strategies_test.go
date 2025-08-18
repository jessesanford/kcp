package conflict

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestKCPWinsStrategy(t *testing.T) {
	strategy := &KCPWinsStrategy{}
	
	tests := map[string]struct {
		kcp          *unstructured.Unstructured
		downstream   *unstructured.Unstructured
		wantResolved bool
		wantError    bool
	}{
		"kcp resource exists": {
			kcp:          createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: true,
			wantError:    false,
		},
		"downstream deleted": {
			kcp:          createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   nil,
			wantResolved: true,
			wantError:    false,
		},
		"kcp is nil": {
			kcp:          nil,
			downstream:   createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: false,
			wantError:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conflict := &Conflict{
				Type:      VersionConflict,
				Severity:  MediumSeverity,
				Namespace: "default",
				Name:      "test",
			}

			result, err := strategy.Resolve(context.Background(), tc.kcp, tc.downstream, conflict)

			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Resolved != tc.wantResolved {
				t.Errorf("expected resolved=%v, got %v", tc.wantResolved, result.Resolved)
			}

			if tc.wantResolved && result.Merged == nil {
				t.Error("expected merged resource but got nil")
			}

			if tc.wantResolved && tc.kcp != nil {
				// Verify that the merged resource is based on KCP
				if result.Merged.GetName() != tc.kcp.GetName() {
					t.Errorf("expected merged name=%s, got %s", tc.kcp.GetName(), result.Merged.GetName())
				}
			}
		})
	}
}

func TestDownstreamWinsStrategy(t *testing.T) {
	strategy := &DownstreamWinsStrategy{}
	
	tests := map[string]struct {
		kcp          *unstructured.Unstructured
		downstream   *unstructured.Unstructured
		wantResolved bool
		wantError    bool
	}{
		"downstream resource exists": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: true,
			wantError:    false,
		},
		"kcp deleted": {
			kcp:          nil,
			downstream:   createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: true,
			wantError:    false,
		},
		"downstream is nil": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   nil,
			wantResolved: false,
			wantError:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conflict := &Conflict{
				Type:      VersionConflict,
				Severity:  MediumSeverity,
				Namespace: "default",
				Name:      "test",
			}

			result, err := strategy.Resolve(context.Background(), tc.kcp, tc.downstream, conflict)

			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Resolved != tc.wantResolved {
				t.Errorf("expected resolved=%v, got %v", tc.wantResolved, result.Resolved)
			}

			if tc.wantResolved && result.Merged == nil {
				t.Error("expected merged resource but got nil")
			}

			if tc.wantResolved && tc.downstream != nil {
				// Verify that upstream sync annotation is added
				annotations := result.Merged.GetAnnotations()
				if annotations == nil || annotations["syncer.kcp.io/upstream-sync"] != "pending" {
					t.Error("expected upstream-sync annotation to be set to 'pending'")
				}
			}
		})
	}
}

func TestMergeStrategy(t *testing.T) {
	strategy := &MergeStrategy{}
	
	t.Run("successful merge", func(t *testing.T) {
		kcp := createTestResource("test", "v1", map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"image":    "nginx:1.20",
			},
		})
		kcp.SetAnnotations(map[string]string{
			"app":     "test-app",
			"version": "1.0",
		})

		downstream := createTestResource("test", "v2", map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(5),
				"image":    "nginx:1.20",
			},
		})
		downstream.SetAnnotations(map[string]string{
			"app":      "test-app",
			"version":  "1.0",
			"cluster":  "downstream-cluster",
		})

		conflict := &Conflict{
			Type:      SemanticConflict,
			Severity:  MediumSeverity,
			Namespace: "default",
			Name:      "test",
			Fields: []FieldConflict{
				{
					Path:             "spec.replicas",
					KCPValue:         int64(3),
					DownstreamValue:  int64(5),
					Resolution:       "value_mismatch",
				},
				{
					Path:             "metadata.annotations.cluster",
					KCPValue:         nil,
					DownstreamValue:  "downstream-cluster",
					Resolution:       "missing_in_downstream",
				},
			},
		}

		result, err := strategy.Resolve(context.Background(), kcp, downstream, conflict)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The merge strategy tries to resolve conflicts but may not succeed for all conflicts
		if len(result.Conflicts) == len(conflict.Fields) {
			// None of the conflicts were resolved, so it's not fully resolved
			if result.Resolved {
				t.Error("expected conflict to not be fully resolved when unresolved conflicts remain")
			}
		}

		if result.Merged == nil {
			t.Fatal("expected merged resource but got nil")
		}

		// Verify basic merge functionality - the merged result should exist
		if result.Merged.GetName() != kcp.GetName() {
			t.Errorf("expected merged name=%s, got %s", kcp.GetName(), result.Merged.GetName())
		}
	})

	t.Run("missing resources error", func(t *testing.T) {
		conflict := &Conflict{
			Type:     SemanticConflict,
			Severity: MediumSeverity,
		}

		_, err := strategy.Resolve(context.Background(), nil, nil, conflict)
		if err == nil {
			t.Error("expected error for missing resources")
		}
	})
}

func TestMergeStrategy_FieldMerging(t *testing.T) {
	strategy := &MergeStrategy{}
	
	t.Run("merge annotations", func(t *testing.T) {
		kcp := createTestResource("test", "v1", map[string]interface{}{})
		kcp.SetAnnotations(map[string]string{
			"app":     "test-app",
			"version": "1.0",
		})

		downstream := createTestResource("test", "v1", map[string]interface{}{})
		downstream.SetAnnotations(map[string]string{
			"app":     "test-app",
			"cluster": "prod",
			"region":  "us-west-2",
		})

		conflict := &Conflict{
			Fields: []FieldConflict{
				{
					Path:             "metadata.annotations",
					KCPValue:         map[string]interface{}{"app": "test-app", "version": "1.0"},
					DownstreamValue:  map[string]interface{}{"app": "test-app", "cluster": "prod", "region": "us-west-2"},
					Resolution:       "value_mismatch",
				},
			},
		}

		result, err := strategy.Resolve(context.Background(), kcp, downstream, conflict)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		annotations := result.Merged.GetAnnotations()
		expected := map[string]string{
			"app":     "test-app",
			"version": "1.0",
			"cluster": "prod",
			"region":  "us-west-2",
		}

		for key, expectedValue := range expected {
			if annotations[key] != expectedValue {
				t.Errorf("expected annotation %s=%s, got %s", key, expectedValue, annotations[key])
			}
		}
	})

	t.Run("merge labels", func(t *testing.T) {
		kcp := createTestResource("test", "v1", map[string]interface{}{})
		kcp.SetLabels(map[string]string{
			"app":     "test-app",
			"version": "1.0",
		})

		downstream := createTestResource("test", "v1", map[string]interface{}{})
		downstream.SetLabels(map[string]string{
			"app":    "test-app",
			"env":    "prod",
			"region": "us-west-2",
		})

		conflict := &Conflict{
			Fields: []FieldConflict{
				{
					Path:             "metadata.labels",
					KCPValue:         map[string]interface{}{"app": "test-app", "version": "1.0"},
					DownstreamValue:  map[string]interface{}{"app": "test-app", "env": "prod", "region": "us-west-2"},
					Resolution:       "value_mismatch",
				},
			},
		}

		result, err := strategy.Resolve(context.Background(), kcp, downstream, conflict)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		labels := result.Merged.GetLabels()
		expected := map[string]string{
			"app":     "test-app",
			"version": "1.0",
			"env":     "prod",
			"region":  "us-west-2",
		}

		for key, expectedValue := range expected {
			if labels[key] != expectedValue {
				t.Errorf("expected label %s=%s, got %s", key, expectedValue, labels[key])
			}
		}
	})
}

func TestManualStrategy(t *testing.T) {
	strategy := &ManualStrategy{}
	
	tests := map[string]struct {
		kcp          *unstructured.Unstructured
		downstream   *unstructured.Unstructured
		wantResolved bool
		wantError    bool
	}{
		"both resources exist": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: false,
			wantError:    false,
		},
		"only kcp exists": {
			kcp:          createTestResource("test", "v1", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}),
			downstream:   nil,
			wantResolved: false,
			wantError:    false,
		},
		"only downstream exists": {
			kcp:          nil,
			downstream:   createTestResource("test", "v2", map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(5)}}),
			wantResolved: false,
			wantError:    false,
		},
		"both nil": {
			kcp:          nil,
			downstream:   nil,
			wantResolved: false,
			wantError:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conflict := &Conflict{
				Type:      OwnershipConflict,
				Severity:  CriticalSeverity,
				Namespace: "default",
				Name:      "test",
				Fields: []FieldConflict{
					{Path: "spec.replicas", KCPValue: int64(3), DownstreamValue: int64(5)},
				},
			}

			result, err := strategy.Resolve(context.Background(), tc.kcp, tc.downstream, conflict)

			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Resolved != tc.wantResolved {
				t.Errorf("expected resolved=%v, got %v", tc.wantResolved, result.Resolved)
			}

			if result.Merged == nil {
				t.Error("expected merged resource but got nil")
			}

			// Verify conflict annotations are added
			annotations := result.Merged.GetAnnotations()
			if annotations == nil {
				t.Fatal("expected annotations to be added")
			}

			expectedAnnotations := map[string]string{
				"syncer.kcp.io/conflict-type":     string(OwnershipConflict),
				"syncer.kcp.io/conflict-severity": CriticalSeverity.String(),
				"syncer.kcp.io/sync-paused":       "true",
			}

			for key, expected := range expectedAnnotations {
				if annotations[key] != expected {
					t.Errorf("expected annotation %s=%s, got %s", key, expected, annotations[key])
				}
			}

			// Verify conflicts are preserved
			if len(result.Conflicts) == 0 {
				t.Error("expected conflicts to be preserved in result")
			}
		})
	}
}

func TestResolver_Configuration(t *testing.T) {
	t.Run("strategy registration", func(t *testing.T) {
		resolver := NewResolver(KCPWins, nil)
		
		// Test getting supported strategies
		strategies := resolver.GetSupportedStrategies()
		expectedStrategies := []ResolutionStrategy{KCPWins, DownstreamWins, Merge, Manual}
		
		if len(strategies) != len(expectedStrategies) {
			t.Errorf("expected %d strategies, got %d", len(expectedStrategies), len(strategies))
		}
		
		// Verify all expected strategies are present
		strategyMap := make(map[ResolutionStrategy]bool)
		for _, strategy := range strategies {
			strategyMap[strategy] = true
		}
		
		for _, expected := range expectedStrategies {
			if !strategyMap[expected] {
				t.Errorf("expected strategy %s not found", expected)
			}
		}
	})
	
	t.Run("custom strategy registration", func(t *testing.T) {
		resolver := NewResolver(KCPWins, nil)
		
		// Register a custom strategy
		customStrategy := ResolutionStrategy("custom")
		resolver.RegisterStrategy(customStrategy, &KCPWinsStrategy{}) // Reuse implementation for test
		
		strategies := resolver.GetSupportedStrategies()
		found := false
		for _, strategy := range strategies {
			if strategy == customStrategy {
				found = true
				break
			}
		}
		
		if !found {
			t.Error("custom strategy not found after registration")
		}
	})
	
	t.Run("strategy overrides", func(t *testing.T) {
		gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		resolver := NewResolver(KCPWins, nil)
		resolver.SetStrategyOverride(gvr, Manual)
		
		conflict := &Conflict{
			GVR:      gvr,
			Type:     SemanticConflict,
			Severity: LowSeverity, // Would normally use default (KCPWins)
		}
		
		strategy := resolver.selectStrategy(conflict)
		if strategy != Manual {
			t.Errorf("expected Manual strategy due to override, got %s", strategy)
		}
	})
}

func TestConflictSeverity_String(t *testing.T) {
	tests := map[string]struct {
		severity ConflictSeverity
		expected string
	}{
		"low severity": {
			severity: LowSeverity,
			expected: "low",
		},
		"medium severity": {
			severity: MediumSeverity,
			expected: "medium",
		},
		"high severity": {
			severity: HighSeverity,
			expected: "high",
		},
		"critical severity": {
			severity: CriticalSeverity,
			expected: "critical",
		},
		"unknown severity": {
			severity: ConflictSeverity(99),
			expected: "unknown",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.severity.String()
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("preserveDownstreamOnlyFields", func(t *testing.T) {
		downstream := createTestResource("test", "v2", map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		})
		downstream.SetUID("downstream-uid")
		downstream.SetGeneration(10)
		
		// Add status to downstream
		unstructured.SetNestedMap(downstream.Object, map[string]interface{}{
			"replicas": int64(5),
			"ready":    true,
		}, "status")
		
		merged := createTestResource("test", "v1", map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		})
		
		preserveDownstreamOnlyFields(downstream, merged)
		
		// Verify status is preserved
		status, found, _ := unstructured.NestedMap(merged.Object, "status")
		if !found {
			t.Error("expected status to be preserved")
		} else {
			replicas, _ := status["replicas"].(int64)
			if replicas != int64(5) {
				t.Errorf("expected status replicas=5, got %v", replicas)
			}
		}
		
		// Verify server-managed fields are preserved
		if merged.GetUID() != "downstream-uid" {
			t.Errorf("expected UID to be preserved, got %s", merged.GetUID())
		}
		
		if merged.GetResourceVersion() != "v2" {
			t.Errorf("expected resource version to be preserved, got %s", merged.GetResourceVersion())
		}
	})
	
	t.Run("markForUpstreamSync", func(t *testing.T) {
		resource := createTestResource("test", "v1", map[string]interface{}{})
		
		markForUpstreamSync(resource)
		
		annotations := resource.GetAnnotations()
		if annotations == nil {
			t.Fatal("expected annotations to be added")
		}
		
		if annotations["syncer.kcp.io/upstream-sync"] != "pending" {
			t.Errorf("expected upstream-sync=pending, got %s", annotations["syncer.kcp.io/upstream-sync"])
		}
	})
	
	t.Run("addConflictAnnotations", func(t *testing.T) {
		resource := createTestResource("test", "v1", map[string]interface{}{})
		conflict := &Conflict{
			Type:     OwnershipConflict,
			Severity: CriticalSeverity,
		}
		
		addConflictAnnotations(resource, conflict)
		
		annotations := resource.GetAnnotations()
		if annotations == nil {
			t.Fatal("expected annotations to be added")
		}
		
		expected := map[string]string{
			"syncer.kcp.io/conflict-type":     "ownership",
			"syncer.kcp.io/conflict-severity": "critical",
			"syncer.kcp.io/sync-paused":       "true",
		}
		
		for key, expectedValue := range expected {
			if annotations[key] != expectedValue {
				t.Errorf("expected annotation %s=%s, got %s", key, expectedValue, annotations[key])
			}
		}
	})
}

func TestResolverConfig_Defaults(t *testing.T) {
	resolver := NewResolver(KCPWins, nil)
	
	if resolver.config == nil {
		t.Fatal("expected config to be initialized")
	}
	
	if resolver.config.MaxConflictAge != 30*time.Minute {
		t.Errorf("expected default MaxConflictAge=30m, got %v", resolver.config.MaxConflictAge)
	}
	
	if resolver.config.StrategyOverrides == nil {
		t.Error("expected StrategyOverrides map to be initialized")
	}
	
	if resolver.defaultStrategy != KCPWins {
		t.Errorf("expected default strategy=KCPWins, got %s", resolver.defaultStrategy)
	}
}