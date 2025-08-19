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

package synctarget

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// FinalizerName is the finalizer added to SyncTarget resources
	FinalizerName = "workload.kcp.io/synctarget-cleanup"
	
	// RequeueAfter defines the interval for periodic health checks
	RequeueAfter = 30 * time.Second
)

// SyncTargetPhase represents the phase of a SyncTarget
type SyncTargetPhase string

const (
	// SyncTargetPhaseUnknown indicates the sync target phase is unknown
	SyncTargetPhaseUnknown SyncTargetPhase = "Unknown"
	// SyncTargetPhaseReady indicates the sync target is ready
	SyncTargetPhaseReady SyncTargetPhase = "Ready"
	// SyncTargetPhaseNotReady indicates the sync target is not ready
	SyncTargetPhaseNotReady SyncTargetPhase = "NotReady"
)

// SyncTargetConditionType represents a condition type for SyncTarget
type SyncTargetConditionType string

const (
	// SyncTargetReady indicates that the sync target is ready
	SyncTargetReady SyncTargetConditionType = "Ready"
)

// SyncTargetSpec defines the desired state of SyncTarget
type SyncTargetSpec struct {
	// KubeConfig is the name of a secret containing kubeconfig data
	KubeConfig string `json:"kubeConfig,omitempty"`
	
	// SyncTargetUID is a unique identifier for this sync target
	SyncTargetUID string `json:"syncTargetUID,omitempty"`
	
	// VirtualWorkspaces defines which virtual workspaces this target serves
	VirtualWorkspaces []VirtualWorkspace `json:"virtualWorkspaces,omitempty"`
}

// VirtualWorkspace represents a virtual workspace configuration
type VirtualWorkspace struct {
	// URL is the virtual workspace URL
	URL string `json:"url"`
}

// SyncTargetStatus defines the observed state of SyncTarget
type SyncTargetStatus struct {
	// Phase represents the current phase of the SyncTarget
	Phase SyncTargetPhase `json:"phase,omitempty"`
	
	// Conditions represent the conditions of the SyncTarget
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// LastHeartbeat is the last time the sync target reported status
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
	
	// AllocatableResources represent the resources allocatable on this sync target
	AllocatableResources corev1.ResourceList `json:"allocatableResources,omitempty"`
	
	// AvailableResources represent the resources currently available on this sync target
	AvailableResources corev1.ResourceList `json:"availableResources,omitempty"`
}

// SyncTarget represents a target cluster for workload synchronization
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Last Heartbeat",type="date",JSONPath=".status.lastHeartbeat"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncTargetSpec   `json:"spec,omitempty"`
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SetCondition sets a condition on the SyncTarget status
func (st *SyncTarget) SetCondition(condition metav1.Condition) {
	for i, existing := range st.Status.Conditions {
		if existing.Type == condition.Type {
			st.Status.Conditions[i] = condition
			return
		}
	}
	st.Status.Conditions = append(st.Status.Conditions, condition)
}

// GetCondition returns the condition with the specified type
func (st *SyncTarget) GetCondition(condType string) *metav1.Condition {
	for i := range st.Status.Conditions {
		if st.Status.Conditions[i].Type == condType {
			return &st.Status.Conditions[i]
		}
	}
	return nil
}

// Controller reconciles SyncTarget objects
type Controller struct {
	client.Client
	Scheme    *runtime.Scheme
	Workspace logicalcluster.Path
}

// NewController creates a new SyncTarget controller
func NewController(client client.Client, scheme *runtime.Scheme, workspace logicalcluster.Path) *Controller {
	return &Controller{
		Client:    client,
		Scheme:    scheme,
		Workspace: workspace,
	}
}

