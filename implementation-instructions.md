# Implementation Instructions: Transformation Types (Wave 2)

## Branch Overview
**Branch**: `feature/tmc-completion/p5w2-transform-types`  
**Wave**: 2 (Extended APIs)  
**Focus**: Workload transformation and mutation types  
**Estimated Lines**: 450 (excluding generated code)  
**Dependencies**: Wave 1 - SyncTarget API  

## Pre-Implementation Setup

### Merge Wave 1 Dependencies
```bash
# Navigate to your worktree
cd /workspaces/kcp-worktrees/phase5/api-foundation/worktrees/p5w2-transform-types

# Fetch and merge Wave 1 branches
git fetch origin
git merge origin/feature/tmc-completion/p5w1-synctarget-api --no-edit
```

## Objectives
Implement transformation types that define how workloads are modified when synchronized to different locations. These types enable location-specific customizations while maintaining workload portability.

## Implementation Checklist

### Step 1: Extend Workload Package (30 lines)
```bash
# The workload package already exists from Wave 1
# Add new files for transformation types
```

Create the following files:
- `pkg/apis/workload/v1alpha1/transform_types.go` - Transformation type definitions
- `pkg/apis/workload/v1alpha1/transform_validation.go` - Transformation validation
- `pkg/apis/workload/v1alpha1/transform_helpers.go` - Transformation helpers

### Step 2: Core Transformation Types (180 lines)

#### File: `pkg/apis/workload/v1alpha1/transform_types.go`
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
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=`.spec.targetRef.kind`
// +kubebuilder:printcolumn:name="Locations",type="integer",JSONPath=`.status.appliedLocations`
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=`.status.conditions[?(@.type=="Active")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// WorkloadTransform defines transformations to apply to workloads
type WorkloadTransform struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   WorkloadTransformSpec   `json:"spec"`
    Status WorkloadTransformStatus `json:"status,omitempty"`
}

// WorkloadTransformSpec defines transformation rules
type WorkloadTransformSpec struct {
    // TargetRef identifies what to transform
    TargetRef TransformTarget `json:"targetRef"`

    // LocationSelectors where to apply transforms
    // +optional
    LocationSelectors []LocationSelector `json:"locationSelectors,omitempty"`

    // Transforms to apply
    Transforms []Transform `json:"transforms"`

    // Priority when multiple transforms match
    // +optional
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=1000
    Priority int32 `json:"priority,omitempty"`

    // Paused stops applying transforms
    // +optional
    Paused bool `json:"paused,omitempty"`
}

// TransformTarget identifies what to transform
type TransformTarget struct {
    // APIVersion of the target
    APIVersion string `json:"apiVersion"`

    // Kind of the target
    Kind string `json:"kind"`

    // Name of specific object
    // +optional
    Name string `json:"name,omitempty"`

    // LabelSelector for multiple objects
    // +optional
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// LocationSelector selects locations for transformation
type LocationSelector struct {
    // Name of specific location
    // +optional
    Name string `json:"name,omitempty"`

    // LabelSelector for locations
    // +optional
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// Transform defines a transformation operation
type Transform struct {
    // Type of transformation
    // +kubebuilder:validation:Enum=JSONPatch;StrategicMerge;Replace;Remove;Annotate;Label
    Type TransformType `json:"type"`

    // JSONPatch operations
    // +optional
    JSONPatch []JSONPatchOperation `json:"jsonPatch,omitempty"`

    // StrategicMerge patch
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    StrategicMerge *runtime.RawExtension `json:"strategicMerge,omitempty"`

    // Replace operation
    // +optional
    Replace *ReplaceOperation `json:"replace,omitempty"`

    // Remove operation
    // +optional
    Remove *RemoveOperation `json:"remove,omitempty"`

    // Annotations to add/update
    // +optional
    Annotations map[string]string `json:"annotations,omitempty"`

    // Labels to add/update
    // +optional
    Labels map[string]string `json:"labels,omitempty"`
}

type TransformType string

const (
    TransformTypeJSONPatch      TransformType = "JSONPatch"
    TransformTypeStrategicMerge TransformType = "StrategicMerge"
    TransformTypeReplace        TransformType = "Replace"
    TransformTypeRemove         TransformType = "Remove"
    TransformTypeAnnotate       TransformType = "Annotate"
    TransformTypeLabel          TransformType = "Label"
)

// JSONPatchOperation defines a JSON patch operation
type JSONPatchOperation struct {
    // Op is the operation type
    // +kubebuilder:validation:Enum=add;remove;replace;copy;move;test
    Op string `json:"op"`

    // Path is the JSON path
    Path string `json:"path"`

    // Value for the operation
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    Value *runtime.RawExtension `json:"value,omitempty"`

    // From path for copy/move
    // +optional
    From string `json:"from,omitempty"`
}

// ReplaceOperation replaces field values
type ReplaceOperation struct {
    // Path to replace
    Path string `json:"path"`

    // Value to set
    // +kubebuilder:pruning:PreserveUnknownFields
    Value runtime.RawExtension `json:"value"`
}

// RemoveOperation removes fields
type RemoveOperation struct {
    // Paths to remove
    Paths []string `json:"paths"`
}

// WorkloadTransformStatus defines the observed state
type WorkloadTransformStatus struct {
    // AppliedLocations count
    AppliedLocations int32 `json:"appliedLocations"`

    // TransformApplications tracks where applied
    // +optional
    TransformApplications []TransformApplication `json:"transformApplications,omitempty"`

    // ObservedGeneration last reconciled
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // LastAppliedTime
    // +optional
    LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

    // Conditions of the transform
    // +optional
    Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// TransformApplication tracks where transform is applied
type TransformApplication struct {
    // LocationName
    LocationName string `json:"locationName"`

    // AppliedAt timestamp
    AppliedAt metav1.Time `json:"appliedAt"`

    // ObjectsTransformed count
    ObjectsTransformed int32 `json:"objectsTransformed"`

    // Success status
    Success bool `json:"success"`

    // Message if failed
    // +optional
    Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadTransformList contains a list of WorkloadTransforms
type WorkloadTransformList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []WorkloadTransform `json:"items"`
}
```

