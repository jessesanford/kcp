/*
Copyright 2023 The KCP Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceDiscoveryResult contains the results of workspace discovery, including
// the workspace itself, its available sync targets, and authorization status.
type WorkspaceDiscoveryResult struct {
	// Workspace is the discovered workspace
	Workspace *tenancyv1alpha1.Workspace
	
	// SyncTargets contains the sync targets available in this workspace
	SyncTargets []*workloadv1alpha1.SyncTarget
	
	// Authorized indicates if the current user is authorized to access this workspace
	Authorized bool
	
	// Error contains any error encountered during discovery for this workspace
	Error error
}

// DiscoveryOptions contains options for workspace discovery operations.
type DiscoveryOptions struct {
	// MaxDepth limits how deep to traverse workspace hierarchies (0 = no limit)
	MaxDepth int
	
	// IncludeNotReady includes workspaces that are not in Ready phase
	IncludeNotReady bool
	
	// LabelSelector filters workspaces by labels
	LabelSelector labels.Selector
	
	// SyncTargetSelector filters sync targets by labels
	SyncTargetSelector labels.Selector
	
	// User is the user context for authorization checks
	User user.Info
}

// WorkspaceDiscoverer provides discovery capabilities for workspaces and their resources
// within the KCP multi-tenant environment.
type WorkspaceDiscoverer interface {
	// DiscoverWorkspaces discovers workspaces based on the provided options
	DiscoverWorkspaces(ctx context.Context, opts DiscoveryOptions) ([]*WorkspaceDiscoveryResult, error)
	
	// DiscoverSyncTargets discovers sync targets within a specific workspace
	DiscoverSyncTargets(ctx context.Context, workspace logicalcluster.Name, opts DiscoveryOptions) ([]*workloadv1alpha1.SyncTarget, error)
	
	// GetWorkspaceHierarchy returns the full hierarchy for a given workspace
	GetWorkspaceHierarchy(ctx context.Context, workspace logicalcluster.Name) ([]*tenancyv1alpha1.Workspace, error)
}

// AuthorizationChecker provides workspace authorization capabilities.
type AuthorizationChecker interface {
	// CanAccessWorkspace checks if a user can access a specific workspace
	CanAccessWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) (bool, error)
	
	// GetPermittedWorkspaces returns all workspaces the user has access to
	GetPermittedWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error)
}

// WorkspaceIndex provides indexed access to workspace data for efficient lookups.
type WorkspaceIndex interface {
	// GetByLabel returns workspaces matching the label selector
	GetByLabel(selector labels.Selector) ([]*tenancyv1alpha1.Workspace, error)
	
	// GetChildren returns direct children of a workspace
	GetChildren(workspace logicalcluster.Name) ([]*tenancyv1alpha1.Workspace, error)
	
	// GetParent returns the parent workspace
	GetParent(workspace logicalcluster.Name) (*tenancyv1alpha1.Workspace, error)
}

// SyncTargetIndex provides indexed access to sync target data.
type SyncTargetIndex interface {
	// GetByWorkspace returns all sync targets in a workspace
	GetByWorkspace(workspace logicalcluster.Name) ([]*workloadv1alpha1.SyncTarget, error)
	
	// GetByLabel returns sync targets matching the label selector
	GetByLabel(workspace logicalcluster.Name, selector labels.Selector) ([]*workloadv1alpha1.SyncTarget, error)
}

// IndexingOptions configures the indexing behavior for workspace discovery.
type IndexingOptions struct {
	// WorkspaceInformer provides workspace change notifications
	WorkspaceInformer cache.SharedIndexInformer
	
	// SyncTargetInformer provides sync target change notifications  
	SyncTargetInformer cache.SharedIndexInformer
	
	// ResyncPeriod defines how often to rebuild indices
	ResyncPeriod metav1.Duration
}