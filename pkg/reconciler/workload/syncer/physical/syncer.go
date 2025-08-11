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

package physical

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer"
)

// PhysicalSyncer implements syncer.WorkloadSyncer for physical Kubernetes clusters
// with workspace isolation and logical cluster awareness.
type PhysicalSyncer struct {
	// Cluster configuration
	cluster       *syncer.ClusterRegistration
	clusterConfig *rest.Config
	
	// Clients
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	
	// State
	healthy bool
	mu      sync.RWMutex
	
	// Configuration
	options *SyncerOptions
	
	// Workspace-aware naming for resource isolation
	naming *syncer.WorkspaceAwareNaming
}

// SyncerOptions contains configuration for the physical syncer
type SyncerOptions struct {
	// ResyncPeriod defines how often to refresh status
	ResyncPeriod time.Duration
	
	// RetryStrategy for failed operations
	RetryStrategy *syncer.RetryStrategy
	
	// EventHandler for sync events
	EventHandler syncer.SyncEventHandler
	
	// Timeout for sync operations
	SyncTimeout time.Duration
	
	// LogicalCluster context for workspace isolation
	LogicalCluster logicalcluster.Name
}

// NewPhysicalSyncer creates a new physical cluster syncer
func NewPhysicalSyncer(
	cluster *syncer.ClusterRegistration,
	clusterConfig *rest.Config,
	options *SyncerOptions,
) (*PhysicalSyncer, error) {
	
	if cluster == nil {
		return nil, fmt.Errorf("cluster registration cannot be nil")
	}
	if clusterConfig == nil {
		return nil, fmt.Errorf("cluster config cannot be nil")
	}
	if options == nil {
		options = DefaultSyncerOptions()
	}
	
	// Create dynamic client for resource operations
	dynamicClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client for cluster %s: %w", cluster.Name, err)
	}
	
	// Create Kubernetes client for core operations
	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client for cluster %s: %w", cluster.Name, err)
	}
	
	syncer := &PhysicalSyncer{
		cluster:       cluster,
		clusterConfig: clusterConfig,
		dynamicClient: dynamicClient,
		kubeClient:    kubeClient,
		healthy:       false,
		options:       options,
		naming:        syncer.NewWorkspaceAwareNaming(options.LogicalCluster),
	}
	
	return syncer, nil
}

// SyncWorkload implements WorkloadSyncer.SyncWorkload
func (s *PhysicalSyncer) SyncWorkload(ctx context.Context,
	cluster *syncer.ClusterRegistration,
	workload runtime.Object,
) error {
	
	logger := klog.FromContext(ctx).WithValues(
		"cluster", cluster.Name,
		"syncer", "physical",
	)
	
	// Validate cluster matches
	if cluster.Name != s.cluster.Name {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.Name, cluster.Name)
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return fmt.Errorf("failed to convert workload to unstructured for cluster %s in workspace %s: %w", cluster.Name, s.options.LogicalCluster, err)
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	
	// Extract resource information with workspace context
	gvk := obj.GetObjectKind().GroupVersionKind()
	workloadRef := syncer.WorkloadRef{
		GVK:                    gvk,
		Namespace:              obj.GetNamespace(),
		Name:                   obj.GetName(),
		LogicalCluster:         s.options.LogicalCluster,
		WorkspaceQualifiedName: s.naming.QualifyName(obj.GetName()),
	}
	
	// Emit sync started event
	if s.options.EventHandler != nil {
		event := &syncer.SyncEvent{
			Type:      syncer.SyncEventStarted,
			Cluster:   cluster.Name,
			Workload:  workloadRef,
			Timestamp: time.Now(),
			Message:   "Starting workload sync to physical cluster",
		}
		if err := s.options.EventHandler.HandleEvent(ctx, event); err != nil {
			logger.Error(err, "Failed to handle sync event")
		}
	}
	
	// Prepare workload for cluster deployment
	clusterWorkload, err := s.prepareWorkloadForCluster(obj)
	if err != nil {
		s.emitSyncFailedEvent(ctx, cluster.Name, workloadRef, err)
		return fmt.Errorf("failed to prepare workload %s/%s for cluster %s in workspace %s: %w", obj.GetNamespace(), obj.GetName(), cluster.Name, s.options.LogicalCluster, err)
	}
	
	// Apply workload to cluster with retry
	operation := func() error {
		return s.applyWorkloadToCluster(ctx, clusterWorkload)
	}
	
	if err := syncer.ExecuteWithRetry(operation, s.options.RetryStrategy); err != nil {
		s.emitSyncFailedEvent(ctx, cluster.Name, workloadRef, err)
		return fmt.Errorf("failed to apply workload %s/%s to cluster %s in workspace %s: %w", obj.GetNamespace(), obj.GetName(), cluster.Name, s.options.LogicalCluster, err)
	}
	
	// Emit sync completed event
	if s.options.EventHandler != nil {
		event := &syncer.SyncEvent{
			Type:      syncer.SyncEventCompleted,
			Cluster:   cluster.Name,
			Workload:  workloadRef,
			Timestamp: time.Now(),
			Message:   "Successfully synced workload to physical cluster",
		}
		if err := s.options.EventHandler.HandleEvent(ctx, event); err != nil {
			logger.Error(err, "Failed to handle sync event")
		}
	}
	
	logger.Info("Successfully synced workload to physical cluster", 
		"resource", workloadRef.Name,
		"namespace", workloadRef.Namespace)
	
	return nil
}

