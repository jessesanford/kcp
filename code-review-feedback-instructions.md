# Code Review Feedback Instructions

## Review Summary
- Branch: feature/tmc-phase4-sync-02a-tunnel-core
- Lines of Code: 579 (excluding SYNCER-WORKTREE-MAP.md removal)
- Overall Status: NEEDS_FIXES
- Critical Issues: 4
- Non-Critical Issues: 3

## Critical Issues (Must Fix)

### 1. Missing Test Files
**Severity**: CRITICAL
**File**: pkg/tunneler/interfaces/
**Problem**: No test files exist for any of the interface definitions. This violates KCP testing standards.
**Fix Instructions**: Create the following test files with complete test implementations:

#### Create `pkg/tunneler/interfaces/tunnel_test.go`:
```go
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

package interfaces

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTunnel implements Tunnel interface for testing
type MockTunnel struct {
	ConnectFunc        func(ctx context.Context) error
	CloseFunc          func() error
	SendFunc           func(ctx context.Context, data []byte) error
	ReceiveFunc        func(ctx context.Context) ([]byte, error)
	SendStreamFunc     func(ctx context.Context) (io.WriteCloser, error)
	ReceiveStreamFunc  func(ctx context.Context) (io.ReadCloser, error)
	StateFunc          func() TunnelState
	StatsFunc          func() TunnelStats
	SetReconnectFunc   func(enabled bool)
	PingFunc           func(ctx context.Context) error
	LocalAddrFunc      func() string
	RemoteAddrFunc     func() string
	ProtocolFunc       func() TunnelProtocol
	WorkspaceFunc      func() logicalcluster.Name

	state     TunnelState
	stats     TunnelStats
	protocol  TunnelProtocol
	workspace logicalcluster.Name
}

func (m *MockTunnel) Connect(ctx context.Context) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx)
	}
	m.state = TunnelStateConnected
	return nil
}

func (m *MockTunnel) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	m.state = TunnelStateClosed
	return nil
}

func (m *MockTunnel) Send(ctx context.Context, data []byte) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, data)
	}
	m.stats.BytesSent += uint64(len(data))
	m.stats.MessagesSent++
	return nil
}

func (m *MockTunnel) Receive(ctx context.Context) ([]byte, error) {
	if m.ReceiveFunc != nil {
		return m.ReceiveFunc(ctx)
	}
	return []byte("test data"), nil
}

func (m *MockTunnel) SendStream(ctx context.Context) (io.WriteCloser, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockTunnel) ReceiveStream(ctx context.Context) (io.ReadCloser, error) {
	if m.ReceiveStreamFunc != nil {
		return m.ReceiveStreamFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *MockTunnel) State() TunnelState {
	if m.StateFunc != nil {
		return m.StateFunc()
	}
	return m.state
}

func (m *MockTunnel) Stats() TunnelStats {
	if m.StatsFunc != nil {
		return m.StatsFunc()
	}
	return m.stats
}

func (m *MockTunnel) SetReconnectEnabled(enabled bool) {
	if m.SetReconnectFunc != nil {
		m.SetReconnectFunc(enabled)
	}
}

func (m *MockTunnel) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockTunnel) LocalAddr() string {
	if m.LocalAddrFunc != nil {
		return m.LocalAddrFunc()
	}
	return "127.0.0.1:8080"
}

func (m *MockTunnel) RemoteAddr() string {
	if m.RemoteAddrFunc != nil {
		return m.RemoteAddrFunc()
	}
	return "remote:443"
}

func (m *MockTunnel) Protocol() TunnelProtocol {
	if m.ProtocolFunc != nil {
		return m.ProtocolFunc()
	}
	return m.protocol
}

func (m *MockTunnel) Workspace() logicalcluster.Name {
	if m.WorkspaceFunc != nil {
		return m.WorkspaceFunc()
	}
	return m.workspace
}

// TestTunnelStates verifies state transitions
func TestTunnelStates(t *testing.T) {
	tests := []struct {
		name           string
		initialState   TunnelState
		operation      func(*MockTunnel) error
		expectedState  TunnelState
		expectError    bool
	}{
		{
			name:          "connect from disconnected",
			initialState:  TunnelStateDisconnected,
			operation:     func(m *MockTunnel) error { return m.Connect(context.Background()) },
			expectedState: TunnelStateConnected,
			expectError:   false,
		},
		{
			name:          "close from connected",
			initialState:  TunnelStateConnected,
			operation:     func(m *MockTunnel) error { return m.Close() },
			expectedState: TunnelStateClosed,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tunnel := &MockTunnel{
				state:     tt.initialState,
				workspace: logicalcluster.Name("test-workspace"),
				protocol:  TunnelProtocolWebSocket,
			}

			err := tt.operation(tunnel)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedState, tunnel.State())
		})
	}
}

// TestTunnelOptions validates options behavior
func TestTunnelOptions(t *testing.T) {
	opts := TunnelOptions{
		Protocol:             TunnelProtocolWebSocket,
		Workspace:            logicalcluster.Name("root:test"),
		MaxReconnectAttempts: 5,
		ReconnectInterval:    time.Second * 2,
		ReadTimeout:          time.Second * 30,
		WriteTimeout:         time.Second * 30,
		BufferSize:           4096,
		EnableCompression:    true,
		Headers: map[string]string{
			"X-Custom-Header": "value",
		},
	}

	assert.Equal(t, TunnelProtocolWebSocket, opts.Protocol)
	assert.Equal(t, logicalcluster.Name("root:test"), opts.Workspace)
	assert.Equal(t, 5, opts.MaxReconnectAttempts)
	assert.Equal(t, time.Second*2, opts.ReconnectInterval)
	assert.True(t, opts.EnableCompression)
	assert.Equal(t, "value", opts.Headers["X-Custom-Header"])
}

// TestTunnelStats validates statistics tracking
func TestTunnelStats(t *testing.T) {
	tunnel := &MockTunnel{
		state:     TunnelStateConnected,
		workspace: logicalcluster.Name("test"),
		protocol:  TunnelProtocolGRPC,
	}

	ctx := context.Background()
	testData := []byte("test message")

	// Send data
	err := tunnel.Send(ctx, testData)
	require.NoError(t, err)

	stats := tunnel.Stats()
	assert.Equal(t, uint64(len(testData)), stats.BytesSent)
	assert.Equal(t, uint64(1), stats.MessagesSent)
}

// TestTunnelFactory validates factory pattern
func TestTunnelFactory(t *testing.T) {
	factory := &MockTunnelFactory{
		CreateTunnelFunc: func(opts TunnelOptions) (Tunnel, error) {
			if opts.Protocol == "" {
				return nil, ErrInvalidConfiguration
			}
			return &MockTunnel{
				protocol:  opts.Protocol,
				workspace: opts.Workspace,
				state:     TunnelStateDisconnected,
			}, nil
		},
		SupportedProtocolsFunc: func() []TunnelProtocol {
			return []TunnelProtocol{
				TunnelProtocolWebSocket,
				TunnelProtocolGRPC,
			}
		},
	}

	t.Run("create valid tunnel", func(t *testing.T) {
		opts := TunnelOptions{
			Protocol:  TunnelProtocolWebSocket,
			Workspace: logicalcluster.Name("test"),
		}
		tunnel, err := factory.CreateTunnel(opts)
		require.NoError(t, err)
		assert.NotNil(t, tunnel)
		assert.Equal(t, TunnelProtocolWebSocket, tunnel.Protocol())
		assert.Equal(t, logicalcluster.Name("test"), tunnel.Workspace())
	})

	t.Run("create with invalid options", func(t *testing.T) {
		opts := TunnelOptions{} // Missing protocol
		tunnel, err := factory.CreateTunnel(opts)
		assert.Error(t, err)
		assert.Nil(t, tunnel)
		assert.Equal(t, ErrInvalidConfiguration, err)
	})

	t.Run("supported protocols", func(t *testing.T) {
		protocols := factory.SupportedProtocols()
		assert.Contains(t, protocols, TunnelProtocolWebSocket)
		assert.Contains(t, protocols, TunnelProtocolGRPC)
	})
}

// MockTunnelFactory implements TunnelFactory for testing
type MockTunnelFactory struct {
	CreateTunnelFunc       func(opts TunnelOptions) (Tunnel, error)
	SupportedProtocolsFunc func() []TunnelProtocol
	ValidateOptionsFunc    func(opts TunnelOptions) error
}

func (m *MockTunnelFactory) CreateTunnel(opts TunnelOptions) (Tunnel, error) {
	if m.CreateTunnelFunc != nil {
		return m.CreateTunnelFunc(opts)
	}
	return nil, errors.New("not implemented")
}

func (m *MockTunnelFactory) SupportedProtocols() []TunnelProtocol {
	if m.SupportedProtocolsFunc != nil {
		return m.SupportedProtocolsFunc()
	}
	return []TunnelProtocol{TunnelProtocolWebSocket}
}

func (m *MockTunnelFactory) ValidateOptions(opts TunnelOptions) error {
	if m.ValidateOptionsFunc != nil {
		return m.ValidateOptionsFunc(opts)
	}
	return nil
}
```

