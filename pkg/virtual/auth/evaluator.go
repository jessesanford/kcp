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
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
	
	"github.com/kcp-dev/logicalcluster/v3"
)

// BasicEvaluator implements core authorization evaluation for virtual workspaces.
type BasicEvaluator struct {
	config        *Config
	decisionCache map[string]*cachedDecision
	cacheMutex    sync.RWMutex
}

// cachedDecision represents a cached authorization decision.
type cachedDecision struct {
	result     AuthorizationResult
	validUntil time.Time
}

// NewBasicEvaluator creates a new BasicEvaluator.
func NewBasicEvaluator(config *Config) Evaluator {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &BasicEvaluator{
		config:        config,
		decisionCache: make(map[string]*cachedDecision),
	}
}

// Authorize evaluates if the subject can perform the specified action.
func (e *BasicEvaluator) Authorize(ctx context.Context, subject *Subject, permission Permission) AuthorizationResult {
	if subject == nil {
		return AuthorizationResult{
			Allowed: false,
			Reason:  "subject cannot be nil",
			Error:   fmt.Errorf("no subject provided"),
		}
	}
	
	// Check cache first
	cacheKey := e.buildCacheKey(subject, permission)
	if cached := e.getCachedDecision(cacheKey); cached != nil {
		return *cached
	}
	
	// Evaluate permission
	result := e.evaluatePermission(ctx, subject, permission)
	
	// Cache decision
	e.cacheDecision(cacheKey, &result)
	
	klog.V(4).InfoS("auth: authorization decision", 
		"subject", subject.User.GetName(),
		"verb", permission.Verb,
		"resource", permission.Resource,
		"allowed", result.Allowed)
	
	return result
}

// evaluatePermission performs core authorization logic.
func (e *BasicEvaluator) evaluatePermission(ctx context.Context, subject *Subject, permission Permission) AuthorizationResult {
	// System users get elevated privileges
	if e.isSystemUser(subject) {
		return AuthorizationResult{
			Allowed: true,
			Reason:  "system user granted access",
		}
	}
	
	// Enforce workspace isolation
	if e.config.WorkspaceIsolation {
		if !e.canAccessWorkspace(subject, permission.LogicalCluster) {
			return AuthorizationResult{
				Allowed: false,
				Reason:  "workspace isolation violation",
				Error:   fmt.Errorf("cannot access cluster %s", permission.LogicalCluster),
			}
		}
	}
	
	// Check specific permission
	if allowed, reason := e.checkPermission(subject, permission); !allowed {
		return AuthorizationResult{
			Allowed: false,
			Reason:  reason,
			Error:   fmt.Errorf("permission denied: %s", reason),
		}
	}
	
	return AuthorizationResult{
		Allowed: true,
		Reason:  "permission granted",
	}
}

// isSystemUser determines if the subject is a system user.
func (e *BasicEvaluator) isSystemUser(subject *Subject) bool {
	userName := subject.User.GetName()
	
	systemPrefixes := []string{"system:", "kubernetes:", "kcp:"}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(userName, prefix) {
			return true
		}
	}
	
	// Check system groups
	for _, group := range subject.Groups {
		systemGroups := []string{
			"system:masters",
			"system:admin",
			"system:cluster-admins",
		}
		for _, sysGroup := range systemGroups {
			if group == sysGroup {
				return true
			}
		}
	}
	
	return false
}

// canAccessWorkspace determines workspace access.
func (e *BasicEvaluator) canAccessWorkspace(subject *Subject, cluster logicalcluster.Name) bool {
	// Same workspace
	if subject.LogicalCluster == cluster {
		return true
	}
	
	// Root workspace access
	if cluster == "root" || strings.HasPrefix(string(cluster), "root:") {
		for _, group := range subject.Groups {
			if strings.Contains(group, "admin") || strings.Contains(group, "root") {
				return true
			}
		}
	}
	
	// Cross-workspace access
	for _, group := range subject.Groups {
		if strings.Contains(group, "workspace-admin") || 
		   strings.Contains(group, "cluster-admin") {
			return true
		}
	}
	
	return false
}

