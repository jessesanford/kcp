# Implementation Instructions: Core Types & API Resources

## Overview
- **Branch**: feature/tmc-phase4-vw-02-core-types
- **Purpose**: Implement APIResource and VirtualWorkspace API types with validation and defaults for KCP integration
- **Target Lines**: 400
- **Dependencies**: Branch vw-01 (interfaces)
- **Estimated Time**: 2 days

## Files to Create

### 1. pkg/apis/virtual/v1alpha1/apiresource_types.go (150 lines)
**Purpose**: Define the APIResource custom resource for virtual workspace configuration

**Types to Define**:
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// APIResource defines resources to be exposed through virtual workspaces
type APIResource struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   APIResourceSpec   `json:"spec,omitempty"`
    Status APIResourceStatus `json:"status,omitempty"`
}

// APIResourceSpec defines the desired state of APIResource
type APIResourceSpec struct {
    // GroupVersion specifies the API group and version
    GroupVersion schema.GroupVersion `json:"groupVersion"`
    
    // Resources lists the resources to expose
    Resources []ResourceDefinition `json:"resources"`
    
    // VirtualWorkspace references the virtual workspace configuration
    VirtualWorkspace VirtualWorkspaceReference `json:"virtualWorkspace"`
    
    // OpenAPISchema contains the OpenAPI schema for resources
    // +optional
    OpenAPISchema *RuntimeRawExtension `json:"openAPISchema,omitempty"`
    
    // AuthorizationPolicy defines access control
    // +optional
    AuthorizationPolicy *AuthorizationPolicy `json:"authorizationPolicy,omitempty"`
}

// ResourceDefinition describes a resource to expose
type ResourceDefinition struct {
    // Name is the plural name of the resource
    Name string `json:"name"`
    
    // SingularName is the singular name of the resource
    SingularName string `json:"singularName"`
    
    // Kind is the kind for this resource
    Kind string `json:"kind"`
    
    // ListKind is the kind for lists of this resource
    // +optional
    ListKind string `json:"listKind,omitempty"`
    
    // Verbs lists the supported verbs
    Verbs []string `json:"verbs"`
    
    // ShortNames are short names for the resource
    // +optional
    ShortNames []string `json:"shortNames,omitempty"`
    
    // Categories is a list of grouped resources
    // +optional
    Categories []string `json:"categories,omitempty"`
    
    // Namespaced indicates if the resource is namespaced
    Namespaced bool `json:"namespaced"`
    
    // SubResources lists any subresources
    // +optional
    SubResources []SubResource `json:"subResources,omitempty"`
}

// SubResource describes a subresource
type SubResource struct {
    // Name of the subresource
    Name string `json:"name"`
    
    // Verbs supported by the subresource
    Verbs []string `json:"verbs"`
}

// VirtualWorkspaceReference references a virtual workspace configuration
type VirtualWorkspaceReference struct {
    // Name of the VirtualWorkspace resource
    Name string `json:"name"`
    
    // Path is the URL path for this workspace
    // +optional
    Path string `json:"path,omitempty"`
}

// AuthorizationPolicy defines access control for resources
type AuthorizationPolicy struct {
    // RequiredPermissions lists permissions needed to access resources
    RequiredPermissions []Permission `json:"requiredPermissions,omitempty"`
    
    // AllowedGroups lists groups with access
    // +optional
    AllowedGroups []string `json:"allowedGroups,omitempty"`
    
    // AllowedUsers lists users with access
    // +optional
    AllowedUsers []string `json:"allowedUsers,omitempty"`
}

// Permission defines a required permission
type Permission struct {
    // Group is the API group
    Group string `json:"group"`
    
    // Resource is the resource type
    Resource string `json:"resource"`
    
    // Verbs are the allowed verbs
    Verbs []string `json:"verbs"`
}

// APIResourceStatus defines the observed state of APIResource
type APIResourceStatus struct {
    // Phase indicates the current state
    // +optional
    Phase APIResourcePhase `json:"phase,omitempty"`
    
    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // VirtualWorkspaceURL is the URL to access this resource
    // +optional
    VirtualWorkspaceURL string `json:"virtualWorkspaceURL,omitempty"`
    
    // LastSyncTime is when the resource was last synced
    // +optional
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
}

// APIResourcePhase represents the phase of an APIResource
type APIResourcePhase string