// GetStatus implements WorkloadSyncer.GetStatus
func (s *PhysicalSyncer) GetStatus(ctx context.Context,
	cluster *syncer.ClusterRegistration,
	workload runtime.Object,
) (*syncer.WorkloadStatus, error) {
	
	// Validate cluster matches
	if cluster.Name != s.cluster.Name {
		return nil, fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.Name, cluster.Name)
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workload to unstructured for cluster %s in workspace %s: %w", cluster.Name, s.options.LogicalCluster, err)
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	gvk := obj.GetObjectKind().GroupVersionKind()
	
	// Get current resource from cluster
	gvr, err := s.gvkToGVR(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to convert GVK %s to GVR for resource %s/%s in workspace %s: %w", gvk, obj.GetNamespace(), obj.GetName(), s.options.LogicalCluster, err)
	}
	
	var resourceInterface dynamic.ResourceInterface = s.dynamicClient.Resource(gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = s.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
	}
	
	// Use workspace-qualified name to ensure isolation
	workspaceQualifiedName := s.naming.QualifyName(obj.GetName())
	clusterResource, err := resourceInterface.Get(ctx, workspaceQualifiedName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return &syncer.WorkloadStatus{
				Ready:          false,
				Phase:          syncer.WorkloadPhaseTerminating,
				LastUpdated:    time.Now(),
				ClusterName:    cluster.Name,
				LogicalCluster: s.options.LogicalCluster,
				Conditions: []syncer.WorkloadCondition{
					{
						Type:               "Ready",
						Status:             syncer.ConditionFalse,
						LastTransitionTime: time.Now(),
						Reason:             "NotFound",
						Message:            "Workload not found in cluster",
					},
				},
			}, nil
		}
		
		return nil, fmt.Errorf("failed to get workload %s/%s from cluster %s in workspace %s: %w", obj.GetNamespace(), obj.GetName(), cluster.Name, s.options.LogicalCluster, err)
	}
	
	// Extract status from cluster resource
	status := s.extractWorkloadStatus(clusterResource, cluster.Name)
	
	return status, nil
}

