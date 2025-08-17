package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&PlacementPolicy{}, func(obj interface{}) {
		SetObjectDefaults_PlacementPolicy(obj.(*PlacementPolicy))
	})
	return nil
}

// SetObjectDefaults_PlacementPolicy sets default values for PlacementPolicy objects.
func SetObjectDefaults_PlacementPolicy(obj *PlacementPolicy) {
	SetDefaults_PlacementPolicy(obj)
}

// SetDefaults_PlacementPolicy sets default values for PlacementPolicy.
// It applies strategy-specific defaults and ensures consistent configuration.
func SetDefaults_PlacementPolicy(obj *PlacementPolicy) {
	// Set default strategy if not specified
	if obj.Spec.Strategy == "" {
		obj.Spec.Strategy = PlacementStrategySpread
	}

	// Set default replicas based on strategy if not specified
	if obj.Spec.Replicas == nil {
		switch obj.Spec.Strategy {
		case PlacementStrategySingleton:
			replicas := int32(1)
			obj.Spec.Replicas = &replicas
		case PlacementStrategyHighAvailability:
			replicas := int32(3)
			obj.Spec.Replicas = &replicas
		case PlacementStrategySpread:
			replicas := int32(1)
			obj.Spec.Replicas = &replicas
		case PlacementStrategyBinpack:
			replicas := int32(1)
			obj.Spec.Replicas = &replicas
		default:
			replicas := int32(1)
			obj.Spec.Replicas = &replicas
		}
	}

	// Set default toleration operator if not specified
	for i := range obj.Spec.Tolerations {
		if obj.Spec.Tolerations[i].Operator == "" {
			obj.Spec.Tolerations[i].Operator = TolerationOpEqual
		}
	}

	// Set default weight for affinity terms if not specified
	if obj.Spec.AffinityRules != nil {
		for i := range obj.Spec.AffinityRules.WorkloadAffinity {
			if obj.Spec.AffinityRules.WorkloadAffinity[i].Weight == nil {
				weight := int32(50) // Medium priority by default
				obj.Spec.AffinityRules.WorkloadAffinity[i].Weight = &weight
			}
		}
		for i := range obj.Spec.AffinityRules.WorkloadAntiAffinity {
			if obj.Spec.AffinityRules.WorkloadAntiAffinity[i].Weight == nil {
				weight := int32(50) // Medium priority by default
				obj.Spec.AffinityRules.WorkloadAntiAffinity[i].Weight = &weight
			}
		}
	}
}