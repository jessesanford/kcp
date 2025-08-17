package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SetDefaults_WorkloadDistribution sets default values for WorkloadDistribution
func SetDefaults_WorkloadDistribution(obj *WorkloadDistribution) {
	// Set default rollout strategy if not specified
	if obj.Spec.RolloutStrategy == nil {
		obj.Spec.RolloutStrategy = &RolloutStrategy{
			Type: RolloutTypeRollingUpdate,
			RollingUpdate: &RollingUpdateStrategy{
				MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			},
		}
	}

	// Set default rolling update values if strategy is rolling update but config is missing
	if obj.Spec.RolloutStrategy.Type == RolloutTypeRollingUpdate && obj.Spec.RolloutStrategy.RollingUpdate == nil {
		obj.Spec.RolloutStrategy.RollingUpdate = &RollingUpdateStrategy{
			MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
		}
	}

	// Set default blue-green values if strategy is blue-green but config is incomplete
	if obj.Spec.RolloutStrategy.Type == RolloutTypeBlueGreen && obj.Spec.RolloutStrategy.BlueGreen != nil {
		if obj.Spec.RolloutStrategy.BlueGreen.ScaleDownDelaySeconds == nil {
			defaultDelay := int32(30) // 30 seconds default
			obj.Spec.RolloutStrategy.BlueGreen.ScaleDownDelaySeconds = &defaultDelay
		}
		if obj.Spec.RolloutStrategy.BlueGreen.PreviewService == "" && obj.Spec.RolloutStrategy.BlueGreen.ActiveService != "" {
			obj.Spec.RolloutStrategy.BlueGreen.PreviewService = obj.Spec.RolloutStrategy.BlueGreen.ActiveService + "-preview"
		}
	}

	// Set default priorities for explicit distributions
	for i := range obj.Spec.Distributions {
		if obj.Spec.Distributions[i].Priority == nil {
			defaultPriority := int32(50) // Medium priority
			obj.Spec.Distributions[i].Priority = &defaultPriority
		}
	}

	// Set default phase if not set
	if obj.Status.Phase == "" {
		obj.Status.Phase = DistributionPhasePending
	}
}

// SetDefaults_RollingUpdateStrategy sets defaults for RollingUpdateStrategy
func SetDefaults_RollingUpdateStrategy(obj *RollingUpdateStrategy) {
	if obj.MaxUnavailable == nil {
		obj.MaxUnavailable = &intstr.IntOrString{Type: intstr.String, StrVal: "25%"}
	}
	if obj.MaxSurge == nil {
		obj.MaxSurge = &intstr.IntOrString{Type: intstr.String, StrVal: "25%"}
	}
}

// SetDefaults_BlueGreenStrategy sets defaults for BlueGreenStrategy  
func SetDefaults_BlueGreenStrategy(obj *BlueGreenStrategy) {
	if obj.ScaleDownDelaySeconds == nil {
		defaultDelay := int32(30)
		obj.ScaleDownDelaySeconds = &defaultDelay
	}
	if obj.PreviewService == "" && obj.ActiveService != "" {
		obj.PreviewService = obj.ActiveService + "-preview"
	}
}