#### Create `pkg/tunneler/interfaces/basic_types_test.go`:
```go
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

package interfaces

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   AuthMethod
		expected string
	}{
		{"none", AuthMethodNone, "none"},
		{"token", AuthMethodToken, "token"},
		{"certificate", AuthMethodCertificate, "certificate"},
		{"mtls", AuthMethodMTLS, "mtls"},
		{"oauth2", AuthMethodOAuth2, "oauth2"},
		{"service account", AuthMethodServiceAccount, "serviceaccount"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.method))
		})
	}
}

func TestOAuth2Config(t *testing.T) {
	config := &OAuth2Config{
		ClientID:     "test-client",
		ClientSecret: "secret",
		TokenURL:     "https://auth.example.com/token",
		Scopes:       []string{"read", "write"},
		RefreshToken: "refresh-token",
		AccessToken:  "access-token",
		TokenExpiry:  time.Now().Add(time.Hour),
	}

	assert.Equal(t, "test-client", config.ClientID)
	assert.Equal(t, "secret", config.ClientSecret)
	assert.Equal(t, "https://auth.example.com/token", config.TokenURL)
	assert.Contains(t, config.Scopes, "read")
	assert.Contains(t, config.Scopes, "write")
	assert.NotEmpty(t, config.RefreshToken)
	assert.NotEmpty(t, config.AccessToken)
	assert.True(t, config.TokenExpiry.After(time.Now()))
}

func TestServiceAccountConfig(t *testing.T) {
	config := &ServiceAccountConfig{
		TokenFile:       "/var/run/secrets/kubernetes.io/serviceaccount/token",
		Namespace:       "default",
		ServiceAccount:  "syncer",
		Audience:        "kcp.io",
		TokenExpiration: time.Hour * 24,
	}

	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", config.TokenFile)
	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, "syncer", config.ServiceAccount)
	assert.Equal(t, "kcp.io", config.Audience)
	assert.Equal(t, time.Hour*24, config.TokenExpiration)
}

func TestAuthCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials AuthCredentials
		valid       bool
	}{
		{
			name: "token auth",
			credentials: AuthCredentials{
				Method: AuthMethodToken,
				Token:  "bearer-token",
			},
			valid: true,
		},
		{
			name: "certificate auth",
			credentials: AuthCredentials{
				Method:          AuthMethodCertificate,
				CertificateFile: "/path/to/cert.pem",
				KeyFile:         "/path/to/key.pem",
				CAFile:          "/path/to/ca.pem",
			},
			valid: true,
		},
		{
			name: "oauth2 auth",
			credentials: AuthCredentials{
				Method: AuthMethodOAuth2,
				OAuth2Config: &OAuth2Config{
					ClientID:     "client",
					ClientSecret: "secret",
					TokenURL:     "https://auth.example.com/token",
				},
			},
			valid: true,
		},
		{
			name: "service account auth",
			credentials: AuthCredentials{
				Method: AuthMethodServiceAccount,
				ServiceAccountConfig: &ServiceAccountConfig{
					TokenFile:      "/var/run/secrets/token",
					Namespace:      "default",
					ServiceAccount: "syncer",
				},
			},
			valid: true,
		},
		{
			name: "insecure skip verify",
			credentials: AuthCredentials{
				Method:                AuthMethodToken,
				Token:                 "token",
				InsecureSkipTLSVerify: true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate method is set
			assert.NotEmpty(t, tt.credentials.Method)

			// Check method-specific fields
			switch tt.credentials.Method {
			case AuthMethodToken:
				if tt.valid {
					assert.NotEmpty(t, tt.credentials.Token)
				}
			case AuthMethodCertificate:
				if tt.valid {
					assert.NotEmpty(t, tt.credentials.CertificateFile)
					assert.NotEmpty(t, tt.credentials.KeyFile)
				}
			case AuthMethodOAuth2:
				if tt.valid {
					assert.NotNil(t, tt.credentials.OAuth2Config)
					assert.NotEmpty(t, tt.credentials.OAuth2Config.ClientID)
				}
			case AuthMethodServiceAccount:
				if tt.valid {
					assert.NotNil(t, tt.credentials.ServiceAccountConfig)
					assert.NotEmpty(t, tt.credentials.ServiceAccountConfig.TokenFile)
				}
			}
		})
	}
}

func TestAuthErrors(t *testing.T) {
	assert.Error(t, ErrUnsupportedAuthMethod)
	assert.Contains(t, ErrUnsupportedAuthMethod.Error(), "unsupported")

	assert.Error(t, ErrInvalidCredentials)
	assert.Contains(t, ErrInvalidCredentials.Error(), "invalid")
}
```

