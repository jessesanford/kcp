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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretTransformerInterface(t *testing.T) {
	transformer := NewSecretTransformer()
	
	// Test interface compliance
	require.NotNil(t, transformer)
	assert.Equal(t, "secret-transformer", transformer.Name())
	
	// Test ShouldTransform
	secret := &corev1.Secret{}
	assert.True(t, transformer.ShouldTransform(secret))
	
	configMap := &corev1.ConfigMap{}
	assert.False(t, transformer.ShouldTransform(configMap))
	
	assert.False(t, transformer.ShouldTransform(nil))
}

func TestSecretTransformerDownstreamAllowedTypes(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	target.SetName("test-target")
	
	allowedTypes := []corev1.SecretType{
		corev1.SecretTypeOpaque,
		corev1.SecretTypeDockerConfigJson,
		"kubernetes.io/dockercfg",
		corev1.SecretTypeTLS,
		corev1.SecretTypeSSHAuth,
		corev1.SecretTypeBasicAuth,
	}
	
	testData := map[corev1.SecretType]map[string][]byte{
		corev1.SecretTypeOpaque: {
			"data": []byte("test-value"),
		},
		corev1.SecretTypeDockerConfigJson: {
			corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
		},
		"kubernetes.io/dockercfg": {
			".dockercfg": []byte(`{"registry":{}}`),
		},
		corev1.SecretTypeTLS: {
			corev1.TLSCertKey:       []byte("cert-data"),
			corev1.TLSPrivateKeyKey: []byte("key-data"),
		},
		corev1.SecretTypeSSHAuth: {
			corev1.SSHAuthPrivateKey: []byte("ssh-key"),
		},
		corev1.SecretTypeBasicAuth: {
			corev1.BasicAuthUsernameKey: []byte("user"),
		},
	}
	
	for _, secretType := range allowedTypes {
		t.Run(string(secretType), func(t *testing.T) {
			secret := createTestSecret("test-secret", string(secretType), testData[secretType])
			
			result, err := transformer.TransformForDownstream(ctx, secret, target)
			require.NoError(t, err)
			require.NotNil(t, result)
			
			resultSecret, ok := result.(*corev1.Secret)
			require.True(t, ok)
			assert.Equal(t, secretType, resultSecret.Type)
		})
	}
}

func TestSecretTransformerDownstreamBlockedTypes(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	target.SetName("test-target")
	
	// Test service account token (blocked type)
	secret := createTestSecret("sa-secret", string(corev1.SecretTypeServiceAccountToken), map[string][]byte{
		"token": []byte("secret-token"),
	})
	
	result, err := transformer.TransformForDownstream(ctx, secret, target)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not allowed for synchronization")
}

func TestSecretTransformerSensitiveKeyFiltering(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	target.SetName("test-target")
	
	// Create secret with both safe and sensitive keys
	secret := createTestSecret("test-secret", string(corev1.SecretTypeOpaque), map[string][]byte{
		"username":        []byte("user"),         // safe
		"database_host":   []byte("db.host"),      // safe
		"data":            []byte("config"),       // safe
		"access_token":    []byte("token123"),     // sensitive key
		"api_key":         []byte("key456"),       // sensitive key
		"bearer_token":    []byte("bearer123"),    // sensitive key
		"oauth_token":     []byte("oauth456"),     // sensitive key
		"service-account": []byte("sa-token"),     // sensitive key
		"my_secret_key":   []byte("mysecret"),     // sensitive pattern
		"user_token":      []byte("usertoken"),    // sensitive pattern
		"app_credential":  []byte("creds"),        // sensitive pattern
	})
	
	result, err := transformer.TransformForDownstream(ctx, secret, target)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultSecret, ok := result.(*corev1.Secret)
	require.True(t, ok)
	
	// Safe keys should remain
	assert.Contains(t, resultSecret.Data, "username")
	assert.Contains(t, resultSecret.Data, "database_host")
	assert.Contains(t, resultSecret.Data, "data")
	
	// Sensitive keys should be removed
	assert.NotContains(t, resultSecret.Data, "access_token")
	assert.NotContains(t, resultSecret.Data, "api_key")
	assert.NotContains(t, resultSecret.Data, "bearer_token")
	assert.NotContains(t, resultSecret.Data, "oauth_token")
	assert.NotContains(t, resultSecret.Data, "service-account")
	assert.NotContains(t, resultSecret.Data, "my_secret_key")
	assert.NotContains(t, resultSecret.Data, "user_token")
	assert.NotContains(t, resultSecret.Data, "app_credential")
}

func TestSecretTransformerUpstreamValidation(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	source := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "test-cluster",
		},
	}
	source.SetName("test-source")
	
	// Test with valid secret
	secret := createTestSecret("test-secret", string(corev1.SecretTypeOpaque), map[string][]byte{
		"username": []byte("user"),
		"data":     []byte("config"),
	})
	
	result, err := transformer.TransformForUpstream(ctx, secret, source)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Secret should be preserved as-is for upstream
	resultSecret, ok := result.(*corev1.Secret)
	require.True(t, ok)
	assert.Equal(t, secret.Name, resultSecret.Name)
	assert.Equal(t, secret.Type, resultSecret.Type)
	assert.Equal(t, len(secret.Data), len(resultSecret.Data))
}

