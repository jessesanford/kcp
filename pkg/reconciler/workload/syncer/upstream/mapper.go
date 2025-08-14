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

package upstream

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ResourceMapper handles resource type and namespace mapping between physical and logical clusters.
// It provides configurable mapping rules for different resource types and namespace transformations
// to ensure proper isolation and organization within KCP workspaces.
type ResourceMapper struct {
	rules           []MappingRule
	namespaceMapper NamespaceMapper
}

// MappingRule defines a transformation rule for a specific resource type
type MappingRule struct {
	FromGVR   schema.GroupVersionResource
	ToGVR     schema.GroupVersionResource
	Transform func(*unstructured.Unstructured) error
	Condition func(*unstructured.Unstructured) bool
}

// NamespaceMapper interface defines methods for namespace transformation
type NamespaceMapper interface {
	// ToLogical transforms a physical namespace to a logical namespace
	ToLogical(physical, syncTargetName string) string
	
	// ToPhysical transforms a logical namespace back to a physical namespace
	ToPhysical(logical, syncTargetName string) (string, error)
	
	// IsLogicalNamespace checks if a namespace follows the logical naming pattern
	IsLogicalNamespace(namespace, syncTargetName string) bool
}

// DefaultNamespaceMapper implements the default namespace mapping strategy
type DefaultNamespaceMapper struct {
	syncTargetName string
	prefix         string
	
	// Regular expression to match logical namespace pattern
	logicalPattern *regexp.Regexp
}

// NewResourceMapper creates a new ResourceMapper with default mapping rules
func NewResourceMapper() *ResourceMapper {
	return &ResourceMapper{
		rules: defaultMappingRules(),
	}
}

// NewDefaultNamespaceMapper creates a new DefaultNamespaceMapper for the given sync target
func NewDefaultNamespaceMapper(syncTargetName string) NamespaceMapper {
	prefix := fmt.Sprintf("kcp-%s-", syncTargetName)
	pattern := fmt.Sprintf("^kcp-%s-(.+)$", regexp.QuoteMeta(syncTargetName))
	
	return &DefaultNamespaceMapper{
		syncTargetName: syncTargetName,
		prefix:         prefix,
		logicalPattern: regexp.MustCompile(pattern),
	}
}

