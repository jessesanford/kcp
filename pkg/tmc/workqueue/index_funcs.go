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
	"strings"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Common index names for TMC workqueues
const (
	// ByWorkspaceIndex indexes queue keys by their logical cluster workspace
	ByWorkspaceIndex = "byWorkspace"
	
	// ByNamespaceIndex indexes queue keys by their namespace (if applicable)
	ByNamespaceIndex = "byNamespace"
	
	// ByResourceTypeIndex indexes queue keys by their resource type
	ByResourceTypeIndex = "byResourceType"
	
	// ByWorkspaceAndNamespaceIndex indexes keys by workspace and namespace combination
	ByWorkspaceAndNamespaceIndex = "byWorkspaceAndNamespace"
)

// WorkspaceIndexFunc creates an IndexFunc that extracts the logical cluster
// from KCP-style queue keys. This follows the standard KCP key format:
// cluster|namespace/name or cluster|name for cluster-scoped resources.
func WorkspaceIndexFunc() IndexFunc {
	return func(key string) ([]string, error) {
		cluster, _, _, err := kcpcache.SplitMetaClusterNamespaceKey(key)
		if err != nil {
			return nil, NewInvalidKeyError(key, "cannot split cluster namespace key: "+err.Error())
		}
		
		if cluster.Empty() {
			return nil, NewInvalidKeyError(key, "empty logical cluster")
		}
		
		return []string{cluster.String()}, nil
	}
}

// NamespaceIndexFunc creates an IndexFunc that extracts the namespace
// from KCP-style queue keys. Returns empty slice for cluster-scoped resources.
func NamespaceIndexFunc() IndexFunc {
	return func(key string) ([]string, error) {
		_, namespace, _, err := kcpcache.SplitMetaClusterNamespaceKey(key)
		if err != nil {
			return nil, NewInvalidKeyError(key, "cannot split cluster namespace key: "+err.Error())
		}
		
		// Return namespace only if it's not empty (cluster-scoped resources have empty namespace)
		if namespace == "" {
			return []string{}, nil
		}
		
		return []string{namespace}, nil
	}
}

// ResourceTypeIndexFunc creates an IndexFunc that extracts the resource type
// from queue keys. This assumes the resource type is encoded in the key somehow.
// For simple cases, this returns a default value, but can be customized.
func ResourceTypeIndexFunc(defaultResourceType string) IndexFunc {
	return func(key string) ([]string, error) {
		// For now, return the default resource type
		// In a real implementation, this could parse the key to determine
		// the actual resource type from the key format or metadata
		if defaultResourceType == "" {
			return []string{"unknown"}, nil
		}
		return []string{defaultResourceType}, nil
	}
}

// WorkspaceAndNamespaceIndexFunc creates an IndexFunc that combines
// workspace and namespace information for more specific indexing.
func WorkspaceAndNamespaceIndexFunc() IndexFunc {
	return func(key string) ([]string, error) {
		cluster, namespace, _, err := kcpcache.SplitMetaClusterNamespaceKey(key)
		if err != nil {
			return nil, NewInvalidKeyError(key, "cannot split cluster namespace key: "+err.Error())
		}
		
		if cluster.Empty() {
			return nil, NewInvalidKeyError(key, "empty logical cluster")
		}
		
		if namespace == "" {
			// Cluster-scoped resource
			return []string{cluster.String() + "|<cluster-scoped>"}, nil
		}
		
		// Namespaced resource
		return []string{cluster.String() + "|" + namespace}, nil
	}
}

// CustomIndexFunc creates an IndexFunc from a custom function that takes
// the logical cluster, namespace, and name components of a KCP key.
// This provides a convenient way to create domain-specific indexers.
func CustomIndexFunc(fn func(cluster logicalcluster.Name, namespace, name string) ([]string, error)) IndexFunc {
	return func(key string) ([]string, error) {
		cluster, namespace, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
		if err != nil {
			return nil, NewInvalidKeyError(key, "cannot split cluster namespace key: "+err.Error())
		}
		
		return fn(cluster, namespace, name)
	}
}

// PrefixIndexFunc creates an IndexFunc that indexes keys by a prefix
// extracted from the resource name. This is useful for grouping related
// resources that follow naming conventions.
func PrefixIndexFunc(separator string, prefixIndex int) IndexFunc {
	return CustomIndexFunc(func(cluster logicalcluster.Name, namespace, name string) ([]string, error) {
		parts := strings.Split(name, separator)
		if len(parts) <= prefixIndex {
			return []string{name}, nil // Return full name if not enough parts
		}
		
		return []string{parts[prefixIndex]}, nil
	})
}

// LabelIndexFunc creates an IndexFunc that extracts index values from
// labels encoded in the queue key. This assumes labels are somehow
// encoded in the key format (this is a simplified example).
func LabelIndexFunc(labelKey string) IndexFunc {
	return func(key string) ([]string, error) {
		// This is a simplified implementation
		// In practice, labels would need to be encoded in the key
		// or retrieved from a cache/store
		
		// For now, we cannot extract labels from just the key
		// This would typically require access to the actual object
		return []string{}, nil
	}
}

// CombinedIndexFunc creates an IndexFunc that combines multiple indexers
// and returns all their index values. This allows multiple indexing
// strategies for the same key.
func CombinedIndexFunc(indexFuncs ...IndexFunc) IndexFunc {
	return func(key string) ([]string, error) {
		var allValues []string
		
		for _, indexFunc := range indexFuncs {
			values, err := indexFunc(key)
			if err != nil {
				return nil, err
			}
			allValues = append(allValues, values...)
		}
		
		return allValues, nil
	}
}