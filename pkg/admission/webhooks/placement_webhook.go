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
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkloadPlacementWebhook validates and mutates WorkloadPlacement resources
// This webhook ensures placement policies are properly configured
// and enforces TMC-specific placement rules

const (
	PlacementPluginName = "workload.kcp.io/WorkloadPlacement"
	
	// Placement strategy types
	StrategySpread   = "Spread"
	StrategyBinpack  = "Binpack"
	StrategyManual   = "Manual"
	
	// Default annotations and labels
	TMCPlacementStrategyAnnotation = "tmc.workload.kcp.io/placement-strategy"
	TMCPlacementPriorityAnnotation = "tmc.workload.kcp.io/placement-priority"
	TMCPlacementConstraintsLabel   = "tmc.workload.kcp.io/constraints"
	TMCPlacementAffinityLabel      = "tmc.workload.kcp.io/affinity"
	TMCPlacementStatusAnnotation   = "tmc.workload.kcp.io/placement-status"
	
	// Validation constants
	MaxTargetClusters     = 50
	MaxConstraintPairs    = 20
	MaxAffinityRules      = 10
	DefaultPriority       = 100
	MinPriority          = 1
	MaxPriority          = 1000
)

// placementWebhook implements admission plugin for WorkloadPlacement resources
type placementWebhook struct {
	*admission.Handler
}

// Ensure that the required admission interfaces are implemented
var _ admission.MutationInterface = &placementWebhook{}
var _ admission.ValidationInterface = &placementWebhook{}
var _ admission.InitializationValidator = &placementWebhook{}

func NewPlacementWebhook(_ io.Reader) (admission.Interface, error) {
	return &placementWebhook{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func RegisterPlacementWebhook(plugins *admission.Plugins) {
	plugins.Register(PlacementPluginName, NewPlacementWebhook)
}

// Admit handles mutation of WorkloadPlacement resources
func (w *placementWebhook) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	clusterName, err := genericapirequest.ClusterNameFrom(ctx)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	// Only handle WorkloadPlacement resources
	if a.GetResource().Group != "workload.kcp.io" || a.GetResource().Resource != "workloadplacements" {
		return nil
	}

	u, ok := a.GetObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected type %T", a.GetObject())
	}

	// Extract placement spec
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

// mutateOnCreate adds default placement configuration on creation
func (w *placementWebhook) mutateOnCreate(u *unstructured.Unstructured, spec map[string]interface{}, clusterName logicalcluster.Name) error {
	// Ensure labels exist
	labels := u.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Add workspace label
	labels[TMCWorkspaceLabel] = clusterName.String()
	
	// Set default constraint label if not present
	if _, found := labels[TMCPlacementConstraintsLabel]; !found {
		labels[TMCPlacementConstraintsLabel] = "none"
	}

	u.SetLabels(labels)

	// Ensure annotations exist
	annotations := u.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Set default strategy if not provided
	if _, found := annotations[TMCPlacementStrategyAnnotation]; !found {
		annotations[TMCPlacementStrategyAnnotation] = StrategySpread
	}

	// Set default priority if not provided
	if _, found := annotations[TMCPlacementPriorityAnnotation]; !found {
		annotations[TMCPlacementPriorityAnnotation] = strconv.Itoa(DefaultPriority)
	}

	// Initialize placement status
	annotations[TMCPlacementStatusAnnotation] = "pending"

	u.SetAnnotations(annotations)

	// Set default strategy in spec if not provided
	if _, found, err := unstructured.NestedString(spec, "strategy"); !found && err == nil {
		spec["strategy"] = StrategySpread
	}

	// Set default replicas if not provided
	if _, found, err := unstructured.NestedInt64(spec, "replicas"); !found && err == nil {
		spec["replicas"] = int64(1)
	}

	return nil
}

// mutateOnUpdate updates placement configuration on resource updates
func (w *placementWebhook) mutateOnUpdate(u *unstructured.Unstructured, spec map[string]interface{}) error {
	annotations := u.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Update placement status based on spec changes
	// In a real implementation, this might be handled by the controller
	if status, found := annotations[TMCPlacementStatusAnnotation]; found && status == "pending" {
		// Keep as pending during updates until controller processes it
		annotations[TMCPlacementStatusAnnotation] = "updating"
	}

	u.SetAnnotations(annotations)
	return nil
}

// Validate ensures WorkloadPlacement resources meet TMC requirements
func (w *placementWebhook) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	// Only handle WorkloadPlacement resources
	if a.GetResource().Group != "workload.kcp.io" || a.GetResource().Resource != "workloadplacements" {
		return nil
	}

	u, ok := a.GetObject().(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected type %T", a.GetObject())
	}

	// Extract placement data
	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}
	if !found {
		return admission.NewForbidden(a, errors.New("spec is required"))
	}

	var allErrs field.ErrorList

	// Validate placement strategy
	allErrs = append(allErrs, w.validateStrategy(spec, field.NewPath("spec", "strategy"))...)

	// Validate target clusters
	allErrs = append(allErrs, w.validateTargetClusters(spec, field.NewPath("spec", "targetClusters"))...)

	// Validate replicas
	allErrs = append(allErrs, w.validateReplicas(spec, field.NewPath("spec", "replicas"))...)

	// Validate placement constraints
	allErrs = append(allErrs, w.validateConstraints(spec, field.NewPath("spec", "constraints"))...)

	// Validate priority annotation
	allErrs = append(allErrs, w.validatePriority(u.GetAnnotations(), field.NewPath("metadata", "annotations", TMCPlacementPriorityAnnotation))...)

	// Validate affinity rules
	allErrs = append(allErrs, w.validateAffinity(spec, field.NewPath("spec", "affinity"))...)

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, allErrs.ToAggregate())
	}

	return nil
}