#### Create `pkg/tunneler/interfaces/auth_test.go`:
```go
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

package interfaces

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTunnelAuthenticator implements TunnelAuthenticator for testing
type MockTunnelAuthenticator struct {
	AuthenticateFunc      func(ctx context.Context, credentials AuthCredentials, workspace logicalcluster.Name) (*AuthContext, error)
	RefreshAuthFunc       func(ctx context.Context, authCtx *AuthContext) (*AuthContext, error)
	ValidateAuthFunc      func(ctx context.Context, authCtx *AuthContext) error
	GetTLSConfigFunc      func(credentials AuthCredentials) (*tls.Config, error)
	SupportedMethodsFunc  func() []AuthMethod
}

func (m *MockTunnelAuthenticator) Authenticate(ctx context.Context, credentials AuthCredentials, workspace logicalcluster.Name) (*AuthContext, error) {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(ctx, credentials, workspace)
	}
	return &AuthContext{
		Workspace: workspace,
		UserInfo: &UserInfo{
			Username: "test-user",
			Groups:   []string{"system:authenticated"},
		},
		AuthTime:   time.Now(),
		ExpiryTime: time.Now().Add(time.Hour),
		SessionID:  "test-session",
	}, nil
}

func (m *MockTunnelAuthenticator) RefreshAuth(ctx context.Context, authCtx *AuthContext) (*AuthContext, error) {
	if m.RefreshAuthFunc != nil {
		return m.RefreshAuthFunc(ctx, authCtx)
	}
	authCtx.ExpiryTime = time.Now().Add(time.Hour)
	return authCtx, nil
}

func (m *MockTunnelAuthenticator) ValidateAuth(ctx context.Context, authCtx *AuthContext) error {
	if m.ValidateAuthFunc != nil {
		return m.ValidateAuthFunc(ctx, authCtx)
	}
	if time.Now().After(authCtx.ExpiryTime) {
		return ErrAuthenticationExpired
	}
	return nil
}

func (m *MockTunnelAuthenticator) GetTLSConfig(credentials AuthCredentials) (*tls.Config, error) {
	if m.GetTLSConfigFunc != nil {
		return m.GetTLSConfigFunc(credentials)
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
	}, nil
}

func (m *MockTunnelAuthenticator) SupportedMethods() []AuthMethod {
	if m.SupportedMethodsFunc != nil {
		return m.SupportedMethodsFunc()
	}
	return []AuthMethod{AuthMethodToken, AuthMethodCertificate}
}

// TestAuthContext validates AuthContext structure
func TestAuthContext(t *testing.T) {
	ctx := &AuthContext{
		Workspace: logicalcluster.Name("root:test"),
		UserInfo: &UserInfo{
			Username: "admin",
			Groups:   []string{"system:masters"},
			UID:      "user-123",
			Extra: map[string][]string{
				"scopes": {"full"},
			},
		},
		Permissions: []Permission{
			{
				Resource:     "tunnels",
				Verb:         "create",
				Namespace:    "",
				ResourceName: "",
			},
		},
		AuthTime:   time.Now(),
		ExpiryTime: time.Now().Add(time.Hour),
		SessionID:  "session-123",
		ClientIP:   "192.168.1.1",
		UserAgent:  "kcp-syncer/1.0",
		Metadata: map[string]interface{}{
			"region": "us-west-2",
		},
	}

	assert.Equal(t, logicalcluster.Name("root:test"), ctx.Workspace)
	assert.Equal(t, "admin", ctx.UserInfo.Username)
	assert.Contains(t, ctx.UserInfo.Groups, "system:masters")
	assert.Equal(t, "user-123", ctx.UserInfo.UID)
	assert.Equal(t, []string{"full"}, ctx.UserInfo.Extra["scopes"])
	assert.Len(t, ctx.Permissions, 1)
	assert.Equal(t, "tunnels", ctx.Permissions[0].Resource)
	assert.Equal(t, "create", ctx.Permissions[0].Verb)
	assert.Equal(t, "session-123", ctx.SessionID)
	assert.Equal(t, "192.168.1.1", ctx.ClientIP)
	assert.Equal(t, "us-west-2", ctx.Metadata["region"])
}

// TestTunnelAuthenticator validates authenticator behavior
func TestTunnelAuthenticator(t *testing.T) {
	authenticator := &MockTunnelAuthenticator{}

	t.Run("successful authentication", func(t *testing.T) {
		ctx := context.Background()
		creds := AuthCredentials{
			Method: AuthMethodToken,
			Token:  "valid-token",
		}
		workspace := logicalcluster.Name("root:test")

		authCtx, err := authenticator.Authenticate(ctx, creds, workspace)
		require.NoError(t, err)
		assert.NotNil(t, authCtx)
		assert.Equal(t, workspace, authCtx.Workspace)
		assert.Equal(t, "test-user", authCtx.UserInfo.Username)
		assert.NotEmpty(t, authCtx.SessionID)
	})

	t.Run("failed authentication", func(t *testing.T) {
		authenticator := &MockTunnelAuthenticator{
			AuthenticateFunc: func(ctx context.Context, credentials AuthCredentials, workspace logicalcluster.Name) (*AuthContext, error) {
				return nil, ErrAuthenticationFailed
			},
		}

		ctx := context.Background()
		creds := AuthCredentials{
			Method: AuthMethodToken,
			Token:  "invalid-token",
		}

		authCtx, err := authenticator.Authenticate(ctx, creds, logicalcluster.Name("test"))
		assert.Error(t, err)
		assert.Equal(t, ErrAuthenticationFailed, err)
		assert.Nil(t, authCtx)
	})

	t.Run("refresh auth", func(t *testing.T) {
		ctx := context.Background()
		authCtx := &AuthContext{
			ExpiryTime: time.Now().Add(time.Minute),
		}

		refreshed, err := authenticator.RefreshAuth(ctx, authCtx)
		require.NoError(t, err)
		assert.True(t, refreshed.ExpiryTime.After(time.Now().Add(time.Minute*30)))
	})

	t.Run("validate expired auth", func(t *testing.T) {
		ctx := context.Background()
		authCtx := &AuthContext{
			ExpiryTime: time.Now().Add(-time.Hour), // Already expired
		}

		err := authenticator.ValidateAuth(ctx, authCtx)
		assert.Error(t, err)
		assert.Equal(t, ErrAuthenticationExpired, err)
	})

	t.Run("get TLS config", func(t *testing.T) {
		creds := AuthCredentials{
			Method: AuthMethodCertificate,
		}

		tlsConfig, err := authenticator.GetTLSConfig(creds)
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig)
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	})

	t.Run("supported methods", func(t *testing.T) {
		methods := authenticator.SupportedMethods()
		assert.Contains(t, methods, AuthMethodToken)
		assert.Contains(t, methods, AuthMethodCertificate)
	})
}

// MockTunnelAuthorizer implements TunnelAuthorizer for testing
type MockTunnelAuthorizer struct {
	AuthorizeFunc             func(ctx context.Context, authCtx *AuthContext, permission Permission) error
	GetUserPermissionsFunc    func(ctx context.Context, authCtx *AuthContext) ([]Permission, error)
	CanCreateTunnelFunc       func(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error
	CanAccessTunnelFunc       func(ctx context.Context, authCtx *AuthContext, connectionID ConnectionID) error
	CanManageConnectionsFunc  func(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error
}

func (m *MockTunnelAuthorizer) Authorize(ctx context.Context, authCtx *AuthContext, permission Permission) error {
	if m.AuthorizeFunc != nil {
		return m.AuthorizeFunc(ctx, authCtx, permission)
	}
	return nil
}

func (m *MockTunnelAuthorizer) GetUserPermissions(ctx context.Context, authCtx *AuthContext) ([]Permission, error) {
	if m.GetUserPermissionsFunc != nil {
		return m.GetUserPermissionsFunc(ctx, authCtx)
	}
	return authCtx.Permissions, nil
}

func (m *MockTunnelAuthorizer) CanCreateTunnel(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error {
	if m.CanCreateTunnelFunc != nil {
		return m.CanCreateTunnelFunc(ctx, authCtx, workspace)
	}
	return nil
}

func (m *MockTunnelAuthorizer) CanAccessTunnel(ctx context.Context, authCtx *AuthContext, connectionID ConnectionID) error {
	if m.CanAccessTunnelFunc != nil {
		return m.CanAccessTunnelFunc(ctx, authCtx, connectionID)
	}
	return nil
}

func (m *MockTunnelAuthorizer) CanManageConnections(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error {
	if m.CanManageConnectionsFunc != nil {
		return m.CanManageConnectionsFunc(ctx, authCtx, workspace)
	}
	return nil
}

// TestTunnelAuthorizer validates authorizer behavior
func TestTunnelAuthorizer(t *testing.T) {
	authorizer := &MockTunnelAuthorizer{}

	authCtx := &AuthContext{
		Workspace: logicalcluster.Name("root:test"),
		UserInfo: &UserInfo{
			Username: "user",
			Groups:   []string{"users"},
		},
		Permissions: []Permission{
			{Resource: "tunnels", Verb: "create"},
			{Resource: "connections", Verb: "list"},
		},
	}

	t.Run("authorize allowed", func(t *testing.T) {
		ctx := context.Background()
		perm := Permission{Resource: "tunnels", Verb: "create"}

		err := authorizer.Authorize(ctx, authCtx, perm)
		assert.NoError(t, err)
	})

	t.Run("authorize denied", func(t *testing.T) {
		authorizer := &MockTunnelAuthorizer{
			AuthorizeFunc: func(ctx context.Context, authCtx *AuthContext, permission Permission) error {
				return ErrUnauthorized
			},
		}

		ctx := context.Background()
		perm := Permission{Resource: "tunnels", Verb: "delete"}

		err := authorizer.Authorize(ctx, authCtx, perm)
		assert.Error(t, err)
		assert.Equal(t, ErrUnauthorized, err)
	})

	t.Run("get user permissions", func(t *testing.T) {
		ctx := context.Background()

		perms, err := authorizer.GetUserPermissions(ctx, authCtx)
		require.NoError(t, err)
		assert.Len(t, perms, 2)
		assert.Equal(t, "tunnels", perms[0].Resource)
		assert.Equal(t, "create", perms[0].Verb)
	})

	t.Run("can create tunnel", func(t *testing.T) {
		ctx := context.Background()
		workspace := logicalcluster.Name("root:test")

		err := authorizer.CanCreateTunnel(ctx, authCtx, workspace)
		assert.NoError(t, err)
	})
}

// TestAuthErrors validates authentication error types
func TestAuthErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "authentication failed",
			err:  ErrAuthenticationFailed,
			msg:  "authentication failed",
		},
		{
			name: "authentication expired",
			err:  ErrAuthenticationExpired,
			msg:  "authentication expired",
		},
		{
			name: "unauthorized",
			err:  ErrUnauthorized,
			msg:  "unauthorized",
		},
		{
			name: "permission denied",
			err:  ErrPermissionDenied,
			msg:  "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.msg)
			assert.True(t, errors.Is(tt.err, tt.err))
		})
	}
}
```