// MapResource maps a resource type to its target type based on configured rules
func (m *ResourceMapper) MapResource(gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (schema.GroupVersionResource, bool) {
	for _, rule := range m.rules {
		if rule.FromGVR == gvr {
			// Check if condition is satisfied (if specified)
			if rule.Condition != nil && !rule.Condition(obj) {
				continue
			}
			
			// Apply transformation if specified
			if rule.Transform != nil {
				if err := rule.Transform(obj); err != nil {
					klog.ErrorS(err, "Failed to apply mapping transformation", 
						"fromGVR", rule.FromGVR, "toGVR", rule.ToGVR)
					continue
				}
			}
			
			return rule.ToGVR, true
		}
	}
	
	// No mapping rule found, return original GVR
	return gvr, false
}

// AddMappingRule adds a new mapping rule to the mapper
func (m *ResourceMapper) AddMappingRule(rule MappingRule) {
	m.rules = append(m.rules, rule)
}

// RemoveMappingRule removes mapping rules for a specific GVR
func (m *ResourceMapper) RemoveMappingRule(gvr schema.GroupVersionResource) {
	filtered := make([]MappingRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.FromGVR != gvr {
			filtered = append(filtered, rule)
		}
	}
	m.rules = filtered
}

// SetNamespaceMapper sets a custom namespace mapper
func (m *ResourceMapper) SetNamespaceMapper(mapper NamespaceMapper) {
	m.namespaceMapper = mapper
}

// GetNamespaceMapper returns the current namespace mapper
func (m *ResourceMapper) GetNamespaceMapper() NamespaceMapper {
	return m.namespaceMapper
}

// ToLogical transforms a physical namespace to a logical namespace using the prefix pattern
func (m *DefaultNamespaceMapper) ToLogical(physical, syncTargetName string) string {
	// Handle special system namespaces
	switch physical {
	case "kube-system", "kube-public", "kube-node-lease":
		return fmt.Sprintf("kcp-%s-system-%s", syncTargetName, strings.TrimPrefix(physical, "kube-"))
	case "default":
		return fmt.Sprintf("kcp-%s-default", syncTargetName)
	}
	
	// For regular namespaces, add the prefix
	return m.prefix + physical
}

// ToPhysical transforms a logical namespace back to a physical namespace
func (m *DefaultNamespaceMapper) ToPhysical(logical, syncTargetName string) (string, error) {
	// Handle special system namespaces
	systemPrefix := fmt.Sprintf("kcp-%s-system-", syncTargetName)
	if strings.HasPrefix(logical, systemPrefix) {
		systemSuffix := strings.TrimPrefix(logical, systemPrefix)
		return "kube-" + systemSuffix, nil
	}
	
	// Handle default namespace
	defaultLogical := fmt.Sprintf("kcp-%s-default", syncTargetName)
	if logical == defaultLogical {
		return "default", nil
	}
	
	// Handle regular namespaces
	if !strings.HasPrefix(logical, m.prefix) {
		return "", fmt.Errorf("namespace %s does not follow expected logical pattern for sync target %s", 
			logical, syncTargetName)
	}
	
	physical := strings.TrimPrefix(logical, m.prefix)
	if physical == "" {
		return "", fmt.Errorf("empty physical namespace after removing prefix from %s", logical)
	}
	
	return physical, nil
}

// IsLogicalNamespace checks if a namespace follows the logical naming pattern
func (m *DefaultNamespaceMapper) IsLogicalNamespace(namespace, syncTargetName string) bool {
	return m.logicalPattern.MatchString(namespace)
}

// defaultMappingRules returns the default set of resource mapping rules
func defaultMappingRules() []MappingRule {
	return []MappingRule{
		// Pod mapping with status filtering
		{
			FromGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			ToGVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Transform: func(obj *unstructured.Unstructured) error {
				// Only sync pods that are in certain phases
				phase, found, err := unstructured.NestedString(obj.Object, "status", "phase")
				if err != nil {
					return err
				}
				if found && (phase == "Failed" || phase == "Unknown") {
					// Skip failed or unknown pods - they're not useful for upstream sync
					return fmt.Errorf("skipping pod in phase %s", phase)
				}
				return nil
			},
			Condition: func(obj *unstructured.Unstructured) bool {
				// Only sync non-system pods
				name := obj.GetName()
				return !strings.HasPrefix(name, "kube-") && !strings.HasPrefix(name, "coredns")
			},
		},
		
		// Service mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			ToGVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			Transform: func(obj *unstructured.Unstructured) error {
				// Clean up service-specific cluster fields
				unstructured.RemoveNestedField(obj.Object, "spec", "clusterIP")
				unstructured.RemoveNestedField(obj.Object, "spec", "clusterIPs")
				return nil
			},
		},
		
		// ConfigMap mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			ToGVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Condition: func(obj *unstructured.Unstructured) bool {
				// Skip system configmaps
				name := obj.GetName()
				return !strings.HasPrefix(name, "kube-") && name != "cluster-info"
			},
		},
		
		// Deployment mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			ToGVR:   schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		
		// ReplicaSet mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
			ToGVR:   schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		},
		
		// StatefulSet mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
			ToGVR:   schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		},
		
		// DaemonSet mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
			ToGVR:   schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		},
		
		// Node mapping with filtering
		{
			FromGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			ToGVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			Transform: func(obj *unstructured.Unstructured) error {
				// Clean up node-specific system information
				unstructured.RemoveNestedField(obj.Object, "status", "nodeInfo", "machineID")
				unstructured.RemoveNestedField(obj.Object, "status", "nodeInfo", "systemUUID")
				unstructured.RemoveNestedField(obj.Object, "status", "addresses")
				return nil
			},
		},
		
		// PersistentVolumeClaim mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
			ToGVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		},
		
		// Ingress mapping
		{
			FromGVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
			ToGVR:   schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
			Transform: func(obj *unstructured.Unstructured) error {
				// Clean up ingress status
				unstructured.RemoveNestedField(obj.Object, "status", "loadBalancer")
				return nil
			},
		},
	}
}

// MappingStats provides statistics about resource mapping operations
type MappingStats struct {
	TotalMapped    int64
	MappingsByGVR  map[schema.GroupVersionResource]int64
	FailedMappings int64
}

// NewMappingStats creates a new MappingStats instance
func NewMappingStats() *MappingStats {
	return &MappingStats{
		MappingsByGVR: make(map[schema.GroupVersionResource]int64),
	}
}

// RecordMapping records a successful mapping operation
func (s *MappingStats) RecordMapping(gvr schema.GroupVersionResource) {
	s.TotalMapped++
	s.MappingsByGVR[gvr]++
}

// RecordFailure records a failed mapping operation
func (s *MappingStats) RecordFailure() {
	s.FailedMappings++
}

// GetStats returns a summary of mapping statistics
func (s *MappingStats) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_mapped":     s.TotalMapped,
		"failed_mappings":  s.FailedMappings,
		"mappings_by_gvr":  s.MappingsByGVR,
	}
}

// ValidateNamespace validates that a namespace name is valid for the target cluster
func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	
	// Check length limits (Kubernetes limit is 253 characters)
	if len(namespace) > 253 {
		return fmt.Errorf("namespace name too long: %d characters (max 253)", len(namespace))
	}
	
	// Check for invalid characters - must be DNS label compatible
	if !isValidDNSName(namespace) {
		return fmt.Errorf("namespace name contains invalid characters: %s", namespace)
	}
	
	return nil
}

// isValidDNSName checks if a string is a valid DNS name
func isValidDNSName(name string) bool {
	// Simple DNS name validation - lowercase letters, numbers, and hyphens
	// Cannot start or end with hyphen
	if len(name) == 0 {
		return false
	}
	
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return false
	}
	
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}
	
	return true
}