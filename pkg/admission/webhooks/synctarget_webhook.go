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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SyncTargetWebhook validates and mutates SyncTarget resources
// This webhook ensures SyncTarget resources have proper configuration
// and enforces TMC-specific validation rules

const (
	SyncTargetPluginName = "workload.kcp.io/SyncTarget"
	
	// Default annotations and labels
	TMCManagedLabel             = "tmc.workload.kcp.io/managed"
	TMCWorkspaceLabel           = "tmc.workload.kcp.io/workspace"
	TMCSchedulingLabel          = "tmc.workload.kcp.io/scheduling"
	TMCCapacityAnnotation       = "tmc.workload.kcp.io/capacity"
	TMCLocationAnnotation       = "tmc.workload.kcp.io/location"
	TMCLastHeartbeatAnnotation  = "tmc.workload.kcp.io/last-heartbeat"
	
	// Validation constants
	MaxClusterNameLength = 63
	MinClusterNameLength = 3
)

// syncTargetWebhook implements admission plugin for SyncTarget resources
type syncTargetWebhook struct {
	*admission.Handler
}

// Ensure that the required admission interfaces are implemented
var _ admission.MutationInterface = &syncTargetWebhook{}
var _ admission.ValidationInterface = &syncTargetWebhook{}
var _ admission.InitializationValidator = &syncTargetWebhook{}

func NewSyncTargetWebhook(_ io.Reader) (admission.Interface, error) {
	return &syncTargetWebhook{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func RegisterSyncTargetWebhook(plugins *admission.Plugins) {
	plugins.Register(SyncTargetPluginName, NewSyncTargetWebhook)
}

// Admit handles mutation of SyncTarget resources
func (w *syncTargetWebhook) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	clusterName, err := genericapirequest.ClusterNameFrom(ctx)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	// Only handle SyncTarget resources
	if a.GetResource().Group != "workload.kcp.io" || a.GetResource().Resource != "synctargets" {
		return nil
	}

	u, ok := a.GetObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected type %T", a.GetObject())
	}

	// Extract SyncTarget data
	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}
	if !found {
		spec = make(map[string]interface{})
	}

	// Mutate based on operation
	if a.GetOperation() == admission.Create {
		if err := w.mutateOnCreate(u, spec, clusterName); err != nil {
			return err
		}
	} else if a.GetOperation() == admission.Update {
		if err := w.mutateOnUpdate(u, spec); err != nil {
			return err
		}
	}

	// Update the spec back to the unstructured object
	if err := unstructured.SetNestedMap(u.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	return nil
}

// mutateOnCreate adds default labels and annotations on creation
func (w *syncTargetWebhook) mutateOnCreate(u *unstructured.Unstructured, spec map[string]interface{}, clusterName logicalcluster.Name) error {
	// Ensure labels exist
	labels := u.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Add TMC managed label
	labels[TMCManagedLabel] = "true"
	labels[TMCWorkspaceLabel] = clusterName.String()
	labels[TMCSchedulingLabel] = "enabled"
	u.SetLabels(labels)

	// Ensure annotations exist
	annotations := u.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Set default location if not provided
	if _, found := annotations[TMCLocationAnnotation]; !found {
		annotations[TMCLocationAnnotation] = "default"
	}

	// Initialize capacity annotation if not provided
	if _, found := annotations[TMCCapacityAnnotation]; !found {
		annotations[TMCCapacityAnnotation] = `{"cpu":"1000m","memory":"1Gi","pods":"100"}`
	}

	u.SetAnnotations(annotations)

	// Set default API server URL scheme to https if not set
	if apiServerURL, found, err := unstructured.NestedString(spec, "apiServerURL"); found && err == nil && apiServerURL != "" {
		if !strings.HasPrefix(apiServerURL, "https://") && !strings.HasPrefix(apiServerURL, "http://") {
			spec["apiServerURL"] = "https://" + apiServerURL
		}
	}

	return nil
}

// mutateOnUpdate updates annotations on resource updates
func (w *syncTargetWebhook) mutateOnUpdate(u *unstructured.Unstructured, spec map[string]interface{}) error {
	annotations := u.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Update last heartbeat annotation on any update
	// In a real implementation, this might be set by the controller
	// annotations[TMCLastHeartbeatAnnotation] = time.Now().Format(time.RFC3339)

	u.SetAnnotations(annotations)
	return nil
}