**Verification**: Run `go test ./pkg/tunneler/interfaces/... -v` to ensure all tests pass.

### 2. Missing ConnectionID Type
**Severity**: CRITICAL
**File**: pkg/tunneler/interfaces/auth.go
**Problem**: The code references `ConnectionID` type in `CanAccessTunnel` method but this type is not defined anywhere.
**Fix Instructions**: Add the following to `pkg/tunneler/interfaces/basic_types.go`:

```go
// ConnectionID uniquely identifies a tunnel connection across the system.
// Format: "<workspace>/<protocol>/<tunnel-id>"
type ConnectionID string

// String returns the string representation of the connection ID
func (id ConnectionID) String() string {
	return string(id)
}

// ParseConnectionID parses a connection ID string into components
func ParseConnectionID(id string) (workspace logicalcluster.Name, protocol TunnelProtocol, tunnelID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid connection ID format: %s", id)
	}
	return logicalcluster.Name(parts[0]), TunnelProtocol(parts[1]), parts[2], nil
}

// NewConnectionID creates a new connection ID from components
func NewConnectionID(workspace logicalcluster.Name, protocol TunnelProtocol, tunnelID string) ConnectionID {
	return ConnectionID(fmt.Sprintf("%s/%s/%s", workspace, protocol, tunnelID))
}
```

