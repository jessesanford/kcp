package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_WorkloadStatusAggregation sets defaults
func SetDefaults_WorkloadStatusAggregation(obj *WorkloadStatusAggregation) {
	// Default update frequency
	if obj.Spec.UpdateFrequency == nil {
		obj.Spec.UpdateFrequency = &metav1.Duration{Duration: 30 * time.Second}
	}

	// Default aggregation policy
	if obj.Spec.AggregationPolicy == nil {
		obj.Spec.AggregationPolicy = &AggregationPolicy{
			Strategy:     AggregationStrategyPessimistic,
			MinLocations: 1,
		}
	}

	// Default priorities for status fields
	for i := range obj.Spec.StatusFields {
		if obj.Spec.StatusFields[i].Priority == 0 {
			obj.Spec.StatusFields[i].Priority = 100
		}

		// Set default display name if not provided
		if obj.Spec.StatusFields[i].DisplayName == "" {
			obj.Spec.StatusFields[i].DisplayName = obj.Spec.StatusFields[i].Path
		}
	}

	// Initialize status if not set
	if obj.Status.AggregatedPhase == "" {
		obj.Status.AggregatedPhase = WorkloadPhaseUnknown
	}

	// Initialize location statuses slice if nil
	if obj.Status.LocationStatuses == nil {
		obj.Status.LocationStatuses = make([]AggregatedLocationStatus, 0)
	}

	// Initialize aggregated fields slice if nil
	if obj.Status.AggregatedFields == nil {
		obj.Status.AggregatedFields = make([]AggregatedField, 0)
	}
}
