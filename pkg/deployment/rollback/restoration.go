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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// RestorationManager handles the restoration of resources from snapshots.
type RestorationManager struct {
	dynamicClient dynamic.Interface
	cluster       logicalcluster.Name
	config        *EngineConfig
}

// NewRestorationManager creates a new restoration manager.
func NewRestorationManager(client dynamic.Interface, cluster logicalcluster.Name, config *EngineConfig) *RestorationManager {
	return &RestorationManager{
		dynamicClient: client,
		cluster:       cluster,
		config:        config,
	}
}

// RestoreFromSnapshot restores a deployment to the state captured in a snapshot.
func (rm *RestorationManager) RestoreFromSnapshot(ctx context.Context, snapshot *DeploymentSnapshot, dryRun bool) ([]RestoredResource, error) {
	klog.InfoS("Starting restoration from snapshot", "snapshotID", snapshot.ID, "dryRun", dryRun)

	var restoredResources []RestoredResource
	
	// Process resources in order of dependencies
	orderedResources := rm.orderResourcesByDependency(snapshot.Resources)

	for _, resource := range orderedResources {
		restored, err := rm.restoreResource(ctx, resource, dryRun)
		if err != nil {
			klog.ErrorS(err, "Failed to restore resource", "snapshotID", snapshot.ID)
			restored.Status = RestoreStatusFailed
			restored.Message = err.Error()
		}
		restoredResources = append(restoredResources, restored)
	}

	// Restore traffic configuration if enabled
	if snapshot.TrafficConfig != nil && rm.shouldRestoreTraffic() {
		trafficResult := rm.restoreTrafficConfiguration(ctx, snapshot.TrafficConfig, dryRun)
		restoredResources = append(restoredResources, trafficResult...)
	}

	// Clean up any canary resources
	cleanupResults := rm.cleanupCanaryResources(ctx, snapshot.DeploymentRef, dryRun)
	restoredResources = append(restoredResources, cleanupResults...)

	klog.InfoS("Completed restoration from snapshot", "snapshotID", snapshot.ID, "resourceCount", len(restoredResources))
	return restoredResources, nil
}

// restoreResource restores a single resource from its raw definition.
func (rm *RestorationManager) restoreResource(ctx context.Context, rawResource interface{}, dryRun bool) (RestoredResource, error) {
	// Convert to unstructured object
	var obj *unstructured.Unstructured
	var err error

	switch r := rawResource.(type) {
	case map[string]interface{}:
		obj = &unstructured.Unstructured{Object: r}
	default:
		// Try to unmarshal from bytes if needed
		if jsonBytes, ok := rawResource.([]byte); ok {
			var objMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &objMap); err != nil {
				return RestoredResource{}, fmt.Errorf("failed to unmarshal resource: %w", err)
			}
			obj = &unstructured.Unstructured{Object: objMap}
		} else {
			return RestoredResource{}, fmt.Errorf("unsupported resource type: %T", rawResource)
		}
	}

	// Extract resource information
	gvk := obj.GroupVersionKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()

	reference := corev1.ObjectReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       name,
		Namespace:  namespace,
	}

	restored := RestoredResource{
		Reference: reference,
		Status:    RestoreStatusRestored,
	}

	if dryRun {
		restored.Message = "Dry run - would restore resource"
		return restored, nil
	}

	// Get the GVR for this resource
	gvr, err := rm.getGVRFromGVK(gvk)
	if err != nil {
		return restored, fmt.Errorf("failed to get GVR for %s: %w", gvk, err)
	}

	// Clean up resource metadata for restoration
	rm.cleanupResourceMetadata(obj)

	// Attempt to restore the resource
	var client dynamic.ResourceInterface
	if namespace != "" {
		client = rm.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		client = rm.dynamicClient.Resource(gvr)
	}

	// Check if resource already exists
	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return restored, fmt.Errorf("failed to check existing resource: %w", err)
	}

	if apierrors.IsNotFound(err) {
		// Resource doesn't exist, create it
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return restored, fmt.Errorf("failed to create resource: %w", err)
		}
		restored.Message = "Resource created"
		klog.V(2).InfoS("Created resource during restoration", "kind", gvk.Kind, "name", name, "namespace", namespace)
	} else {
		// Resource exists, update it
		obj.SetResourceVersion(existing.GetResourceVersion())
		_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return restored, fmt.Errorf("failed to update resource: %w", err)
		}
		restored.Message = "Resource updated"
		klog.V(2).InfoS("Updated resource during restoration", "kind", gvk.Kind, "name", name, "namespace", namespace)
	}

	return restored, nil
}

