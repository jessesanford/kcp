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

package applier

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ApplyResult represents the result of applying a single resource.
type ApplyResult struct {
	// GVR is the GroupVersionResource of the applied resource
	GVR schema.GroupVersionResource
	// Namespace is the namespace of the resource (empty for cluster-scoped)
	Namespace string
	// Name is the name of the resource
	Name string
	// Operation indicates what operation was performed (create, update, apply, noop)
	Operation string
	// Success indicates whether the operation succeeded
	Success bool
	// Error contains any error that occurred during the operation
	Error error
	// Applied is the resource as it exists after the operation
	Applied *unstructured.Unstructured
	// Attempts is the number of retry attempts made
	Attempts int
	// Duration is how long the operation took
	Duration time.Duration
}

// BatchResult aggregates results from multiple apply operations.
type BatchResult struct {
	// Results contains all individual operation results
	Results []*ApplyResult
	// Total is the total number of operations attempted
	Total int
	// Succeeded is the number of successful operations
	Succeeded int
	// Failed is the number of failed operations
	Failed int
	// Duration is the total time for all operations
	Duration time.Duration
}

// SuccessRate returns the percentage of successful operations.
func (br *BatchResult) SuccessRate() float64 {
	if br.Total == 0 {
		return 0.0
	}
	return float64(br.Succeeded) / float64(br.Total) * 100.0
}

// Errors returns all errors from failed operations.
func (br *BatchResult) Errors() []error {
	var errors []error
	for _, result := range br.Results {
		if result.Error != nil {
			errors = append(errors, result.Error)
		}
	}
	return errors
}

// Summary returns a human-readable summary of the batch result.
func (br *BatchResult) Summary() string {
	if br.Total == 0 {
		return "No operations performed"
	}
	
	if br.Failed == 0 {
		return "All operations succeeded"
	}
	
	if br.Succeeded == 0 {
		return "All operations failed"
	}
	
	return "Partial success: some operations failed"
}