func TestSecretTransformerTypeSpecificValidation(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	target.SetName("test-target")
	
	tests := []struct {
		name        string
		secretType  string
		data        map[string][]byte
		expectError bool
		errorMsg    string
	}{
		{
			name:       "valid docker config json",
			secretType: string(corev1.SecretTypeDockerConfigJson),
			data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
			},
			expectError: false,
		},
		{
			name:       "invalid docker config json - missing key",
			secretType: string(corev1.SecretTypeDockerConfigJson),
			data: map[string][]byte{
				"wrong-key": []byte(`{"auths":{}}`),
			},
			expectError: true,
			errorMsg:    "missing required key",
		},
		{
			name:       "valid docker cfg",
			secretType: "kubernetes.io/dockercfg",
			data: map[string][]byte{
				".dockercfg": []byte(`{"registry":{}}`),
			},
			expectError: false,
		},
		{
			name:       "valid TLS secret",
			secretType: string(corev1.SecretTypeTLS),
			data: map[string][]byte{
				corev1.TLSCertKey:       []byte("cert-data"),
				corev1.TLSPrivateKeyKey: []byte("key-data"),
			},
			expectError: false,
		},
		{
			name:       "invalid TLS secret - missing cert",
			secretType: string(corev1.SecretTypeTLS),
			data: map[string][]byte{
				corev1.TLSPrivateKeyKey: []byte("key-data"),
			},
			expectError: true,
			errorMsg:    "missing certificate key",
		},
		{
			name:       "valid SSH auth",
			secretType: string(corev1.SecretTypeSSHAuth),
			data: map[string][]byte{
				corev1.SSHAuthPrivateKey: []byte("ssh-key"),
			},
			expectError: false,
		},
		{
			name:       "valid basic auth with username",
			secretType: string(corev1.SecretTypeBasicAuth),
			data: map[string][]byte{
				corev1.BasicAuthUsernameKey: []byte("user"),
			},
			expectError: false,
		},
		{
			name:       "valid basic auth with password",
			secretType: string(corev1.SecretTypeBasicAuth),
			data: map[string][]byte{
				corev1.BasicAuthPasswordKey: []byte("pass"),
			},
			expectError: false,
		},
		{
			name:       "invalid basic auth - empty",
			secretType: string(corev1.SecretTypeBasicAuth),
			data:       map[string][]byte{},
			expectError: true,
			errorMsg:    "must have username or password",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := createTestSecret("test-secret", tt.secretType, tt.data)
			
			result, err := transformer.TransformForDownstream(ctx, secret, target)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestSecretTransformerDataValidation(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	target.SetName("test-target")
	
	tests := []struct {
		name        string
		data        map[string][]byte
		expectError bool
		description string
	}{
		{
			name: "valid data entries",
			data: map[string][]byte{
				"username": []byte("user"),
				"config":   []byte("some-config"),
			},
			expectError: false,
			description: "normal data should pass",
		},
		{
			name: "dangerous key with path traversal",
			data: map[string][]byte{
				"../etc/passwd": []byte("content"),
			},
			expectError: false, // Should be filtered out, not error
			description: "path traversal keys should be filtered",
		},
		{
			name: "key with slash",
			data: map[string][]byte{
				"config/file": []byte("content"),
			},
			expectError: false, // Should be filtered out, not error  
			description: "keys with slashes should be filtered",
		},
		{
			name: "valid base64 data",
			data: map[string][]byte{
				"cert.b64": []byte(base64.StdEncoding.EncodeToString([]byte("cert-data"))),
			},
			expectError: false,
			description: "valid base64 should pass",
		},
		{
			name: "invalid base64 data",
			data: map[string][]byte{
				"cert.b64": []byte("invalid-base64!@#"),
			},
			expectError: false, // Should be filtered out, not error
			description: "invalid base64 should be filtered",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := createTestSecret("test-secret", string(corev1.SecretTypeOpaque), tt.data)
			
			result, err := transformer.TransformForDownstream(ctx, secret, target)
			
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				require.NotNil(t, result, tt.description)
			}
		})
	}
}

func TestSecretTransformerWithNilObject(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{Spec: SyncTargetSpec{ClusterName: "test"}}
	target.SetName("test-target")
	
	result, err := transformer.TransformForDownstream(ctx, nil, target)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot transform nil object")
	
	result, err = transformer.TransformForUpstream(ctx, nil, target)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot transform nil object")
}