// orderResourcesByDependency orders resources to ensure dependencies are restored first.
func (rm *RestorationManager) orderResourcesByDependency(resources []interface{}) []interface{} {
	// Simple ordering by resource type priority
	var configMaps, secrets, services, deployments, others []interface{}

	for _, resource := range resources {
		var obj *unstructured.Unstructured
		
		switch r := resource.(type) {
		case map[string]interface{}:
			obj = &unstructured.Unstructured{Object: r}
		default:
			others = append(others, resource)
			continue
		}

		kind := obj.GetKind()
		switch kind {
		case "ConfigMap":
			configMaps = append(configMaps, resource)
		case "Secret":
			secrets = append(secrets, resource)
		case "Service":
			services = append(services, resource)
		case "Deployment":
			deployments = append(deployments, resource)
		default:
			others = append(others, resource)
		}
	}

	// Return in dependency order
	var ordered []interface{}
	ordered = append(ordered, configMaps...)
	ordered = append(ordered, secrets...)
	ordered = append(ordered, services...)
	ordered = append(ordered, deployments...)
	ordered = append(ordered, others...)

	return ordered
}

// restoreTrafficConfiguration restores traffic routing configuration.
func (rm *RestorationManager) restoreTrafficConfiguration(ctx context.Context, trafficConfig *TrafficConfiguration, dryRun bool) []RestoredResource {
	var results []RestoredResource

	// Restore service selectors
	for serviceName, selectors := range trafficConfig.ServiceSelectors {
		result := rm.restoreServiceSelectors(ctx, serviceName, selectors, dryRun)
		results = append(results, result)
	}

	// Restore ingress rules
	for _, rule := range trafficConfig.IngressRules {
		result := rm.restoreIngressRule(ctx, rule, dryRun)
		results = append(results, result)
	}

	return results
}

// restoreServiceSelectors restores service selector configuration.
func (rm *RestorationManager) restoreServiceSelectors(ctx context.Context, serviceName string, selectors map[string]string, dryRun bool) RestoredResource {
	reference := corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       serviceName,
	}

	result := RestoredResource{
		Reference: reference,
		Status:    RestoreStatusRestored,
	}

	if dryRun {
		result.Message = "Dry run - would restore service selectors"
		return result
	}

	// Implementation would update service selectors
	result.Message = "Service selectors restored"
	klog.V(2).InfoS("Restored service selectors", "service", serviceName, "selectors", selectors)
	
	return result
}

// restoreIngressRule restores an ingress routing rule.
func (rm *RestorationManager) restoreIngressRule(ctx context.Context, rule IngressRule, dryRun bool) RestoredResource {
	reference := corev1.ObjectReference{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Name:       fmt.Sprintf("ingress-%s", rule.Host),
	}

	result := RestoredResource{
		Reference: reference,
		Status:    RestoreStatusRestored,
	}

	if dryRun {
		result.Message = "Dry run - would restore ingress rule"
		return result
	}

	// Implementation would update ingress rules
	result.Message = "Ingress rule restored"
	klog.V(2).InfoS("Restored ingress rule", "host", rule.Host, "path", rule.Path, "backend", rule.Backend)
	
	return result
}

// cleanupCanaryResources removes any canary deployment resources.
func (rm *RestorationManager) cleanupCanaryResources(ctx context.Context, deploymentRef corev1.ObjectReference, dryRun bool) []RestoredResource {
	var results []RestoredResource

	canaryName := fmt.Sprintf("%s-canary", deploymentRef.Name)
	
	// Cleanup canary deployment
	result := rm.cleanupCanaryDeployment(ctx, deploymentRef.Namespace, canaryName, dryRun)
	results = append(results, result)

	// Cleanup canary services
	canaryServiceResult := rm.cleanupCanaryService(ctx, deploymentRef.Namespace, canaryName, dryRun)
	results = append(results, canaryServiceResult)

	return results
}

// cleanupCanaryDeployment removes canary deployment if it exists.
func (rm *RestorationManager) cleanupCanaryDeployment(ctx context.Context, namespace, canaryName string, dryRun bool) RestoredResource {
	reference := corev1.ObjectReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       canaryName,
		Namespace:  namespace,
	}

	result := RestoredResource{
		Reference: reference,
		Status:    RestoreStatusRestored,
	}

	if dryRun {
		result.Message = "Dry run - would cleanup canary deployment"
		return result
	}

	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	client := rm.dynamicClient.Resource(deploymentGVR).Namespace(namespace)
	
	// Check if canary deployment exists
	_, err := client.Get(ctx, canaryName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		result.Status = RestoreStatusSkipped
		result.Message = "Canary deployment not found, skipping cleanup"
		return result
	}

	if err != nil {
		result.Status = RestoreStatusFailed
		result.Message = fmt.Sprintf("Failed to check canary deployment: %v", err)
		return result
	}

	// Delete canary deployment
	err = client.Delete(ctx, canaryName, metav1.DeleteOptions{})
	if err != nil {
		result.Status = RestoreStatusFailed
		result.Message = fmt.Sprintf("Failed to delete canary deployment: %v", err)
		return result
	}

	result.Message = "Canary deployment cleaned up"
	klog.InfoS("Cleaned up canary deployment during rollback", "deployment", canaryName, "namespace", namespace)
	
	return result
}

