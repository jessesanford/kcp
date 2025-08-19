/*
Copyright 2022 The KCP Authors.

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

package framework

import (
	"testing"
	"time"
)

func TestNewTestContext(t *testing.T) {
	tc := NewTestContext(t)
	
	if tc.T != t {
		t.Errorf("Expected T to be %v, got %v", t, tc.T)
	}
	
	if tc.Context == nil {
		t.Error("Expected Context to be non-nil")
	}
	
	if tc.CancelFunc == nil {
		t.Error("Expected CancelFunc to be non-nil")
	}
	
	if tc.TestTimeout != 10*time.Minute {
		t.Errorf("Expected TestTimeout to be 10m, got %v", tc.TestTimeout)
	}
	
	if tc.PollInterval != 1*time.Second {
		t.Errorf("Expected PollInterval to be 1s, got %v", tc.PollInterval)
	}
}

func TestTestContextCleanup(t *testing.T) {
	tc := NewTestContext(t)
	
	cleanupCalled := false
	tc.AddCleanup(func() {
		cleanupCalled = true
	})
	
	tc.Cleanup()
	
	if !cleanupCalled {
		t.Error("Expected cleanup function to be called")
	}
}

func TestTestHelper(t *testing.T) {
	h := NewTestHelper()
	
	t.Run("GenerateTestName", func(t *testing.T) {
		name := h.GenerateTestName("test")
		if name == "" {
			t.Error("Expected non-empty test name")
		}
		
		// Should start with prefix
		if len(name) < 4 || name[:4] != "test" {
			t.Errorf("Expected name to start with 'test', got %s", name)
		}
	})
	
	t.Run("SanitizeName", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"TestName", "testname"},
			{"test_name", "test-name"},
			{"test.name", "test-name"},
			{"test/name", "test-name"},
			{"test\\name", "test-name"},
			{"-test-", "test"},
		}
		
		for _, test := range tests {
			result := h.SanitizeName(test.input)
			if result != test.expected {
				t.Errorf("SanitizeName(%s) = %s, want %s", test.input, result, test.expected)
			}
		}
	})
	
	t.Run("GenerateTestLabels", func(t *testing.T) {
		testName := "my-test"
		labels := h.GenerateTestLabels(testName)
		
		if labels["test.kcp.io/test-name"] != testName {
			t.Errorf("Expected test-name label to be %s, got %s", testName, labels["test.kcp.io/test-name"])
		}
		
		if labels["test.kcp.io/created-by"] != "kcp-test-framework" {
			t.Errorf("Expected created-by label to be 'kcp-test-framework', got %s", labels["test.kcp.io/created-by"])
		}
	})
	
	t.Run("HasRequiredLabels", func(t *testing.T) {
		required := map[string]string{
			"app":     "test",
			"version": "v1",
		}
		
		actual := map[string]string{
			"app":        "test",
			"version":    "v1",
			"extra":      "label",
		}
		
		if !h.HasRequiredLabels(required, actual) {
			t.Error("Expected HasRequiredLabels to return true")
		}
		
		actualMissing := map[string]string{
			"app": "test",
			// missing "version"
		}
		
		if h.HasRequiredLabels(required, actualMissing) {
			t.Error("Expected HasRequiredLabels to return false for missing labels")
		}
	})
	
	t.Run("IsTestResource", func(t *testing.T) {
		testLabels := h.GenerateTestLabels("test")
		if !h.IsTestResource(testLabels) {
			t.Error("Expected IsTestResource to return true for test framework labels")
		}
		
		otherLabels := map[string]string{
			"app": "other",
		}
		if h.IsTestResource(otherLabels) {
			t.Error("Expected IsTestResource to return false for non-test labels")
		}
	})
}