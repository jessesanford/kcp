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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// Standard condition types for NegotiatedAPIResource
const (
	// NegotiationConditionCompatible indicates whether the API is compatible across all targets
	NegotiationConditionCompatible = "Compatible"

	// NegotiationConditionNegotiated indicates whether negotiation has completed
	NegotiationConditionNegotiated = "Negotiated"

	// NegotiationConditionPublishable indicates whether the API can be published
	NegotiationConditionPublishable = "Publishable"
)

// Standard condition reasons
const (
	// NegotiationReasonCompatible indicates successful compatibility across all targets
	NegotiationReasonCompatible = "Compatible"

	// NegotiationReasonIncompatible indicates incompatibility found
	NegotiationReasonIncompatible = "Incompatible"

	// NegotiationReasonNegotiating indicates negotiation is in progress
	NegotiationReasonNegotiating = "Negotiating"

	// NegotiationReasonNoTargets indicates no sync targets available for negotiation
	NegotiationReasonNoTargets = "NoTargets"

	// NegotiationReasonSchemaConflict indicates schema conflicts between targets
	NegotiationReasonSchemaConflict = "SchemaConflict"
)

// IsCompatible returns true if the API is compatible across all sync targets.
func (n *NegotiatedAPIResource) IsCompatible() bool {
	return n.Status.Phase == NegotiationCompatible
}

// IsNegotiated returns true if negotiation has completed (successfully or unsuccessfully).
func (n *NegotiatedAPIResource) IsNegotiated() bool {
	return conditions.IsTrue(n, NegotiationConditionNegotiated)
}

// IsPublishable returns true if the API can be published to sync targets.
func (n *NegotiatedAPIResource) IsPublishable() bool {
	return conditions.IsTrue(n, NegotiationConditionPublishable) && n.Spec.Publish
}

// IsInProgress returns true if negotiation is currently in progress.
func (n *NegotiatedAPIResource) IsInProgress() bool {
	return n.Status.Phase == NegotiationNegotiating
}

// SetCondition sets a condition on the NegotiatedAPIResource.
// This is a convenience method for managing conditions consistently.
func (n *NegotiatedAPIResource) SetCondition(condition conditionsv1alpha1.Condition) {
	conditions.Set(n, condition)
}

