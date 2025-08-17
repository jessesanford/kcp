# Implementation Instructions: SyncTarget API Types (Wave 1)

## Branch Overview
**Branch**: `feature/tmc-completion/p5w1-synctarget-api`  
**Wave**: 1 (Core API Types)  
**Focus**: SyncTarget API types for TMC workload management  
**Estimated Lines**: 600 (excluding generated code)  
**Dependencies**: None (first wave)  

## Objectives
Implement the foundational SyncTarget API types that define how physical clusters are represented and managed in the TMC system. These types will be the cornerstone for workload placement and synchronization.

## Implementation Checklist

### Step 1: Package Structure Setup (50 lines)
```bash
# Create the API package structure
mkdir -p pkg/apis/workload/v1alpha1
mkdir -p pkg/apis/workload/install
```

Create the following files:
- `pkg/apis/workload/v1alpha1/doc.go` - Package documentation
- `pkg/apis/workload/v1alpha1/register.go` - API registration  
- `pkg/apis/workload/v1alpha1/types.go` - Main type definitions
- `pkg/apis/workload/install/install.go` - Scheme installation

### Step 2: Core SyncTarget Types (250 lines)

#### File: `pkg/apis/workload/v1alpha1/types.go`
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=`.spec.cells[0].name`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Syncer",type="string",JSONPath=`.status.syncerIdentity`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// SyncTarget defines a physical cluster target for workload synchronization
type SyncTarget struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   SyncTargetSpec   `json:"spec"`
    Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget
type SyncTargetSpec struct {
    // Cells defines the cells this SyncTarget supports
    // +kubebuilder:validation:MinItems=1
    Cells []Cell `json:"cells"`

    // SupportedAPIExports defines which APIs this target can sync
    // +optional
    SupportedAPIExports []APIExportReference `json:"supportedAPIExports,omitempty"`

    // Unschedulable marks this SyncTarget as unavailable for new workloads
    // +optional
    Unschedulable bool `json:"unschedulable,omitempty"`

    // EvictAfter defines when to evict workloads after target becomes unhealthy
    // +optional
    // +kubebuilder:validation:Type=string
    // +kubebuilder:validation:Format=duration
    EvictAfter *metav1.Duration `json:"evictAfter,omitempty"`
}

// Cell represents a failure domain or location
type Cell struct {
    // Name is the cell identifier
    Name string `json:"name"`

    // Labels for the cell
    // +optional
    Labels map[string]string `json:"labels,omitempty"`

    // Taints applied to this cell
    // +optional
    Taints []Taint `json:"taints,omitempty"`
}

// Taint represents a taint on a cell
type Taint struct {
    // Key is the taint key
    Key string `json:"key"`

    // Value is the taint value
    // +optional
    Value string `json:"value,omitempty"`

    // Effect is the taint effect
    // +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
    Effect TaintEffect `json:"effect"`
}

type TaintEffect string

const (
    TaintEffectNoSchedule       TaintEffect = "NoSchedule"
    TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
    TaintEffectNoExecute        TaintEffect = "NoExecute"
)

// APIExportReference references an APIExport
type APIExportReference struct {
    // Workspace is the workspace containing the APIExport
    Workspace string `json:"workspace"`

    // Name is the APIExport name
    Name string `json:"name"`
}

// SyncTargetStatus defines the observed state
type SyncTargetStatus struct {
    // Allocatable resources on this target
    // +optional
    Allocatable ResourceList `json:"allocatable,omitempty"`

    // Capacity resources on this target
    // +optional
    Capacity ResourceList `json:"capacity,omitempty"`

    // SyncerIdentity identifies the syncer
    // +optional
    SyncerIdentity string `json:"syncerIdentity,omitempty"`

    // LastHeartbeatTime is when the syncer last sent heartbeat
    // +optional
    LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

    // VirtualWorkspaces this target is exposed through
    // +optional
    VirtualWorkspaces []VirtualWorkspace `json:"virtualWorkspaces,omitempty"`

    // Conditions represent the observations of the current state
    // +optional
    Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// ResourceList is a map of resource quantities
type ResourceList map[string]resource.Quantity

// VirtualWorkspace represents a virtual workspace URL
type VirtualWorkspace struct {
    // URL is the virtual workspace URL
    URL string `json:"url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetList contains a list of SyncTargets
type SyncTargetList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []SyncTarget `json:"items"`
}
```

### Step 3: API Registration (100 lines)

#### File: `pkg/apis/workload/v1alpha1/register.go`
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

const GroupName = "workload.kcp.io"

var (
    SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
    SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
    AddToScheme        = SchemeBuilder.AddToScheme
)

// Resource returns a GroupResource for the given resource name
func Resource(resource string) schema.GroupResource {
    return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(SchemeGroupVersion,
        &SyncTarget{},
        &SyncTargetList{},
    )
    metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
    return nil
}
```

### Step 4: Validation and Defaults (150 lines)

#### File: `pkg/apis/workload/v1alpha1/validation.go`
```go
package v1alpha1

import (
    "fmt"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSyncTarget validates a SyncTarget
func ValidateSyncTarget(target *SyncTarget) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate spec
    allErrs = append(allErrs, validateSyncTargetSpec(&target.Spec, field.NewPath("spec"))...)
    
    return allErrs
}

func validateSyncTargetSpec(spec *SyncTargetSpec, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Cells validation
    if len(spec.Cells) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("cells"), "at least one cell is required"))
    }
    
    for i, cell := range spec.Cells {
        if cell.Name == "" {
            allErrs = append(allErrs, field.Required(fldPath.Child("cells").Index(i).Child("name"), "cell name is required"))
        }
    }
    
    return allErrs
}
```

#### File: `pkg/apis/workload/v1alpha1/defaults.go`
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_SyncTarget sets defaults for SyncTarget
func SetDefaults_SyncTarget(obj *SyncTarget) {
    if obj.Spec.EvictAfter == nil {
        defaultEvictAfter := metav1.Duration{Duration: 5 * time.Minute}
        obj.Spec.EvictAfter = &defaultEvictAfter
    }
}
```

