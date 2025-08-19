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

package discovery

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/endpoints/discovery"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"

	"github.com/kcp-dev/kcp/pkg/features"
)

// TMCVirtualWorkspaceDiscovery provides API discovery for TMC resources
// within virtual workspaces, enabling clients to discover available TMC APIs.
type TMCVirtualWorkspaceDiscovery struct {
	dynamicClient     kcpdynamic.ClusterInterface
	workspace         logicalcluster.Name
	tmcResources      []TMCResourceInfo
	serverVersion     *version.Info
}

// TMCResourceInfo contains information about a TMC resource for discovery.
type TMCResourceInfo struct {
	// GroupVersionResource identifies the resource
	GroupVersionResource schema.GroupVersionResource
	
	// Kind is the resource kind name
	Kind string
	
	// Namespaced indicates if the resource is namespaced
	Namespaced bool
	
	// Verbs lists the supported operations
	Verbs []string
	
	// Categories lists the resource categories
	Categories []string
}

// TMCDiscoveryConfig configures the TMC virtual workspace discovery.
type TMCDiscoveryConfig struct {
	// DynamicClient provides access to cluster resources
	DynamicClient kcpdynamic.ClusterInterface
	
	// Workspace specifies the target logical cluster
	Workspace logicalcluster.Name
	
	// ServerVersion provides version information
	ServerVersion *version.Info
}

// NewTMCVirtualWorkspaceDiscovery creates a new TMC virtual workspace discovery service.
func NewTMCVirtualWorkspaceDiscovery(config TMCDiscoveryConfig) *TMCVirtualWorkspaceDiscovery {
	return &TMCVirtualWorkspaceDiscovery{
		dynamicClient: config.DynamicClient,
		workspace:     config.Workspace,
		tmcResources:  getDefaultTMCResources(),
		serverVersion: config.ServerVersion,
	}
}

// ServerVersion returns the server version information.
func (d *TMCVirtualWorkspaceDiscovery) ServerVersion() *version.Info {
	if d.serverVersion != nil {
		return d.serverVersion
	}
	
	// Default version information
	return &version.Info{
		Major:      "1",
		Minor:      "0",
		GitVersion: "v1.0.0+tmc",
		GitCommit:  "tmc-virtual-workspace",
		BuildDate:  "",
		GoVersion:  "",
		Compiler:   "",
		Platform:   "",
	}
}

// ServerGroups returns the supported API groups for TMC virtual workspace.
func (d *TMCVirtualWorkspaceDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return &metav1.APIGroupList{}, nil
	}

	groups := make(map[string]*metav1.APIGroup)
	
	// Group TMC resources by API group
	for _, resource := range d.tmcResources {
		groupName := resource.GroupVersionResource.Group
		
		if _, exists := groups[groupName]; !exists {
			groups[groupName] = &metav1.APIGroup{
				Name: groupName,
				Versions: []metav1.GroupVersionForDiscovery{},
			}
		}
		
		// Add version if not already present
		version := resource.GroupVersionResource.Version
		found := false
		for _, v := range groups[groupName].Versions {
			if v.Version == version {
				found = true
				break
			}
		}
		
		if !found {
			groups[groupName].Versions = append(groups[groupName].Versions, metav1.GroupVersionForDiscovery{
				GroupVersion: resource.GroupVersionResource.GroupVersion().String(),
				Version:      version,
			})
		}
	}
	
	// Convert to slice and sort
	var groupList []metav1.APIGroup
	for _, group := range groups {
		// Sort versions
		sort.Slice(group.Versions, func(i, j int) bool {
			return group.Versions[i].Version < group.Versions[j].Version
		})
		
		// Set preferred version (latest)
		if len(group.Versions) > 0 {
			group.PreferredVersion = group.Versions[len(group.Versions)-1]
		}
		
		groupList = append(groupList, *group)
	}
	
	// Sort groups
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].Name < groupList[j].Name
	})

	klog.V(4).Infof("TMC virtual workspace discovery returning %d API groups", len(groupList))
	
	return &metav1.APIGroupList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "APIGroupList",
		},
		Groups: groupList,
	}, nil
}

// ServerResourcesForGroupVersion returns the resources for a specific group version.
func (d *TMCVirtualWorkspaceDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return &metav1.APIResourceList{}, nil
	}

	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid group version %s: %w", groupVersion, err)
	}
	
	var resources []metav1.APIResource
	
	// Find resources matching the group version
	for _, resource := range d.tmcResources {
		if resource.GroupVersionResource.Group == gv.Group && resource.GroupVersionResource.Version == gv.Version {
			apiResource := metav1.APIResource{
				Name:         resource.GroupVersionResource.Resource,
				SingularName: getSingularName(resource.GroupVersionResource.Resource),
				Namespaced:   resource.Namespaced,
				Kind:         resource.Kind,
				Verbs:        resource.Verbs,
				Categories:   resource.Categories,
			}
			resources = append(resources, apiResource)
		}
	}
	
	// Sort resources
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	klog.V(4).Infof("TMC virtual workspace discovery returning %d resources for group version %s", 
		len(resources), groupVersion)
	
	return &metav1.APIResourceList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "APIResourceList",
		},
		GroupVersion: groupVersion,
		APIResources: resources,
	}, nil
}

// getDefaultTMCResources returns the default set of TMC resources for discovery.
func getDefaultTMCResources() []TMCResourceInfo {
	return []TMCResourceInfo{
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "clusterregistrations",
			},
			Kind:       "ClusterRegistration",
			Namespaced: false,
			Verbs:      []string{"get", "list", "create", "update", "patch", "delete"},
			Categories: []string{"tmc", "cluster"},
		},
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "workloadplacements",
			},
			Kind:       "WorkloadPlacement",
			Namespaced: true,
			Verbs:      []string{"get", "list", "create", "update", "patch", "delete"},
			Categories: []string{"tmc", "placement"},
		},
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "syncerconfigs",
			},
			Kind:       "SyncerConfig",
			Namespaced: false,
			Verbs:      []string{"get", "list", "create", "update", "patch", "delete"},
			Categories: []string{"tmc", "syncer"},
		},
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "workloadsyncs",
			},
			Kind:       "WorkloadSync",
			Namespaced: true,
			Verbs:      []string{"get", "list", "create", "update", "patch"},
			Categories: []string{"tmc", "sync"},
		},
		{
			GroupVersionResource: schema.GroupVersionResource{
				Group:    "tmc.kcp.io",
				Version:  "v1alpha1",
				Resource: "syncertunnels",
			},
			Kind:       "SyncerTunnel",
			Namespaced: false,
			Verbs:      []string{"get", "list", "create", "update", "patch", "delete"},
			Categories: []string{"tmc", "syncer"},
		},
	}
}

// getSingularName converts a plural resource name to singular.
func getSingularName(plural string) string {
	// Simple conversion: remove 's' suffix
	if len(plural) > 1 && plural[len(plural)-1] == 's' {
		return plural[:len(plural)-1]
	}
	return plural
}