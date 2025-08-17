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

package syncer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	corelogicalcluster "github.com/kcp-dev/logicalcluster/v3"
)

// VirtualWorkspace provides a virtual view of synced resources across multiple
// clusters and workspaces. It aggregates resources from different physical clusters
// and presents them as a unified virtual workspace.
//
// The VirtualWorkspace coordinates with syncer components to:
// - Transform cluster resources to virtual view
// - Apply placement decisions for resource distribution
// - Handle cross-cluster resource aggregation
// - Provide virtual REST storage for unified access
type VirtualWorkspace struct {
	// logger provides structured logging for the virtual workspace
	logger logr.Logger

	// kcpClient provides access to KCP cluster-aware APIs
	kcpClient kcpclientset.ClusterInterface

	// dynamicClient enables dynamic access to Kubernetes resources  
	// Note: Would use actual dynamic client interface when available
	dynamicClient interface{}

	// informerFactory provides shared informers for efficient resource watching
	informerFactory kcpinformers.SharedInformerFactory

	// apiBindingInformer watches APIBinding resources for API availability
	apiBindingInformer interface{} // apisinformers.APIBindingClusterInformer when available

	// authz handles authorization for virtual workspace requests
	authz authorizer.Authorizer

	// ready tracks the readiness state of the virtual workspace
	ready      bool
	readyMutex sync.RWMutex

	// resourcesLock protects access to resource tracking data
	resourcesLock sync.RWMutex

	// virtualResources maps GroupVersionResource to virtual resource metadata
	virtualResources map[schema.GroupVersionResource]*VirtualResourceInfo

	// clusterResources tracks resources available in each cluster
	clusterResources map[corelogicalcluster.Name]sets.Set[schema.GroupVersionResource]

	// syncerState tracks the state of syncers for each cluster
	syncerState map[corelogicalcluster.Name]*SyncerState

	// transformations holds resource transformation rules
	transformations *ResourceTransformationManager

	// placementManager coordinates resource placement decisions
	placementManager PlacementManager
}

// VirtualResourceInfo contains metadata about a virtual resource
type VirtualResourceInfo struct {
	// GroupVersionResource identifies the resource type
	GroupVersionResource schema.GroupVersionResource

	// Namespaced indicates if the resource is namespace-scoped
	Namespaced bool

	// Clusters tracks which clusters provide this resource
	Clusters sets.Set[corelogicalcluster.Name]

	// TransformationRules define how to transform resources
	TransformationRules []TransformationRule

	// PlacementPolicy defines placement constraints
	PlacementPolicy *PlacementPolicy

	// LastSyncTime tracks the last successful sync
	LastSyncTime time.Time
}

// SyncerState tracks the state of a syncer for a specific cluster
type SyncerState struct {
	// ClusterName identifies the logical cluster
	ClusterName corelogicalcluster.Name

	// Available indicates if the syncer is currently available
	Available bool

	// LastHeartbeat tracks the last heartbeat from the syncer
	LastHeartbeat time.Time

	// SupportedResources lists resources supported by this syncer
	SupportedResources sets.Set[schema.GroupVersionResource]

	// SyncStatus tracks sync status for each resource
	SyncStatus map[schema.GroupVersionResource]*ResourceSyncStatus
}

// ResourceSyncStatus tracks the sync status of a specific resource
type ResourceSyncStatus struct {
	// State indicates the current sync state
	State SyncState

	// LastSyncAttempt records when sync was last attempted
	LastSyncAttempt time.Time

	// LastSuccessfulSync records when sync last succeeded
	LastSuccessfulSync time.Time

	// ErrorCount tracks consecutive sync errors
	ErrorCount int

	// LastError contains the most recent sync error
	LastError string
}

// SyncState represents the state of resource synchronization
type SyncState string

const (
	// SyncStateUnknown indicates sync state is unknown
	SyncStateUnknown SyncState = "Unknown"

	// SyncStatePending indicates sync is pending
	SyncStatePending SyncState = "Pending"

	// SyncStateInProgress indicates sync is in progress
	SyncStateInProgress SyncState = "InProgress"

	// SyncStateSynced indicates resource is synced
	SyncStateSynced SyncState = "Synced"

	// SyncStateError indicates sync failed
	SyncStateError SyncState = "Error"
)