**Verification**: Add `"strings"` import and compile with `go build ./pkg/tunneler/interfaces/...`

### 3. Missing Package Documentation
**Severity**: CRITICAL
**File**: pkg/tunneler/interfaces/
**Problem**: No doc.go file exists for package-level documentation.
**Fix Instructions**: Create `pkg/tunneler/interfaces/doc.go`:

```go
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

// Package interfaces defines the core abstractions for the KCP tunneling subsystem.
//
// This package provides protocol-agnostic interfaces for establishing secure,
// bidirectional communication channels between KCP control plane and edge clusters.
// The design supports multiple tunneling protocols (WebSocket, gRPC, HTTP) while
// maintaining workspace isolation and comprehensive security controls.
//
// # Core Components
//
// The package is organized around several key abstractions:
//
// - Tunnel: The primary interface for bidirectional communication channels
// - TunnelFactory: Factory pattern for creating protocol-specific tunnel instances
// - TunnelAuthenticator: Handles authentication of tunnel connections
// - TunnelAuthorizer: Manages authorization and permission checks
// - TunnelAuthManager: Combines authentication and authorization with caching
//
// # Authentication
//
// The system supports multiple authentication methods:
//   - Bearer tokens for simple API key authentication
//   - Client certificates for mTLS connections
//   - OAuth2 for third-party authentication flows
//   - Kubernetes service accounts for native K8s integration
//
// # Workspace Isolation
//
// All tunnel operations are workspace-aware, ensuring complete isolation between
// different logical clusters. Each tunnel is bound to a specific workspace and
// cannot access resources from other workspaces.
//
// # Protocol Support
//
// The abstraction layer supports multiple tunneling protocols:
//   - WebSocket: For browser-compatible bidirectional streaming
//   - gRPC: For high-performance RPC with streaming support
//   - HTTP: For simple request-response patterns
//
// # Usage Example
//
//	factory := NewTunnelFactory()
//	tunnel, err := factory.CreateTunnel(TunnelOptions{
//	    Protocol:  TunnelProtocolWebSocket,
//	    Workspace: logicalcluster.Name("root:my-workspace"),
//	})
//	if err != nil {
//	    return err
//	}
//	defer tunnel.Close()
//	
//	if err := tunnel.Connect(ctx); err != nil {
//	    return err
//	}
//	
//	// Send data
//	if err := tunnel.Send(ctx, []byte("hello")); err != nil {
//	    return err
//	}
//	
//	// Receive data
//	data, err := tunnel.Receive(ctx)
//	if err != nil {
//	    return err
//	}
//
// # Thread Safety
//
// All interfaces in this package are designed to be thread-safe. Implementations
// must support concurrent operations from multiple goroutines without external
// synchronization.
//
// # Error Handling
//
// The package defines standard error types for common failure scenarios:
//   - ErrUnsupportedProtocol: Requested protocol is not supported
//   - ErrTunnelClosed: Operation attempted on closed tunnel
//   - ErrAuthenticationFailed: Authentication credentials invalid
//   - ErrUnauthorized: User lacks required permissions
//
// Implementations should use these standard errors where appropriate and may
// define additional protocol-specific errors as needed.
package interfaces
```

