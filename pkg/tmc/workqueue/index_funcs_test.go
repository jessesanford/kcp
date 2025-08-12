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
	"reflect"
	"sort"
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestWorkspaceIndexFunc(t *testing.T) {
	tests := map[string]struct {
		key           string
		expectedValues []string
		expectError   bool
	}{
		"cluster-scoped resource": {
			key:           "root:test-workspace|test-resource",
			expectedValues: []string{"root:test-workspace"},
			expectError:   false,
		},
		"namespaced resource": {
			key:           "root:test-workspace|test-namespace/test-resource",
			expectedValues: []string{"root:test-workspace"},
			expectError:   false,
		},
		"invalid key": {
			key:         "invalid-key-format",
			expectError: true,
		},
		"empty cluster": {
			key:         "|test-namespace/test-resource",
			expectError: true,
		},
	}

	indexFunc := WorkspaceIndexFunc()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			values, err := indexFunc(tc.key)

			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if !reflect.DeepEqual(values, tc.expectedValues) {
					t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
				}
			}
		})
	}
}

func TestNamespaceIndexFunc(t *testing.T) {
	tests := map[string]struct {
		key           string
		expectedValues []string
		expectError   bool
	}{
		"cluster-scoped resource": {
			key:           "root:test-workspace|test-resource",
			expectedValues: []string{}, // Empty slice for cluster-scoped
			expectError:   false,
		},
		"namespaced resource": {
			key:           "root:test-workspace|test-namespace/test-resource",
			expectedValues: []string{"test-namespace"},
			expectError:   false,
		},
		"invalid key": {
			key:         "",
			expectedValues: []string{},
			expectError: false, // Empty key should not error but return no namespace
		},
	}

	indexFunc := NamespaceIndexFunc()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			values, err := indexFunc(tc.key)

			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if !reflect.DeepEqual(values, tc.expectedValues) {
					t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
				}
			}
		})
	}
}

func TestResourceTypeIndexFunc(t *testing.T) {
	tests := map[string]struct {
		defaultType   string
		key           string
		expectedValues []string
	}{
		"with default type": {
			defaultType:   "pods",
			key:           "root:test|test-namespace/test-pod",
			expectedValues: []string{"pods"},
		},
		"empty default type": {
			defaultType:   "",
			key:           "root:test|test-resource",
			expectedValues: []string{"unknown"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			indexFunc := ResourceTypeIndexFunc(tc.defaultType)
			values, err := indexFunc(tc.key)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(values, tc.expectedValues) {
				t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
			}
		})
	}
}

func TestWorkspaceAndNamespaceIndexFunc(t *testing.T) {
	tests := map[string]struct {
		key           string
		expectedValues []string
		expectError   bool
	}{
		"cluster-scoped resource": {
			key:           "root:test-workspace|test-resource",
			expectedValues: []string{"root:test-workspace|<cluster-scoped>"},
			expectError:   false,
		},
		"namespaced resource": {
			key:           "root:test-workspace|test-namespace/test-resource",
			expectedValues: []string{"root:test-workspace|test-namespace"},
			expectError:   false,
		},
		"invalid key": {
			key:         "invalid-key-format",
			expectError: true,
		},
		"empty cluster": {
			key:         "|test-namespace/test-resource",
			expectError: true,
		},
	}

	indexFunc := WorkspaceAndNamespaceIndexFunc()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			values, err := indexFunc(tc.key)

			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if !reflect.DeepEqual(values, tc.expectedValues) {
					t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
				}
			}
		})
	}
}