// Validate ensures SyncTarget resources meet TMC requirements
func (w *syncTargetWebhook) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	// Only handle SyncTarget resources
	if a.GetResource().Group != "workload.kcp.io" || a.GetResource().Resource != "synctargets" {
		return nil
	}

	u, ok := a.GetObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected type %T", a.GetObject())
	}

	// Extract SyncTarget data
	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}
	if !found {
		return admission.NewForbidden(a, errors.New("spec is required"))
	}

	var allErrs field.ErrorList

	// Validate cluster name
	if clusterName := u.GetName(); clusterName != "" {
		allErrs = append(allErrs, w.validateClusterName(clusterName, field.NewPath("metadata", "name"))...)
	}

	// Validate API server URL
	allErrs = append(allErrs, w.validateAPIServerURL(spec, field.NewPath("spec", "apiServerURL"))...)

	// Validate capacity annotation
	allErrs = append(allErrs, w.validateCapacity(u.GetAnnotations(), field.NewPath("metadata", "annotations", TMCCapacityAnnotation))...)

	// Validate labels
	allErrs = append(allErrs, w.validateLabels(u.GetLabels(), field.NewPath("metadata", "labels"))...)

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, allErrs.ToAggregate())
	}

	return nil
}

// validateClusterName ensures cluster name meets requirements
func (w *syncTargetWebhook) validateClusterName(name string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(name) < MinClusterNameLength {
		allErrs = append(allErrs, field.Invalid(fldPath, name, fmt.Sprintf("must be at least %d characters", MinClusterNameLength)))
	}
	if len(name) > MaxClusterNameLength {
		allErrs = append(allErrs, field.Invalid(fldPath, name, fmt.Sprintf("must be at most %d characters", MaxClusterNameLength)))
	}

	// Cluster name should follow DNS naming conventions
	if !isValidDNSName(name) {
		allErrs = append(allErrs, field.Invalid(fldPath, name, "must be a valid DNS name"))
	}

	return allErrs
}

// validateAPIServerURL ensures the API server URL is valid and secure
func (w *syncTargetWebhook) validateAPIServerURL(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	apiServerURL, found, err := unstructured.NestedString(spec, "apiServerURL")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["apiServerURL"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found || apiServerURL == "" {
		allErrs = append(allErrs, field.Required(fldPath, "apiServerURL is required"))
		return allErrs
	}

	// Parse URL
	parsedURL, err := url.Parse(apiServerURL)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, apiServerURL, fmt.Sprintf("invalid URL: %v", err)))
		return allErrs
	}

	// Require HTTPS for security
	if parsedURL.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(fldPath, apiServerURL, "must use https scheme for security"))
	}

	// Require host
	if parsedURL.Host == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, apiServerURL, "host is required"))
	}

	return allErrs
}

// validateCapacity ensures capacity annotation has valid resource quantities
func (w *syncTargetWebhook) validateCapacity(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	capacityStr, found := annotations[TMCCapacityAnnotation]
	if !found {
		return allErrs // Optional field
	}

	// In a real implementation, this would parse JSON and validate resource quantities
	// For now, we'll do basic validation
	if capacityStr == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, capacityStr, "capacity cannot be empty"))
		return allErrs
	}

	// Basic validation for resource format
	if !strings.Contains(capacityStr, "cpu") || !strings.Contains(capacityStr, "memory") {
		allErrs = append(allErrs, field.Invalid(fldPath, capacityStr, "must contain at least cpu and memory resources"))
	}

	// Validate that it looks like JSON
	if !strings.HasPrefix(capacityStr, "{") || !strings.HasSuffix(capacityStr, "}") {
		allErrs = append(allErrs, field.Invalid(fldPath, capacityStr, "must be valid JSON"))
	}

	return allErrs
}

// validateLabels ensures required TMC labels are present and valid
func (w *syncTargetWebhook) validateLabels(labels map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate TMC managed label exists
	if managed, found := labels[TMCManagedLabel]; found && managed != "true" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(TMCManagedLabel), managed, "must be 'true'"))
	}

	// Validate scheduling label if present
	if scheduling, found := labels[TMCSchedulingLabel]; found {
		validValues := sets.NewString("enabled", "disabled", "drain")
		if !validValues.Has(scheduling) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child(TMCSchedulingLabel), scheduling, validValues.List()))
		}
	}

	return allErrs
}

// isValidDNSName checks if a string is a valid DNS name
func isValidDNSName(name string) bool {
	if len(name) == 0 {
		return false
	}
	
	// Basic DNS name validation
	for i, c := range name {
		if i == 0 || i == len(name)-1 {
			if !isAlphanumeric(c) {
				return false
			}
		} else {
			if !isAlphanumeric(c) && c != '-' {
				return false
			}
		}
	}
	return true
}

// isAlphanumeric checks if a character is alphanumeric
func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (w *syncTargetWebhook) ValidateInitialization() error {
	return nil
}