// DeleteWorkload implements WorkloadSyncer.DeleteWorkload
func (s *PhysicalSyncer) DeleteWorkload(ctx context.Context,
	cluster *syncer.ClusterRegistration,
	workload runtime.Object,
) error {
	
	// Validate cluster matches
	if cluster.Name != s.cluster.Name {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.Name, cluster.Name)
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return fmt.Errorf("failed to convert workload to unstructured for deletion in cluster %s workspace %s: %w", cluster.Name, s.options.LogicalCluster, err)
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	gvk := obj.GetObjectKind().GroupVersionKind()
	
	// Get GVR for resource
	gvr, err := s.gvkToGVR(gvk)
	if err != nil {
		return fmt.Errorf("failed to convert GVK %s to GVR for resource %s/%s deletion in workspace %s: %w", gvk, obj.GetNamespace(), obj.GetName(), s.options.LogicalCluster, err)
	}
	
	var resourceInterface dynamic.ResourceInterface = s.dynamicClient.Resource(gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = s.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
	}
	
	// Use workspace-qualified name to ensure isolation
	workspaceQualifiedName := s.naming.QualifyName(obj.GetName())
	
	// Verify the resource exists and belongs to our workspace before deletion
	existingResource, getErr := resourceInterface.Get(ctx, workspaceQualifiedName, metav1.GetOptions{})
	if getErr != nil && !errors.IsNotFound(getErr) {
		return fmt.Errorf("failed to verify resource ownership for %s/%s before deletion in cluster %s workspace %s: %w", obj.GetNamespace(), obj.GetName(), cluster.Name, s.options.LogicalCluster, getErr)
	}
	
	if !errors.IsNotFound(getErr) && !s.isOwnedByWorkspace(existingResource) {
		return fmt.Errorf("cannot delete resource %s: not owned by workspace %s", workspaceQualifiedName, s.options.LogicalCluster)
	}
	
	// Delete resource from cluster
	deletePolicy := metav1.DeletePropagationForeground
	err = resourceInterface.Delete(ctx, workspaceQualifiedName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to verify resource ownership for %s/%s before deletion in cluster %s workspace %s: %w", obj.GetNamespace(), obj.GetName(), cluster.Name, s.options.LogicalCluster, getErr)
	}
	
	klog.FromContext(ctx).Info("Successfully deleted workload from physical cluster",
		"cluster", cluster.Name,
		"resource", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// HealthCheck implements WorkloadSyncer.HealthCheck
func (s *PhysicalSyncer) HealthCheck(ctx context.Context,
	cluster *syncer.ClusterRegistration,
) error {
	
	// Validate cluster matches
	if cluster.Name != s.cluster.Name {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.Name, cluster.Name)
	}
	
	// Perform basic connectivity test
	_, err := s.kubeClient.Discovery().ServerVersion()
	if err != nil {
		s.setHealthy(false)
		return fmt.Errorf("failed to connect to cluster %s in workspace %s: %w", cluster.Name, s.options.LogicalCluster, err)
	}
	
	s.setHealthy(true)
	return nil
}

// Helper methods

func (s *PhysicalSyncer) prepareWorkloadForCluster(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Create a deep copy to avoid modifying the original
	clusterObj := obj.DeepCopy()
	
	// Apply workspace-aware naming to ensure isolation
	workspaceQualifiedName := s.naming.QualifyName(clusterObj.GetName())
	clusterObj.SetName(workspaceQualifiedName)
	
	// Add workspace context to annotations for traceability
	annotations := clusterObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["syncer.kcp.io/logical-cluster"] = string(s.options.LogicalCluster)
	annotations["syncer.kcp.io/original-name"] = obj.GetName()
	
	// Remove KCP-specific annotations and labels that shouldn't be in physical cluster
	delete(annotations, "kcp.io/cluster")
	delete(annotations, "kcp.io/placement")
	clusterObj.SetAnnotations(annotations)
	
	labels := clusterObj.GetLabels()
	if labels != nil {
		delete(labels, "kcp.io/workspace")
		// Add workspace isolation label
		labels["syncer.kcp.io/workspace"] = sanitizeForKubernetes(string(s.options.LogicalCluster))
		clusterObj.SetLabels(labels)
	} else {
		labels = map[string]string{
			"syncer.kcp.io/workspace": sanitizeForKubernetes(string(s.options.LogicalCluster)),
		}
		clusterObj.SetLabels(labels)
	}
	
	// Clear resource version and UID for cluster creation
	clusterObj.SetResourceVersion("")
	clusterObj.SetUID("")
	clusterObj.SetSelfLink("")
	
	// Remove managed fields
	clusterObj.SetManagedFields(nil)
	
	return clusterObj, nil
}

func (s *PhysicalSyncer) applyWorkloadToCluster(ctx context.Context, obj *unstructured.Unstructured) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	gvr, err := s.gvkToGVR(gvk)
	if err != nil {
		return fmt.Errorf("failed to convert GVK to GVR: %w", err)
	}
	
	var resourceInterface dynamic.ResourceInterface = s.dynamicClient.Resource(gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = s.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
	}
	
	// Validate that this resource belongs to our workspace
	if !s.naming.IsWorkspaceResource(obj.GetName()) {
		return fmt.Errorf("resource %s does not belong to workspace %s", obj.GetName(), s.options.LogicalCluster)
	}
	
	// Try to create, if it exists, update
	_, err = resourceInterface.Create(ctx, obj, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		// Get the current resource to preserve resource version
		current, getErr := resourceInterface.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get current resource for update: %w", getErr)
		}
		
		// Verify this resource belongs to our workspace
		if !s.isOwnedByWorkspace(current) {
			return fmt.Errorf("resource %s is owned by different workspace", obj.GetName())
		}
		
		// Preserve resource version for update
		obj.SetResourceVersion(current.GetResourceVersion())
		
		_, err = resourceInterface.Update(ctx, obj, metav1.UpdateOptions{})
	}
	
	return err
}

