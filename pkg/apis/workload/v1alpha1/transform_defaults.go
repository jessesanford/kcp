package v1alpha1

// SetDefaults_WorkloadTransform sets defaults
func SetDefaults_WorkloadTransform(obj *WorkloadTransform) {
	// Default priority
	if obj.Spec.Priority == 0 {
		obj.Spec.Priority = 100
	}
}