const (
    APIResourcePhasePending   APIResourcePhase = "Pending"
    APIResourcePhaseReady     APIResourcePhase = "Ready"
    APIResourcePhaseNotReady  APIResourcePhase = "NotReady"
    APIResourcePhaseTerminating APIResourcePhase = "Terminating"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIResourceList contains a list of APIResource
type APIResourceList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []APIResource `json:"items"`
}
```

### 2. pkg/apis/virtual/v1alpha1/virtualworkspace_types.go (100 lines)
**Purpose**: Define the VirtualWorkspace custom resource for workspace configuration

**Types to Define**:
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    corev1 "k8s.io/api/core/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualWorkspace defines a virtual workspace configuration
type VirtualWorkspace struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   VirtualWorkspaceSpec   `json:"spec,omitempty"`
    Status VirtualWorkspaceStatus `json:"status,omitempty"`
}

// VirtualWorkspaceSpec defines the desired state of VirtualWorkspace
type VirtualWorkspaceSpec struct {
    // URL is the base URL for this virtual workspace
    URL string `json:"url"`
    
    // APIExportReference references the APIExport to serve
    // +optional
    APIExportReference *corev1.LocalObjectReference `json:"apiExportReference,omitempty"`
    
    // Authentication configures authentication for the workspace
    // +optional
    Authentication *AuthenticationConfig `json:"authentication,omitempty"`
    
    // RateLimiting configures rate limiting
    // +optional
    RateLimiting *RateLimitConfig `json:"rateLimiting,omitempty"`
    
    // Caching configures caching behavior
    // +optional
    Caching *CacheConfig `json:"caching,omitempty"`
}

// AuthenticationConfig defines authentication settings
type AuthenticationConfig struct {
    // Type specifies the authentication type
    Type AuthenticationType `json:"type"`
    
    // CertificateAuthorityData contains PEM-encoded CA certificates
    // +optional
    CertificateAuthorityData []byte `json:"certificateAuthorityData,omitempty"`
    
    // ClientCertificateData contains PEM-encoded client certificate
    // +optional
    ClientCertificateData []byte `json:"clientCertificateData,omitempty"`
}

// AuthenticationType specifies the type of authentication
type AuthenticationType string

const (
    AuthenticationTypeNone        AuthenticationType = "None"
    AuthenticationTypeCertificate AuthenticationType = "Certificate"
    AuthenticationTypeToken       AuthenticationType = "Token"
    AuthenticationTypeOIDC        AuthenticationType = "OIDC"
)

// RateLimitConfig defines rate limiting settings
type RateLimitConfig struct {
    // QPS is queries per second allowed
    QPS int32 `json:"qps"`
    
    // Burst is the burst size
    Burst int32 `json:"burst"`
    
    // PerUserLimits enables per-user rate limiting
    // +optional
    PerUserLimits bool `json:"perUserLimits,omitempty"`
}

// CacheConfig defines caching settings
type CacheConfig struct {
    // TTLSeconds is the cache TTL in seconds
    TTLSeconds int32 `json:"ttlSeconds"`
    
    // MaxSize is the maximum cache size in MB
    // +optional
    MaxSize int32 `json:"maxSize,omitempty"`
}

// VirtualWorkspaceStatus defines the observed state of VirtualWorkspace
type VirtualWorkspaceStatus struct {
    // URL is the actual URL for accessing the workspace
    // +optional
    URL string `json:"url,omitempty"`
    
    // Phase indicates the current state
    // +optional
    Phase VirtualWorkspacePhase `json:"phase,omitempty"`
    
    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // ConnectedClients is the current number of connected clients
    // +optional
    ConnectedClients int32 `json:"connectedClients,omitempty"`
}

// VirtualWorkspacePhase represents the phase of a VirtualWorkspace
type VirtualWorkspacePhase string

const (
    VirtualWorkspacePhasePending      VirtualWorkspacePhase = "Pending"
    VirtualWorkspacePhaseInitializing VirtualWorkspacePhase = "Initializing"
    VirtualWorkspacePhaseReady        VirtualWorkspacePhase = "Ready"
    VirtualWorkspacePhaseTerminating  VirtualWorkspacePhase = "Terminating"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualWorkspaceList contains a list of VirtualWorkspace
type VirtualWorkspaceList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []VirtualWorkspace `json:"items"`
}
```

### 3. pkg/apis/virtual/v1alpha1/validation.go (80 lines)
**Purpose**: Implement validation logic for API types