func (s *PhysicalSyncer) extractWorkloadStatus(obj *unstructured.Unstructured, clusterName string) *syncer.WorkloadStatus {
	status := &syncer.WorkloadStatus{
		LastUpdated:    time.Now(),
		ClusterName:    clusterName,
		LogicalCluster: s.options.LogicalCluster,
		Resources:      []syncer.ResourceStatus{},
		Conditions:     []syncer.WorkloadCondition{},
	}
	
	// Extract status from the resource
	statusObj, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		status.Ready = false
		status.Phase = syncer.WorkloadPhaseUnknown
		return status
	}
	
	// Check for ready condition
	conditions, found, _ := unstructured.NestedSlice(statusObj, "conditions")
	if found {
		for _, condition := range conditions {
			if conditionMap, ok := condition.(map[string]interface{}); ok {
				conditionType, _ := conditionMap["type"].(string)
				conditionStatus, _ := conditionMap["status"].(string)
				
				if conditionType == "Ready" {
					status.Ready = conditionStatus == "True"
				}
				
				// Add to conditions list
				wc := syncer.WorkloadCondition{
					Type:   conditionType,
					Status: syncer.ConditionStatus(conditionStatus),
				}
				
				if reason, ok := conditionMap["reason"].(string); ok {
					wc.Reason = reason
				}
				if message, ok := conditionMap["message"].(string); ok {
					wc.Message = message
				}
				
				status.Conditions = append(status.Conditions, wc)
			}
		}
	}
	
	// Determine phase based on ready status
	if status.Ready {
		status.Phase = syncer.WorkloadPhaseReady
	} else {
		status.Phase = syncer.WorkloadPhasePending
	}
	
	// Add resource status with workspace context
	gvk := obj.GetObjectKind().GroupVersionKind()
	resourceStatus := syncer.ResourceStatus{
		GVK:             gvk,
		Namespace:       obj.GetNamespace(),
		Name:            s.naming.ExtractOriginalName(obj.GetName()),
		Ready:           status.Ready,
		Phase:           string(status.Phase),
		WorkspacePrefix: sanitizeForKubernetes(string(s.options.LogicalCluster)),
	}
	status.Resources = append(status.Resources, resourceStatus)
	
	return status
}

func (s *PhysicalSyncer) gvkToGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Simple conversion - in a real implementation, this would use discovery
	// For now, implement common mappings
	mapping := map[schema.GroupVersionKind]schema.GroupVersionResource{
		{Group: "", Version: "v1", Kind: "Pod"}:                                {Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Kind: "Service"}:                            {Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Kind: "ConfigMap"}:                          {Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Kind: "Secret"}:                             {Group: "", Version: "v1", Resource: "secrets"},
		{Group: "apps", Version: "v1", Kind: "Deployment"}:                     {Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet"}:                     {Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"}:                      {Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"}:                    {Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	
	gvr, found := mapping[gvk]
	if !found {
		return schema.GroupVersionResource{}, fmt.Errorf("no GVR mapping found for GVK %s", gvk)
	}
	
	return gvr, nil
}

func (s *PhysicalSyncer) setHealthy(healthy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthy = healthy
}

func (s *PhysicalSyncer) emitSyncFailedEvent(ctx context.Context, clusterName string, workloadRef syncer.WorkloadRef, err error) {
	if s.options.EventHandler != nil {
		event := &syncer.SyncEvent{
			Type:           syncer.SyncEventFailed,
			Cluster:        clusterName,
			Workload:       workloadRef,
			Timestamp:      time.Now(),
			Message:        "Failed to sync workload to physical cluster",
			Error:          err,
			LogicalCluster: s.options.LogicalCluster,
		}
		if handleErr := s.options.EventHandler.HandleEvent(ctx, event); handleErr != nil {
			klog.FromContext(ctx).Error(handleErr, "Failed to handle sync failed event")
		}
	}
}

// DefaultSyncerOptions returns default options for physical syncer
func DefaultSyncerOptions() *SyncerOptions {
	return &SyncerOptions{
		ResyncPeriod:  30 * time.Second,
		RetryStrategy: syncer.DefaultRetryStrategy(),
		SyncTimeout:   5 * time.Minute,
	}
}

// Helper functions for workspace isolation

// isOwnedByWorkspace checks if a resource belongs to this syncer's workspace
func (s *PhysicalSyncer) isOwnedByWorkspace(obj *unstructured.Unstructured) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	
	workspaceLabel, exists := labels["syncer.kcp.io/workspace"]
	if !exists {
		return false
	}
	
	expectedWorkspace := sanitizeForKubernetes(string(s.options.LogicalCluster))
	return workspaceLabel == expectedWorkspace
}

// sanitizeForKubernetes ensures a string is valid for use in Kubernetes resource names
func sanitizeForKubernetes(s string) string {
	// Replace invalid characters with hyphens
	result := ""
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		} else if r >= 'A' && r <= 'Z' {
			result += string(r - 'A' + 'a') // Convert to lowercase
		} else if r == ':' || r == '/' {
			result += "-"
		}
	}
	return result
}