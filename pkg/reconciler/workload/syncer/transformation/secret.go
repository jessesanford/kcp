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

package transformation

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// secretTransformer handles secret sanitization and filtering during synchronization.
// It prevents sensitive data leakage and validates secret integrity.
type secretTransformer struct {
	// allowedSecretTypes defines which secret types can be synchronized
	allowedSecretTypes map[corev1.SecretType]bool
	
	// sensitiveKeys are data keys that should be filtered out
	sensitiveKeys map[string]bool
}

// NewSecretTransformer creates a new secret transformer with security policies.
func NewSecretTransformer() ResourceTransformer {
	return &secretTransformer{
		allowedSecretTypes: map[corev1.SecretType]bool{
			// Allow common application secret types
			corev1.SecretTypeOpaque:                true,
			corev1.SecretTypeDockerConfigJson:      true,
			"kubernetes.io/dockercfg":             true,
			corev1.SecretTypeTLS:                   true,
			corev1.SecretTypeSSHAuth:               true,
			corev1.SecretTypeBasicAuth:             true,
			
			// Block service account tokens - these should not sync across clusters
			corev1.SecretTypeServiceAccountToken:   false,
		},
		
		sensitiveKeys: map[string]bool{
			// Common sensitive data keys to filter
			"token":           true,
			"access_token":    true,
			"refresh_token":   true,
			"api_key":         true,
			"api-key":         true,
			"secret":          true,
			"private_key":     true,
			"private-key":     true,
			"service-account": true,
		},
	}
}

// Name returns the transformer name
func (t *secretTransformer) Name() string {
	return "secret-transformer"
}

// ShouldTransform returns true only for Secret objects
func (t *secretTransformer) ShouldTransform(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	
	_, ok := obj.(*corev1.Secret)
	return ok
}

// TransformForDownstream sanitizes secrets when syncing to physical clusters
func (t *secretTransformer) TransformForDownstream(ctx context.Context, obj runtime.Object, target *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return obj, nil // Not a secret, pass through
	}
	
	// Check if this secret type is allowed to be synced
	if allowed, exists := t.allowedSecretTypes[secret.Type]; exists && !allowed {
		klog.V(3).InfoS("Blocking secret sync due to type restrictions",
			"secretName", secret.Name,
			"secretType", secret.Type,
			"namespace", secret.Namespace,
			"targetCluster", target.Spec.ClusterName)
		return nil, fmt.Errorf("secret type %s is not allowed for synchronization", secret.Type)
	}
	
	// Create a deep copy to avoid modifying original
	result := secret.DeepCopy()
	
	// Sanitize the secret data
	if err := t.sanitizeSecretData(result); err != nil {
		return nil, fmt.Errorf("failed to sanitize secret data: %w", err)
	}
	
	// Validate the secret after sanitization
	if err := t.validateSecret(result); err != nil {
		return nil, fmt.Errorf("secret validation failed after sanitization: %w", err)
	}
	
	klog.V(4).InfoS("Successfully sanitized secret for downstream sync",
		"secretName", result.Name,
		"secretType", result.Type,
		"namespace", result.Namespace,
		"targetCluster", target.Spec.ClusterName,
		"dataKeys", len(result.Data))
	
	return result, nil
}

// TransformForUpstream handles secrets when syncing back to KCP
func (t *secretTransformer) TransformForUpstream(ctx context.Context, obj runtime.Object, source *SyncTarget) (runtime.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot transform nil object")
	}
	
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return obj, nil // Not a secret, pass through
	}
	
	// For upstream sync, we generally want to preserve the secret as-is
	// since it may have been updated in the physical cluster
	result := secret.DeepCopy()
	
	// However, we should still validate it meets our security requirements
	if err := t.validateSecret(result); err != nil {
		klog.V(3).InfoS("Secret validation failed during upstream sync, skipping",
			"secretName", secret.Name,
			"namespace", secret.Namespace,
			"sourceCluster", source.Spec.ClusterName,
			"error", err)
		return nil, fmt.Errorf("secret validation failed during upstream sync: %w", err)
	}
	
	klog.V(4).InfoS("Processing secret for upstream sync",
		"secretName", result.Name,
		"secretType", result.Type,
		"namespace", result.Namespace,
		"sourceCluster", source.Spec.ClusterName,
		"dataKeys", len(result.Data))
	
	return result, nil
}