### Step 3: Transform Validation (90 lines)

#### File: `pkg/apis/workload/v1alpha1/transform_validation.go`
```go
package v1alpha1

import (
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkloadTransform validates a WorkloadTransform
func ValidateWorkloadTransform(transform *WorkloadTransform) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate spec
    allErrs = append(allErrs, validateWorkloadTransformSpec(&transform.Spec, field.NewPath("spec"))...)
    
    return allErrs
}

func validateWorkloadTransformSpec(spec *WorkloadTransformSpec, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate TargetRef
    if spec.TargetRef.APIVersion == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("targetRef", "apiVersion"), "apiVersion is required"))
    }
    if spec.TargetRef.Kind == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("targetRef", "kind"), "kind is required"))
    }
    
    // Must specify either name or labelSelector
    if spec.TargetRef.Name == "" && spec.TargetRef.LabelSelector == nil {
        allErrs = append(allErrs, field.Required(fldPath.Child("targetRef"), "must specify name or labelSelector"))
    }
    
    // Validate Transforms
    if len(spec.Transforms) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("transforms"), "at least one transform is required"))
    }
    
    for i, transform := range spec.Transforms {
        allErrs = append(allErrs, validateTransform(&transform, fldPath.Child("transforms").Index(i))...)
    }
    
    return allErrs
}

func validateTransform(transform *Transform, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate type-specific fields
    switch transform.Type {
    case TransformTypeJSONPatch:
        if len(transform.JSONPatch) == 0 {
            allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch"), "jsonPatch operations required"))
        }
        for j, op := range transform.JSONPatch {
            if op.Path == "" {
                allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch").Index(j).Child("path"), "path is required"))
            }
            if op.Op == "copy" || op.Op == "move" {
                if op.From == "" {
                    allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch").Index(j).Child("from"), "from is required for copy/move"))
                }
            }
        }
    case TransformTypeStrategicMerge:
        if transform.StrategicMerge == nil {
            allErrs = append(allErrs, field.Required(fldPath.Child("strategicMerge"), "strategicMerge patch required"))
        }
    case TransformTypeReplace:
        if transform.Replace == nil {
            allErrs = append(allErrs, field.Required(fldPath.Child("replace"), "replace operation required"))
        } else if transform.Replace.Path == "" {
            allErrs = append(allErrs, field.Required(fldPath.Child("replace", "path"), "path is required"))
        }
    case TransformTypeRemove:
        if transform.Remove == nil || len(transform.Remove.Paths) == 0 {
            allErrs = append(allErrs, field.Required(fldPath.Child("remove", "paths"), "paths required for remove"))
        }
    case TransformTypeAnnotate:
        if len(transform.Annotations) == 0 {
            allErrs = append(allErrs, field.Required(fldPath.Child("annotations"), "annotations required"))
        }
    case TransformTypeLabel:
        if len(transform.Labels) == 0 {
            allErrs = append(allErrs, field.Required(fldPath.Child("labels"), "labels required"))
        }
    }
    
    return allErrs
}
```