**Functions to Implement**:
```go
package v1alpha1

import (
    "fmt"
    "net/url"
    "strings"
    
    "k8s.io/apimachinery/pkg/util/validation"
    "k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAPIResource validates an APIResource object
func ValidateAPIResource(ar *APIResource) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate spec
    allErrs = append(allErrs, ValidateAPIResourceSpec(&ar.Spec, field.NewPath("spec"))...)
    
    return allErrs
}

// ValidateAPIResourceSpec validates an APIResourceSpec
func ValidateAPIResourceSpec(spec *APIResourceSpec, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate GroupVersion
    if spec.GroupVersion.Group == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("groupVersion", "group"), "group is required"))
    }
    if spec.GroupVersion.Version == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("groupVersion", "version"), "version is required"))
    }
    
    // Validate Resources
    if len(spec.Resources) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("resources"), "at least one resource is required"))
    }
    
    for i, resource := range spec.Resources {
        allErrs = append(allErrs, ValidateResourceDefinition(&resource, fldPath.Child("resources").Index(i))...)
    }
    
    // Validate VirtualWorkspace reference
    if spec.VirtualWorkspace.Name == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("virtualWorkspace", "name"), "virtual workspace name is required"))
    }
    
    return allErrs
}

// ValidateResourceDefinition validates a ResourceDefinition
func ValidateResourceDefinition(rd *ResourceDefinition, fldPath *field.Path) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate required fields
    if rd.Name == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("name"), "resource name is required"))
    }
    if rd.Kind == "" {
        allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
    }
    if len(rd.Verbs) == 0 {
        allErrs = append(allErrs, field.Required(fldPath.Child("verbs"), "at least one verb is required"))
    }
    
    // Validate verbs
    validVerbs := map[string]bool{
        "get": true, "list": true, "watch": true,
        "create": true, "update": true, "patch": true,
        "delete": true, "deletecollection": true,
    }
    
    for i, verb := range rd.Verbs {
        if !validVerbs[strings.ToLower(verb)] {
            allErrs = append(allErrs, field.Invalid(fldPath.Child("verbs").Index(i), verb, "invalid verb"))
        }
    }
    
    return allErrs
}

// ValidateVirtualWorkspace validates a VirtualWorkspace object
func ValidateVirtualWorkspace(vw *VirtualWorkspace) field.ErrorList {
    allErrs := field.ErrorList{}
    
    // Validate URL
    if vw.Spec.URL == "" {
        allErrs = append(allErrs, field.Required(field.NewPath("spec", "url"), "URL is required"))
    } else if _, err := url.Parse(vw.Spec.URL); err != nil {
        allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "url"), vw.Spec.URL, "invalid URL"))
    }
    
    return allErrs
}
```

### 4. pkg/apis/virtual/v1alpha1/defaults.go (70 lines)
**Purpose**: Implement defaulting logic for API types

**Functions to Implement**:
```go
package v1alpha1

import (
    "fmt"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_APIResource sets defaults for APIResource
func SetDefaults_APIResource(ar *APIResource) {
    // Set default phase if not set
    if ar.Status.Phase == "" {
        ar.Status.Phase = APIResourcePhasePending
    }
    
    // Set defaults for each resource
    for i := range ar.Spec.Resources {
        SetDefaults_ResourceDefinition(&ar.Spec.Resources[i])
    }
    
    // Set default virtual workspace path if not specified
    if ar.Spec.VirtualWorkspace.Path == "" {
        ar.Spec.VirtualWorkspace.Path = fmt.Sprintf("/apis/%s", ar.Spec.GroupVersion.String())
    }
    
    // Initialize conditions if not set
    if ar.Status.Conditions == nil {
        ar.Status.Conditions = []metav1.Condition{}
    }
}

// SetDefaults_ResourceDefinition sets defaults for ResourceDefinition
func SetDefaults_ResourceDefinition(rd *ResourceDefinition) {
    // Set default singular name if not specified
    if rd.SingularName == "" && rd.Name != "" {
        // Simple singularization (remove trailing 's')
        if len(rd.Name) > 1 && rd.Name[len(rd.Name)-1] == 's' {
            rd.SingularName = rd.Name[:len(rd.Name)-1]
        } else {
            rd.SingularName = rd.Name
        }
    }
    
    // Set default list kind if not specified
    if rd.ListKind == "" && rd.Kind != "" {
        rd.ListKind = rd.Kind + "List"
    }
    
    // Ensure standard verbs are lowercase
    for i, verb := range rd.Verbs {
        rd.Verbs[i] = strings.ToLower(verb)
    }
}

// SetDefaults_VirtualWorkspace sets defaults for VirtualWorkspace
func SetDefaults_VirtualWorkspace(vw *VirtualWorkspace) {
    // Set default authentication type if not specified
    if vw.Spec.Authentication == nil {
        vw.Spec.Authentication = &AuthenticationConfig{
            Type: AuthenticationTypeCertificate,
        }
    }
    
    // Set default rate limiting if not specified
    if vw.Spec.RateLimiting == nil {
        vw.Spec.RateLimiting = &RateLimitConfig{
            QPS:   100,
            Burst: 200,
        }
    }
    
    // Set default caching if not specified
    if vw.Spec.Caching == nil {
        vw.Spec.Caching = &CacheConfig{
            TTLSeconds: 60,
            MaxSize:    100, // 100 MB
        }
    }
    
    // Set default phase if not set
    if vw.Status.Phase == "" {
        vw.Status.Phase = VirtualWorkspacePhasePending
    }
}
```

