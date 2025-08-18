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

package rollback

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SnapshotManager handles creation, storage, and retrieval of deployment snapshots.
type SnapshotManager struct {
	dynamicClient dynamic.Interface
	cluster       logicalcluster.Name
	config        *EngineConfig
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(client dynamic.Interface, cluster logicalcluster.Name, config *EngineConfig) *SnapshotManager {
	return &SnapshotManager{
		dynamicClient: client,
		cluster:       cluster,
		config:        config,
	}
}

// CreateSnapshot captures the current state of a deployment and related resources.
func (sm *SnapshotManager) CreateSnapshot(ctx context.Context, deploymentRef corev1.ObjectReference, version string) (*DeploymentSnapshot, error) {
	klog.V(2).InfoS("Creating deployment snapshot", "deployment", deploymentRef.Name, "version", version)

	// Generate unique snapshot ID
	snapshotID := sm.generateSnapshotID(deploymentRef, version)

	// Collect deployment resources
	resources, err := sm.collectDeploymentResources(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to collect deployment resources: %w", err)
	}

	// Extract configuration
	config := sm.extractConfiguration(resources)

	// Collect traffic configuration
	trafficConfig, err := sm.collectTrafficConfiguration(ctx, deploymentRef)
	if err != nil {
		klog.V(2).InfoS("Failed to collect traffic config, continuing without it", "error", err)
		trafficConfig = nil
	}

	// Create snapshot
	snapshot := &DeploymentSnapshot{
		ID:            snapshotID,
		Version:       version,
		CreatedAt:     metav1.NewTime(time.Now()),
		DeploymentRef: deploymentRef,
		Resources:     resources,
		Configuration: config,
		TrafficConfig: trafficConfig,
		ConfigHash:    sm.calculateConfigHash(config),
		Labels: map[string]string{
			"deployment.kcp.io/name":    deploymentRef.Name,
			"deployment.kcp.io/version": version,
		},
		Annotations: map[string]string{
			"snapshot.kcp.io/created-by": "rollback-engine",
		},
	}

	// Store snapshot
	if err := sm.storeSnapshot(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("failed to store snapshot: %w", err)
	}

	klog.V(2).InfoS("Successfully created deployment snapshot", "snapshotID", snapshotID)
	return snapshot, nil
}

// GetSnapshot retrieves a snapshot by ID.
func (sm *SnapshotManager) GetSnapshot(ctx context.Context, snapshotID string) (*DeploymentSnapshot, error) {
	// Implementation would retrieve from storage (ConfigMap, etcd, etc.)
	// For now, this is a placeholder that would be implemented based on storage backend
	klog.V(2).InfoS("Retrieving snapshot", "snapshotID", snapshotID)
	
	// This would typically interact with a storage backend
	snapshot, err := sm.retrieveSnapshotFromStorage(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve snapshot %s: %w", snapshotID, err)
	}

	return snapshot, nil
}

// ListSnapshots returns snapshots for a deployment, sorted by creation time.
func (sm *SnapshotManager) ListSnapshots(ctx context.Context, deploymentRef corev1.ObjectReference) ([]*DeploymentSnapshot, error) {
	klog.V(2).InfoS("Listing snapshots for deployment", "deployment", deploymentRef.Name)

	snapshots, err := sm.listSnapshotsFromStorage(ctx, deploymentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	// Sort by creation time (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Time.After(snapshots[j].CreatedAt.Time)
	})

	return snapshots, nil
}

// DeleteSnapshot removes a snapshot from storage.
func (sm *SnapshotManager) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	klog.V(2).InfoS("Deleting snapshot", "snapshotID", snapshotID)

	if err := sm.deleteSnapshotFromStorage(ctx, snapshotID); err != nil {
		return fmt.Errorf("failed to delete snapshot %s: %w", snapshotID, err)
	}

	return nil
}

// ValidateSnapshot ensures a snapshot is valid and complete.
func (sm *SnapshotManager) ValidateSnapshot(ctx context.Context, snapshot *DeploymentSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if snapshot.ID == "" {
		return fmt.Errorf("snapshot ID is empty")
	}

	if len(snapshot.Resources) == 0 {
		return fmt.Errorf("snapshot contains no resources")
	}

	// Validate config hash
	expectedHash := sm.calculateConfigHash(snapshot.Configuration)
	if snapshot.ConfigHash != expectedHash {
		return fmt.Errorf("snapshot config hash mismatch: expected %s, got %s", expectedHash, snapshot.ConfigHash)
	}

	// Validate resource integrity
	for i, resource := range snapshot.Resources {
		if resource.Raw == nil {
			return fmt.Errorf("resource %d has nil raw data", i)
		}
	}

	klog.V(2).InfoS("Snapshot validation passed", "snapshotID", snapshot.ID)
	return nil
}

