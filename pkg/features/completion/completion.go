package completion
// FeatureFlagsCompletion marks completion of feature flags setup for TMC
type FeatureFlagsCompletion struct {
    Complete bool
}

// NewFeatureFlagsCompletion creates completion marker
func NewFeatureFlagsCompletion() *FeatureFlagsCompletion {
    return &FeatureFlagsCompletion{Complete: true}
}
