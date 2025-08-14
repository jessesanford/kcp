/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package syncer

import (
	"crypto/x509"
	"fmt"
	"strings"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// DefaultAuthConfig provides a basic implementation of AuthConfig for testing and development.
type DefaultAuthConfig struct {
	// syncTargetResolver maps syncer IDs to SyncTarget resources
	syncTargetResolver map[string]*workloadv1alpha1.SyncTarget
}

// NewDefaultAuthConfig creates a default authentication configuration.
func NewDefaultAuthConfig() *DefaultAuthConfig {
	return &DefaultAuthConfig{
		syncTargetResolver: make(map[string]*workloadv1alpha1.SyncTarget),
	}
}

// RegisterSyncTarget registers a SyncTarget for a specific syncer ID.
func (c *DefaultAuthConfig) RegisterSyncTarget(syncerID string, syncTarget *workloadv1alpha1.SyncTarget) {
	c.syncTargetResolver[syncerID] = syncTarget
}

// ValidateCertificate validates a syncer's client certificate.
// In a production system, this would:
//   - Verify certificate chain against a trusted CA
//   - Check certificate expiration
//   - Validate certificate purpose and key usage
//   - Ensure the certificate was issued for syncer usage
func (c *DefaultAuthConfig) ValidateCertificate(userInfo user.Info) error {
	if userInfo == nil {
		return fmt.Errorf("no user information provided")
	}

	username := userInfo.GetName()
	if username == "" {
		return fmt.Errorf("no username in certificate")
	}

	klog.V(4).InfoS("Validating syncer certificate", "username", username)

	// Check if this is a syncer certificate based on username pattern
	if !strings.HasPrefix(username, "system:syncer:") {
		return fmt.Errorf("certificate not issued for syncer usage, username: %s", username)
	}

	// Extract syncer ID from certificate subject
	syncerID := extractSyncerIDFromUsername(username)
	if syncerID == "" {
		return fmt.Errorf("could not extract syncer ID from certificate username: %s", username)
	}

	// In a real implementation, additional certificate validation would happen here:
	// - Verify against CA bundle
	// - Check expiration dates  
	// - Validate certificate extensions
	// - Check revocation status

	klog.V(4).InfoS("Certificate validation passed", "username", username, "syncerID", syncerID)
	return nil
}

// GetSyncTargetForSyncer retrieves the SyncTarget associated with a syncer ID and workspace.
func (c *DefaultAuthConfig) GetSyncTargetForSyncer(syncerID, workspace string) (*workloadv1alpha1.SyncTarget, error) {
	if syncerID == "" {
		return nil, fmt.Errorf("syncer ID is required")
	}

	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}

	klog.V(4).InfoS("Looking up SyncTarget", "syncerID", syncerID, "workspace", workspace)

	// Look up the sync target for this syncer
	syncTarget, exists := c.syncTargetResolver[syncerID]
	if !exists {
		klog.V(4).InfoS("SyncTarget not found for syncer", "syncerID", syncerID)
		return nil, nil
	}

	// Verify the workspace matches (basic implementation)
	// In a real system, this would check workspace access permissions
	if syncTarget.Namespace != workspace && workspace != "default" {
		return nil, fmt.Errorf("syncer %s not authorized for workspace %s", syncerID, workspace)
	}

	return syncTarget.DeepCopy(), nil
}

// extractSyncerIDFromUsername extracts the syncer ID from a certificate username.
// Expected format: "system:syncer:<syncer-id>"
func extractSyncerIDFromUsername(username string) string {
	const prefix = "system:syncer:"
	if !strings.HasPrefix(username, prefix) {
		return ""
	}
	
	syncerID := username[len(prefix):]
	if syncerID == "" {
		return ""
	}
	
	return syncerID
}

// ValidateCertificateChain validates a complete certificate chain for a syncer.
// This is a more comprehensive validation function that can be used in production.
func ValidateCertificateChain(certs []*x509.Certificate, caCertPool *x509.CertPool) error {
	if len(certs) == 0 {
		return fmt.Errorf("no certificates provided")
	}

	clientCert := certs[0]
	
	// Create intermediate pool from remaining certificates
	intermediatePool := x509.NewCertPool()
	for i := 1; i < len(certs); i++ {
		intermediatePool.AddCert(certs[i])
	}

	// Verify certificate chain
	opts := x509.VerifyOptions{
		Roots:         caCertPool,
		Intermediates: intermediatePool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	chains, err := clientCert.Verify(opts)
	if err != nil {
		return fmt.Errorf("certificate chain validation failed: %w", err)
	}

	if len(chains) == 0 {
		return fmt.Errorf("no valid certificate chains found")
	}

	// Additional validation: check that certificate is intended for syncer usage
	if !isSyncerCertificate(clientCert) {
		return fmt.Errorf("certificate not issued for syncer usage")
	}

	klog.V(4).InfoS("Certificate chain validation passed", "subject", clientCert.Subject.String())
	return nil
}

// isSyncerCertificate checks if a certificate was issued specifically for syncer usage.
func isSyncerCertificate(cert *x509.Certificate) bool {
	// Check common name pattern
	if strings.HasPrefix(cert.Subject.CommonName, "system:syncer:") {
		return true
	}

	// Check for syncer-specific certificate extensions or OUs
	for _, ou := range cert.Subject.OrganizationalUnit {
		if ou == "system:syncers" {
			return true
		}
	}

	return false
}