// validateStrategy ensures placement strategy is valid
func (w *placementWebhook) validateStrategy(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	strategy, found, err := unstructured.NestedString(spec, "strategy")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["strategy"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found || strategy == "" {
		allErrs = append(allErrs, field.Required(fldPath, "strategy is required"))
		return allErrs
	}

	validStrategies := sets.NewString(StrategySpread, StrategyBinpack, StrategyManual)
	if !validStrategies.Has(strategy) {
		allErrs = append(allErrs, field.NotSupported(fldPath, strategy, validStrategies.List()))
	}

	return allErrs
}

// validateTargetClusters ensures target clusters are properly specified
func (w *placementWebhook) validateTargetClusters(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	targetClusters, found, err := unstructured.NestedSlice(spec, "targetClusters")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["targetClusters"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found {
		return allErrs // Optional field for some strategies
	}

	if len(targetClusters) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, targetClusters, "cannot be empty if specified"))
		return allErrs
	}

	if len(targetClusters) > MaxTargetClusters {
		allErrs = append(allErrs, field.TooMany(fldPath, len(targetClusters), MaxTargetClusters))
	}

	// Validate each target cluster
	clusterNames := sets.NewString()
	for i, cluster := range targetClusters {
		clusterMap, ok := cluster.(map[string]interface{})
		if !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), cluster, "must be an object"))
			continue
		}

		clusterName, found, err := unstructured.NestedString(clusterMap, "name")
		if err != nil || !found || clusterName == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("name"), "cluster name is required"))
			continue
		}

		// Check for duplicate cluster names
		if clusterNames.Has(clusterName) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), clusterName))
		} else {
			clusterNames.Insert(clusterName)
		}

		// Validate cluster name format
		if !isValidDNSName(clusterName) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("name"), clusterName, "must be a valid DNS name"))
		}
	}

	return allErrs
}

// validateReplicas ensures replica count is valid
func (w *placementWebhook) validateReplicas(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	replicas, found, err := unstructured.NestedInt64(spec, "replicas")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["replicas"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found {
		return allErrs // Optional field with default
	}

	if replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, replicas, "must be non-negative"))
	}

	if replicas == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, replicas, "must be greater than 0"))
	}

	return allErrs
}

// validateConstraints ensures placement constraints are valid
func (w *placementWebhook) validateConstraints(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	constraints, found, err := unstructured.NestedMap(spec, "constraints")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["constraints"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found {
		return allErrs // Optional field
	}

	if len(constraints) > MaxConstraintPairs {
		allErrs = append(allErrs, field.TooMany(fldPath, len(constraints), MaxConstraintPairs))
	}

	// Validate constraint key-value pairs
	for key, value := range constraints {
		if key == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(key), key, "constraint key cannot be empty"))
		}
		if valueStr, ok := value.(string); ok {
			if valueStr == "" {
				allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "constraint value cannot be empty"))
			}
		} else {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "constraint value must be a string"))
		}
	}

	return allErrs
}

// validatePriority ensures placement priority is within valid range
func (w *placementWebhook) validatePriority(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	priorityStr, found := annotations[TMCPlacementPriorityAnnotation]
	if !found {
		return allErrs // Optional field with default
	}

	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, priorityStr, "must be a valid integer"))
		return allErrs
	}

	if priority < MinPriority || priority > MaxPriority {
		allErrs = append(allErrs, field.Invalid(fldPath, priority, fmt.Sprintf("must be between %d and %d", MinPriority, MaxPriority)))
	}

	return allErrs
}

// validateAffinity ensures affinity rules are properly configured
func (w *placementWebhook) validateAffinity(spec map[string]interface{}, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	affinity, found, err := unstructured.NestedMap(spec, "affinity")
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec["affinity"], fmt.Sprintf("failed to parse: %v", err)))
		return allErrs
	}
	if !found {
		return allErrs // Optional field
	}

	// Validate node affinity
	if nodeAffinity, found, err := unstructured.NestedSlice(affinity, "nodeAffinity"); found && err == nil {
		if len(nodeAffinity) > MaxAffinityRules {
			allErrs = append(allErrs, field.TooMany(fldPath.Child("nodeAffinity"), len(nodeAffinity), MaxAffinityRules))
		}

		for i, rule := range nodeAffinity {
			if ruleMap, ok := rule.(map[string]interface{}); ok {
				if key, found, err := unstructured.NestedString(ruleMap, "key"); !found || err != nil || key == "" {
					allErrs = append(allErrs, field.Required(fldPath.Child("nodeAffinity").Index(i).Child("key"), "affinity key is required"))
				}
			}
		}
	}

	// Validate cluster affinity
	if clusterAffinity, found, err := unstructured.NestedSlice(affinity, "clusterAffinity"); found && err == nil {
		if len(clusterAffinity) > MaxAffinityRules {
			allErrs = append(allErrs, field.TooMany(fldPath.Child("clusterAffinity"), len(clusterAffinity), MaxAffinityRules))
		}
	}

	return allErrs
}

func (w *placementWebhook) ValidateInitialization() error {
	return nil
}