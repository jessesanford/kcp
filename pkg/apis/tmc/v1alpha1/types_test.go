/*
Copyright 2025 The KCP Authors.

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestClusterRegistrationValidation(t *testing.T) {
	tests := map[string]struct {
		cluster       *ClusterRegistration
		expectValid   bool
		expectedField string
	}{
		"valid cluster registration": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: ClusterEndpoint{
						URL: "https://cluster.example.com",
					},
				},
			},
			expectValid: true,
		},
		"missing location": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: ClusterRegistrationSpec{
					ClusterEndpoint: ClusterEndpoint{
						URL: "https://cluster.example.com",
					},
				},
			},
			expectValid:   false,
			expectedField: "spec.location",
		},
		"missing cluster URL": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: ClusterEndpoint{
						URL: "",
					},
				},
			},
			expectValid:   false,
			expectedField: "spec.clusterEndpoint.url",
		},
		"invalid taint effect": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: ClusterEndpoint{
						URL: "https://cluster.example.com",
					},
					Taints: []ClusterTaint{
						{
							Key:    "test-taint",
							Value:  "test-value",
							Effect: "InvalidEffect",
						},
					},
				},
			},
			expectValid:   false,
			expectedField: "spec.taints[0].effect",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := ValidateClusterRegistration(tc.cluster)
			if tc.expectValid && len(errs) > 0 {
				t.Errorf("expected valid cluster registration, got errors: %v", errs)
			}
			if !tc.expectValid && len(errs) == 0 {
				t.Errorf("expected validation errors, got none")
			}
			if !tc.expectValid && len(errs) > 0 {
				found := false
				for _, err := range errs {
					if err.Field == tc.expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %s, got errors: %v", tc.expectedField, errs)
				}
			}
		})
	}
}

func TestWorkloadPlacementValidation(t *testing.T) {
	tests := map[string]struct {
		placement     *WorkloadPlacement
		expectValid   bool
		expectedField string
	}{
		"valid workload placement": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: WorkloadPlacementSpec{
					WorkspaceSelector: WorkspaceSelector{
						Name: "test-workspace",
					},
					ResourceSelector: ResourceSelector{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					PlacementPolicy: PlacementPolicy{
						Strategy: SpreadPlacementStrategy,
						ClusterSelector: ClusterSelector{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			expectValid: true,
		},
		"missing workspace selector": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: WorkloadPlacementSpec{
					ResourceSelector: ResourceSelector{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					PlacementPolicy: PlacementPolicy{
						Strategy: SpreadPlacementStrategy,
						ClusterSelector: ClusterSelector{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			expectValid:   false,
			expectedField: "spec.workspaceSelector",
		},
		"invalid placement strategy": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: WorkloadPlacementSpec{
					WorkspaceSelector: WorkspaceSelector{
						Name: "test-workspace",
					},
					ResourceSelector: ResourceSelector{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					PlacementPolicy: PlacementPolicy{
						Strategy: "InvalidStrategy",
						ClusterSelector: ClusterSelector{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			expectValid:   false,
			expectedField: "spec.placementPolicy.strategy",
		},
		"invalid priority": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-placement",
				},
				Spec: WorkloadPlacementSpec{
					WorkspaceSelector: WorkspaceSelector{
						Name: "test-workspace",
					},
					ResourceSelector: ResourceSelector{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					PlacementPolicy: PlacementPolicy{
						Strategy: SpreadPlacementStrategy,
						ClusterSelector: ClusterSelector{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
					Priority: 1500, // Above maximum
				},
			},
			expectValid:   false,
			expectedField: "spec.priority",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := ValidateWorkloadPlacement(tc.placement)
			if tc.expectValid && len(errs) > 0 {
				t.Errorf("expected valid workload placement, got errors: %v", errs)
			}
			if !tc.expectValid && len(errs) == 0 {
				t.Errorf("expected validation errors, got none")
			}
			if !tc.expectValid && len(errs) > 0 {
				found := false
				for _, err := range errs {
					if err.Field == tc.expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %s, got errors: %v", tc.expectedField, errs)
				}
			}
		})
	}
}

func TestDeepCopyFunctionality(t *testing.T) {
	original := &ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Labels: map[string]string{
				"location": "us-west-2",
			},
		},
		Spec: ClusterRegistrationSpec{
			Location: "us-west-2",
			ClusterEndpoint: ClusterEndpoint{
				URL: "https://cluster.example.com",
			},
			Capabilities: []ClusterCapability{
				{
					Type:      "storage",
					Available: true,
					Attributes: map[string]string{
						"type": "ssd",
					},
				},
			},
		},
		Status: ClusterRegistrationStatus{
			Phase: ClusterRegistrationPhaseReady,
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   ClusterRegistrationReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	copy := original.DeepCopy()

	// Modify original to test independence
	original.Spec.Location = "us-east-1"
	original.Spec.Capabilities[0].Attributes["type"] = "hdd"
	original.Status.Phase = ClusterRegistrationPhaseFailed

	// Check that copy remains unchanged
	if copy.Spec.Location != "us-west-2" {
		t.Errorf("expected copy location to be us-west-2, got %s", copy.Spec.Location)
	}
	if copy.Spec.Capabilities[0].Attributes["type"] != "ssd" {
		t.Errorf("expected copy capability type to be ssd, got %s", copy.Spec.Capabilities[0].Attributes["type"])
	}
	if copy.Status.Phase != ClusterRegistrationPhaseReady {
		t.Errorf("expected copy phase to be Ready, got %s", copy.Status.Phase)
	}
}

func TestConditionImplementations(t *testing.T) {
	// Test ClusterRegistration conditions
	cr := &ClusterRegistration{}
	conditions := conditionsv1alpha1.Conditions{
		{
			Type:   ClusterRegistrationReady,
			Status: corev1.ConditionTrue,
		},
	}

	cr.SetConditions(conditions)
	retrievedConditions := cr.GetConditions()

	if len(retrievedConditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(retrievedConditions))
	}
	if retrievedConditions[0].Type != ClusterRegistrationReady {
		t.Errorf("expected condition type %s, got %s", ClusterRegistrationReady, retrievedConditions[0].Type)
	}

	// Test WorkloadPlacement conditions
	wp := &WorkloadPlacement{}
	wp.SetConditions(conditions)
	retrievedConditions = wp.GetConditions()

	if len(retrievedConditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(retrievedConditions))
	}
}

func TestAPIRegistration(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := Install(scheme); err != nil {
		t.Fatalf("failed to install TMC API: %v", err)
	}

	// Test that types are registered
	gvk := SchemeGroupVersion.WithKind("ClusterRegistration")
	obj, err := scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create ClusterRegistration from scheme: %v", err)
	}
	if _, ok := obj.(*ClusterRegistration); !ok {
		t.Errorf("expected *ClusterRegistration, got %T", obj)
	}

	gvk = SchemeGroupVersion.WithKind("WorkloadPlacement")
	obj, err = scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create WorkloadPlacement from scheme: %v", err)
	}
	if _, ok := obj.(*WorkloadPlacement); !ok {
		t.Errorf("expected *WorkloadPlacement, got %T", obj)
	}
}

// ValidateClusterRegistration provides validation for ClusterRegistration
func ValidateClusterRegistration(cr *ClusterRegistration) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	if cr.Spec.Location == "" {
		errs = append(errs, field.Required(specPath.Child("location"), "location is required"))
	}

	if cr.Spec.ClusterEndpoint.URL == "" {
		errs = append(errs, field.Required(specPath.Child("clusterEndpoint", "url"), "cluster URL is required"))
	}

	for i, taint := range cr.Spec.Taints {
		taintPath := specPath.Child("taints").Index(i)
		if taint.Effect != TaintEffectNoSchedule && taint.Effect != TaintEffectPreferNoSchedule && taint.Effect != TaintEffectNoExecute {
			errs = append(errs, field.Invalid(taintPath.Child("effect"), taint.Effect, "must be one of NoSchedule, PreferNoSchedule, or NoExecute"))
		}
	}

	return errs
}

// ValidateWorkloadPlacement provides validation for WorkloadPlacement
func ValidateWorkloadPlacement(wp *WorkloadPlacement) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// Validate workspace selector
	if wp.Spec.WorkspaceSelector.Name == "" && wp.Spec.WorkspaceSelector.LabelSelector == nil && wp.Spec.WorkspaceSelector.Path == "" {
		errs = append(errs, field.Required(specPath.Child("workspaceSelector"), "at least one workspace selector method is required"))
	}

	// Validate placement strategy
	validStrategies := []PlacementStrategy{SpreadPlacementStrategy, BinpackPlacementStrategy, AffinityPlacementStrategy, CustomPlacementStrategy}
	valid := false
	for _, strategy := range validStrategies {
		if wp.Spec.PlacementPolicy.Strategy == strategy {
			valid = true
			break
		}
	}
	if !valid {
		errs = append(errs, field.Invalid(specPath.Child("placementPolicy", "strategy"), wp.Spec.PlacementPolicy.Strategy, "must be one of Spread, Binpack, Affinity, or Custom"))
	}

	// Validate priority
	if wp.Spec.Priority < 0 || wp.Spec.Priority > 1000 {
		errs = append(errs, field.Invalid(specPath.Child("priority"), wp.Spec.Priority, "must be between 0 and 1000"))
	}

	return errs
}