**Verification**: Run `go doc ./pkg/tunneler/interfaces` to verify documentation is accessible.

### 4. Import Validation
**Severity**: CRITICAL  
**File**: pkg/tunneler/interfaces/basic_types.go
**Problem**: Missing "strings" import needed for ConnectionID implementation.
**Fix Instructions**: Add to imports in `pkg/tunneler/interfaces/basic_types.go`:

```go
import (
	"fmt"
	"strings"  // Add this line
	"time"
)
```

**Verification**: Run `go build ./pkg/tunneler/interfaces/...` to ensure compilation succeeds.

## Non-Critical Issues

### 1. Example Code Missing
**Severity**: MINOR
**File**: pkg/tunneler/interfaces/
**Problem**: No example_test.go file showing usage patterns.
**Fix Instructions**: Create `pkg/tunneler/interfaces/example_test.go`:

```go
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

package interfaces_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kcp-dev/kcp/pkg/tunneler/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
)

// ExampleTunnel demonstrates basic tunnel usage
func ExampleTunnel() {
	// This would typically come from a factory
	var tunnel interfaces.Tunnel

	ctx := context.Background()

	// Connect to remote endpoint
	if err := tunnel.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer tunnel.Close()

	// Send a message
	message := []byte("Hello, KCP!")
	if err := tunnel.Send(ctx, message); err != nil {
		log.Fatal(err)
	}

	// Receive response
	response, err := tunnel.Receive(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Received: %s\n", response)
}

// ExampleTunnelFactory demonstrates creating tunnels with factory
func ExampleTunnelFactory() {
	// This would be a real implementation
	var factory interfaces.TunnelFactory

	// Configure tunnel options
	opts := interfaces.TunnelOptions{
		Protocol:             interfaces.TunnelProtocolWebSocket,
		Workspace:            logicalcluster.Name("root:my-workspace"),
		MaxReconnectAttempts: 5,
		ReconnectInterval:    time.Second * 2,
		EnableCompression:    true,
	}

	// Create tunnel instance
	tunnel, err := factory.CreateTunnel(opts)
	if err != nil {
		log.Fatal(err)
	}

	// Use the tunnel
	ctx := context.Background()
	if err := tunnel.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer tunnel.Close()

	fmt.Printf("Connected via %s to workspace %s\n", tunnel.Protocol(), tunnel.Workspace())
}

// ExampleAuthCredentials demonstrates setting up authentication
func ExampleAuthCredentials() {
	// Token-based authentication
	tokenCreds := interfaces.AuthCredentials{
		Method: interfaces.AuthMethodToken,
		Token:  "my-bearer-token",
	}

	// Certificate-based authentication
	certCreds := interfaces.AuthCredentials{
		Method:          interfaces.AuthMethodCertificate,
		CertificateFile: "/path/to/client.crt",
		KeyFile:         "/path/to/client.key",
		CAFile:          "/path/to/ca.crt",
	}

	// OAuth2 authentication
	oauthCreds := interfaces.AuthCredentials{
		Method: interfaces.AuthMethodOAuth2,
		OAuth2Config: &interfaces.OAuth2Config{
			ClientID:     "my-client-id",
			ClientSecret: "my-client-secret",
			TokenURL:     "https://auth.example.com/token",
			Scopes:       []string{"tunnel:create", "tunnel:manage"},
		},
	}

	fmt.Printf("Token auth: %v\n", tokenCreds.Method)
	fmt.Printf("Cert auth: %v\n", certCreds.Method)
	fmt.Printf("OAuth2 auth: %v\n", oauthCreds.Method)
}
```

