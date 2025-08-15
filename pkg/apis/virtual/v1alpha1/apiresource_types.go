package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RuntimeRawExtension is used for embedded raw JSON/YAML data
type RuntimeRawExtension struct {
	runtime.RawExtension
}

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
	APIResourcePhasePending     APIResourcePhase = "Pending"
	APIResourcePhaseReady       APIResourcePhase = "Ready"
	APIResourcePhaseNotReady    APIResourcePhase = "NotReady"
	APIResourcePhaseTerminating APIResourcePhase = "Terminating"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIResourceList contains a list of APIResource
type APIResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIResource `json:"items"`
}