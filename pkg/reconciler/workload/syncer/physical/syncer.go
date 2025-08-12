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
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// PhysicalSyncer implements WorkloadSyncer for physical Kubernetes clusters
type PhysicalSyncer struct {
	// Cluster configuration
	cluster       tmc.ClusterTarget
	clusterConfig *rest.Config
	
	// Clients
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	
	// State
	healthy bool
	mu      sync.RWMutex
	
	// Configuration
	options *SyncerOptions
}

// SyncerOptions contains configuration for the physical syncer
type SyncerOptions struct {
	// ResyncPeriod defines how often to refresh status
	ResyncPeriod time.Duration
	
	// RetryStrategy for failed operations
	RetryStrategy *tmc.RetryStrategy
	
	// EventHandler for sync events
	EventHandler tmc.SyncEventHandler
	
	// Timeout for sync operations
	SyncTimeout time.Duration
	
	// LogicalCluster context
	LogicalCluster logicalcluster.Name
}

// NewPhysicalSyncer creates a new physical cluster syncer
func NewPhysicalSyncer(
	cluster tmc.ClusterTarget,
	clusterConfig *rest.Config,
	options *SyncerOptions,
) (*PhysicalSyncer, error) {
	
	if cluster == nil {
		return nil, fmt.Errorf("cluster target cannot be nil")
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
		return nil, fmt.Errorf("failed to create dynamic client for cluster %s: %w", cluster.GetName(), err)
	}
	
	// Create Kubernetes client for core operations
	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client for cluster %s: %w", cluster.GetName(), err)
	}
	
	syncer := &PhysicalSyncer{
		cluster:       cluster,
		clusterConfig: clusterConfig,
		dynamicClient: dynamicClient,
		kubeClient:    kubeClient,
		healthy:       false,
		options:       options,
	}
	
	return syncer, nil
}

// SyncWorkload implements WorkloadSyncer.SyncWorkload
func (s *PhysicalSyncer) SyncWorkload(ctx context.Context,
	cluster tmc.ClusterTarget,
	workload runtime.Object,
) error {
	
	logger := klog.FromContext(ctx).WithValues(
		"cluster", cluster.GetName(),
		"syncer", "physical",
	)
	
	// Validate cluster matches
	if cluster.GetName() != s.cluster.GetName() {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.GetName(), cluster.GetName())
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "sync").
			WithMessage("Failed to convert workload to unstructured").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			Build()
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	
	// Extract resource information
	gvk := obj.GetObjectKind().GroupVersionKind()
	workloadRef := tmc.WorkloadRef{
		GVK:       gvk,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
	
	// Emit sync started event
	if s.options.EventHandler != nil {
		event := &tmc.SyncEvent{
			Type:      tmc.SyncEventStarted,
			Cluster:   cluster.GetName(),
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
		s.emitSyncFailedEvent(ctx, cluster.GetName(), workloadRef, err)
		return tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "sync").
			WithMessage("Failed to prepare workload for cluster").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	// Apply workload to cluster with retry
	operation := func() error {
		return s.applyWorkloadToCluster(ctx, clusterWorkload)
	}
	
	if err := tmc.ExecuteWithRetry(operation, s.options.RetryStrategy); err != nil {
		s.emitSyncFailedEvent(ctx, cluster.GetName(), workloadRef, err)
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "physical-syncer", "sync").
			WithMessage("Failed to apply workload to cluster").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	// Emit sync completed event
	if s.options.EventHandler != nil {
		event := &tmc.SyncEvent{
			Type:      tmc.SyncEventCompleted,
			Cluster:   cluster.GetName(),
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
	cluster tmc.ClusterTarget,
	workload runtime.Object,
) (*tmc.WorkloadStatus, error) {
	
	// Validate cluster matches
	if cluster.GetName() != s.cluster.GetName() {
		return nil, fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.GetName(), cluster.GetName())
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "get-status").
			WithMessage("Failed to convert workload to unstructured").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			Build()
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	gvk := obj.GetObjectKind().GroupVersionKind()
	
	// Get current resource from cluster
	gvr, err := s.gvkToGVR(gvk)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "get-status").
			WithMessage("Failed to convert GVK to GVR").
			WithCause(err).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	var resourceInterface dynamic.ResourceInterface = s.dynamicClient.Resource(gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = s.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
	}
	
	clusterResource, err := resourceInterface.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return &tmc.WorkloadStatus{
				Ready:       false,
				Phase:       tmc.WorkloadPhaseTerminating,
				LastUpdated: time.Now(),
				ClusterName: cluster.GetName(),
				Conditions: []tmc.WorkloadCondition{
					{
						Type:               "Ready",
						Status:             tmc.ConditionFalse,
						LastTransitionTime: time.Now(),
						Reason:             "NotFound",
						Message:            "Workload not found in cluster",
					},
				},
			}, nil
		}
		
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "physical-syncer", "get-status").
			WithMessage("Failed to get workload from cluster").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	// Extract status from cluster resource
	status := s.extractWorkloadStatus(clusterResource, cluster.GetName())
	
	return status, nil
}