**Verification**: Run `go test -run Example ./pkg/tunneler/interfaces/`

### 2. Benchmark Tests Missing
**Severity**: MINOR
**File**: pkg/tunneler/interfaces/
**Problem**: No benchmark tests for performance validation.
**Fix Instructions**: Add to `pkg/tunneler/interfaces/tunnel_test.go`:

```go
// BenchmarkTunnelSend measures send operation performance
func BenchmarkTunnelSend(b *testing.B) {
	tunnel := &MockTunnel{
		state:     TunnelStateConnected,
		workspace: logicalcluster.Name("test"),
		protocol:  TunnelProtocolWebSocket,
	}

	ctx := context.Background()
	data := make([]byte, 1024) // 1KB message

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = tunnel.Send(ctx, data)
	}

	b.ReportMetric(float64(b.N*len(data))/float64(b.Elapsed().Seconds()), "bytes/sec")
}

// BenchmarkTunnelReceive measures receive operation performance
func BenchmarkTunnelReceive(b *testing.B) {
	data := make([]byte, 1024)
	tunnel := &MockTunnel{
		ReceiveFunc: func(ctx context.Context) ([]byte, error) {
			return data, nil
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tunnel.Receive(ctx)
	}
}
```

**Verification**: Run `go test -bench=. ./pkg/tunneler/interfaces/`

