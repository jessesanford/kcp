# Implementation Instructions: PR3 - API Implementation

## PR Overview

**Purpose**: Implement the full TMC API types with structs, validation, and defaulting
**Target Line Count**: 450 lines (excluding generated code)
**Dependencies**: PR1 (after merge) - implements the interfaces defined there
**Feature Flag**: `TMCFeatureGate` (same master flag)

## Files to Create

### 1. pkg/apis/tmc/v1alpha1/types_cluster.go (120 lines)
```
Full ClusterRegistration type implementation
Expected content:
- ClusterRegistration struct (40 lines)
  - TypeMeta
  - ObjectMeta
  - Spec with fields (location, capabilities, etc.)
  - Status with conditions
- ClusterRegistrationList struct (10 lines)
- Interface implementation methods (30 lines)
  - GetLocation(), GetCapabilities(), IsReady(), etc.
- Validation methods (20 lines)
- Defaulting methods (20 lines)
```

### 2. pkg/apis/tmc/v1alpha1/types_placement.go (120 lines)
```
Full WorkloadPlacement type implementation
Expected content:
- WorkloadPlacement struct (40 lines)
  - TypeMeta
  - ObjectMeta
  - Spec with selector, strategy, targetClusters
  - Status with placement results
- WorkloadPlacementList struct (10 lines)
- Interface implementation methods (30 lines)
  - GetTargetClusters(), GetStrategy(), IsPlaced(), etc.
- Validation methods (20 lines)
- Defaulting methods (20 lines)
```

### 3. pkg/apis/tmc/v1alpha1/types_shared.go (80 lines)
```
Shared types used by both resources
Expected content:
- PlacementStrategy type (20 lines)
- ClusterCapability type (20 lines)
- LocationSpec type (20 lines)
- Common condition helpers (20 lines)
```

### 4. pkg/apis/tmc/v1alpha1/types_cluster_test.go (65 lines)
```
Tests for ClusterRegistration
Expected tests:
- Interface compliance test
- Validation tests
- Defaulting tests
- Status update tests
```

### 5. pkg/apis/tmc/v1alpha1/types_placement_test.go (65 lines)
```
Tests for WorkloadPlacement
Expected tests:
- Interface compliance test
- Validation tests
- Defaulting tests
- Placement result tests
```

## Files to Generate

### Generated Files (NOT counted in line limit):
- pkg/apis/tmc/v1alpha1/zz_generated.deepcopy.go
- config/crds/tmc.kcp.io_clusterregistrations.yaml
- config/crds/tmc.kcp.io_workloadplacements.yaml

## Extraction Instructions

### From Legacy PR1 (03a-pr1-api-foundation)

1. **Copy and enhance types_cluster.go**:
```bash
# Copy the base implementation
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr1-api-foundation/pkg/apis/tmc/v1alpha1/types_cluster.go \
   pkg/apis/tmc/v1alpha1/types_cluster.go

# Then enhance with:
# - Interface implementation methods from PR1
# - Additional validation logic
# - Better status management
```

2. **Copy test file**:
```bash
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr1-api-foundation/pkg/apis/tmc/v1alpha1/types_cluster_test.go \
   pkg/apis/tmc/v1alpha1/types_cluster_test.go

# Update to test interface compliance
```

### From Legacy PR2 (03a-pr2-placement-api)

1. **Copy all placement API files**:
```bash
# Copy placement types
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr2-placement-api/pkg/apis/tmc/v1alpha1/types_placement.go \
   pkg/apis/tmc/v1alpha1/types_placement.go

# Copy shared types
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr2-placement-api/pkg/apis/tmc/v1alpha1/types_shared.go \
   pkg/apis/tmc/v1alpha1/types_shared.go

# Copy tests
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr2-placement-api/pkg/apis/tmc/v1alpha1/types_placement_test.go \
   pkg/apis/tmc/v1alpha1/types_placement_test.go

cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr2-placement-api/pkg/apis/tmc/v1alpha1/types_shared_test.go \
   pkg/apis/tmc/v1alpha1/types_shared_test.go
```

2. **Update register.go**:
```bash
# Copy registration if needed
cp /workspaces/kcp-worktrees/legacy/phase3-original/03a-pr2-placement-api/pkg/apis/tmc/v1alpha1/register.go \
   pkg/apis/tmc/v1alpha1/register.go

# Ensure it registers all types
```

