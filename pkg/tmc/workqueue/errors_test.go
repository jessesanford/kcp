// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workqueue

import (
	"errors"
	"testing"
)

func TestIndexNotFoundError(t *testing.T) {
	indexName := "test-index"
	err := NewIndexNotFoundError(indexName)

	// Test error interface
	expectedMsg := "index not found: test-index"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	// Test type assertion
	indexErr, ok := err.(*IndexNotFoundError)
	if !ok {
		t.Error("Expected IndexNotFoundError type")
	}

	if indexErr.IndexName != indexName {
		t.Errorf("Expected IndexName %q, got %q", indexName, indexErr.IndexName)
	}

	// Test IsIndexNotFound
	if !IsIndexNotFound(err) {
		t.Error("IsIndexNotFound should return true for IndexNotFoundError")
	}

	// Test IsIndexNotFound with different error type
	otherErr := errors.New("other error")
	if IsIndexNotFound(otherErr) {
		t.Error("IsIndexNotFound should return false for other error types")
	}
}

func TestInvalidKeyError(t *testing.T) {
	key := "invalid-key"
	reason := "missing cluster information"
	err := NewInvalidKeyError(key, reason)

	// Test error interface
	expectedMsg := "invalid key \"invalid-key\": missing cluster information"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	// Test type assertion
	keyErr, ok := err.(*InvalidKeyError)
	if !ok {
		t.Error("Expected InvalidKeyError type")
	}

	if keyErr.Key != key {
		t.Errorf("Expected Key %q, got %q", key, keyErr.Key)
	}

	if keyErr.Reason != reason {
		t.Errorf("Expected Reason %q, got %q", reason, keyErr.Reason)
	}

	// Test IsInvalidKey
	if !IsInvalidKey(err) {
		t.Error("IsInvalidKey should return true for InvalidKeyError")
	}

	// Test IsInvalidKey with different error type
	otherErr := errors.New("other error")
	if IsInvalidKey(otherErr) {
		t.Error("IsInvalidKey should return false for other error types")
	}
}

func TestErrorTypeCheckers(t *testing.T) {
	// Test with nil error
	if IsIndexNotFound(nil) {
		t.Error("IsIndexNotFound should return false for nil error")
	}

	if IsInvalidKey(nil) {
		t.Error("IsInvalidKey should return false for nil error")
	}

	// Test with generic error
	genericErr := errors.New("generic error")
	
	if IsIndexNotFound(genericErr) {
		t.Error("IsIndexNotFound should return false for generic error")
	}

	if IsInvalidKey(genericErr) {
		t.Error("IsInvalidKey should return false for generic error")
	}

	// Test cross-type checking
	indexErr := NewIndexNotFoundError("test")
	keyErr := NewInvalidKeyError("test", "test")

	if IsInvalidKey(indexErr) {
		t.Error("IsInvalidKey should return false for IndexNotFoundError")
	}

	if IsIndexNotFound(keyErr) {
		t.Error("IsIndexNotFound should return false for InvalidKeyError")
	}
}