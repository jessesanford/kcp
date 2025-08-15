package v1alpha1

import (
	"fmt"
	"strings"

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

	// Initialize conditions if not set
	if vw.Status.Conditions == nil {
		vw.Status.Conditions = []metav1.Condition{}
	}
}