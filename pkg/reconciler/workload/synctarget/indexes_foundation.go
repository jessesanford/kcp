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

package synctarget

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	// IndexByCluster is the index name for cluster-based lookups
	IndexByCluster = "synctarget.cluster"

	// IndexByWorkspace is the index name for workspace-based lookups
	IndexByWorkspace = "synctarget.workspace"
)

// IndexByClusterFunc returns the cluster name for indexing
func IndexByClusterFunc(obj interface{}) ([]string, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return nil, nil
	}

	accessor, err := meta.Accessor(runtimeObj)
	if err != nil {
		return nil, err
	}

	// Look for cluster annotation or label
	cluster := accessor.GetAnnotations()["kcp.io/cluster"]
	if cluster == "" {
		cluster = accessor.GetLabels()["kcp.io/cluster"]
	}

	if cluster == "" {
		return nil, nil
	}

	return []string{cluster}, nil
}

// IndexByWorkspaceFunc returns the workspace for indexing
func IndexByWorkspaceFunc(obj interface{}) ([]string, error) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return nil, nil
	}

	accessor, err := meta.Accessor(runtimeObj)
	if err != nil {
		return nil, err
	}

	workspace := accessor.GetAnnotations()["kcp.io/workspace"]
	if workspace == "" {
		workspace = accessor.GetLabels()["kcp.io/workspace"]
	}

	if workspace == "" {
		return nil, nil
	}

	return []string{workspace}, nil
}

// AddIndexes adds custom indexes to the informer
func AddIndexes(informer cache.SharedIndexInformer) error {
	if err := informer.AddIndexers(cache.Indexers{
		IndexByCluster: IndexByClusterFunc,
	}); err != nil {
		return err
	}

	if err := informer.AddIndexers(cache.Indexers{
		IndexByWorkspace: IndexByWorkspaceFunc,
	}); err != nil {
		return err
	}

	return nil
}