### 3. README Documentation
**Severity**: MINOR
**File**: pkg/tunneler/
**Problem**: No README.md file explaining the tunneler subsystem.
**Fix Instructions**: Create `pkg/tunneler/README.md`:

```markdown
# KCP Tunneler Subsystem

The tunneler subsystem provides secure, bidirectional communication channels between the KCP control plane and edge clusters.

## Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│  KCP Control    │────▶│   Tunneler   │────▶│    Edge     │
│     Plane       │◀────│  Subsystem   │◀────│   Cluster   │
└─────────────────┘     └──────────────┘     └─────────────┘
        ▲                      │                      ▲
        │                      │                      │
        └──────────────────────┴──────────────────────┘
              Workspace-Isolated Communication
```

## Components

- **interfaces/**: Core abstractions and contracts
- **websocket/**: WebSocket protocol implementation
- **grpc/**: gRPC protocol implementation  
- **auth/**: Authentication and authorization implementations
- **manager/**: Connection management and pooling

## Usage

See [interfaces/doc.go](interfaces/doc.go) for detailed API documentation and usage examples.

## Testing

```bash
# Run unit tests
go test ./pkg/tunneler/...

# Run with coverage
go test -cover ./pkg/tunneler/...

# Run benchmarks
go test -bench=. ./pkg/tunneler/interfaces/
```

## Security

All tunnel connections require authentication and operate within workspace boundaries for complete isolation.
```

**Verification**: Ensure file renders correctly in markdown viewer.

## Test Coverage Report

The implementation currently has 0% test coverage. After implementing the test files above, expected coverage should be:
- tunnel.go: ~85% coverage
- basic_types.go: ~90% coverage  
- auth.go: ~85% coverage

Run coverage with: `go test -cover ./pkg/tunneler/interfaces/`

## KCP Pattern Compliance Checklist

✅ **Workspace Isolation**: All interfaces include logicalcluster.Name support
✅ **Copyright Headers**: All files have proper Apache 2.0 license headers
✅ **Error Handling**: Standard error variables defined and exported
✅ **Thread Safety**: Documentation specifies thread-safety requirements
✅ **Interface Design**: Follows Go interface best practices
❌ **Testing**: Missing comprehensive test coverage
❌ **Documentation**: Missing package documentation (doc.go)
❌ **Examples**: Missing example code
✅ **Naming Conventions**: Follows Go and KCP naming standards
✅ **Dependencies**: Properly imports KCP packages (logicalcluster)

## Linting Issues

Run `golangci-lint run ./pkg/tunneler/interfaces/...` after fixes to ensure no linting issues remain.

## Final Notes

1. The split into sync-02a and sync-02b successfully keeps each PR under 800 lines
2. Core interfaces are well-designed and follow KCP patterns
3. Main issues are missing tests and documentation
4. After implementing fixes, run full test suite before creating PR
5. Consider adding integration tests in a follow-up PR

Total estimated lines after fixes:
- Implementation: ~579 lines (current)
- Tests: ~1200 lines (new)
- Documentation: ~200 lines (new)
- Total per PR: Still split across sync-02a and sync-02b to maintain size limits