func TestCustomIndexFunc(t *testing.T) {
	// Create a custom function that returns the resource name
	customFunc := func(cluster logicalcluster.Name, namespace, name string) ([]string, error) {
		return []string{name}, nil
	}

	indexFunc := CustomIndexFunc(customFunc)

	tests := map[string]struct {
		key           string
		expectedValues []string
		expectError   bool
	}{
		"valid key": {
			key:           "root:test|test-namespace/test-resource",
			expectedValues: []string{"test-resource"},
			expectError:   false,
		},
		"cluster-scoped": {
			key:           "root:test|test-resource",
			expectedValues: []string{"test-resource"},
			expectError:   false,
		},
		"invalid key": {
			key:         "",
			expectedValues: []string{""},
			expectError: false, // Empty key should just return empty resource name
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			values, err := indexFunc(tc.key)

			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v", tc.expectError, err)
			}

			if !tc.expectError {
				if !reflect.DeepEqual(values, tc.expectedValues) {
					t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
				}
			}
		})
	}
}

func TestPrefixIndexFunc(t *testing.T) {
	tests := map[string]struct {
		separator     string
		prefixIndex   int
		key           string
		expectedValues []string
	}{
		"dash separator, first part": {
			separator:     "-",
			prefixIndex:   0,
			key:           "root:test|namespace/my-app-deployment",
			expectedValues: []string{"my"},
		},
		"dash separator, second part": {
			separator:     "-",
			prefixIndex:   1,
			key:           "root:test|namespace/my-app-deployment",
			expectedValues: []string{"app"},
		},
		"not enough parts": {
			separator:     "-",
			prefixIndex:   5,
			key:           "root:test|namespace/simple",
			expectedValues: []string{"simple"}, // Returns full name
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			indexFunc := PrefixIndexFunc(tc.separator, tc.prefixIndex)
			values, err := indexFunc(tc.key)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(values, tc.expectedValues) {
				t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
			}
		})
	}
}

func TestCombinedIndexFunc(t *testing.T) {
	// Create combined indexer with workspace and namespace indexers
	indexFunc := CombinedIndexFunc(
		WorkspaceIndexFunc(),
		NamespaceIndexFunc(),
	)

	tests := map[string]struct {
		key           string
		expectedValues []string
		expectError   bool
	}{
		"namespaced resource": {
			key:           "root:test-workspace|test-namespace/test-resource",
			expectedValues: []string{"root:test-workspace", "test-namespace"},
			expectError:   false,
		},
		"cluster-scoped resource": {
			key:           "root:test-workspace|test-resource",
			expectedValues: []string{"root:test-workspace"}, // Namespace indexer returns empty
			expectError:   false,
		},
		"invalid key": {
			key:         "invalid",
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			values, err := indexFunc(tc.key)

			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v", tc.expectError, err)
			}

			if !tc.expectError {
				// Sort both slices for comparison since order doesn't matter
				sort.Strings(values)
				sort.Strings(tc.expectedValues)
				
				if !reflect.DeepEqual(values, tc.expectedValues) {
					t.Errorf("Expected values %v, got %v", tc.expectedValues, values)
				}
			}
		})
	}
}

func TestLabelIndexFunc(t *testing.T) {
	// Test the simplified label index function
	indexFunc := LabelIndexFunc("app")
	values, err := indexFunc("root:test|namespace/resource")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return empty slice since labels can't be extracted from key alone
	if len(values) != 0 {
		t.Errorf("Expected empty slice, got %v", values)
	}
}

func TestIndexConstants(t *testing.T) {
	// Verify that our index constants are properly defined
	expectedConstants := map[string]string{
		"ByWorkspaceIndex":            ByWorkspaceIndex,
		"ByNamespaceIndex":            ByNamespaceIndex,
		"ByResourceTypeIndex":         ByResourceTypeIndex,
		"ByWorkspaceAndNamespaceIndex": ByWorkspaceAndNamespaceIndex,
	}

	for name, constant := range expectedConstants {
		if constant == "" {
			t.Errorf("Constant %s should not be empty", name)
		}
	}

	// Check for uniqueness
	values := make(map[string]bool)
	for _, constant := range expectedConstants {
		if values[constant] {
			t.Errorf("Duplicate constant value: %s", constant)
		}
		values[constant] = true
	}
}