### Step 4: Transform Helpers (80 lines)

#### File: `pkg/apis/workload/v1alpha1/transform_helpers.go`
```go
package v1alpha1

import (
    "encoding/json"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
    "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/util/conditions"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
    // TransformConditionActive indicates transform is active
    TransformConditionActive = "Active"
    
    // TransformConditionApplied indicates successful application
    TransformConditionApplied = "Applied"
)

// IsActive returns true if transform is active
func (t *WorkloadTransform) IsActive() bool {
    return !t.Spec.Paused && conditions.IsTrue(t, TransformConditionActive)
}

// IsApplied returns true if transform is successfully applied
func (t *WorkloadTransform) IsApplied() bool {
    return conditions.IsTrue(t, TransformConditionApplied)
}

// SetCondition sets a condition on the WorkloadTransform
func (t *WorkloadTransform) SetCondition(condition conditionsv1alpha1.Condition) {
    conditions.Set(t, condition)
}

// GetApplicationForLocation returns application status for a location
func (t *WorkloadTransform) GetApplicationForLocation(locationName string) *TransformApplication {
    for i := range t.Status.TransformApplications {
        if t.Status.TransformApplications[i].LocationName == locationName {
            return &t.Status.TransformApplications[i]
        }
    }
    return nil
}

// ApplyTransform applies the transform to an object
func (t *WorkloadTransform) ApplyTransform(obj *unstructured.Unstructured) error {
    for _, transform := range t.Spec.Transforms {
        if err := applyTransformOperation(obj, transform); err != nil {
            return err
        }
    }
    return nil
}

func applyTransformOperation(obj *unstructured.Unstructured, transform Transform) error {
    switch transform.Type {
    case TransformTypeAnnotate:
        annotations := obj.GetAnnotations()
        if annotations == nil {
            annotations = make(map[string]string)
        }
        for k, v := range transform.Annotations {
            annotations[k] = v
        }
        obj.SetAnnotations(annotations)
        
    case TransformTypeLabel:
        labels := obj.GetLabels()
        if labels == nil {
            labels = make(map[string]string)
        }
        for k, v := range transform.Labels {
            labels[k] = v
        }
        obj.SetLabels(labels)
        
    // Other transform types would be implemented here
    }
    
    return nil
}

// MatchesTarget checks if an object matches the transform target
func (t *WorkloadTransform) MatchesTarget(obj *unstructured.Unstructured) bool {
    if obj.GetAPIVersion() != t.Spec.TargetRef.APIVersion {
        return false
    }
    if obj.GetKind() != t.Spec.TargetRef.Kind {
        return false
    }
    if t.Spec.TargetRef.Name != "" && obj.GetName() != t.Spec.TargetRef.Name {
        return false
    }
    // LabelSelector matching would be implemented here
    return true
}
```

