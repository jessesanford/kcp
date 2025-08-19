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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"github.com/kcp-dev/logicalcluster/v3"
)

// TMCVirtualWorkspaceConfig holds configuration for the TMC virtual workspace.
type TMCVirtualWorkspaceConfig struct {
	// RootPathPrefix is the root path prefix for virtual workspace endpoints
	RootPathPrefix string

	// EnabledWorkspaces specifies which workspaces should have TMC virtual access
	EnabledWorkspaces []logicalcluster.Name

	// AllowedResources specifies which TMC resources are available through the virtual workspace
	AllowedResources []schema.GroupVersionResource

	// MaxConcurrentRequests limits the number of concurrent requests to the virtual workspace
	MaxConcurrentRequests int
}

// TMCVirtualWorkspaceStatus tracks the status of the TMC virtual workspace.
type TMCVirtualWorkspaceStatus struct {
	// Ready indicates if the TMC virtual workspace is ready to serve requests
	Ready bool

	// ActiveConnections tracks the number of active connections to the virtual workspace
	ActiveConnections int

	// LastError contains the last error encountered by the virtual workspace
	LastError error

	// AvailableResources lists the TMC resources currently available through the virtual workspace
	AvailableResources []schema.GroupVersionResource
}

// TMCResourceFilter provides filtering capabilities for TMC resources in virtual workspaces.
type TMCResourceFilter struct {
	// IncludeNamespaces specifies namespaces to include (empty means all)
	IncludeNamespaces []string

	// ExcludeNamespaces specifies namespaces to exclude
	ExcludeNamespaces []string

	// LabelSelector applies a label selector filter to resources
	LabelSelector string

	// FieldSelector applies a field selector filter to resources
	FieldSelector string
}

// Default TMC virtual workspace configuration values.
const (
	DefaultMaxConcurrentRequests = 100
	DefaultRootPathPrefix        = "/services/tmc"
)

// DefaultTMCVirtualWorkspaceConfig returns a default TMC virtual workspace configuration.
func DefaultTMCVirtualWorkspaceConfig() *TMCVirtualWorkspaceConfig {
	return &TMCVirtualWorkspaceConfig{
		RootPathPrefix:        DefaultRootPathPrefix,
		EnabledWorkspaces:     []logicalcluster.Name{},
		AllowedResources:      DefaultTMCResources(),
		MaxConcurrentRequests: DefaultMaxConcurrentRequests,
	}
}

// DefaultTMCResources returns the default set of TMC resources available in virtual workspaces.
func DefaultTMCResources() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "clusterregistrations"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "syncerconfigs"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "workloadsyncs"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Resource: "syncertunnels"},
	}
}