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

package tmc

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpdynamic "github.com/kcp-dev/client-go/dynamic"
	kcpkubernetesinformers "github.com/kcp-dev/client-go/informers"
	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"

	virtualframework "github.com/kcp-dev/kcp/pkg/virtual/framework"
	virtualdynamic "github.com/kcp-dev/kcp/pkg/virtual/framework/dynamic"
	virtualrootapiserver "github.com/kcp-dev/kcp/pkg/virtual/framework/rootapiserver"
	"github.com/kcp-dev/kcp/pkg/features"
)

const (
	// TMCVirtualWorkspaceName is the name of the TMC virtual workspace
	TMCVirtualWorkspaceName = "tmc"
)

// BuildVirtualWorkspace creates a virtual workspace for TMC operations.
// This provides isolated access to TMC resources across multiple physical clusters.
func BuildVirtualWorkspace(
	rootPathPrefix string,
	kubeClusterClient kcpdynamic.ClusterInterface,
	dynamicClusterClient kcpdynamic.ClusterInterface,
	kcpInformerFactory kcpinformers.SharedInformerFactory,
	kubeInformerFactory kcpkubernetesinformers.SharedInformerFactory,
) ([]virtualrootapiserver.NamedVirtualWorkspace, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		klog.V(2).Info("TMC virtual workspace disabled by feature gate")
		return nil, nil
	}

	// Create TMC virtual workspace
	tmcVirtualWorkspace, err := buildTMCVirtualWorkspace(
		rootPathPrefix,
		kubeClusterClient,
		dynamicClusterClient,
		kcpInformerFactory,
		kubeInformerFactory,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build TMC virtual workspace: %w", err)
	}

	return []virtualrootapiserver.NamedVirtualWorkspace{
		{
			Name:             TMCVirtualWorkspaceName,
			VirtualWorkspace: tmcVirtualWorkspace,
		},
	}, nil
}

// buildTMCVirtualWorkspace builds the TMC-specific virtual workspace implementation.
func buildTMCVirtualWorkspace(
	rootPathPrefix string,
	kubeClusterClient kcpdynamic.ClusterInterface,
	dynamicClusterClient kcpdynamic.ClusterInterface,
	kcpInformerFactory kcpinformers.SharedInformerFactory,
	kubeInformerFactory kcpkubernetesinformers.SharedInformerFactory,
) (virtualframework.VirtualWorkspace, error) {
	// TMC resources that should be available in the virtual workspace
	tmcResources := []schema.GroupVersionResource{
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "clusterregistrations"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "syncerconfigs"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "workloadsyncs"},
	}

	readyChecker := func(ctx context.Context) error {
		// Check if TMC APIs are available
		apiExportInformer := kcpInformerFactory.Apis().V1alpha1().APIExports()
		apiExports, err := apiExportInformer.Lister().List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list API exports: %w", err)
		}

		// Look for TMC API export
		for _, export := range apiExports {
			if export.Name == "tmc.kcp.io" {
				// Found TMC API export, virtual workspace is ready
				return nil
			}
		}

		return fmt.Errorf("TMC API export not found")
	}

	// Build dynamic virtual workspace for TMC resources
	return virtualdynamic.NewDynamicVirtualWorkspace(
		dynamicClusterClient,
		tmcResources,
		readyChecker,
	), nil
}

// getTMCWorkspaces returns the list of workspaces that have TMC resources.
func getTMCWorkspaces(ctx context.Context, kcpInformerFactory kcpinformers.SharedInformerFactory) ([]logicalcluster.Name, error) {
	// Get all workspaces that have TMC API exports
	apiExportInformer := kcpInformerFactory.Apis().V1alpha1().APIExports()
	apiExports, err := apiExportInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list API exports: %w", err)
	}

	var tmcWorkspaces []logicalcluster.Name
	for _, export := range apiExports {
		if export.Name == "tmc.kcp.io" {
			workspace := logicalcluster.From(export)
			tmcWorkspaces = append(tmcWorkspaces, workspace)
		}
	}

	return tmcWorkspaces, nil
}