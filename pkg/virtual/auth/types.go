/*
Copyright 2025 The KCP Authors.

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

package auth

import (
	"context"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

// Permission represents a permission that can be granted or denied.
type Permission struct {
	Verb           string
	Resource       string
	Namespace      string
	LogicalCluster logicalcluster.Name
}

// Subject represents an authenticated identity.
type Subject struct {
	User           user.Info
	Groups         []string
	Extra          map[string][]string
	LogicalCluster logicalcluster.Name
}

// AuthorizationResult represents the result of an authorization check.
type AuthorizationResult struct {
	Allowed bool
	Reason  string
	Error   error
}

// TokenInfo represents information about an authentication token.
type TokenInfo struct {
	Subject   Subject
	ExpiresAt time.Time
	Scopes    []string
	Workspace logicalcluster.Name
}

// Provider defines the interface for authentication providers.
type Provider interface {
	ValidateToken(ctx context.Context, token string) (*TokenInfo, error)
	ExtractSubject(ctx context.Context) (*Subject, error)
	IsReady() error
}

// Evaluator defines the interface for authorization evaluators.
type Evaluator interface {
	Authorize(ctx context.Context, subject *Subject, permission Permission) AuthorizationResult
	CanAccess(ctx context.Context, subject *Subject, verb, resource string, cluster logicalcluster.Name) bool
	GetPermissions(ctx context.Context, subject *Subject, cluster logicalcluster.Name) ([]Permission, error)
}

// Config holds configuration for auth components.
type Config struct {
	TokenValidationTimeout time.Duration
	CacheSize              int
	CacheTTL               time.Duration
	WorkspaceIsolation     bool
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		TokenValidationTimeout: 30 * time.Second,
		CacheSize:              1000,
		CacheTTL:               5 * time.Minute,
		WorkspaceIsolation:     true,
	}
}

// AuthAttributes extends authorizer.Attributes with KCP-specific context.
type AuthAttributes struct {
	authorizer.Attributes
	LogicalCluster logicalcluster.Name
	Subject        *Subject
}

// NewAuthAttributes creates a new AuthAttributes instance.
func NewAuthAttributes(attrs authorizer.Attributes, cluster logicalcluster.Name, subject *Subject) *AuthAttributes {
	return &AuthAttributes{
		Attributes:     attrs,
		LogicalCluster: cluster,
		Subject:        subject,
	}
}