// TransformationRule defines how to transform a resource
type TransformationRule struct {
	// Name identifies the transformation rule
	Name string

	// SourceGVR is the source GroupVersionResource
	SourceGVR schema.GroupVersionResource

	// TargetGVR is the target GroupVersionResource
	TargetGVR schema.GroupVersionResource

	// FieldMappings define field transformation mappings
	FieldMappings []FieldMapping

	// NamespaceMapping defines namespace transformation
	NamespaceMapping *NamespaceMapping
}

// FieldMapping defines field transformation between source and target
type FieldMapping struct {
	// SourceField is the JSONPath to the source field
	SourceField string

	// TargetField is the JSONPath to the target field
	TargetField string

	// Transform is the transformation function to apply
	Transform TransformFunction
}

// NamespaceMapping defines namespace transformation rules
type NamespaceMapping struct {
	// SourceNamespace is the source namespace pattern
	SourceNamespace string

	// TargetNamespace is the target namespace pattern
	TargetNamespace string

	// Strategy defines the mapping strategy
	Strategy NamespaceMappingStrategy
}

// TransformFunction defines a transformation function type
type TransformFunction string

const (
	// TransformIdentity performs identity transformation
	TransformIdentity TransformFunction = "identity"

	// TransformPrefix adds a prefix to the value
	TransformPrefix TransformFunction = "prefix"

	// TransformSuffix adds a suffix to the value
	TransformSuffix TransformFunction = "suffix"

	// TransformReplace performs string replacement
	TransformReplace TransformFunction = "replace"
)

// NamespaceMappingStrategy defines namespace mapping strategies
type NamespaceMappingStrategy string

const (
	// NamespaceMappingIdentity preserves original namespace
	NamespaceMappingIdentity NamespaceMappingStrategy = "identity"

	// NamespaceMappingPrefix adds cluster prefix to namespace
	NamespaceMappingPrefix NamespaceMappingStrategy = "prefix"

	// NamespaceMappingMap uses explicit mapping
	NamespaceMappingMap NamespaceMappingStrategy = "map"
)

// PlacementPolicy defines resource placement constraints
type PlacementPolicy struct {
	// Name identifies the placement policy
	Name string

	// ClusterSelector selects target clusters for placement
	ClusterSelector labels.Selector

	// ResourceSelector selects resources for placement
	ResourceSelector labels.Selector

	// Strategy defines the placement strategy
	Strategy PlacementStrategy

	// Constraints define placement constraints
	Constraints []PlacementConstraint
}

// PlacementStrategy defines placement strategies
type PlacementStrategy string

const (
	// PlacementStrategySpread spreads resources across clusters
	PlacementStrategySpread PlacementStrategy = "spread"

	// PlacementStrategyPack packs resources into fewer clusters
	PlacementStrategyPack PlacementStrategy = "pack"

	// PlacementStrategyExplicit uses explicit cluster assignment
	PlacementStrategyExplicit PlacementStrategy = "explicit"
)

// PlacementConstraint defines a placement constraint
type PlacementConstraint struct {
	// Type is the constraint type
	Type PlacementConstraintType

	// Key is the constraint key
	Key string

	// Values are the allowed values
	Values []string
}

// PlacementConstraintType defines constraint types
type PlacementConstraintType string

const (
	// PlacementConstraintAffinity defines affinity constraints
	PlacementConstraintAffinity PlacementConstraintType = "affinity"

	// PlacementConstraintAntiAffinity defines anti-affinity constraints
	PlacementConstraintAntiAffinity PlacementConstraintType = "antiAffinity"

	// PlacementConstraintTopology defines topology constraints
	PlacementConstraintTopology PlacementConstraintType = "topology"
)

// ResourceTransformationManager manages resource transformations
type ResourceTransformationManager struct {
	// transformations maps GVR to transformation rules
	transformations map[schema.GroupVersionResource][]TransformationRule

	// lock protects access to transformations
	lock sync.RWMutex
}

// PlacementManager coordinates resource placement decisions
type PlacementManager interface {
	// PlaceResource determines where to place a resource
	PlaceResource(ctx context.Context, resource *unstructured.Unstructured, policy *PlacementPolicy) ([]corelogicalcluster.Name, error)

	// UpdatePlacement updates placement for existing resource
	UpdatePlacement(ctx context.Context, resource *unstructured.Unstructured, clusters []corelogicalcluster.Name) error

	// GetPlacement returns current placement for a resource
	GetPlacement(ctx context.Context, resource *unstructured.Unstructured) ([]corelogicalcluster.Name, error)
}