### Step 5: Transform Defaults (40 lines)

#### File: `pkg/apis/workload/v1alpha1/transform_defaults.go`
```go
package v1alpha1

// SetDefaults_WorkloadTransform sets defaults
func SetDefaults_WorkloadTransform(obj *WorkloadTransform) {
    // Default priority
    if obj.Spec.Priority == 0 {
        obj.Spec.Priority = 100
    }
}
```

### Step 6: Update Registration (30 lines)

#### File: `pkg/apis/workload/v1alpha1/register.go` (UPDATE)
```go
// Add to existing addKnownTypes function:
func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(SchemeGroupVersion,
        &SyncTarget{},
        &SyncTargetList{},
        &WorkloadTransform{},  // ADD THIS
        &WorkloadTransformList{}, // ADD THIS
    )
    metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
    return nil
}
```

### Step 7: Code Generation

Run code generation after implementing types:
```bash
# Add deepcopy generation markers
make generate

# Generate CRDs
make generate-crd
```

## Testing Requirements

### Unit Tests (Location: `pkg/apis/workload/v1alpha1/`)
1. **Transform Validation Tests** (`transform_validation_test.go`)
   - Valid transform configurations
   - Invalid transform operations
   - Target matching validation

2. **Transform Helper Tests** (`transform_helpers_test.go`)
   - Transform application logic
   - Target matching
   - Condition management

3. **Transform Defaults Tests** (`transform_defaults_test.go`)
   - Priority defaults

### Integration Tests
Create integration test in `test/e2e/transform/`:
```go
// Test WorkloadTransform CRUD operations
// Test transform application to objects
// Test priority ordering
// Test location-specific transforms
```

## KCP Patterns to Follow

1. **Workspace Awareness**
   - Transforms apply within workspace
   - Respect workspace boundaries

2. **Location-Specific**
   - Apply per-location transforms
   - Support location selectors

3. **Priority System**
   - Handle multiple matching transforms
   - Apply in priority order

4. **Declarative Mutations**
   - Define desired mutations
   - Controller applies them

## Integration Points

1. **With SyncTarget (Wave 1)**
   - Apply transforms per target
   - Location-specific customization

2. **With WorkloadDistribution (Wave 2)**
   - Transforms applied during distribution
   - Per-location modifications

3. **With Syncer Interfaces (Wave 3)**
   - Syncer applies transforms
   - Transform chain processing

## Validation Checklist

- [ ] Package extends existing workload package
- [ ] All types have deepcopy markers
- [ ] CRD generation markers present
- [ ] Validation logic comprehensive
- [ ] Defaults properly set
- [ ] Helper functions useful
- [ ] Documentation complete
- [ ] Unit tests written
- [ ] Integration tests planned
- [ ] Code generation successful
- [ ] No compilation errors
- [ ] Follows KCP API conventions
- [ ] Under 800 lines (excluding generated)
- [ ] Properly imports Wave 1 types

## Commit Structure

Suggested commits for this branch:
1. "feat(api): add WorkloadTransform types to workload API"
2. "feat(api): implement transform operations and patches"
3. "feat(api): add validation for transform operations"
4. "feat(api): add helper functions for transform application"
5. "test(api): add unit tests for WorkloadTransform"
6. "chore: run code generation for transform types"

## Success Criteria

- WorkloadTransform API enables flexible workload mutations
- Supports multiple transform types (JSONPatch, Strategic Merge, etc.)
- Location-specific transforms work
- Priority system functions correctly
- Transform application logic solid
- All validation passes
- Tests provide >80% coverage
- Ready for Wave 3 syncer integration