// sanitizeSecretData removes sensitive data keys and validates remaining data
func (t *secretTransformer) sanitizeSecretData(secret *corev1.Secret) error {
	if secret.Data == nil {
		return nil
	}
	
	originalKeyCount := len(secret.Data)
	
	// Filter out sensitive keys
	for key := range secret.Data {
		if t.isSensitiveKey(key) {
			klog.V(5).InfoS("Removing sensitive data key",
				"secretName", secret.Name,
				"key", key,
				"namespace", secret.Namespace)
			delete(secret.Data, key)
		}
	}
	
	// Validate remaining data
	for key, value := range secret.Data {
		if err := t.validateSecretDataEntry(key, value); err != nil {
			klog.V(3).InfoS("Removing invalid data entry",
				"secretName", secret.Name,
				"key", key,
				"error", err)
			delete(secret.Data, key)
		}
	}
	
	filteredKeyCount := len(secret.Data)
	if originalKeyCount != filteredKeyCount {
		klog.V(4).InfoS("Secret data sanitization completed",
			"secretName", secret.Name,
			"originalKeys", originalKeyCount,
			"filteredKeys", filteredKeyCount)
	}
	
	return nil
}

// validateSecret performs comprehensive validation of the secret
func (t *secretTransformer) validateSecret(secret *corev1.Secret) error {
	if secret == nil {
		return fmt.Errorf("secret is nil")
	}
	
	// Check secret name and namespace
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}
	
	// Validate secret type
	if secret.Type == "" {
		klog.V(4).InfoS("Setting default secret type", "secretName", secret.Name)
		secret.Type = corev1.SecretTypeOpaque
	}
	
	// Type-specific validation
	switch secret.Type {
	case corev1.SecretTypeDockerConfigJson:
		return t.validateDockerConfigJsonSecret(secret)
	case "kubernetes.io/dockercfg":
		return t.validateDockerCfgSecret(secret)
	case corev1.SecretTypeTLS:
		return t.validateTLSSecret(secret)
	case corev1.SecretTypeSSHAuth:
		return t.validateSSHAuthSecret(secret)
	case corev1.SecretTypeBasicAuth:
		return t.validateBasicAuthSecret(secret)
	case corev1.SecretTypeOpaque:
		// Opaque secrets don't have specific validation requirements
		return nil
	default:
		return fmt.Errorf("unsupported secret type: %s", secret.Type)
	}
}

// validateSecretDataEntry validates individual data entries
func (t *secretTransformer) validateSecretDataEntry(key string, value []byte) error {
	if key == "" {
		return fmt.Errorf("empty key not allowed")
	}
	
	// Check for potentially dangerous keys
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		return fmt.Errorf("key contains invalid path characters: %s", key)
	}
	
	// Validate base64 encoding if expected
	if strings.HasSuffix(key, ".b64") {
		_, err := base64.StdEncoding.DecodeString(string(value))
		if err != nil {
			return fmt.Errorf("invalid base64 encoding for key %s: %w", key, err)
		}
	}
	
	// Check size limits (1MB max per entry)
	if len(value) > 1024*1024 {
		return fmt.Errorf("data entry %s exceeds size limit", key)
	}
	
	return nil
}

// isSensitiveKey checks if a data key is considered sensitive
func (t *secretTransformer) isSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	
	// Direct match
	if sensitive, exists := t.sensitiveKeys[keyLower]; exists && sensitive {
		return true
	}
	
	// Pattern matching for common sensitive key patterns
	sensitivePatterns := []string{
		"token",
		"key",
		"secret",
		"password",
		"passwd",
		"credential",
		"auth",
	}
	
	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}
	
	return false
}

// Type-specific validation methods
func (t *secretTransformer) validateDockerConfigJsonSecret(secret *corev1.Secret) error {
	if _, exists := secret.Data[corev1.DockerConfigJsonKey]; !exists {
		return fmt.Errorf("docker config secret missing required key: %s", corev1.DockerConfigJsonKey)
	}
	return nil
}

func (t *secretTransformer) validateDockerCfgSecret(secret *corev1.Secret) error {
	if _, exists := secret.Data[".dockercfg"]; !exists {
		return fmt.Errorf("docker cfg secret missing required key: .dockercfg")
	}
	return nil
}

func (t *secretTransformer) validateTLSSecret(secret *corev1.Secret) error {
	if _, hasCert := secret.Data[corev1.TLSCertKey]; !hasCert {
		return fmt.Errorf("TLS secret missing certificate key: %s", corev1.TLSCertKey)
	}
	if _, hasKey := secret.Data[corev1.TLSPrivateKeyKey]; !hasKey {
		return fmt.Errorf("TLS secret missing private key: %s", corev1.TLSPrivateKeyKey)
	}
	return nil
}

func (t *secretTransformer) validateSSHAuthSecret(secret *corev1.Secret) error {
	if _, exists := secret.Data[corev1.SSHAuthPrivateKey]; !exists {
		return fmt.Errorf("SSH auth secret missing private key: %s", corev1.SSHAuthPrivateKey)
	}
	return nil
}

func (t *secretTransformer) validateBasicAuthSecret(secret *corev1.Secret) error {
	hasUsername := len(secret.Data[corev1.BasicAuthUsernameKey]) > 0
	hasPassword := len(secret.Data[corev1.BasicAuthPasswordKey]) > 0
	
	if !hasUsername && !hasPassword {
		return fmt.Errorf("basic auth secret must have username or password")
	}
	return nil
}