// VirtualWorkspaceConfig configures a VirtualWorkspace
type VirtualWorkspaceConfig struct {
	// Logger provides structured logging
	Logger logr.Logger

	// KCPClient provides access to KCP APIs
	KCPClient kcpclientset.ClusterInterface

	// DynamicClient provides dynamic resource access
	DynamicClient interface{}

	// InformerFactory provides shared informers
	InformerFactory kcpinformers.SharedInformerFactory

	// Authorizer handles authorization
	Authorizer authorizer.Authorizer

	// PlacementManager coordinates placement decisions
	PlacementManager PlacementManager
}

// NewVirtualWorkspace creates a new VirtualWorkspace instance
func NewVirtualWorkspace(config VirtualWorkspaceConfig) (*VirtualWorkspace, error) {
	if config.Logger.GetSink() == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if config.KCPClient == nil {
		return nil, fmt.Errorf("KCP client is required")
	}
	if config.DynamicClient == nil {
		return nil, fmt.Errorf("dynamic client is required")
	}
	if config.InformerFactory == nil {
		return nil, fmt.Errorf("informer factory is required")
	}
	if config.Authorizer == nil {
		return nil, fmt.Errorf("authorizer is required")
	}
	if config.PlacementManager == nil {
		return nil, fmt.Errorf("placement manager is required")
	}

	vw := &VirtualWorkspace{
		logger:           config.Logger.WithName("virtual-workspace"),
		kcpClient:        config.KCPClient,
		dynamicClient:    config.DynamicClient,
		informerFactory:  config.InformerFactory,
		authz:            config.Authorizer,
		virtualResources: make(map[schema.GroupVersionResource]*VirtualResourceInfo),
		clusterResources: make(map[corelogicalcluster.Name]sets.Set[schema.GroupVersionResource]),
		syncerState:      make(map[corelogicalcluster.Name]*SyncerState),
		transformations:  NewResourceTransformationManager(),
		placementManager: config.PlacementManager,
	}

	// Initialize API binding informer - would use real implementation when available
	// vw.apiBindingInformer = config.InformerFactory.Apis().V1alpha1().APIBindings()
	vw.apiBindingInformer = nil

	return vw, nil
}

// NewResourceTransformationManager creates a new transformation manager
func NewResourceTransformationManager() *ResourceTransformationManager {
	return &ResourceTransformationManager{
		transformations: make(map[schema.GroupVersionResource][]TransformationRule),
	}
}

// AddTransformation adds a transformation rule
func (rtm *ResourceTransformationManager) AddTransformation(rule TransformationRule) {
	rtm.lock.Lock()
	defer rtm.lock.Unlock()

	rules := rtm.transformations[rule.SourceGVR]
	rules = append(rules, rule)
	rtm.transformations[rule.SourceGVR] = rules
}

// GetTransformations returns transformation rules for a GVR
func (rtm *ResourceTransformationManager) GetTransformations(gvr schema.GroupVersionResource) []TransformationRule {
	rtm.lock.RLock()
	defer rtm.lock.RUnlock()

	return rtm.transformations[gvr]
}

// TransformResource applies transformations to a resource
func (rtm *ResourceTransformationManager) TransformResource(ctx context.Context, resource *unstructured.Unstructured, rules []TransformationRule) (*unstructured.Unstructured, error) {
	if len(rules) == 0 {
		// Return a deep copy to avoid mutations
		return resource.DeepCopy(), nil
	}

	// Start with a deep copy of the resource
	transformed := resource.DeepCopy()

	// Apply each transformation rule
	for _, rule := range rules {
		if err := rtm.applyTransformationRule(ctx, transformed, rule); err != nil {
			return nil, fmt.Errorf("failed to apply transformation rule %s: %w", rule.Name, err)
		}
	}

	return transformed, nil
}

// applyTransformationRule applies a single transformation rule
func (rtm *ResourceTransformationManager) applyTransformationRule(ctx context.Context, resource *unstructured.Unstructured, rule TransformationRule) error {
	// Apply field mappings
	for _, mapping := range rule.FieldMappings {
		if err := rtm.applyFieldMapping(resource, mapping); err != nil {
			return fmt.Errorf("failed to apply field mapping %s->%s: %w", mapping.SourceField, mapping.TargetField, err)
		}
	}

	// Apply namespace mapping if specified
	if rule.NamespaceMapping != nil {
		if err := rtm.applyNamespaceMapping(resource, rule.NamespaceMapping); err != nil {
			return fmt.Errorf("failed to apply namespace mapping: %w", err)
		}
	}

	return nil
}