// CleanupExpiredSnapshots removes old snapshots based on retention policy.
func (sm *SnapshotManager) CleanupExpiredSnapshots(ctx context.Context, deploymentRef corev1.ObjectReference) error {
	snapshots, err := sm.ListSnapshots(ctx, deploymentRef)
	if err != nil {
		return fmt.Errorf("failed to list snapshots for cleanup: %w", err)
	}

	if len(snapshots) == 0 {
		return nil
	}

	var toDelete []*DeploymentSnapshot

	// Apply max snapshots limit
	if sm.config.MaxSnapshots > 0 && len(snapshots) > sm.config.MaxSnapshots {
		toDelete = append(toDelete, snapshots[sm.config.MaxSnapshots:]...)
	}

	// Apply retention duration
	if sm.config.SnapshotRetentionDuration != nil {
		cutoffTime := time.Now().Add(-sm.config.SnapshotRetentionDuration.Duration)
		for _, snapshot := range snapshots {
			if snapshot.CreatedAt.Time.Before(cutoffTime) {
				// Only add if not already in toDelete list
				found := false
				for _, existing := range toDelete {
					if existing.ID == snapshot.ID {
						found = true
						break
					}
				}
				if !found {
					toDelete = append(toDelete, snapshot)
				}
			}
		}
	}

	// Delete expired snapshots
	for _, snapshot := range toDelete {
		if err := sm.DeleteSnapshot(ctx, snapshot.ID); err != nil {
			klog.ErrorS(err, "Failed to delete expired snapshot", "snapshotID", snapshot.ID)
			// Continue with other deletions
		} else {
			klog.V(2).InfoS("Deleted expired snapshot", "snapshotID", snapshot.ID)
		}
	}

	if len(toDelete) > 0 {
		klog.InfoS("Cleaned up expired snapshots", "count", len(toDelete), "deployment", deploymentRef.Name)
	}

	return nil
}

// generateSnapshotID creates a unique identifier for a snapshot.
func (sm *SnapshotManager) generateSnapshotID(deploymentRef corev1.ObjectReference, version string) string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%s-%s-%s-%d", deploymentRef.Namespace, deploymentRef.Name, version, timestamp)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("snap-%x", hash[:8])
}

// collectDeploymentResources gathers all resources related to a deployment.
func (sm *SnapshotManager) collectDeploymentResources(ctx context.Context, deploymentRef corev1.ObjectReference) ([]runtime.RawExtension, error) {
	var resources []runtime.RawExtension

	// Collect main deployment resource
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	deployment, err := sm.dynamicClient.Resource(deploymentGVR).
		Namespace(deploymentRef.Namespace).
		Get(ctx, deploymentRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	raw, err := json.Marshal(deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}
	resources = append(resources, runtime.RawExtension{Raw: raw})

	// Collect related services
	serviceGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	services, err := sm.dynamicClient.Resource(serviceGVR).
		Namespace(deploymentRef.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", deploymentRef.Name),
		})
	if err == nil {
		for _, service := range services.Items {
			raw, err := json.Marshal(service)
			if err == nil {
				resources = append(resources, runtime.RawExtension{Raw: raw})
			}
		}
	}

	// Collect related ConfigMaps
	configMapGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	configMaps, err := sm.dynamicClient.Resource(configMapGVR).
		Namespace(deploymentRef.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", deploymentRef.Name),
		})
	if err == nil {
		for _, cm := range configMaps.Items {
			raw, err := json.Marshal(cm)
			if err == nil {
				resources = append(resources, runtime.RawExtension{Raw: raw})
			}
		}
	}

	return resources, nil
}

// extractConfiguration extracts key configuration values from resources.
func (sm *SnapshotManager) extractConfiguration(resources []runtime.RawExtension) map[string]string {
	config := make(map[string]string)

	for _, resource := range resources {
		var obj map[string]interface{}
		if err := json.Unmarshal(resource.Raw, &obj); err != nil {
			continue
		}

		// Extract common configuration values
		if kind, ok := obj["kind"].(string); ok {
			switch kind {
			case "Deployment":
				sm.extractDeploymentConfig(obj, config)
			case "Service":
				sm.extractServiceConfig(obj, config)
			case "ConfigMap":
				sm.extractConfigMapConfig(obj, config)
			}
		}
	}

	return config
}

