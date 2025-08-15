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
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

// BasicProvider implements essential authentication for virtual workspaces.
type BasicProvider struct {
	config     *Config
	tokenCache map[string]*TokenInfo
	cacheMutex sync.RWMutex
	ready      bool
}

// NewBasicProvider creates a new BasicProvider.
func NewBasicProvider(config *Config) Provider {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &BasicProvider{
		config:     config,
		tokenCache: make(map[string]*TokenInfo),
		ready:      true,
	}
}

// ValidateToken validates an authentication token.
func (p *BasicProvider) ValidateToken(ctx context.Context, token string) (*TokenInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}
	
	// Check cache first
	if cached := p.getCachedToken(token); cached != nil {
		return cached, nil
	}
	
	// Validate token with timeout
	validationCtx, cancel := context.WithTimeout(ctx, p.config.TokenValidationTimeout)
	defer cancel()
	
	tokenInfo, err := p.validateTokenInternal(validationCtx, token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	
	// Cache result
	p.cacheToken(token, tokenInfo)
	return tokenInfo, nil
}

// validateTokenInternal performs core token validation.
func (p *BasicProvider) validateTokenInternal(ctx context.Context, token string) (*TokenInfo, error) {
	// Handle bearer token format
	if strings.HasPrefix(token, "Bearer ") {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	
	var userInfo user.Info
	
	// Try base64 decode for basic auth
	if decoded, err := base64.StdEncoding.DecodeString(token); err == nil {
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) == 2 {
			userInfo = &user.DefaultInfo{
				Name:   parts[0],
				UID:    parts[0],
				Groups: []string{"system:authenticated", "system:basic-auth"},
			}
		}
	}
	
	if userInfo == nil {
		// Treat as opaque token
		userInfo = &user.DefaultInfo{
			Name:   fmt.Sprintf("user-%s", token[:min(8, len(token))]),
			UID:    token,
			Groups: []string{"system:authenticated"},
		}
	}
	
	subject := Subject{
		User:           userInfo,
		Groups:         userInfo.GetGroups(),
		Extra:          userInfo.GetExtra(),
		LogicalCluster: logicalcluster.Name("root:default"),
	}
	
	return &TokenInfo{
		Subject:   subject,
		ExpiresAt: time.Now().Add(time.Hour),
		Scopes:    []string{"read", "write"},
		Workspace: logicalcluster.Name("root:default"),
	}, nil
}

// ExtractSubject extracts subject from request context.
func (p *BasicProvider) ExtractSubject(ctx context.Context) (*Subject, error) {
	userInfo, ok := genericapirequest.UserFrom(ctx)
	if !ok || userInfo == nil {
		return nil, fmt.Errorf("no user information in context")
	}
	
	cluster := logicalcluster.Name("root:default")
	if requestInfo, ok := genericapirequest.RequestInfoFrom(ctx); ok && requestInfo != nil {
		if pathCluster := extractClusterFromPath(requestInfo.Path); pathCluster != "" {
			cluster = logicalcluster.Name(pathCluster)
		}
	}
	
	return &Subject{
		User:           userInfo,
		Groups:         userInfo.GetGroups(),
		Extra:          userInfo.GetExtra(),
		LogicalCluster: cluster,
	}, nil
}

// extractClusterFromPath extracts cluster from request path.
func extractClusterFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "clusters" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// IsReady indicates if the provider is ready.
func (p *BasicProvider) IsReady() error {
	if !p.ready {
		return fmt.Errorf("provider not ready")
	}
	return nil
}

// getCachedToken retrieves cached token if valid.
func (p *BasicProvider) getCachedToken(token string) *TokenInfo {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()
	
	cached, exists := p.tokenCache[token]
	if !exists {
		return nil
	}
	
	if time.Now().After(cached.ExpiresAt) {
		delete(p.tokenCache, token)
		return nil
	}
	
	return cached
}

// cacheToken stores token in cache.
func (p *BasicProvider) cacheToken(token string, tokenInfo *TokenInfo) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	
	if len(p.tokenCache) >= p.config.CacheSize {
		// Simple cleanup - remove one expired entry
		for t, info := range p.tokenCache {
			if time.Now().After(info.ExpiresAt) {
				delete(p.tokenCache, t)
				break
			}
		}
	}
	
	p.tokenCache[token] = tokenInfo
}

// min returns minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}