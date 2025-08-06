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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestSessionAffinityPolicy_Validation(t *testing.T) {
	validMetadata := metav1.ObjectMeta{
		Name:      "test-policy",
		Namespace: "default",
	}

	validWorkloadSelector := WorkloadSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "test"},
		},
	}

	validClusterSelector := ClusterSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	}

	validStickinessPolicy := StickinessPolicy{
		Type:     StickinessTypeSoft,
		Duration: metav1.Duration{Duration: time.Hour},
	}

	tests := map[string]struct {
		policy       SessionAffinityPolicy
		wantValidErr bool
	}{
		"valid minimal policy": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
				},
			},
			wantValidErr: false,
		},
		"valid policy with full configuration": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypePersistentSession,
					StickinessPolicy: validStickinessPolicy,
					AffinityRules: []AffinityRule{
						{
							Name: "test-rule",
							Type: AffinityRuleTypeConstraint,
							Constraint: &AffinityConstraint{
								RequiredClusterLabels: map[string]string{"zone": "us-west"},
								MaxLatency:             &metav1.Duration{Duration: time.Millisecond * 100},
							},
							Weight: 80,
						},
					},
					BindingPersistence: &BindingPersistenceConfig{
						Enabled:     true,
						StorageType: BindingStorageTypeConfigMap,
						TTL:         metav1.Duration{Duration: time.Hour * 24},
					},
					FailoverPolicy: &AffinityFailoverPolicy{
						Strategy:               FailoverStrategyDelayed,
						DelayBeforeFailover:    metav1.Duration{Duration: time.Minute * 5},
						MaxFailoverAttempts:    3,
						BackoffMultiplier:      2.0,
						AlternativeClusterSelector: &ClusterSelector{
							LocationSelector: []string{"us-east"},
						},
					},
					Weight: 75,
				},
			},
			wantValidErr: false,
		},
		"missing workload selector": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
				},
			},
			wantValidErr: true,
		},
		"missing cluster selector": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
				},
			},
			wantValidErr: true,
		},
		"invalid weight - too high": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
					Weight:           150, // exceeds max of 100
				},
			},
			wantValidErr: true,
		},
		"invalid weight - too low": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
					Weight:           -5, // invalid negative value
				},
			},
			wantValidErr: true,
		},
		"invalid affinity rule weight": {
			policy: SessionAffinityPolicy{
				ObjectMeta: validMetadata,
				Spec: SessionAffinityPolicySpec{
					WorkloadSelector: validWorkloadSelector,
					ClusterSelector:  validClusterSelector,
					AffinityType:     SessionAffinityTypeClientIP,
					StickinessPolicy: validStickinessPolicy,
					AffinityRules: []AffinityRule{
						{
							Name:   "invalid-weight-rule",
							Type:   AffinityRuleTypeConstraint,
							Weight: 150, // exceeds max of 100
						},
					},
				},
			},
			wantValidErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateSessionAffinityPolicy(&tc.policy)
			hasErr := err != nil
			if hasErr != tc.wantValidErr {
				t.Errorf("validateSessionAffinityPolicy() error = %v, wantValidErr %v", err, tc.wantValidErr)
			}
		})
	}
}