// extractDeploymentConfig extracts configuration from deployment resources.
func (sm *SnapshotManager) extractDeploymentConfig(obj map[string]interface{}, config map[string]string) {
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if replicas, ok := spec["replicas"].(float64); ok {
			config["deployment.replicas"] = fmt.Sprintf("%.0f", replicas)
		}
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if spec, ok := template["spec"].(map[string]interface{}); ok {
				if containers, ok := spec["containers"].([]interface{}); ok && len(containers) > 0 {
					if container, ok := containers[0].(map[string]interface{}); ok {
						if image, ok := container["image"].(string); ok {
							config["deployment.image"] = image
						}
					}
				}
			}
		}
	}
}

// extractServiceConfig extracts configuration from service resources.
func (sm *SnapshotManager) extractServiceConfig(obj map[string]interface{}, config map[string]string) {
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			if spec, ok := obj["spec"].(map[string]interface{}); ok {
				if serviceType, ok := spec["type"].(string); ok {
					config[fmt.Sprintf("service.%s.type", name)] = serviceType
				}
			}
		}
	}
}

// extractConfigMapConfig extracts configuration from ConfigMap resources.
func (sm *SnapshotManager) extractConfigMapConfig(obj map[string]interface{}, config map[string]string) {
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			if data, ok := obj["data"].(map[string]interface{}); ok {
				for key := range data {
					config[fmt.Sprintf("configmap.%s.%s", name, key)] = "present"
				}
			}
		}
	}
}

// collectTrafficConfiguration gathers traffic routing configuration.
func (sm *SnapshotManager) collectTrafficConfiguration(ctx context.Context, deploymentRef corev1.ObjectReference) (*TrafficConfiguration, error) {
	trafficConfig := &TrafficConfiguration{
		ServiceSelectors:   make(map[string]map[string]string),
		WeightDistribution: make(map[string]int32),
	}

	// Collect service selectors
	serviceGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	services, err := sm.dynamicClient.Resource(serviceGVR).
		Namespace(deploymentRef.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range services.Items {
		var serviceObj corev1.Service
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(service.Object, &serviceObj); err == nil {
			if serviceObj.Spec.Selector != nil {
				trafficConfig.ServiceSelectors[serviceObj.Name] = serviceObj.Spec.Selector
			}
		}
	}

	return trafficConfig, nil
}

// calculateConfigHash generates a hash of the configuration for integrity checking.
func (sm *SnapshotManager) calculateConfigHash(config map[string]string) string {
	// Sort keys for consistent hashing
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build sorted config string
	var configStr string
	for _, key := range keys {
		configStr += fmt.Sprintf("%s=%s;", key, config[key])
	}

	hash := sha256.Sum256([]byte(configStr))
	return fmt.Sprintf("%x", hash)
}

// Storage interface methods (to be implemented based on storage backend)

// storeSnapshot stores a snapshot in the configured storage backend.
func (sm *SnapshotManager) storeSnapshot(ctx context.Context, snapshot *DeploymentSnapshot) error {
	// This would be implemented based on the storage backend (ConfigMap, CRD, external storage)
	klog.V(2).InfoS("Storing snapshot", "snapshotID", snapshot.ID)
	// Placeholder implementation
	return nil
}

// retrieveSnapshotFromStorage retrieves a snapshot from storage.
func (sm *SnapshotManager) retrieveSnapshotFromStorage(ctx context.Context, snapshotID string) (*DeploymentSnapshot, error) {
	// This would be implemented based on the storage backend
	klog.V(2).InfoS("Retrieving snapshot from storage", "snapshotID", snapshotID)
	// Placeholder implementation
	return nil, fmt.Errorf("snapshot %s not found", snapshotID)
}

// listSnapshotsFromStorage lists snapshots from storage.
func (sm *SnapshotManager) listSnapshotsFromStorage(ctx context.Context, deploymentRef corev1.ObjectReference) ([]*DeploymentSnapshot, error) {
	// This would be implemented based on the storage backend
	klog.V(2).InfoS("Listing snapshots from storage", "deployment", deploymentRef.Name)
	// Placeholder implementation
	return []*DeploymentSnapshot{}, nil
}

// deleteSnapshotFromStorage removes a snapshot from storage.
func (sm *SnapshotManager) deleteSnapshotFromStorage(ctx context.Context, snapshotID string) error {
	// This would be implemented based on the storage backend
	klog.V(2).InfoS("Deleting snapshot from storage", "snapshotID", snapshotID)
	// Placeholder implementation
	return nil
}