### 5. pkg/apis/virtual/v1alpha1/register.go (30 lines)
**Purpose**: Register types with the Kubernetes API scheme

**Functions to Implement**:
```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is the group version for virtual workspace APIs
var GroupVersion = schema.GroupVersion{Group: "virtual.kcp.io", Version: "v1alpha1"}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
    return GroupVersion.WithResource(resource).GroupResource()
}

var (
    // SchemeBuilder builds the scheme
    SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
    
    // AddToScheme adds types to the scheme
    AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes adds the list of known types to the given scheme
func addKnownTypes(scheme *runtime.Scheme) error {
    scheme.AddKnownTypes(GroupVersion,
        &APIResource{},
        &APIResourceList{},
        &VirtualWorkspace{},
        &VirtualWorkspaceList{},
    )
    metav1.AddToGroupVersion(scheme, GroupVersion)
    return nil
}
```

### 6. pkg/apis/virtual/v1alpha1/doc.go (10 lines)
**Purpose**: Package documentation and code generation directives

```go
// +k8s:deepcopy-gen=package
// +groupName=virtual.kcp.io

// Package v1alpha1 contains API types for virtual workspace management.
// These types define how virtual workspaces are configured and managed
// within the KCP system.
package v1alpha1
```

### 7. pkg/apis/virtual/v1alpha1/helpers.go (30 lines)
**Purpose**: Helper functions for working with API types

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsReady returns true if the resource is ready
func (ar *APIResource) IsReady() bool {
    return ar.Status.Phase == APIResourcePhaseReady
}

// GetCondition returns the condition with the given type
func (ar *APIResource) GetCondition(conditionType string) *metav1.Condition {
    for i := range ar.Status.Conditions {
        if ar.Status.Conditions[i].Type == conditionType {
            return &ar.Status.Conditions[i]
        }
    }
    return nil
}

// IsReady returns true if the virtual workspace is ready
func (vw *VirtualWorkspace) IsReady() bool {
    return vw.Status.Phase == VirtualWorkspacePhaseReady
}

// GetCondition returns the condition with the given type
func (vw *VirtualWorkspace) GetCondition(conditionType string) *metav1.Condition {
    for i := range vw.Status.Conditions {
        if vw.Status.Conditions[i].Type == conditionType {
            return &vw.Status.Conditions[i]
        }
    }
    return nil
}
```

## Implementation Steps

1. **Create package structure**:
   - Create `pkg/apis/virtual/v1alpha1/` directory
   - Add `doc.go` with package documentation

2. **Implement core types**:
   - Start with `apiresource_types.go` for APIResource CRD
   - Add `virtualworkspace_types.go` for VirtualWorkspace CRD
   - Ensure proper kubebuilder markers for code generation

3. **Add validation and defaults**:
   - Implement `validation.go` with comprehensive validation
   - Add `defaults.go` for sensible defaults
   - Include `helpers.go` for utility functions

4. **Register types**:
   - Create `register.go` to register with scheme
   - Ensure proper group/version setup

5. **Run code generation**:
   - Execute `make generate` to generate deepcopy functions
   - Verify generated code compiles

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Validation of valid and invalid resources
  - Default value application
  - Helper function behavior
  - Type registration

## Integration Points
- Uses: Interfaces from branch vw-01
- Provides: API types for controller and provider implementations

## Acceptance Criteria
- [ ] All types defined with proper markers
- [ ] Validation logic comprehensive and tested
- [ ] Defaults applied correctly
- [ ] Code generation successful (deepcopy)
- [ ] Types register with scheme properly
- [ ] Helper functions work as expected
- [ ] Follows KCP API patterns
- [ ] No linting errors

## Common Pitfalls
- **Don't forget code generation markers**: Required for deepcopy and CRD generation
- **Validate all fields**: Comprehensive validation prevents runtime issues
- **Use proper API versioning**: Follow Kubernetes API conventions
- **Include status subresource**: Required for controller status updates
- **Test validation thoroughly**: Edge cases in validation logic
- **Follow naming conventions**: Consistent with Kubernetes APIs

## Code Review Focus
- API design following Kubernetes conventions
- Comprehensive validation coverage
- Proper use of conditions in status
- Backward compatibility considerations
- Clear and complete documentation