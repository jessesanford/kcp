/*
Copyright The KCP Authors.

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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/util/conditions"
)

const (
	// DiscoveryConditionReady indicates discovery is ready
	DiscoveryConditionReady = "Ready"

	// DiscoveryConditionComplete indicates discovery completed
	DiscoveryConditionComplete = "Complete"

	// NegotiationConditionReady indicates negotiation is ready
	NegotiationConditionReady = "Ready"

	// NegotiationConditionCompatible indicates negotiation found compatible API
	NegotiationConditionCompatible = "Compatible"
)

// IsDiscovered returns true if discovery is complete
func (d *APIDiscovery) IsDiscovered() bool {
	return d.Status.Phase == DiscoveryPhaseDiscovered
}

// IsComplete returns true if discovery completed successfully
func (d *APIDiscovery) IsComplete() bool {
	return conditions.IsTrue(d, DiscoveryConditionComplete)
}

// SetCondition sets a condition
func (d *APIDiscovery) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(d, condition)
}

// FindAPIGroup finds a specific API group in discoveries
func (d *APIDiscovery) FindAPIGroup(groupName string) *DiscoveredAPIGroup {
	for i := range d.Status.APIGroups {
		if d.Status.APIGroups[i].Name == groupName {
			return &d.Status.APIGroups[i]
		}
	}
	return nil
}

// FindResource finds a specific resource in discoveries
func (d *APIDiscovery) FindResource(group, version, kind string) *DiscoveredResource {
	apiGroup := d.FindAPIGroup(group)
	if apiGroup == nil {
		return nil
	}

	for _, v := range apiGroup.Versions {
		if v.Version == version {
			for i, r := range v.Resources {
				if r.Kind == kind {
					return &v.Resources[i]
				}
			}
		}
	}
	return nil
}

// SupportsResource checks if a resource is supported
func (d *APIDiscovery) SupportsResource(apiVersion, kind string) bool {
	parts := strings.Split(apiVersion, "/")
	var group, version string

	if len(parts) == 1 {
		// Core API (v1)
		group = ""
		version = parts[0]
	} else {
		// Named group (apps/v1)
		group = parts[0]
		version = parts[1]
	}

	return d.FindResource(group, version, kind) != nil
}

// GetEnabledFeatures returns list of enabled features
func (d *APIDiscovery) GetEnabledFeatures() []string {
	features := make([]string, 0, len(d.Status.Features))
	for _, f := range d.Status.Features {
		if f.Enabled {
			features = append(features, f.Name)
		}
	}
	return features
}

// IsFeatureEnabled checks if a feature is enabled
func (d *APIDiscovery) IsFeatureEnabled(featureName string) bool {
	for _, f := range d.Status.Features {
		if f.Name == featureName {
			return f.Enabled
		}
	}
	return false
}

// ShouldRefresh checks if discovery should be refreshed
func (d *APIDiscovery) ShouldRefresh(now metav1.Time) bool {
	if d.Spec.Paused {
		return false
	}

	if d.Status.LastDiscoveryTime == nil {
		return true
	}

	interval := d.Spec.RefreshInterval
	if interval == nil {
		return false // No automatic refresh
	}

	nextRefresh := d.Status.LastDiscoveryTime.Add(interval.Duration)
	return now.After(nextRefresh)
}

// NegotiatedAPIResource helper methods

// IsNegotiated returns true if negotiation is complete
func (n *NegotiatedAPIResource) IsNegotiated() bool {
	return n.Status.Phase == NegotiationPhaseCompatible
}

// IsCompatible returns true if the API is compatible
func (n *NegotiatedAPIResource) IsCompatible() bool {
	return conditions.IsTrue(n, NegotiationConditionCompatible)
}

// SetCondition sets a condition on the negotiated resource
func (n *NegotiatedAPIResource) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(n, condition)
}

// GetCompatibilityScore returns the compatibility score or 0 if not set
func (n *NegotiatedAPIResource) GetCompatibilityScore() int32 {
	if n.Status.CompatibilityScore == nil {
		return 0
	}
	return *n.Status.CompatibilityScore
}

// HasRequiredVerb checks if a required verb is supported
func (n *NegotiatedAPIResource) HasRequiredVerb(verb string) bool {
	for _, supported := range n.Status.SupportedVerbs {
		if supported == verb {
			return true
		}
	}
	return false
}

// GetAPIVersionString returns the API version in group/version format
func (a *APIResourceRef) GetAPIVersionString() string {
	if a.Group == "" {
		return a.Version
	}
	return a.Group + "/" + a.Version
}

// FilterResource checks if a resource passes the given filter
func (f *ResourceFilter) FilterResource(group, version, kind string) bool {
	matches := true

	// Check group filter
	if f.Group != "" && f.Group != group {
		matches = false
	}

	// Check version filter
	if f.Version != "" && f.Version != version {
		matches = false
	}

	// Check kind filter
	if f.Kind != "" && f.Kind != kind {
		matches = false
	}

	// Apply action based on match result
	if f.Action == FilterActionInclude {
		return matches
	} else {
		return !matches
	}
}

// ShouldIncludeResource checks if a resource should be included based on discovery policy
func (p *DiscoveryPolicy) ShouldIncludeResource(group, version, kind string) bool {
	// Check version stability
	isAlpha := strings.Contains(version, "alpha")
	isBeta := strings.Contains(version, "beta")

	// Apply stability filters
	if isAlpha && !p.IncludeAlpha {
		return false
	}

	if isBeta && !p.IncludeBeta {
		return false
	}

	// If no filters are defined, include by default
	if len(p.ResourceFilters) == 0 {
		return true
	}

	// Apply resource filters
	for _, filter := range p.ResourceFilters {
		if filter.FilterResource(group, version, kind) {
			return filter.Action == FilterActionInclude
		}
	}

	// Default behavior when no filter matches
	return true
}
