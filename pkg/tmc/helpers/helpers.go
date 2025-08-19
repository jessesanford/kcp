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

package helpers

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TMC helper constants
const (
	// TMCFinalizer is the finalizer used for TMC resources
	TMCFinalizer = "tmc.kcp.io/finalizer"
	
	// TMCLabelPrefix is the common label prefix for TMC resources
	TMCLabelPrefix = "tmc.kcp.io"
	
	// TMCAnnotationPrefix is the common annotation prefix for TMC resources
	TMCAnnotationPrefix = "tmc.kcp.io"
	
	// Default timeouts
	DefaultReconcileTimeout = 30 * time.Second
	DefaultSyncTimeout     = 10 * time.Second
)

// Common TMC labels
const (
	WorkspaceLabel      = TMCLabelPrefix + "/workspace"
	ClusterLabel       = TMCLabelPrefix + "/cluster"
	LocationLabel      = TMCLabelPrefix + "/location"
	SyncerLabel        = TMCLabelPrefix + "/syncer"
	ManagedByLabel     = TMCLabelPrefix + "/managed-by"
	ComponentLabel     = TMCLabelPrefix + "/component"
)

// Common TMC annotations
const (
	LastSyncAnnotation       = TMCAnnotationPrefix + "/last-sync"
	SyncStatusAnnotation     = TMCAnnotationPrefix + "/sync-status"
	PlacementAnnotation      = TMCAnnotationPrefix + "/placement"
	OriginWorkspaceAnnotation = TMCAnnotationPrefix + "/origin-workspace"
)

// TMCLabels provides helpers for working with TMC labels.
type TMCLabels map[string]string

// NewTMCLabels creates a new TMC labels map.
func NewTMCLabels() TMCLabels {
	return make(TMCLabels)
}

// WithWorkspace adds the workspace label.
func (l TMCLabels) WithWorkspace(workspace logicalcluster.Name) TMCLabels {
	l[WorkspaceLabel] = string(workspace)
	return l
}

// WithCluster adds the cluster label.
func (l TMCLabels) WithCluster(cluster string) TMCLabels {
	l[ClusterLabel] = cluster
	return l
}

// WithLocation adds the location label.
func (l TMCLabels) WithLocation(location string) TMCLabels {
	l[LocationLabel] = location
	return l
}

// WithSyncer adds the syncer label.
func (l TMCLabels) WithSyncer(syncer string) TMCLabels {
	l[SyncerLabel] = syncer
	return l
}

// WithManagedBy adds the managed-by label.
func (l TMCLabels) WithManagedBy(managedBy string) TMCLabels {
	l[ManagedByLabel] = managedBy
	return l
}

// WithComponent adds the component label.
func (l TMCLabels) WithComponent(component string) TMCLabels {
	l[ComponentLabel] = component
	return l
}

// ToSelector returns a label selector.
func (l TMCLabels) ToSelector() labels.Selector {
	return labels.SelectorFromSet(l)
}

// TMCAnnotations provides helpers for working with TMC annotations.
type TMCAnnotations map[string]string

// NewTMCAnnotations creates a new TMC annotations map.
func NewTMCAnnotations() TMCAnnotations {
	return make(TMCAnnotations)
}

// WithLastSync adds the last sync annotation.
func (a TMCAnnotations) WithLastSync(timestamp time.Time) TMCAnnotations {
	a[LastSyncAnnotation] = timestamp.Format(time.RFC3339)
	return a
}

// WithSyncStatus adds the sync status annotation.
func (a TMCAnnotations) WithSyncStatus(status string) TMCAnnotations {
	a[SyncStatusAnnotation] = status
	return a
}

// WithPlacement adds the placement annotation.
func (a TMCAnnotations) WithPlacement(placement string) TMCAnnotations {
	a[PlacementAnnotation] = placement
	return a
}

// WithOriginWorkspace adds the origin workspace annotation.
func (a TMCAnnotations) WithOriginWorkspace(workspace logicalcluster.Name) TMCAnnotations {
	a[OriginWorkspaceAnnotation] = string(workspace)
	return a
}

// GetLastSync gets the last sync timestamp from annotations.
func GetLastSync(annotations map[string]string) (time.Time, error) {
	if annotations == nil {
		return time.Time{}, fmt.Errorf("annotations is nil")
	}
	
	lastSyncStr, exists := annotations[LastSyncAnnotation]
	if !exists {
		return time.Time{}, fmt.Errorf("last sync annotation not found")
	}
	
	return time.Parse(time.RFC3339, lastSyncStr)
}

// GetSyncStatus gets the sync status from annotations.
func GetSyncStatus(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	return annotations[SyncStatusAnnotation]
}