func TestStickyBinding_Validation(t *testing.T) {
	validMetadata := metav1.ObjectMeta{
		Name:      "test-binding",
		Namespace: "default",
	}

	expirationTime := metav1.Now().Add(time.Hour)

	tests := map[string]struct {
		binding      StickyBinding
		wantValidErr bool
	}{
		"valid minimal binding": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					SessionIdentifier: "session-12345",
					TargetCluster:     "cluster-1",
					BindingType:       SessionAffinityTypeClientIP,
				},
			},
			wantValidErr: false,
		},
		"valid binding with full configuration": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					SessionIdentifier: "session-67890",
					TargetCluster:     "cluster-2",
					BindingType:       SessionAffinityTypeCookie,
					WorkloadReference: &ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "web-app",
						Namespace:  "default",
					},
					ExpiresAt: &metav1.Time{Time: expirationTime},
					BindingMetadata: map[string]string{
						"source": "auto-generated",
						"region": "us-west-2",
					},
					Weight: 75,
					AutoRenewal: &BindingAutoRenewal{
						Enabled:             true,
						RenewalInterval:     metav1.Duration{Duration: time.Minute * 30},
						MaxRenewals:         12,
						RenewalThreshold:    metav1.Duration{Duration: time.Minute * 5},
						StopOnInactivity:    true,
						InactivityThreshold: metav1.Duration{Duration: time.Hour},
					},
					ConflictResolution: &BindingConflictResolution{
						Strategy:              ConflictResolutionStrategyHighestWeight,
						AllowMultipleBindings: false,
						MaxBindingsPerSession: 1,
					},
				},
			},
			wantValidErr: false,
		},
		"missing session identifier": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					TargetCluster: "cluster-1",
					BindingType:   SessionAffinityTypeClientIP,
				},
			},
			wantValidErr: true,
		},
		"missing target cluster": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					SessionIdentifier: "session-12345",
					BindingType:       SessionAffinityTypeClientIP,
				},
			},
			wantValidErr: true,
		},
		"invalid weight - too high": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					SessionIdentifier: "session-12345",
					TargetCluster:     "cluster-1",
					BindingType:       SessionAffinityTypeClientIP,
					Weight:            150, // exceeds max of 100
				},
			},
			wantValidErr: true,
		},
		"invalid weight - too low": {
			binding: StickyBinding{
				ObjectMeta: validMetadata,
				Spec: StickyBindingSpec{
					SessionIdentifier: "session-12345",
					TargetCluster:     "cluster-1",
					BindingType:       SessionAffinityTypeClientIP,
					Weight:            -10, // invalid negative value
				},
			},
			wantValidErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateStickyBinding(&tc.binding)
			hasErr := err != nil
			if hasErr != tc.wantValidErr {
				t.Errorf("validateStickyBinding() error = %v, wantValidErr %v", err, tc.wantValidErr)
			}
		})
	}
}

func TestSessionBindingConstraint_Validation(t *testing.T) {
	validMetadata := metav1.ObjectMeta{
		Name:      "test-constraint",
		Namespace: "default",
	}

	validTarget := ConstraintTarget{
		Type: ConstraintTargetTypeCluster,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"tier": "production"},
		},
	}

	tests := map[string]struct {
		constraint   SessionBindingConstraint
		wantValidErr bool
	}{
		"valid minimal constraint": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
					Target:         validTarget,
					Parameters: map[string]string{
						"maxBindings": "100",
					},
				},
			},
			wantValidErr: false,
		},
		"valid constraint with full configuration": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType:   BindingConstraintTypeResourceUtilizationLimit,
					Target:           validTarget,
					Enforcement:      ConstraintEnforcementWarning,
					MaxViolations:    5,
					ViolationAction:  ViolationActionWarn,
					GracePeriod:      metav1.Duration{Duration: time.Minute * 10},
					Parameters: map[string]string{
						"cpuThreshold":    "80",
						"memoryThreshold": "85",
					},
					Exemptions: []ConstraintExemption{
						{
							Name:   "emergency-override",
							Reason: "Allow higher utilization during emergencies",
							Target: ConstraintTarget{
								Type:  ConstraintTargetTypeNamespace,
								Names: []string{"critical-apps"},
							},
							ExpiresAt: &metav1.Time{Time: time.Now().Add(time.Hour * 24)},
							Conditions: []ExemptionCondition{
								{
									Type: ExemptionConditionTypeEmergency,
									Parameters: map[string]string{
										"severity": "critical",
									},
								},
							},
						},
					},
				},
			},
			wantValidErr: false,
		},
		"missing constraint type": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					Target: validTarget,
				},
			},
			wantValidErr: true,
		},
		"missing target": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
				},
			},
			wantValidErr: true,
		},
		"invalid max violations - too high": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
					Target:         validTarget,
					MaxViolations:  150, // exceeds max of 100
				},
			},
			wantValidErr: true,
		},
		"invalid exemption - missing name": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
					Target:         validTarget,
					Exemptions: []ConstraintExemption{
						{
							Reason: "Missing name",
							Target: validTarget,
						},
					},
				},
			},
			wantValidErr: true,
		},
		"invalid exemption - missing reason": {
			constraint: SessionBindingConstraint{
				ObjectMeta: validMetadata,
				Spec: SessionBindingConstraintSpec{
					ConstraintType: BindingConstraintTypeMaxBindingsPerCluster,
					Target:         validTarget,
					Exemptions: []ConstraintExemption{
						{
							Name:   "test-exemption",
							Target: validTarget,
						},
					},
				},
			},
			wantValidErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateSessionBindingConstraint(&tc.constraint)
			hasErr := err != nil
			if hasErr != tc.wantValidErr {
				t.Errorf("validateSessionBindingConstraint() error = %v, wantValidErr %v", err, tc.wantValidErr)
			}
		})
	}
}