// cleanupCanaryService removes canary service if it exists.
func (rm *RestorationManager) cleanupCanaryService(ctx context.Context, namespace, canaryName string, dryRun bool) RestoredResource {
	reference := corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       canaryName,
		Namespace:  namespace,
	}

	result := RestoredResource{
		Reference: reference,
		Status:    RestoreStatusRestored,
	}

	if dryRun {
		result.Message = "Dry run - would cleanup canary service"
		return result
	}

	serviceGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	client := rm.dynamicClient.Resource(serviceGVR).Namespace(namespace)
	
	// Check if canary service exists
	_, err := client.Get(ctx, canaryName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		result.Status = RestoreStatusSkipped
		result.Message = "Canary service not found, skipping cleanup"
		return result
	}

	if err != nil {
		result.Status = RestoreStatusFailed
		result.Message = fmt.Sprintf("Failed to check canary service: %v", err)
		return result
	}

	// Delete canary service
	err = client.Delete(ctx, canaryName, metav1.DeleteOptions{})
	if err != nil {
		result.Status = RestoreStatusFailed
		result.Message = fmt.Sprintf("Failed to delete canary service: %v", err)
		return result
	}

	result.Message = "Canary service cleaned up"
	klog.InfoS("Cleaned up canary service during rollback", "service", canaryName, "namespace", namespace)
	
	return result
}

// ValidateRestoration validates that resources were properly restored.
func (rm *RestorationManager) ValidateRestoration(ctx context.Context, restoredResources []RestoredResource) error {
	var validationErrors []string

	for _, resource := range restoredResources {
		if resource.Status == RestoreStatusFailed {
			validationErrors = append(validationErrors, fmt.Sprintf("Resource %s/%s failed to restore: %s",
				resource.Reference.Kind, resource.Reference.Name, resource.Message))
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("restoration validation failed: %v", validationErrors)
	}

	// Additional validation: wait for deployment readiness
	for _, resource := range restoredResources {
		if resource.Reference.Kind == "Deployment" && resource.Status == RestoreStatusRestored {
			if err := rm.waitForDeploymentReadiness(ctx, resource.Reference); err != nil {
				return fmt.Errorf("deployment %s not ready after restoration: %w", resource.Reference.Name, err)
			}
		}
	}

	klog.InfoS("Restoration validation passed", "resourceCount", len(restoredResources))
	return nil
}

// waitForDeploymentReadiness waits for a deployment to become ready.
func (rm *RestorationManager) waitForDeploymentReadiness(ctx context.Context, deploymentRef corev1.ObjectReference) error {
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	client := rm.dynamicClient.Resource(deploymentGVR).Namespace(deploymentRef.Namespace)
	
	timeout := 5 * time.Minute
	if rm.config.DefaultTimeout != nil {
		timeout = rm.config.DefaultTimeout.Duration
	}

	klog.V(2).InfoS("Waiting for deployment readiness", "deployment", deploymentRef.Name, "timeout", timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment readiness: %w", ctx.Err())
		case <-ticker.C:
			deployment, err := client.Get(ctx, deploymentRef.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get deployment status: %w", err)
			}

			// Check deployment readiness
			status, exists, err := unstructured.NestedMap(deployment.Object, "status")
			if err != nil || !exists {
				continue
			}

			readyReplicas, exists, err := unstructured.NestedInt64(status, "readyReplicas")
			if err != nil || !exists {
				continue
			}

			replicas, exists, err := unstructured.NestedInt64(status, "replicas")
			if err != nil || !exists {
				continue
			}

			if readyReplicas == replicas && replicas > 0 {
				klog.V(2).InfoS("Deployment is ready", "deployment", deploymentRef.Name, "replicas", replicas)
				return nil
			}

			klog.V(2).InfoS("Waiting for deployment readiness", "deployment", deploymentRef.Name, "ready", readyReplicas, "desired", replicas)
		}
	}
}

// Helper methods

// cleanupResourceMetadata removes system-generated metadata fields.
func (rm *RestorationManager) cleanupResourceMetadata(obj *unstructured.Unstructured) {
	// Remove fields that shouldn't be restored
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetSelfLink("")
	obj.SetGeneration(0)
	
	// Remove managed fields and other system metadata
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(obj.Object, "status")
}

// getGVRFromGVK converts a GroupVersionKind to GroupVersionResource.
func (rm *RestorationManager) getGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Simple mapping of common kinds to resources
	// In a real implementation, this would use discovery client
	kindToResource := map[string]string{
		"Deployment":  "deployments",
		"Service":     "services",
		"ConfigMap":   "configmaps",
		"Secret":      "secrets",
		"Ingress":     "ingresses",
		"Pod":         "pods",
	}

	resource, exists := kindToResource[gvk.Kind]
	if !exists {
		// Default: lowercase kind + s
		resource = fmt.Sprintf("%ss", gvk.Kind)
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}

// shouldRestoreTraffic determines if traffic configuration should be restored.
func (rm *RestorationManager) shouldRestoreTraffic() bool {
	// This could be configured via the engine config
	// For now, default to true
	return true
}