### Modifications During Extraction

1. **Add interface implementations**:
   - Each type must implement its corresponding interface from PR1
   - Add getter methods for interface compliance

2. **Enhance validation**:
   - Add comprehensive validation methods
   - Use KCP validation patterns

3. **Improve status management**:
   - Add condition helpers
   - Implement status update methods

## Implementation Details

### Type Structure Example

```go
// ClusterRegistration represents a cluster available for workload placement.
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
type ClusterRegistration struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
    Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// Implement the interface from PR1
func (c *ClusterRegistration) GetLocation() string {
    return c.Spec.Location
}

func (c *ClusterRegistration) GetCapabilities() []string {
    return c.Spec.Capabilities
}

func (c *ClusterRegistration) IsReady() bool {
    return meta.IsStatusConditionTrue(c.Status.Conditions, "Ready")
}
```

### Validation Pattern

```go
// ValidateClusterRegistration validates a ClusterRegistration object
func ValidateClusterRegistration(cr *ClusterRegistration) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate spec
    allErrs = append(allErrs, ValidateClusterRegistrationSpec(&cr.Spec, field.NewPath("spec"))...)
    
    return allErrs
}

func ValidateClusterRegistrationSpec(spec *ClusterRegistrationSpec, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate location
    if spec.Location == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("location"), "location is required"))
    }
    
    // Validate capabilities
    for i, cap := range spec.Capabilities {
        if err := ValidateCapability(cap); err != nil {
            allErrs = append(allErrs, field.Invalid(fldPath.Child("capabilities").Index(i), cap, err.Error()))
        }
    }
    
    return allErrs
}
```

### Defaulting Pattern

```go
// SetDefaults_ClusterRegistration sets default values
func SetDefaults_ClusterRegistration(cr *ClusterRegistration) {
    if cr.Spec.Location == "" {
        cr.Spec.Location = "default"
    }
    
    if len(cr.Spec.Capabilities) == 0 {
        cr.Spec.Capabilities = []string{"general"}
    }
}
```

## Testing Requirements

### Required Tests

1. **Interface Compliance**:
```go
func TestClusterRegistrationImplementsInterface(t *testing.T) {
    // Verify the type implements the interface from PR1
    var _ v1alpha1.ClusterRegistrationInterface = &ClusterRegistration{}
}
```

2. **Validation Tests**:
```go
func TestValidateClusterRegistration(t *testing.T) {
    tests := []struct {
        name    string
        cr      *ClusterRegistration
        wantErr bool
    }{
        // Test cases
    }
    // Implementation
}
```

3. **Defaulting Tests**:
```go
func TestSetDefaults_ClusterRegistration(t *testing.T) {
    // Test that defaults are properly applied
}
```

## Code Generation

After implementing types, run:

```bash
# Generate deepcopy
make generate

# Generate CRDs
make manifests

# Verify generated files
ls -la pkg/apis/tmc/v1alpha1/zz_generated.deepcopy.go
ls -la config/crds/tmc.kcp.io_*.yaml
```

## Verification Checklist

- [ ] All types implement their interfaces from PR1
- [ ] Validation methods are comprehensive
- [ ] Defaulting logic is applied
- [ ] Status subresource is properly defined
- [ ] CRD markers are correct
- [ ] Tests pass for all types
- [ ] Deepcopy is generated successfully
- [ ] CRDs are generated successfully
- [ ] Line count under 450 (excluding generated):
  ```bash
  /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc2-impl2/pr3-api-implementation
  ```
- [ ] Code compiles and tests pass:
  ```bash
  go build ./pkg/apis/tmc/v1alpha1/...
  go test ./pkg/apis/tmc/v1alpha1/...
  ```

## Integration with PR1

This PR depends on PR1 being merged first. After PR1 is merged:

1. Rebase this branch on main
2. Import the interfaces package from PR1
3. Ensure all types implement the required interfaces
4. Run tests to verify compliance

## Notes

- This PR provides the concrete implementation of TMC APIs
- Must maintain compatibility with interfaces from PR1
- Generated code (deepcopy, CRDs) does not count toward line limit
- Focus on clean, well-validated API types following KCP patterns