func TestAffinityRule_WeightValidation(t *testing.T) {
	tests := map[string]struct {
		rule     AffinityRule
		wantErrs bool
	}{
		"valid weight": {
			rule: AffinityRule{
				Name:   "test-rule",
				Type:   AffinityRuleTypeConstraint,
				Weight: 50,
			},
			wantErrs: false,
		},
		"minimum valid weight": {
			rule: AffinityRule{
				Name:   "min-weight-rule",
				Type:   AffinityRuleTypePreference,
				Weight: 1,
			},
			wantErrs: false,
		},
		"maximum valid weight": {
			rule: AffinityRule{
				Name:   "max-weight-rule",
				Type:   AffinityRuleTypeRequirement,
				Weight: 100,
			},
			wantErrs: false,
		},
		"weight too low": {
			rule: AffinityRule{
				Name:   "low-weight-rule",
				Type:   AffinityRuleTypeConstraint,
				Weight: -5, // invalid negative value
			},
			wantErrs: true,
		},
		"weight too high": {
			rule: AffinityRule{
				Name:   "high-weight-rule",
				Type:   AffinityRuleTypePreference,
				Weight: 101,
			},
			wantErrs: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := validateAffinityRuleWeight(&tc.rule, field.NewPath("affinityRules").Index(0))
			hasErrs := len(errs) > 0
			if hasErrs != tc.wantErrs {
				t.Errorf("validateAffinityRuleWeight() errors = %v, wantErrs %v", errs, tc.wantErrs)
			}
		})
	}
}

func TestStickinessPolicy_Validation(t *testing.T) {
	tests := map[string]struct {
		policy   StickinessPolicy
		wantErrs bool
	}{
		"valid policy": {
			policy: StickinessPolicy{
				Type:                  StickinessTypeSoft,
				Duration:              metav1.Duration{Duration: time.Hour},
				MaxBindings:           3,
				BreakOnClusterFailure: true,
			},
			wantErrs: false,
		},
		"max bindings too low": {
			policy: StickinessPolicy{
				Type:        StickinessTypeHard,
				Duration:    metav1.Duration{Duration: time.Hour},
				MaxBindings: -2, // invalid negative value
			},
			wantErrs: true,
		},
		"max bindings too high": {
			policy: StickinessPolicy{
				Type:        StickinessTypeAdaptive,
				Duration:    metav1.Duration{Duration: time.Hour},
				MaxBindings: 15, // above max of 10
			},
			wantErrs: true,
		},
		"valid rebalancing policy": {
			policy: StickinessPolicy{
				Type:     StickinessTypeSoft,
				Duration: metav1.Duration{Duration: time.Hour},
				RebalancingPolicy: &RebalancingPolicy{
					Enabled:                true,
					Trigger:                RebalancingTriggerLoadImbalance,
					LoadImbalanceThreshold: 75,
					MaxSessionsToMove:      50,
					DrainTimeout:           metav1.Duration{Duration: time.Minute * 10},
				},
			},
			wantErrs: false,
		},
		"invalid rebalancing threshold - too low": {
			policy: StickinessPolicy{
				Type:     StickinessTypeSoft,
				Duration: metav1.Duration{Duration: time.Hour},
				RebalancingPolicy: &RebalancingPolicy{
					Enabled:                true,
					Trigger:                RebalancingTriggerLoadImbalance,
					LoadImbalanceThreshold: 5, // below min of 10
				},
			},
			wantErrs: true,
		},
		"invalid rebalancing threshold - too high": {
			policy: StickinessPolicy{
				Type:     StickinessTypeSoft,
				Duration: metav1.Duration{Duration: time.Hour},
				RebalancingPolicy: &RebalancingPolicy{
					Enabled:                true,
					Trigger:                RebalancingTriggerLoadImbalance,
					LoadImbalanceThreshold: 95, // above max of 90
				},
			},
			wantErrs: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := validateStickinessPolicy(&tc.policy, field.NewPath("stickinessPolicy"))
			hasErrs := len(errs) > 0
			if hasErrs != tc.wantErrs {
				t.Errorf("validateStickinessPolicy() errors = %v, wantErrs %v", errs, tc.wantErrs)
			}
		})
	}
}