func TestSecretTransformerWithNonSecretObject(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{Spec: SyncTargetSpec{ClusterName: "test"}}
	target.SetName("test-target")
	
	// Test with ConfigMap (not a Secret)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Data:       map[string]string{"key": "value"},
	}
	
	result, err := transformer.TransformForDownstream(ctx, configMap, target)
	require.NoError(t, err)
	assert.Equal(t, configMap, result) // Should pass through unchanged
	
	result, err = transformer.TransformForUpstream(ctx, configMap, target)
	require.NoError(t, err)
	assert.Equal(t, configMap, result) // Should pass through unchanged
}

func TestSecretTransformerSizeLimit(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	target := &SyncTarget{
		Spec: SyncTargetSpec{ClusterName: "test-cluster"},
	}
	target.SetName("test-target")
	
	// Create data that exceeds the 1MB limit
	largeData := make([]byte, 1024*1024+1) // 1MB + 1 byte
	for i := range largeData {
		largeData[i] = byte('a')
	}
	
	secret := createTestSecret("test-secret", string(corev1.SecretTypeOpaque), map[string][]byte{
		"large-data": largeData,
		"normal":     []byte("small"),
	})
	
	result, err := transformer.TransformForDownstream(ctx, secret, target)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultSecret, ok := result.(*corev1.Secret)
	require.True(t, ok)
	
	// Large data should be filtered out
	assert.NotContains(t, resultSecret.Data, "large-data")
	// Normal data should remain
	assert.Contains(t, resultSecret.Data, "normal")
}

// Helper function to create test secrets
func createTestSecret(name, secretType string, data map[string][]byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Type: corev1.SecretType(secretType),
		Data: data,
	}
	
	return secret
}

// Integration test that validates all transformers work together
func TestSecretTransformerIntegration(t *testing.T) {
	ctx := context.Background()
	transformer := NewSecretTransformer()
	
	target := &SyncTarget{
		Spec: SyncTargetSpec{
			ClusterName: "prod-cluster",
			Namespace:   "app-namespace",
		},
	}
	target.SetName("prod-target")
	
	// Create a comprehensive test secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-secret",
			Namespace: "default",
			Labels: map[string]string{
				"app":  "test-app",
				"tier": "backend",
			},
			Annotations: map[string]string{
				"description": "Application configuration secret",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			// Safe data that should be preserved
			"database_host":     []byte("db.example.com"),
			"database_port":     []byte("5432"),
			"database_name":     []byte("appdb"),
			"app_config":        []byte("config-data"),
			
			// Sensitive data that should be filtered
			"api_key":           []byte("secret-key"),
			"access_token":      []byte("token123"),
			"bearer_token":      []byte("bearer123"),
			"my_app_secret":     []byte("hidden"),
			
			// Invalid data that should be filtered
			"../etc/passwd":    []byte("malicious"),
			"config/file":      []byte("path-traversal"),
		},
	}
	
	// Test downstream transformation
	downstreamResult, err := transformer.TransformForDownstream(ctx, secret, target)
	require.NoError(t, err)
	require.NotNil(t, downstreamResult)
	
	downstreamSecret, ok := downstreamResult.(*corev1.Secret)
	require.True(t, ok)
	
	// Verify metadata is preserved
	assert.Equal(t, "app-secret", downstreamSecret.Name)
	assert.Equal(t, "default", downstreamSecret.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, downstreamSecret.Type)
	assert.Equal(t, "test-app", downstreamSecret.Labels["app"])
	assert.Equal(t, "backend", downstreamSecret.Labels["tier"])
	assert.Equal(t, "Application configuration secret", downstreamSecret.Annotations["description"])
	
	// Verify safe data is preserved
	assert.Contains(t, downstreamSecret.Data, "database_host")
	assert.Contains(t, downstreamSecret.Data, "database_port")
	assert.Contains(t, downstreamSecret.Data, "database_name")
	assert.Contains(t, downstreamSecret.Data, "app_config")
	assert.Equal(t, []byte("db.example.com"), downstreamSecret.Data["database_host"])
	assert.Equal(t, []byte("5432"), downstreamSecret.Data["database_port"])
	
	// Verify sensitive data is filtered
	assert.NotContains(t, downstreamSecret.Data, "api_key")
	assert.NotContains(t, downstreamSecret.Data, "access_token")
	assert.NotContains(t, downstreamSecret.Data, "bearer_token")
	assert.NotContains(t, downstreamSecret.Data, "my_app_secret")
	
	// Verify invalid data is filtered
	assert.NotContains(t, downstreamSecret.Data, "../etc/passwd")
	assert.NotContains(t, downstreamSecret.Data, "config/file")
	
	// Test upstream transformation
	upstreamResult, err := transformer.TransformForUpstream(ctx, downstreamSecret, target)
	require.NoError(t, err)
	require.NotNil(t, upstreamResult)
	
	upstreamSecret, ok := upstreamResult.(*corev1.Secret)
	require.True(t, ok)
	
	// Upstream should preserve the filtered secret as-is
	assert.Equal(t, downstreamSecret.Name, upstreamSecret.Name)
	assert.Equal(t, downstreamSecret.Type, upstreamSecret.Type)
	assert.Equal(t, len(downstreamSecret.Data), len(upstreamSecret.Data))
}