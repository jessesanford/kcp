/*
Copyright 2023 The KCP Authors.

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

package cel

import (
	"testing"
)

func TestSimpleCELEvaluator(t *testing.T) {
	// Basic test to verify package structure
	evaluator, err := NewCELEvaluator(nil)
	if err != nil {
		t.Fatalf("failed to create evaluator: %v", err)
	}

	if evaluator == nil {
		t.Fatal("expected non-nil evaluator")
	}

	// Test basic expression compilation
	compiled, err := evaluator.CompileExpression("true")
	if err != nil {
		t.Fatalf("failed to compile simple expression: %v", err)
	}

	if compiled == nil {
		t.Fatal("expected non-nil compiled expression")
	}

	if compiled.Expression != "true" {
		t.Errorf("expected expression 'true', got %s", compiled.Expression)
	}
}

func TestFunctionRegistry(t *testing.T) {
	registry := NewFunctionRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}

	// Test function registration
	fn := NewHasLabelFunction()
	err := registry.Register(fn)
	if err != nil {
		t.Fatalf("failed to register function: %v", err)
	}

	// Test retrieval
	retrieved, exists := registry.Get("hasLabel")
	if !exists {
		t.Fatal("expected function to exist")
	}

	if retrieved.Name() != "hasLabel" {
		t.Errorf("expected function name 'hasLabel', got %s", retrieved.Name())
	}
}

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}

	if cache.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", cache.Size())
	}

	// Test basic cache operations
	hash := "test-hash"
	expr := &CompiledExpression{
		Expression: "true",
		Hash:       hash,
	}

	cache.Set(hash, expr)
	if cache.Size() != 1 {
		t.Errorf("expected cache size 1, got %d", cache.Size())
	}

	retrieved, exists := cache.Get(hash)
	if !exists {
		t.Fatal("expected cache entry to exist")
	}

	if retrieved.Hash != hash {
		t.Errorf("expected hash %s, got %s", hash, retrieved.Hash)
	}

	cache.Delete(hash)
	if cache.Size() != 0 {
		t.Errorf("expected empty cache after delete, got size %d", cache.Size())
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test ParseWorkspaceName
	_, err := ParseWorkspaceName("test-workspace")
	if err != nil {
		t.Fatalf("failed to parse valid workspace name: %v", err)
	}

	_, err = ParseWorkspaceName("")
	if err == nil {
		t.Fatal("expected error for empty workspace name")
	}

	_, err = ParseWorkspaceName("invalid workspace")
	if err == nil {
		t.Fatal("expected error for workspace name with spaces")
	}
}