// validateSessionAffinityPolicy performs validation on SessionAffinityPolicy
func validateSessionAffinityPolicy(policy *SessionAffinityPolicy) error {
	var errs field.ErrorList
	
	// Validate required fields
	if policy.Spec.WorkloadSelector.LabelSelector == nil && 
		len(policy.Spec.WorkloadSelector.WorkloadTypes) == 0 &&
		policy.Spec.WorkloadSelector.NamespaceSelector == nil {
		errs = append(errs, field.Required(
			field.NewPath("spec", "workloadSelector"), 
			"at least one selector field must be specified"))
	}
	
	if policy.Spec.ClusterSelector.LabelSelector == nil &&
		len(policy.Spec.ClusterSelector.LocationSelector) == 0 &&
		len(policy.Spec.ClusterSelector.ClusterNames) == 0 {
		errs = append(errs, field.Required(
			field.NewPath("spec", "clusterSelector"),
			"at least one selector field must be specified"))
	}

	// Validate weight range (0 is default and valid)
	if policy.Spec.Weight < 0 || policy.Spec.Weight > 100 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec", "weight"),
			policy.Spec.Weight,
			"weight must be between 0 and 100"))
	}

	// Validate affinity rules
	for i, rule := range policy.Spec.AffinityRules {
		ruleErrs := validateAffinityRuleWeight(&rule, field.NewPath("spec", "affinityRules").Index(i))
		errs = append(errs, ruleErrs...)
	}

	// Validate stickiness policy
	stickinessErrs := validateStickinessPolicy(&policy.Spec.StickinessPolicy, field.NewPath("spec", "stickinessPolicy"))
	errs = append(errs, stickinessErrs...)

	if len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

// validateStickyBinding performs validation on StickyBinding
func validateStickyBinding(binding *StickyBinding) error {
	var errs field.ErrorList

	// Validate required fields
	if binding.Spec.SessionIdentifier == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "sessionIdentifier"),
			"session identifier is required"))
	}

	if binding.Spec.TargetCluster == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "targetCluster"),
			"target cluster is required"))
	}

	// Validate weight range (0 is default and valid)
	if binding.Spec.Weight < 0 || binding.Spec.Weight > 100 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec", "weight"),
			binding.Spec.Weight,
			"weight must be between 0 and 100"))
	}

	if len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

// validateSessionBindingConstraint performs validation on SessionBindingConstraint
func validateSessionBindingConstraint(constraint *SessionBindingConstraint) error {
	var errs field.ErrorList

	// Validate required fields
	if constraint.Spec.ConstraintType == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "constraintType"),
			"constraint type is required"))
	}

	if constraint.Spec.Target.Type == "" {
		errs = append(errs, field.Required(
			field.NewPath("spec", "target", "type"),
			"target type is required"))
	}

	// Validate max violations range
	if constraint.Spec.MaxViolations > 100 {
		errs = append(errs, field.Invalid(
			field.NewPath("spec", "maxViolations"),
			constraint.Spec.MaxViolations,
			"max violations cannot exceed 100"))
	}

	// Validate exemptions
	for i, exemption := range constraint.Spec.Exemptions {
		exemptionPath := field.NewPath("spec", "exemptions").Index(i)
		if exemption.Name == "" {
			errs = append(errs, field.Required(
				exemptionPath.Child("name"),
				"exemption name is required"))
		}
		if exemption.Reason == "" {
			errs = append(errs, field.Required(
				exemptionPath.Child("reason"),
				"exemption reason is required"))
		}
	}

	if len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

// validateAffinityRuleWeight validates affinity rule weight
func validateAffinityRuleWeight(rule *AffinityRule, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	// Validate weight range (0 is default and valid)
	if rule.Weight < 0 || rule.Weight > 100 {
		errs = append(errs, field.Invalid(
			path.Child("weight"),
			rule.Weight,
			"weight must be between 0 and 100"))
	}

	return errs
}

// validateStickinessPolicy validates stickiness policy configuration
func validateStickinessPolicy(policy *StickinessPolicy, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	// Validate max bindings range (0 is default and valid)
	if policy.MaxBindings < 0 || policy.MaxBindings > 10 {
		errs = append(errs, field.Invalid(
			path.Child("maxBindings"),
			policy.MaxBindings,
			"max bindings must be between 0 and 10"))
	}

	// Validate rebalancing policy if present
	if policy.RebalancingPolicy != nil {
		rebalancingPath := path.Child("rebalancingPolicy")
		// Validate rebalancing threshold (0 is default and valid)
		if policy.RebalancingPolicy.LoadImbalanceThreshold != 0 && 
			(policy.RebalancingPolicy.LoadImbalanceThreshold < 10 || 
			 policy.RebalancingPolicy.LoadImbalanceThreshold > 90) {
			errs = append(errs, field.Invalid(
				rebalancingPath.Child("loadImbalanceThreshold"),
				policy.RebalancingPolicy.LoadImbalanceThreshold,
				"load imbalance threshold must be between 10 and 90"))
		}
	}

	return errs
}