// DeleteWorkload implements WorkloadSyncer.DeleteWorkload
func (s *PhysicalSyncer) DeleteWorkload(ctx context.Context,
	cluster tmc.ClusterTarget,
	workload runtime.Object,
) error {
	
	// Validate cluster matches
	if cluster.GetName() != s.cluster.GetName() {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.GetName(), cluster.GetName())
	}
	
	// Convert to unstructured for dynamic operations
	unstructuredWorkload, err := runtime.DefaultUnstructuredConverter.ToUnstructured(workload)
	if err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "delete").
			WithMessage("Failed to convert workload to unstructured").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			Build()
	}
	
	obj := &unstructured.Unstructured{Object: unstructuredWorkload}
	gvk := obj.GetObjectKind().GroupVersionKind()
	
	// Get GVR for resource
	gvr, err := s.gvkToGVR(gvk)
	if err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeResourceValidation, "physical-syncer", "delete").
			WithMessage("Failed to convert GVK to GVR").
			WithCause(err).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	var resourceInterface dynamic.ResourceInterface = s.dynamicClient.Resource(gvr)
	if obj.GetNamespace() != "" {
		resourceInterface = s.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace())
	}
	
	// Delete resource from cluster
	deletePolicy := metav1.DeletePropagationForeground
	err = resourceInterface.Delete(ctx, obj.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	
	if err != nil && !errors.IsNotFound(err) {
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "physical-syncer", "delete").
			WithMessage("Failed to delete workload from cluster").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			WithResource(gvk, obj.GetNamespace(), obj.GetName()).
			Build()
	}
	
	klog.FromContext(ctx).Info("Successfully deleted workload from physical cluster",
		"cluster", cluster.GetName(),
		"resource", obj.GetName(),
		"namespace", obj.GetNamespace())
	
	return nil
}

// HealthCheck implements WorkloadSyncer.HealthCheck
func (s *PhysicalSyncer) HealthCheck(ctx context.Context,
	cluster tmc.ClusterTarget,
) error {
	
	// Validate cluster matches
	if cluster.GetName() != s.cluster.GetName() {
		return fmt.Errorf("cluster name mismatch: expected %s, got %s", s.cluster.GetName(), cluster.GetName())
	}
	
	// Perform basic connectivity test
	_, err := s.kubeClient.Discovery().ServerVersion()
	if err != nil {
		s.setHealthy(false)
		return tmc.NewTMCError(tmc.TMCErrorTypeClusterUnreachable, "physical-syncer", "health-check").
			WithMessage("Failed to connect to cluster").
			WithCause(err).
			WithCluster(cluster.GetName(), string(s.options.LogicalCluster)).
			Build()
	}
	
	s.setHealthy(true)
	return nil
}

// Helper methods

func (s *PhysicalSyncer) prepareWorkloadForCluster(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// Create a deep copy to avoid modifying the original
	clusterObj := obj.DeepCopy()
	
	// Remove KCP-specific annotations and labels
	annotations := clusterObj.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kcp.io/cluster")
		delete(annotations, "kcp.io/placement")
		clusterObj.SetAnnotations(annotations)
	}
	
	labels := clusterObj.GetLabels()
	if labels != nil {
		delete(labels, "kcp.io/workspace")
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
	
	// Try to create, if it exists, update
	_, err = resourceInterface.Create(ctx, obj, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		// Get the current resource to preserve resource version
		current, getErr := resourceInterface.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get current resource for update: %w", getErr)
		}
		
		// Preserve resource version for update
		obj.SetResourceVersion(current.GetResourceVersion())
		
		_, err = resourceInterface.Update(ctx, obj, metav1.UpdateOptions{})
	}
	
	return err
}

func (s *PhysicalSyncer) extractWorkloadStatus(obj *unstructured.Unstructured, clusterName string) *tmc.WorkloadStatus {
	status := &tmc.WorkloadStatus{
		LastUpdated: time.Now(),
		ClusterName: clusterName,
		Resources:   []tmc.ResourceStatus{},
		Conditions:  []tmc.WorkloadCondition{},
	}
	
	// Extract status from the resource
	statusObj, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		status.Ready = false
		status.Phase = tmc.WorkloadPhaseUnknown
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
				wc := tmc.WorkloadCondition{
					Type:   conditionType,
					Status: tmc.ConditionStatus(conditionStatus),
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
		status.Phase = tmc.WorkloadPhaseReady
	} else {
		status.Phase = tmc.WorkloadPhasePending
	}
	
	// Add resource status
	gvk := obj.GetObjectKind().GroupVersionKind()
	resourceStatus := tmc.ResourceStatus{
		GVK:       gvk,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Ready:     status.Ready,
		Phase:     string(status.Phase),
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

func (s *PhysicalSyncer) emitSyncFailedEvent(ctx context.Context, clusterName string, workloadRef tmc.WorkloadRef, err error) {
	if s.options.EventHandler != nil {
		event := &tmc.SyncEvent{
			Type:      tmc.SyncEventFailed,
			Cluster:   clusterName,
			Workload:  workloadRef,
			Timestamp: time.Now(),
			Message:   "Failed to sync workload to physical cluster",
			Error:     err,
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
		RetryStrategy: tmc.DefaultRetryStrategy(),
		SyncTimeout:   5 * time.Minute,
	}
}