// SetCompatibleCondition sets the Compatible condition with appropriate reason and message.
func (n *NegotiatedAPIResource) SetCompatibleCondition(compatible bool, reason, message string) {
	status := metav1.ConditionTrue
	if !compatible {
		status = metav1.ConditionFalse
	}

	condition := conditionsv1alpha1.Condition{
		Type:               NegotiationConditionCompatible,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	n.SetCondition(condition)
}

// SetNegotiatedCondition sets the Negotiated condition to indicate completion.
func (n *NegotiatedAPIResource) SetNegotiatedCondition(success bool, reason, message string) {
	status := metav1.ConditionTrue
	if !success {
		status = metav1.ConditionFalse
	}

	condition := conditionsv1alpha1.Condition{
		Type:               NegotiationConditionNegotiated,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	n.SetCondition(condition)
}

// SetPublishableCondition sets the Publishable condition based on negotiation results.
func (n *NegotiatedAPIResource) SetPublishableCondition(publishable bool, reason, message string) {
	status := metav1.ConditionTrue
	if !publishable {
		status = metav1.ConditionFalse
	}

	condition := conditionsv1alpha1.Condition{
		Type:               NegotiationConditionPublishable,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	n.SetCondition(condition)
}

// GetCompatibleLocationNames returns the names of all compatible sync target locations.
func (n *NegotiatedAPIResource) GetCompatibleLocationNames() []string {
	names := make([]string, 0, len(n.Status.CompatibleLocations))
	for _, loc := range n.Status.CompatibleLocations {
		names = append(names, loc.Name)
	}
	return names
}

// GetIncompatibleLocationNames returns the names of all incompatible sync target locations.
func (n *NegotiatedAPIResource) GetIncompatibleLocationNames() []string {
	names := make([]string, 0, len(n.Status.IncompatibleLocations))
	for _, loc := range n.Status.IncompatibleLocations {
		names = append(names, loc.Name)
	}
	return names
}

// HasLocation checks if a specific location name exists in either compatible or incompatible lists.
func (n *NegotiatedAPIResource) HasLocation(locationName string) bool {
	// Check compatible locations
	for _, loc := range n.Status.CompatibleLocations {
		if loc.Name == locationName {
			return true
		}
	}

	// Check incompatible locations
	for _, loc := range n.Status.IncompatibleLocations {
		if loc.Name == locationName {
			return true
		}
	}

	return false
}

// IsLocationCompatible checks if a specific location is in the compatible list.
func (n *NegotiatedAPIResource) IsLocationCompatible(locationName string) bool {
	for _, loc := range n.Status.CompatibleLocations {
		if loc.Name == locationName {
			return true
		}
	}
	return false
}

// GetLocationConstraints returns constraints for a specific compatible location.
func (n *NegotiatedAPIResource) GetLocationConstraints(locationName string) []LocationConstraint {
	for _, loc := range n.Status.CompatibleLocations {
		if loc.Name == locationName {
			return loc.Constraints
		}
	}
	return nil
}

// AddCompatibleLocation adds a location to the compatible list, removing it from incompatible if present.
func (n *NegotiatedAPIResource) AddCompatibleLocation(location CompatibleLocation) {
	// Remove from incompatible list if present
	n.removeIncompatibleLocation(location.Name)

	// Update or add to compatible list
	found := false
	for i, loc := range n.Status.CompatibleLocations {
		if loc.Name == location.Name {
			n.Status.CompatibleLocations[i] = location
			found = true
			break
		}
	}

	if !found {
		n.Status.CompatibleLocations = append(n.Status.CompatibleLocations, location)
	}
}

// AddIncompatibleLocation adds a location to the incompatible list, removing it from compatible if present.
func (n *NegotiatedAPIResource) AddIncompatibleLocation(location IncompatibleLocation) {
	// Remove from compatible list if present
	n.removeCompatibleLocation(location.Name)

	// Update or add to incompatible list
	found := false
	for i, loc := range n.Status.IncompatibleLocations {
		if loc.Name == location.Name {
			n.Status.IncompatibleLocations[i] = location
			found = true
			break
		}
	}

	if !found {
		n.Status.IncompatibleLocations = append(n.Status.IncompatibleLocations, location)
	}
}

// removeCompatibleLocation removes a location from the compatible list.
func (n *NegotiatedAPIResource) removeCompatibleLocation(locationName string) {
	for i, loc := range n.Status.CompatibleLocations {
		if loc.Name == locationName {
			n.Status.CompatibleLocations = append(n.Status.CompatibleLocations[:i], n.Status.CompatibleLocations[i+1:]...)
			break
		}
	}
}

// removeIncompatibleLocation removes a location from the incompatible list.
func (n *NegotiatedAPIResource) removeIncompatibleLocation(locationName string) {
	for i, loc := range n.Status.IncompatibleLocations {
		if loc.Name == locationName {
			n.Status.IncompatibleLocations = append(n.Status.IncompatibleLocations[:i], n.Status.IncompatibleLocations[i+1:]...)
			break
		}
	}
}

// GetResourceByName finds a specific resource negotiation by name.
func (n *NegotiatedAPIResource) GetResourceByName(resourceName string) *ResourceNegotiation {
	for i := range n.Spec.Resources {
		if n.Spec.Resources[i].Resource == resourceName {
			return &n.Spec.Resources[i]
		}
	}
	return nil
}

// HasResource checks if a specific resource is being negotiated.
func (n *NegotiatedAPIResource) HasResource(resourceName string) bool {
	return n.GetResourceByName(resourceName) != nil
}

// UpdateNegotiationTimestamp updates the last negotiation time to now.
func (n *NegotiatedAPIResource) UpdateNegotiationTimestamp() {
	now := metav1.Now()
	n.Status.LastNegotiationTime = &now
}
