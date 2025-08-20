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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"regexp"
)

// TMCConfig represents the configuration for TMC (Transport Management Controller)
type TMCConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TMCConfigSpec   `json:"spec,omitempty"`
	Status TMCConfigStatus `json:"status,omitempty"`
}

// TMCConfigSpec defines the desired state of TMCConfig
type TMCConfigSpec struct {
	// FeatureFlags enables or disables specific features
	FeatureFlags map[string]bool `json:"featureFlags,omitempty"`
}

// TMCConfigStatus defines the observed state of TMCConfig
type TMCConfigStatus struct {
	// Conditions represent the latest available observations of the config's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase represents the current phase of the TMC configuration
	Phase string `json:"phase,omitempty"`
	// ObservedGeneration represents the generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// TMCConfigList contains a list of TMCConfig
type TMCConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TMCConfig `json:"items"`
}

// TMCStatus represents a general status structure for TMC objects
type TMCStatus struct {
	// Conditions represent the latest available observations of the object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase represents the current phase
	Phase string `json:"phase,omitempty"`
	// ObservedGeneration represents the generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ResourceIdentifier identifies a Kubernetes resource
type ResourceIdentifier struct {
	// Group is the API group of the resource
	Group string `json:"group,omitempty"`
	// Version is the API version of the resource
	Version string `json:"version"`
	// Resource is the resource name (plural)
	Resource string `json:"resource"`
	// Kind is the kind of the resource
	Kind string `json:"kind,omitempty"`
	// Namespace is the namespace of the resource (empty for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`
	// Name is the name of the resource
	Name string `json:"name,omitempty"`
}

// ClusterIdentifier identifies a cluster
type ClusterIdentifier struct {
	// Name is the name of the cluster
	Name string `json:"name"`
	// Region is the region where the cluster is located
	Region string `json:"region,omitempty"`
	// Zone is the availability zone where the cluster is located
	Zone string `json:"zone,omitempty"`
	// Provider is the cloud provider
	Provider string `json:"provider,omitempty"`
	// Environment is the environment type
	Environment string `json:"environment,omitempty"`
	// Labels are additional labels for the cluster
	Labels map[string]string `json:"labels,omitempty"`
}

// Validation functions

var (
	// DNS-1123 compliant names
	nameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	// Kubernetes API version pattern
	versionRegex = regexp.MustCompile(`^v\d+((alpha|beta)\d+)?$`)
	// Valid TMC phases
	validPhases = map[string]bool{
		"Pending":      true,
		"Running":      true,
		"Succeeded":    true,
		"Failed":       true,
		"Unknown":      true,
		"Terminating":  true,
	}
	// Valid cloud providers
	validProviders = map[string]bool{
		"aws":       true,
		"gcp":       true,
		"azure":     true,
		"alibaba":   true,
		"ibm":       true,
		"oracle":    true,
		"baremetal": true,
		"onprem":    true,
	}
	// Valid environments
	validEnvironments = map[string]bool{
		"prod":        true,
		"production":  true,
		"staging":     true,
		"dev":         true,
		"development": true,
		"test":        true,
		"testing":     true,
		"qa":          true,
		"sandbox":     true,
	}
)

// ValidateTMCConfig validates a TMCConfig object
func ValidateTMCConfig(config *TMCConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if config.Name == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("metadata", "name"), "name is required"))
	} else if !nameRegex.MatchString(config.Name) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"), config.Name, "name must be DNS-1123 compliant"))
	}
	
	allErrs = append(allErrs, ValidateTMCConfigSpec(&config.Spec, field.NewPath("spec"))...)
	
	return allErrs
}

// ValidateTMCConfigSpec validates a TMCConfigSpec
func ValidateTMCConfigSpec(spec *TMCConfigSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if spec.FeatureFlags != nil {
		for flag := range spec.FeatureFlags {
			if flag == "" {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("featureFlags"), flag, "feature flag name cannot be empty"))
			}
		}
	}
	
	return allErrs
}

// ValidateTMCStatus validates a TMCStatus object
func ValidateTMCStatus(status *TMCStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Validate phase
	if status.Phase != "" && !validPhases[status.Phase] {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("phase"), status.Phase, "invalid phase"))
	}
	
	// Validate conditions
	conditionTypes := make(map[string]bool)
	for i, condition := range status.Conditions {
		condPath := fldPath.Child("conditions").Index(i)
		
		if condition.Type == "" {
			allErrs = append(allErrs, field.Required(condPath.Child("type"), "condition type is required"))
		} else if conditionTypes[condition.Type] {
			allErrs = append(allErrs, field.Duplicate(condPath.Child("type"), condition.Type))
		} else {
			conditionTypes[condition.Type] = true
		}
		
		if condition.Status == "" {
			allErrs = append(allErrs, field.Required(condPath.Child("status"), "condition status is required"))
		}
	}
	
	return allErrs
}

// ValidateResourceIdentifier validates a ResourceIdentifier
func ValidateResourceIdentifier(id *ResourceIdentifier, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if id.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	} else if !versionRegex.MatchString(id.Version) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), id.Version, "invalid version format"))
	}
	
	if id.Resource == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("resource"), "resource is required"))
	} else if !nameRegex.MatchString(id.Resource) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resource"), id.Resource, "resource must be DNS-1123 compliant"))
	}
	
	if id.Namespace != "" && !nameRegex.MatchString(id.Namespace) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), id.Namespace, "namespace must be DNS-1123 compliant"))
	}
	
	return allErrs
}

// ValidateClusterIdentifier validates a ClusterIdentifier
func ValidateClusterIdentifier(id *ClusterIdentifier, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if id.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	} else if !nameRegex.MatchString(id.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), id.Name, "name must be DNS-1123 compliant"))
	}
	
	if id.Provider != "" && !validProviders[id.Provider] {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider"), id.Provider, "invalid provider"))
	}
	
	if id.Environment != "" && !validEnvironments[id.Environment] {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("environment"), id.Environment, "invalid environment"))
	}
	
	// Validate label names and values
	for key, value := range id.Labels {
		if len(key) > 253 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels").Key(key), key, "label key too long"))
		}
		if len(value) > 253 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels").Key(key), value, "label value too long"))
		}
	}
	
	return allErrs
}

// DeepCopy methods (basic implementations)

func (in *TMCConfig) DeepCopy() *TMCConfig {
	if in == nil {
		return nil
	}
	out := new(TMCConfig)
	in.DeepCopyInto(out)
	return out
}

func (in *TMCConfig) DeepCopyInto(out *TMCConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *TMCConfigSpec) DeepCopyInto(out *TMCConfigSpec) {
	*out = *in
	if in.FeatureFlags != nil {
		in, out := &in.FeatureFlags, &out.FeatureFlags
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

func (in *TMCConfigStatus) DeepCopyInto(out *TMCConfigStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *TMCConfigList) DeepCopy() *TMCConfigList {
	if in == nil {
		return nil
	}
	out := new(TMCConfigList)
	in.DeepCopyInto(out)
	return out
}

func (in *TMCConfigList) DeepCopyInto(out *TMCConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TMCConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *TMCStatus) DeepCopy() *TMCStatus {
	if in == nil {
		return nil
	}
	out := new(TMCStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *TMCStatus) DeepCopyInto(out *TMCStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}