// applyFieldMapping applies a field mapping transformation
func (rtm *ResourceTransformationManager) applyFieldMapping(resource *unstructured.Unstructured, mapping FieldMapping) error {
	// Extract source value using JSONPath
	sourceValue, found, err := unstructured.NestedFieldNoCopy(resource.Object, strings.Split(mapping.SourceField, ".")...)
	if err != nil {
		return fmt.Errorf("failed to get source field %s: %w", mapping.SourceField, err)
	}
	if !found {
		// Source field doesn't exist, skip this mapping
		return nil
	}

	// Apply transformation
	transformedValue, err := rtm.applyTransformFunction(sourceValue, mapping.Transform)
	if err != nil {
		return fmt.Errorf("failed to apply transform %s: %w", mapping.Transform, err)
	}

	// Set target value using JSONPath
	if err := unstructured.SetNestedField(resource.Object, transformedValue, strings.Split(mapping.TargetField, ".")...); err != nil {
		return fmt.Errorf("failed to set target field %s: %w", mapping.TargetField, err)
	}

	return nil
}

// applyNamespaceMapping applies namespace mapping transformation
func (rtm *ResourceTransformationManager) applyNamespaceMapping(resource *unstructured.Unstructured, mapping *NamespaceMapping) error {
	currentNamespace := resource.GetNamespace()
	if currentNamespace == "" {
		// Not a namespaced resource
		return nil
	}

	var newNamespace string
	switch mapping.Strategy {
	case NamespaceMappingIdentity:
		newNamespace = currentNamespace
	case NamespaceMappingPrefix:
		newNamespace = mapping.TargetNamespace + "-" + currentNamespace
	case NamespaceMappingMap:
		// For mapping strategy, TargetNamespace should contain the mapping logic
		// This is a simplified implementation
		newNamespace = mapping.TargetNamespace
	default:
		return fmt.Errorf("unsupported namespace mapping strategy: %s", mapping.Strategy)
	}

	resource.SetNamespace(newNamespace)
	return nil
}

// applyTransformFunction applies a transformation function to a value
func (rtm *ResourceTransformationManager) applyTransformFunction(value interface{}, transform TransformFunction) (interface{}, error) {
	switch transform {
	case TransformIdentity:
		return value, nil
	case TransformPrefix:
		if str, ok := value.(string); ok {
			return "transformed-" + str, nil
		}
		return value, nil
	case TransformSuffix:
		if str, ok := value.(string); ok {
			return str + "-transformed", nil
		}
		return value, nil
	default:
		return value, fmt.Errorf("unsupported transform function: %s", transform)
	}
}

// Start starts the virtual workspace
func (vw *VirtualWorkspace) Start(ctx context.Context) error {
	vw.logger.Info("Starting virtual workspace")

	// Start informers
	vw.informerFactory.Start(ctx.Done())

	// Wait for informers to sync - disabled until real implementation is available
	// if !cache.WaitForCacheSync(ctx.Done(), vw.apiBindingInformer.Informer().HasSynced) {
	//	return fmt.Errorf("failed to wait for informers to sync")
	// }

	// Set up event handlers
	if err := vw.setupEventHandlers(ctx); err != nil {
		return fmt.Errorf("failed to setup event handlers: %w", err)
	}

	// Mark as ready
	vw.setReady(true)

	vw.logger.Info("Virtual workspace started successfully")
	return nil
}

// setupEventHandlers sets up informer event handlers
func (vw *VirtualWorkspace) setupEventHandlers(ctx context.Context) error {
	// Handle APIBinding events to track API availability - disabled until real implementation
	// _, err := vw.apiBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc:    vw.onAPIBindingAdd,
	//	UpdateFunc: vw.onAPIBindingUpdate,
	//	DeleteFunc: vw.onAPIBindingDelete,
	// })
	// if err != nil {
	//	return fmt.Errorf("failed to add APIBinding event handler: %w", err)
	// }

	return nil
}

// onAPIBindingAdd handles APIBinding add events
func (vw *VirtualWorkspace) onAPIBindingAdd(obj interface{}) {
	vw.logger.V(2).Info("APIBinding added", "obj", obj)
	// TODO: Update virtual resource availability based on APIBinding
}

// onAPIBindingUpdate handles APIBinding update events
func (vw *VirtualWorkspace) onAPIBindingUpdate(oldObj, newObj interface{}) {
	vw.logger.V(2).Info("APIBinding updated", "oldObj", oldObj, "newObj", newObj)
	// TODO: Update virtual resource availability based on APIBinding changes
}