### Step 5: Conditions and Status Helpers (50 lines)

#### File: `pkg/apis/workload/v1alpha1/helpers.go`
```go
package v1alpha1

import (
    conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/util/conditions"
)

const (
    // SyncTargetConditionReady indicates the SyncTarget is ready
    SyncTargetConditionReady = "Ready"
    
    // SyncTargetConditionHeartbeat indicates syncer heartbeat status
    SyncTargetConditionHeartbeat = "Heartbeat"
)

// IsReady returns true if the SyncTarget is ready
func (s *SyncTarget) IsReady() bool {
    return conditions.IsTrue(s, SyncTargetConditionReady)
}

// SetCondition sets a condition on the SyncTarget
func (s *SyncTarget) SetCondition(condition conditionsv1alpha1.Condition) {
    conditions.Set(s, condition)
}

// GetCondition gets a condition from the SyncTarget
func (s *SyncTarget) GetCondition(conditionType string) *conditionsv1alpha1.Condition {
    return conditions.Get(s, conditionType)
}
```

### Step 6: Code Generation

Run code generation after implementing types:
```bash
# Add deepcopy generation markers
make generate

# Generate CRDs
make generate-crd
```

## Testing Requirements

### Unit Tests (Location: `pkg/apis/workload/v1alpha1/`)
1. **Validation Tests** (`validation_test.go`)
   - Valid SyncTarget configurations
   - Invalid configurations (missing cells, invalid taints)
   - Edge cases (empty specs, nil values)

2. **Defaults Tests** (`defaults_test.go`)
   - Default eviction timeout
   - Preserving user-provided values

3. **Helper Tests** (`helpers_test.go`)
   - Condition management
   - Ready state determination

### Integration Tests
Create integration test in `test/e2e/synctarget/`:
```go
// Test SyncTarget CRUD operations
// Test condition updates
// Test status updates
```

## KCP Patterns to Follow

1. **Workspace Awareness**
   - Types must be workspace-scoped
   - Use logical cluster paths in references

2. **Condition Management**
   - Use KCP's condition utilities
   - Follow standard condition types

3. **APIExport Integration**
   - Reference APIExports properly
   - Support virtual workspace URLs

4. **Multi-tenancy**
   - Ensure proper isolation
   - Support workspace hierarchies

## Integration Points

1. **With Placement System**
   - SyncTargets are referenced by placement decisions
   - Cell information used for location constraints

2. **With Syncer**
   - Syncer updates SyncTarget status
   - Heartbeat mechanism maintains liveness

3. **With Virtual Workspaces**
   - SyncTargets exposed through virtual workspaces
   - URLs stored in status

## Validation Checklist

- [ ] Package structure created correctly
- [ ] All types have deepcopy markers
- [ ] CRD generation markers present
- [ ] Validation logic comprehensive
- [ ] Defaults properly set
- [ ] Conditions follow KCP patterns
- [ ] Documentation complete
- [ ] Unit tests written
- [ ] Integration tests planned
- [ ] Code generation successful
- [ ] No compilation errors
- [ ] Follows KCP API conventions
- [ ] Under 800 lines (excluding generated)

## Commit Structure

Suggested commits for this branch:
1. "feat(api): add SyncTarget API package structure"
2. "feat(api): implement core SyncTarget types and spec"
3. "feat(api): add validation and defaults for SyncTarget"
4. "feat(api): add condition helpers and status management"
5. "test(api): add unit tests for SyncTarget API"
6. "chore: run code generation for SyncTarget types"

## Success Criteria

- SyncTarget API fully implements workload target representation
- All validation passes
- Code generation successful
- Tests provide >80% coverage
- Documentation complete
- Ready for Wave 2 dependencies