// ResourceKey represents a unique key for a Kubernetes resource.
type ResourceKey struct {
	Namespace string
	Name      string
	GVR       schema.GroupVersionResource
}

// String returns the string representation of the resource key.
func (k ResourceKey) String() string {
	if k.Namespace == "" {
		return fmt.Sprintf("%s/%s", k.GVR, k.Name)
	}
	return fmt.Sprintf("%s/%s/%s", k.GVR, k.Namespace, k.Name)
}

// NamespacedName returns the types.NamespacedName for this resource.
func (k ResourceKey) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: k.Namespace,
		Name:      k.Name,
	}
}

// NewResourceKey creates a new resource key.
func NewResourceKey(gvr schema.GroupVersionResource, namespace, name string) ResourceKey {
	return ResourceKey{
		Namespace: namespace,
		Name:      name,
		GVR:       gvr,
	}
}

// Condition helpers

// SetCondition sets a condition on a conditions slice, replacing any existing condition of the same type.
func SetCondition(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason, message string) {
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	
	for i, condition := range *conditions {
		if condition.Type == conditionType {
			// Update existing condition
			if condition.Status != status {
				newCondition.LastTransitionTime = metav1.NewTime(time.Now())
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			(*conditions)[i] = newCondition
			return
		}
	}
	
	// Add new condition
	*conditions = append(*conditions, newCondition)
}

// GetCondition finds a condition of the specified type.
func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue checks if a condition is true.
func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := GetCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// IsConditionFalse checks if a condition is false.
func IsConditionFalse(conditions []metav1.Condition, conditionType string) bool {
	condition := GetCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionFalse
}

// RemoveCondition removes a condition from the conditions slice.
func RemoveCondition(conditions *[]metav1.Condition, conditionType string) {
	for i, condition := range *conditions {
		if condition.Type == conditionType {
			*conditions = append((*conditions)[:i], (*conditions)[i+1:]...)
			return
		}
	}
}

// Finalizer helpers

// HasFinalizer checks if an object has the specified finalizer.
func HasFinalizer(obj metav1.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer adds a finalizer to an object if it doesn't already exist.
func AddFinalizer(obj metav1.Object, finalizer string) bool {
	if HasFinalizer(obj, finalizer) {
		return false
	}
	obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	return true
}

// RemoveFinalizer removes a finalizer from an object if it exists.
func RemoveFinalizer(obj metav1.Object, finalizer string) bool {
	finalizers := obj.GetFinalizers()
	for i, f := range finalizers {
		if f == finalizer {
			obj.SetFinalizers(append(finalizers[:i], finalizers[i+1:]...))
			return true
		}
	}
	return false
}

// HasTMCFinalizer checks if an object has the TMC finalizer.
func HasTMCFinalizer(obj metav1.Object) bool {
	return HasFinalizer(obj, TMCFinalizer)
}

// AddTMCFinalizer adds the TMC finalizer to an object.
func AddTMCFinalizer(obj metav1.Object) bool {
	return AddFinalizer(obj, TMCFinalizer)
}

// RemoveTMCFinalizer removes the TMC finalizer from an object.
func RemoveTMCFinalizer(obj metav1.Object) bool {
	return RemoveFinalizer(obj, TMCFinalizer)
}

// Utility functions

// GenerateResourceName generates a unique resource name based on workspace and base name.
func GenerateResourceName(workspace logicalcluster.Name, baseName string) string {
	workspaceStr := string(workspace)
	// Replace colons with dashes for valid Kubernetes names
	workspaceStr = strings.ReplaceAll(workspaceStr, ":", "-")
	return fmt.Sprintf("%s-%s", workspaceStr, baseName)
}

// ParseWorkspaceFromName attempts to parse the workspace from a generated resource name.
func ParseWorkspaceFromName(resourceName string) (logicalcluster.Name, string, error) {
	parts := strings.SplitN(resourceName, "-", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid resource name format: %s", resourceName)
	}
	
	workspaceStr := strings.ReplaceAll(parts[0], "-", ":")
	return logicalcluster.Name(workspaceStr), parts[1], nil
}

// IsTMCResource checks if a GVR is a TMC resource.
func IsTMCResource(gvr schema.GroupVersionResource) bool {
	return gvr.Group == "tmc.kcp.io"
}

// LogWithContext creates a logger with TMC context information.
func LogWithContext(ctx context.Context, workspace logicalcluster.Name, component string) klog.Logger {
	logger := klog.FromContext(ctx)
	return logger.WithValues(
		"workspace", workspace,
		"component", component,
		"subsystem", "tmc",
	)
}