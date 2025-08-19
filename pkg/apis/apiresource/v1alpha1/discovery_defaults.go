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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulting functions to the scheme
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&APIDiscovery{}, func(obj interface{}) {
		SetDefaults_APIDiscovery(obj.(*APIDiscovery))
	})
	scheme.AddTypeDefaultingFunc(&NegotiatedAPIResource{}, func(obj interface{}) {
		SetDefaults_NegotiatedAPIResource(obj.(*NegotiatedAPIResource))
	})
	return nil
}

// SetDefaults_APIDiscovery sets defaults for APIDiscovery
func SetDefaults_APIDiscovery(obj *APIDiscovery) {
	// Default refresh interval
	if obj.Spec.RefreshInterval == nil {
		obj.Spec.RefreshInterval = &metav1.Duration{Duration: 5 * time.Minute}
	}

	// Default discovery policy
	if obj.Spec.DiscoveryPolicy == nil {
		obj.Spec.DiscoveryPolicy = &DiscoveryPolicy{
			Scope:             DiscoveryScopeFull,
			IncludeDeprecated: false,
			IncludeBeta:       true,
			IncludeAlpha:      false,
		}
	}

	// Set default scope if not specified
	if obj.Spec.DiscoveryPolicy.Scope == "" {
		obj.Spec.DiscoveryPolicy.Scope = DiscoveryScopeFull
	}
}

// SetDefaults_NegotiatedAPIResource sets defaults for NegotiatedAPIResource
func SetDefaults_NegotiatedAPIResource(obj *NegotiatedAPIResource) {
	// Default requirements
	if obj.Spec.Requirements == nil {
		obj.Spec.Requirements = &APIRequirements{
			RequiredVerbs: []string{"get", "list", "watch"},
		}
	}

	// Ensure basic verbs are included if verbs are specified
	if len(obj.Spec.Requirements.RequiredVerbs) > 0 {
		basicVerbs := []string{"get", "list"}
		verbMap := make(map[string]bool)

		for _, verb := range obj.Spec.Requirements.RequiredVerbs {
			verbMap[verb] = true
		}

		for _, basicVerb := range basicVerbs {
			if !verbMap[basicVerb] {
				obj.Spec.Requirements.RequiredVerbs = append(obj.Spec.Requirements.RequiredVerbs, basicVerb)
			}
		}
	}
}
