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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kcp-dev/logicalcluster/v3"
)

// StatusTransformer handles transforming status for upstream synchronization
type StatusTransformer struct {
	workspace       logicalcluster.Name
	namespacePrefix string
}

// NewStatusTransformer creates a new status transformer
func NewStatusTransformer(workspace logicalcluster.Name) *StatusTransformer {
	return &StatusTransformer{
		workspace:       workspace,
		namespacePrefix: "kcp-",
	}
}

// TransformForUpstream transforms status from downstream format to upstream KCP format
func (t *StatusTransformer) TransformForUpstream(status interface{}, downstreamObj *unstructured.Unstructured) (interface{}, error) {
	if status == nil {
		return nil, nil
	}

	// For now, simple implementation that returns status as-is after basic cleanup
	switch statusMap := status.(type) {
	case map[string]interface{}:
		// Deep copy to avoid modifying original
		result := make(map[string]interface{})
		for k, v := range statusMap {
			result[k] = v
		}

		// Clean downstream-specific fields
		t.cleanDownstreamSpecificFields(result)
		
		return result, nil

	default:
		// For non-map statuses, return as-is
		return status, nil
	}
}

// cleanDownstreamSpecificFields removes downstream-specific fields from status
func (t *StatusTransformer) cleanDownstreamSpecificFields(statusMap map[string]interface{}) {
	// Remove downstream-specific fields if present in status
	fieldsToRemove := []string{
		"downstreamCluster",
		"syncTarget", 
		"syncedAt",
		"kcp.io/sync-target",
		"kcp.io/workspace",
	}

	for _, field := range fieldsToRemove {
		delete(statusMap, field)
	}
}

// reverseNamespaceTransform converts downstream namespace back to upstream namespace
func (t *StatusTransformer) reverseNamespaceTransform(downstreamNamespace string) string {
	if downstreamNamespace == "" {
		return ""
	}

	// Remove workspace-specific prefix: kcp-{workspace}-{original}
	prefix := fmt.Sprintf("%s%s-", t.namespacePrefix, t.workspace)
	if strings.HasPrefix(downstreamNamespace, prefix) {
		return strings.TrimPrefix(downstreamNamespace, prefix)
	}

	return downstreamNamespace
}