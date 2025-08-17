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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DefaultStatusExtractor provides default status extraction for any resource
type DefaultStatusExtractor struct{}

// ExtractStatus extracts the entire status subresource
func (e *DefaultStatusExtractor) ExtractStatus(obj *unstructured.Unstructured) (interface{}, error) {
	status, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to extract status: %w", err)
	}
	if !found {
		return nil, nil
	}
	return status, nil
}

// ShouldExtract returns true if the object has a status subresource
func (e *DefaultStatusExtractor) ShouldExtract(obj *unstructured.Unstructured) bool {
	_, found, _ := unstructured.NestedFieldNoCopy(obj.Object, "status")
	return found
}

// GetExtractorForResource returns the appropriate extractor for a given resource type
func GetExtractorForResource(obj *unstructured.Unstructured) StatusExtractor {
	// For now, use default extractor for all resources
	// Future implementations can add specific extractors
	return &DefaultStatusExtractor{}
}