// checkPermission evaluates specific permission.
func (e *BasicEvaluator) checkPermission(subject *Subject, permission Permission) (bool, string) {
	verb := permission.Verb
	
	// Admin access
	for _, group := range subject.Groups {
		if strings.Contains(group, "admin") {
			return true, "admin access"
		}
	}
	
	// Read operations
	readVerbs := []string{"get", "list", "watch"}
	for _, readVerb := range readVerbs {
		if verb == readVerb {
			return true, "read access granted"
		}
	}
	
	// Write operations
	writeVerbs := []string{"create", "update", "patch", "delete"}
	for _, writeVerb := range writeVerbs {
		if verb == writeVerb {
			// Check write access
			for _, group := range subject.Groups {
				if strings.Contains(group, "writer") || 
				   strings.Contains(group, "editor") {
					return true, "write access granted"
				}
			}
			return false, "write access denied"
		}
	}
	
	// Wildcard
	if verb == "*" {
		for _, group := range subject.Groups {
			if strings.Contains(group, "admin") {
				return true, "full access granted"
			}
		}
		return false, "full access denied"
	}
	
	return false, "unknown verb"
}

// CanAccess is a convenience method for simple access checks.
func (e *BasicEvaluator) CanAccess(ctx context.Context, subject *Subject, verb, resource string, cluster logicalcluster.Name) bool {
	permission := Permission{
		Verb:           verb,
		Resource:       resource,
		LogicalCluster: cluster,
	}
	
	result := e.Authorize(ctx, subject, permission)
	return result.Allowed
}

// GetPermissions returns permissions for a subject in a workspace.
func (e *BasicEvaluator) GetPermissions(ctx context.Context, subject *Subject, cluster logicalcluster.Name) ([]Permission, error) {
	if subject == nil {
		return nil, fmt.Errorf("subject cannot be nil")
	}
	
	var permissions []Permission
	
	// Basic read permissions
	readResources := []string{"pods", "services", "configmaps", "namespaces"}
	for _, resource := range readResources {
		for _, verb := range []string{"get", "list", "watch"} {
			permissions = append(permissions, Permission{
				Verb:           verb,
				Resource:       resource,
				LogicalCluster: cluster,
			})
		}
	}
	
	// Write permissions for appropriate groups
	for _, group := range subject.Groups {
		if strings.Contains(group, "writer") || strings.Contains(group, "editor") {
			writeResources := []string{"pods", "services", "configmaps"}
			for _, resource := range writeResources {
				for _, verb := range []string{"create", "update", "patch"} {
					permissions = append(permissions, Permission{
						Verb:           verb,
						Resource:       resource,
						LogicalCluster: cluster,
					})
				}
			}
			break
		}
	}
	
	// Admin permissions
	for _, group := range subject.Groups {
		if strings.Contains(group, "admin") {
			permissions = append(permissions, Permission{
				Verb:           "*",
				Resource:       "*",
				LogicalCluster: cluster,
			})
			break
		}
	}
	
	return permissions, nil
}

// buildCacheKey creates a cache key for the authorization decision.
func (e *BasicEvaluator) buildCacheKey(subject *Subject, permission Permission) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s",
		subject.User.GetName(),
		strings.Join(subject.Groups, ","),
		permission.Verb,
		permission.Resource,
		permission.LogicalCluster)
}

// getCachedDecision retrieves cached decision if valid.
func (e *BasicEvaluator) getCachedDecision(key string) *AuthorizationResult {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()
	
	cached, exists := e.decisionCache[key]
	if !exists {
		return nil
	}
	
	if time.Now().After(cached.validUntil) {
		delete(e.decisionCache, key)
		return nil
	}
	
	return &cached.result
}

// cacheDecision stores authorization decision.
func (e *BasicEvaluator) cacheDecision(key string, result *AuthorizationResult) {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()
	
	// Simple cleanup
	if len(e.decisionCache) >= e.config.CacheSize {
		for k, cached := range e.decisionCache {
			if time.Now().After(cached.validUntil) {
				delete(e.decisionCache, k)
				break
			}
		}
	}
	
	e.decisionCache[key] = &cachedDecision{
		result:     *result,
		validUntil: time.Now().Add(e.config.CacheTTL),
	}
}