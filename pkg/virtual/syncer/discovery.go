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

package syncer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// DiscoveryInfo provides API discovery information filtered for a specific syncer.
type DiscoveryInfo struct {
	syncerID          string
	workspace         string
	supportedResources []string
}

// NewDiscoveryInfo creates discovery info for a syncer based on its permissions.
func NewDiscoveryInfo(syncerID, workspace string, syncTarget *workloadv1alpha1.SyncTarget) *DiscoveryInfo {
	discovery := &DiscoveryInfo{
		syncerID:  syncerID,
		workspace: workspace,
	}

	// Determine supported resources based on SyncTarget configuration
	if syncTarget != nil && len(syncTarget.Spec.SupportedResourceTypes) > 0 {
		discovery.supportedResources = syncTarget.Spec.SupportedResourceTypes
	} else {
		// Default set of resources that syncers typically need access to
		discovery.supportedResources = []string{
			"synctargets",
			"pods",
			"services",
			"deployments",
			"configmaps",
			"secrets",
		}
	}

	return discovery
}

// GetAPIGroupList returns the API groups available to this syncer.
func (d *DiscoveryInfo) GetAPIGroupList() *metav1.APIGroupList {
	groups := &metav1.APIGroupList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIGroupList",
			APIVersion: "v1",
		},
	}

	// Always include the workload API group for synctargets
	workloadGroup := metav1.APIGroup{
		Name: "workload.kcp.io",
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: workloadv1alpha1.SchemeGroupVersion.String(),
				Version:      workloadv1alpha1.SchemeGroupVersion.Version,
			},
		},
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: workloadv1alpha1.SchemeGroupVersion.String(),
			Version:      workloadv1alpha1.SchemeGroupVersion.Version,
		},
	}
	groups.Groups = append(groups.Groups, workloadGroup)

	// Add core API group if syncer needs access to core resources
	if d.supportsResource("pods") || d.supportsResource("services") || d.supportsResource("configmaps") {
		coreGroup := metav1.APIGroup{
			Name: "",
			Versions: []metav1.GroupVersionForDiscovery{
				{
					GroupVersion: "v1",
					Version:      "v1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "v1",
				Version:      "v1",
			},
		}
		groups.Groups = append(groups.Groups, coreGroup)
	}

	klog.V(4).InfoS("Generated API group list for syncer", "syncerID", d.syncerID, "groups", len(groups.Groups))
	return groups
}

// GetAPIResourceList returns the API resources available in a specific group/version.
func (d *DiscoveryInfo) GetAPIResourceList(groupVersion string) *metav1.APIResourceList {
	resources := &metav1.APIResourceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIResourceList",
			APIVersion: "v1",
		},
		GroupVersion: groupVersion,
	}

	switch groupVersion {
	case workloadv1alpha1.SchemeGroupVersion.String():
		// Add workload resources that this syncer can access
		if d.supportsResource("synctargets") {
			resources.APIResources = append(resources.APIResources, metav1.APIResource{
				Name:       "synctargets",
				Namespaced: false,
				Kind:       "SyncTarget",
				Verbs:      []string{"get", "list", "watch"},
			})
		}

	case "v1":
		// Add core resources that this syncer can access
		if d.supportsResource("pods") {
			resources.APIResources = append(resources.APIResources, metav1.APIResource{
				Name:       "pods",
				Namespaced: true,
				Kind:       "Pod",
				Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			})
		}

		if d.supportsResource("services") {
			resources.APIResources = append(resources.APIResources, metav1.APIResource{
				Name:       "services",
				Namespaced: true,
				Kind:       "Service",
				Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			})
		}

		if d.supportsResource("configmaps") {
			resources.APIResources = append(resources.APIResources, metav1.APIResource{
				Name:       "configmaps",
				Namespaced: true,
				Kind:       "ConfigMap",
				Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			})
		}

		if d.supportsResource("secrets") {
			resources.APIResources = append(resources.APIResources, metav1.APIResource{
				Name:       "secrets",
				Namespaced: true,
				Kind:       "Secret",
				Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			})
		}
	}

	klog.V(4).InfoS("Generated API resource list for syncer", 
		"syncerID", d.syncerID, 
		"groupVersion", groupVersion, 
		"resources", len(resources.APIResources))

	return resources
}

// GetVersion returns version information for the syncer API endpoint.
func (d *DiscoveryInfo) GetVersion() *version.Info {
	return &version.Info{
		Major:        "1",
		Minor:        "0",
		GitVersion:   "v1.0.0+kcp-syncer",
		GitCommit:    "unknown",
		GitTreeState: "clean",
		BuildDate:    "unknown",
		GoVersion:    "unknown",
		Compiler:     "gc",
		Platform:     "linux/amd64",
	}
}

// supportsResource checks if this syncer supports access to a specific resource type.
func (d *DiscoveryInfo) supportsResource(resourceType string) bool {
	for _, supported := range d.supportedResources {
		if supported == resourceType {
			return true
		}
	}
	return false
}