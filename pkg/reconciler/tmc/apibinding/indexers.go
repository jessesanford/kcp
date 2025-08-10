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

package apibinding

import (
	"fmt"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	"github.com/kcp-dev/kcp/sdk/client"
)

const (
	// IndexTMCAPIBindingsByExport indexes TMC APIBindings by their APIExport reference
	IndexTMCAPIBindingsByExport = "tmcAPIBindingsByExport"
	
	// IndexTMCAPIBindingsByWorkspace indexes TMC APIBindings by logical cluster
	IndexTMCAPIBindingsByWorkspace = "tmcAPIBindingsByWorkspace"
)

// IndexTMCAPIBindingsByExportFunc indexes TMC APIBindings by their referenced APIExport.
// This allows efficient lookups of APIBindings that reference a particular TMC APIExport,
// which is useful when an APIExport changes and we need to find affected APIBindings.
func IndexTMCAPIBindingsByExportFunc(obj interface{}) ([]string, error) {
	binding, ok := obj.(*apisv1alpha2.APIBinding)
	if !ok {
		return []string{}, fmt.Errorf("obj is supposed to be an APIBinding, but is %T", obj)
	}

	// Only index TMC-related APIBindings
	if !isTMCAPIBinding(binding) {
		return []string{}, nil
	}

	if binding.Spec.Reference.Export == nil {
		return []string{}, nil
	}

	export := binding.Spec.Reference.Export
	clusterName := logicalcluster.From(binding)
	
	// Create index key from the export reference
	var exportCluster logicalcluster.Name
	if export.Path != "" {
		exportCluster = logicalcluster.NewPath(export.Path).Last()
	} else {
		exportCluster = clusterName
	}

	key := client.ToClusterAwareKey(exportCluster.Path(), export.Name)
	return []string{key}, nil
}

// IndexTMCAPIBindingsByWorkspaceFunc indexes TMC APIBindings by their workspace (logical cluster).
// This enables efficient workspace-based queries for TMC functionality management.
func IndexTMCAPIBindingsByWorkspaceFunc(obj interface{}) ([]string, error) {
	binding, ok := obj.(*apisv1alpha2.APIBinding)
	if !ok {
		return []string{}, fmt.Errorf("obj is supposed to be an APIBinding, but is %T", obj)
	}

	// Only index TMC-related APIBindings
	if !isTMCAPIBinding(binding) {
		return []string{}, nil
	}

	clusterName := logicalcluster.From(binding)
	return []string{clusterName.String()}, nil
}

// GetTMCAPIExportKey generates a consistent key for TMC APIExport lookups
func GetTMCAPIExportKey(clusterPath logicalcluster.Path, exportName string) string {
	return client.ToClusterAwareKey(clusterPath, exportName)
}

// GetWorkspaceKey generates a consistent key for workspace-based lookups
func GetWorkspaceKey(clusterName logicalcluster.Name) string {
	return clusterName.String()
}

// GetTMCAPIBindingKey generates a unique key for TMC APIBinding identification
func GetTMCAPIBindingKey(binding *apisv1alpha2.APIBinding) (string, error) {
	if !isTMCAPIBinding(binding) {
		return "", fmt.Errorf("APIBinding %s/%s is not a TMC APIBinding", 
			logicalcluster.From(binding), binding.Name)
	}

	clusterName := logicalcluster.From(binding)
	return client.ToClusterAwareKey(clusterName.Path(), binding.Name), nil
}

// IsTMCClusterRegistrationBinding determines if an APIBinding is for ClusterRegistration API
func IsTMCClusterRegistrationBinding(binding *apisv1alpha2.APIBinding) bool {
	if binding.Spec.Reference.Export == nil {
		return false
	}
	return binding.Spec.Reference.Export.Name == ClusterRegistrationAPIExport
}

// IsTMCWorkloadPlacementBinding determines if an APIBinding is for WorkloadPlacement API
func IsTMCWorkloadPlacementBinding(binding *apisv1alpha2.APIBinding) bool {
	if binding.Spec.Reference.Export == nil {
		return false
	}
	return binding.Spec.Reference.Export.Name == WorkloadPlacementAPIExport
}

// GetTMCAPIBindingType returns the type of TMC APIBinding
func GetTMCAPIBindingType(binding *apisv1alpha2.APIBinding) string {
	if binding.Spec.Reference.Export == nil {
		return "unknown"
	}

	exportName := binding.Spec.Reference.Export.Name
	switch exportName {
	case ClusterRegistrationAPIExport:
		return "cluster-registration"
	case WorkloadPlacementAPIExport:
		return "workload-placement"
	default:
		if len(exportName) > len(TMCAPIExportPrefix) && exportName[:len(TMCAPIExportPrefix)] == TMCAPIExportPrefix {
			return "tmc-generic"
		}
		return "non-tmc"
	}
}

// FilterTMCAPIBindings filters a list of APIBindings to only include TMC-related ones
func FilterTMCAPIBindings(bindings []*apisv1alpha2.APIBinding) []*apisv1alpha2.APIBinding {
	var tmcBindings []*apisv1alpha2.APIBinding
	for _, binding := range bindings {
		if isTMCAPIBinding(binding) {
			tmcBindings = append(tmcBindings, binding)
		}
	}
	return tmcBindings
}

// GroupTMCAPIBindingsByType groups TMC APIBindings by their type
func GroupTMCAPIBindingsByType(bindings []*apisv1alpha2.APIBinding) map[string][]*apisv1alpha2.APIBinding {
	groups := make(map[string][]*apisv1alpha2.APIBinding)
	
	for _, binding := range bindings {
		if !isTMCAPIBinding(binding) {
			continue
		}
		
		bindingType := GetTMCAPIBindingType(binding)
		groups[bindingType] = append(groups[bindingType], binding)
	}
	
	return groups
}

// ValidateTMCAPIBinding performs basic validation on TMC APIBinding structure
func ValidateTMCAPIBinding(binding *apisv1alpha2.APIBinding) error {
	if !isTMCAPIBinding(binding) {
		return fmt.Errorf("APIBinding %s/%s is not a TMC APIBinding", 
			logicalcluster.From(binding), binding.Name)
	}

	if binding.Spec.Reference.Export == nil {
		return fmt.Errorf("TMC APIBinding %s/%s has no export reference", 
			logicalcluster.From(binding), binding.Name)
	}

	exportName := binding.Spec.Reference.Export.Name
	if exportName == "" {
		return fmt.Errorf("TMC APIBinding %s/%s has empty export name", 
			logicalcluster.From(binding), binding.Name)
	}

	return nil
}