// onAPIBindingDelete handles APIBinding delete events
func (vw *VirtualWorkspace) onAPIBindingDelete(obj interface{}) {
	vw.logger.V(2).Info("APIBinding deleted", "obj", obj)
	// TODO: Remove virtual resource availability based on APIBinding removal
}

// setReady sets the ready state of the virtual workspace
func (vw *VirtualWorkspace) setReady(ready bool) {
	vw.readyMutex.Lock()
	defer vw.readyMutex.Unlock()
	vw.ready = ready
}

// IsReady returns whether the virtual workspace is ready
func (vw *VirtualWorkspace) IsReady() error {
	vw.readyMutex.RLock()
	defer vw.readyMutex.RUnlock()
	if !vw.ready {
		return fmt.Errorf("virtual workspace is not ready")
	}
	return nil
}

// Authorize implements the authorizer.Authorizer interface
func (vw *VirtualWorkspace) Authorize(ctx context.Context, attr authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	return vw.authz.Authorize(ctx, attr)
}

// ResolveRootPath implements the RootPathResolver interface
func (vw *VirtualWorkspace) ResolveRootPath(urlPath string, ctx context.Context) (accepted bool, prefixToStrip string, completedContext context.Context) {
	// Parse the URL path to determine if it's a syncer virtual workspace request
	const syncerPrefix = "/services/syncer/"
	if !strings.HasPrefix(urlPath, syncerPrefix) {
		return false, "", ctx
	}

	// Extract syncer workspace information from the path
	remaining := strings.TrimPrefix(urlPath, syncerPrefix)
	parts := strings.Split(remaining, "/")
	
	if len(parts) < 3 || parts[0] != "clusters" {
		return false, "", ctx
	}

	clusterName := corelogicalcluster.Name(parts[1])
	
	// Validate that we have a syncer for this cluster
	vw.resourcesLock.RLock()
	syncerState, exists := vw.syncerState[clusterName]
	vw.resourcesLock.RUnlock()

	if !exists || !syncerState.Available {
		return false, "", ctx
	}

	// Add cluster information to context
	ctx = request.WithCluster(ctx, request.Cluster{Name: clusterName})

	return true, syncerPrefix + "clusters/" + string(clusterName), ctx
}

// Register implements the VirtualWorkspace interface
func (vw *VirtualWorkspace) Register(name string, rootAPIServerConfig genericapiserver.CompletedConfig, delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, error) {
	vw.logger.Info("Registering virtual workspace", "name", name)

	// This is a placeholder implementation - the actual registration would
	// integrate with KCP's virtual workspace framework
	vw.logger.Info("Virtual workspace registered", "name", name)
	
	// Return the delegate as-is for now
	return delegateAPIServer, nil
}


// RegisterSyncer registers a syncer for a cluster
func (vw *VirtualWorkspace) RegisterSyncer(ctx context.Context, clusterName corelogicalcluster.Name, supportedResources []schema.GroupVersionResource) error {
	vw.logger.Info("Registering syncer", "cluster", clusterName, "resources", len(supportedResources))

	vw.resourcesLock.Lock()
	defer vw.resourcesLock.Unlock()

	// Create syncer state
	syncerState := &SyncerState{
		ClusterName:        clusterName,
		Available:          true,
		LastHeartbeat:      time.Now(),
		SupportedResources: sets.New(supportedResources...),
		SyncStatus:         make(map[schema.GroupVersionResource]*ResourceSyncStatus),
	}

	// Initialize sync status for each resource
	for _, gvr := range supportedResources {
		syncerState.SyncStatus[gvr] = &ResourceSyncStatus{
			State:               SyncStatePending,
			LastSyncAttempt:     time.Time{},
			LastSuccessfulSync:  time.Time{},
			ErrorCount:          0,
			LastError:           "",
		}
	}

	vw.syncerState[clusterName] = syncerState
	vw.clusterResources[clusterName] = sets.New(supportedResources...)

	// Update virtual resources
	vw.updateVirtualResources(supportedResources, clusterName)

	vw.logger.Info("Syncer registered successfully", "cluster", clusterName)
	return nil
}