// Reconcile handles SyncTarget reconciliation
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("synctarget", req.NamespacedName)
	ctx = log.IntoContext(ctx, logger)

	logger.V(2).Info("Starting reconciliation")

	// Get the SyncTarget
	syncTarget := &SyncTarget{}
	if err := c.Get(ctx, req.NamespacedName, syncTarget); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("SyncTarget not found, assuming deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SyncTarget")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !syncTarget.DeletionTimestamp.IsZero() {
		logger.V(2).Info("Handling deletion")
		return c.handleDeletion(ctx, syncTarget)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(syncTarget, FinalizerName) {
		logger.V(4).Info("Adding finalizer")
		controllerutil.AddFinalizer(syncTarget, FinalizerName)
		if err := c.Update(ctx, syncTarget); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate connection
	if err := c.validateConnection(ctx, syncTarget); err != nil {
		logger.Error(err, "Connection validation failed")
		return c.updateStatusError(ctx, syncTarget, err)
	}

	// Update status to ready
	return c.updateStatusReady(ctx, syncTarget)
}

// validateConnection checks SyncTarget connectivity
func (c *Controller) validateConnection(ctx context.Context, st *SyncTarget) error {
	logger := log.FromContext(ctx)

	// Skip validation if no kubeconfig is specified
	if st.Spec.KubeConfig == "" {
		return fmt.Errorf("kubeconfig secret name not specified")
	}

	// Get kubeconfig secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: st.Namespace,
		Name:      st.Spec.KubeConfig,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get kubeconfig secret %s: %w", st.Spec.KubeConfig, err)
	}

	// Parse kubeconfig
	kubeconfig, exists := secret.Data["kubeconfig"]
	if !exists {
		return fmt.Errorf("kubeconfig key not found in secret %s", st.Spec.KubeConfig)
	}

	// Create client and test connection
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("invalid kubeconfig in secret %s: %w", st.Spec.KubeConfig, err)
	}

	// Test with discovery client for minimal overhead
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Test connectivity by getting server version
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to target cluster: %w", err)
	}

	logger.V(4).Info("Successfully validated connection", "serverVersion", version.String())
	return nil
}

// updateStatusReady updates status to ready condition
func (c *Controller) updateStatusReady(ctx context.Context, st *SyncTarget) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	condition := metav1.Condition{
		Type:               string(SyncTargetReady),
		Status:             metav1.ConditionTrue,
		Reason:             "Connected",
		Message:            "Successfully connected to target cluster",
		LastTransitionTime: metav1.Now(),
	}

	st.SetCondition(condition)
	st.Status.Phase = SyncTargetPhaseReady
	now := metav1.Now()
	st.Status.LastHeartbeat = &now

	if err := c.Status().Update(ctx, st); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.V(2).Info("Updated status to ready", "phase", st.Status.Phase)

	// Requeue for periodic health check
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// updateStatusError updates status when there's an error
func (c *Controller) updateStatusError(ctx context.Context, st *SyncTarget, syncErr error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	condition := metav1.Condition{
		Type:               string(SyncTargetReady),
		Status:             metav1.ConditionFalse,
		Reason:             "ConnectionFailed",
		Message:            syncErr.Error(),
		LastTransitionTime: metav1.Now(),
	}

	st.SetCondition(condition)
	st.Status.Phase = SyncTargetPhaseNotReady
	now := metav1.Now()
	st.Status.LastHeartbeat = &now

	if err := c.Status().Update(ctx, st); err != nil {
		logger.Error(err, "Failed to update error status")
		return ctrl.Result{}, err
	}

	logger.V(2).Info("Updated status to not ready", "phase", st.Status.Phase, "error", syncErr)

	// Requeue with backoff for retry
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// handleDeletion handles cleanup when SyncTarget is being deleted
func (c *Controller) handleDeletion(ctx context.Context, st *SyncTarget) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Perform cleanup operations here
	// TODO: Add cleanup logic for associated resources in follow-up PR
	
	logger.V(4).Info("Performing cleanup operations")

	// Remove finalizer to allow deletion
	controllerutil.RemoveFinalizer(st, FinalizerName)
	if err := c.Update(ctx, st); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.V(2).Info("Successfully cleaned up SyncTarget")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&SyncTarget{}).
		Complete(c)
}

// SyncTargetReconciler implements reconcile.Reconciler for backward compatibility
type SyncTargetReconciler struct {
	*Controller
}

// Reconcile implements reconcile.Reconciler
func (r *SyncTargetReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return r.Controller.Reconcile(ctx, ctrl.Request(req))
}