// UnregisterSyncer unregisters a syncer for a cluster
func (vw *VirtualWorkspace) UnregisterSyncer(ctx context.Context, clusterName corelogicalcluster.Name) error {
	vw.logger.Info("Unregistering syncer", "cluster", clusterName)

	vw.resourcesLock.Lock()
	defer vw.resourcesLock.Unlock()

	// Remove syncer state
	delete(vw.syncerState, clusterName)
	
	// Remove cluster resources
	resources := vw.clusterResources[clusterName]
	delete(vw.clusterResources, clusterName)

	// Update virtual resources to remove this cluster
	for gvr := range resources {
		if virtualRes := vw.virtualResources[gvr]; virtualRes != nil {
			virtualRes.Clusters.Delete(clusterName)
			if virtualRes.Clusters.Len() == 0 {
				delete(vw.virtualResources, gvr)
			}
		}
	}

	vw.logger.Info("Syncer unregistered successfully", "cluster", clusterName)
	return nil
}

// updateVirtualResources updates the virtual resource mappings
func (vw *VirtualWorkspace) updateVirtualResources(resources []schema.GroupVersionResource, clusterName corelogicalcluster.Name) {
	for _, gvr := range resources {
		virtualRes := vw.virtualResources[gvr]
		if virtualRes == nil {
			virtualRes = &VirtualResourceInfo{
				GroupVersionResource: gvr,
				Namespaced:           true, // Default to namespaced, would be determined from schema
				Clusters:             sets.New[corelogicalcluster.Name](),
				TransformationRules:  []TransformationRule{},
				PlacementPolicy:      nil,
				LastSyncTime:         time.Now(),
			}
			vw.virtualResources[gvr] = virtualRes
		}
		virtualRes.Clusters.Insert(clusterName)
	}
}

// UpdateSyncStatus updates the sync status for a resource
func (vw *VirtualWorkspace) UpdateSyncStatus(ctx context.Context, clusterName corelogicalcluster.Name, gvr schema.GroupVersionResource, state SyncState, err error) {
	vw.resourcesLock.Lock()
	defer vw.resourcesLock.Unlock()

	syncerState := vw.syncerState[clusterName]
	if syncerState == nil {
		return
	}

	syncStatus := syncerState.SyncStatus[gvr]
	if syncStatus == nil {
		return
	}

	syncStatus.State = state
	syncStatus.LastSyncAttempt = time.Now()

	if err != nil {
		syncStatus.ErrorCount++
		syncStatus.LastError = err.Error()
	} else {
		syncStatus.ErrorCount = 0
		syncStatus.LastError = ""
		if state == SyncStateSynced {
			syncStatus.LastSuccessfulSync = time.Now()
		}
	}

	// Update virtual resource sync time
	if virtualRes := vw.virtualResources[gvr]; virtualRes != nil {
		virtualRes.LastSyncTime = time.Now()
	}
}

// GetVirtualResources returns the current set of virtual resources
func (vw *VirtualWorkspace) GetVirtualResources() map[schema.GroupVersionResource]*VirtualResourceInfo {
	vw.resourcesLock.RLock()
	defer vw.resourcesLock.RUnlock()

	result := make(map[schema.GroupVersionResource]*VirtualResourceInfo)
	for gvr, info := range vw.virtualResources {
		// Deep copy the info
		result[gvr] = &VirtualResourceInfo{
			GroupVersionResource: info.GroupVersionResource,
			Namespaced:           info.Namespaced,
			Clusters:             info.Clusters.Clone(),
			TransformationRules:  append([]TransformationRule{}, info.TransformationRules...),
			PlacementPolicy:      info.PlacementPolicy, // Note: shallow copy
			LastSyncTime:         info.LastSyncTime,
		}
	}

	return result
}

// GetSyncerState returns the state of all syncers
func (vw *VirtualWorkspace) GetSyncerState() map[corelogicalcluster.Name]*SyncerState {
	vw.resourcesLock.RLock()
	defer vw.resourcesLock.RUnlock()

	result := make(map[corelogicalcluster.Name]*SyncerState)
	for cluster, state := range vw.syncerState {
		// Create a deep copy
		stateCopy := &SyncerState{
			ClusterName:        state.ClusterName,
			Available:          state.Available,
			LastHeartbeat:      state.LastHeartbeat,
			SupportedResources: state.SupportedResources.Clone(),
			SyncStatus:         make(map[schema.GroupVersionResource]*ResourceSyncStatus),
		}
		
		for gvr, status := range state.SyncStatus {
			stateCopy.SyncStatus[gvr] = &ResourceSyncStatus{
				State:              status.State,
				LastSyncAttempt:    status.LastSyncAttempt,
				LastSuccessfulSync: status.LastSuccessfulSync,
				ErrorCount:         status.ErrorCount,
				LastError:          status.LastError,
			}
		}
		
		result[cluster] = stateCopy
	}

	return result
}