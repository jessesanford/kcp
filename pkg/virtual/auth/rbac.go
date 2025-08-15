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
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// RBACEvaluator implements advanced RBAC evaluation with policy aggregation.
// It extends the basic authorization with comprehensive role-based access control,
// supporting inheritance, aggregation, and workspace isolation.
type RBACEvaluator struct {
	config             *RBACConfig
	policyEngine       *PolicyEngine
	cache              *PermissionCache
	roleResolver       RoleResolver
	bindingCache       map[string]*RoleBindingEntry
	policyCache        map[string]*PolicyDecision
	inheritanceGraph   map[logicalcluster.Name][]logicalcluster.Name
	mu                 sync.RWMutex
	lastCacheUpdate    time.Time
	cacheInvalidations int64
}

// RBACConfig defines configuration for RBAC evaluation.
type RBACConfig struct {
	// CacheConfig defines caching behavior
	CacheConfig *CacheConfig
	
	// PolicyConfig defines policy engine configuration
	PolicyConfig *PolicyConfig
	
	// InheritanceEnabled controls workspace permission inheritance
	InheritanceEnabled bool
	
	// AggregationEnabled controls role aggregation
	AggregationEnabled bool
	
	// MaxInheritanceDepth limits inheritance traversal depth
	MaxInheritanceDepth int
	
	// CacheRefreshInterval defines how often to refresh cached data
	CacheRefreshInterval time.Duration
	
	// EnablePolicyAudit controls detailed policy audit logging
	EnablePolicyAudit bool
}

// DefaultRBACConfig returns default RBAC configuration.
func DefaultRBACConfig() *RBACConfig {
	return &RBACConfig{
		CacheConfig:          DefaultCacheConfig(),
		PolicyConfig:         DefaultPolicyConfig(),
		InheritanceEnabled:   true,
		AggregationEnabled:   true,
		MaxInheritanceDepth:  10,
		CacheRefreshInterval: 5 * time.Minute,
		EnablePolicyAudit:    false,
	}
}

// RoleBindingEntry represents a cached role binding with metadata.
type RoleBindingEntry struct {
	Binding       *rbacv1.RoleBinding
	ClusterBinding *rbacv1.ClusterRoleBinding
	Workspace     logicalcluster.Name
	LastUpdated   time.Time
	ExpiresAt     time.Time
}

// PolicyDecision represents a cached policy evaluation result.
type PolicyDecision struct {
	UserInfo        user.Info
	Attributes      authorizer.Attributes
	Decision        authorizer.Decision
	Reason          string
	Workspace       logicalcluster.Name
	RolesApplied    []string
	InheritancePath []logicalcluster.Name
	EvaluatedAt     time.Time
	ExpiresAt       time.Time
}

// RoleResolver provides role resolution capabilities.
type RoleResolver interface {
	// ResolveRoles resolves all roles for a user in a workspace
	ResolveRoles(ctx context.Context, userInfo user.Info, workspace logicalcluster.Name) ([]string, error)
	
	// GetRoleBindings returns all role bindings for a workspace
	GetRoleBindings(ctx context.Context, workspace logicalcluster.Name) ([]*rbacv1.RoleBinding, error)
	
	// GetClusterRoleBindings returns cluster role bindings
	GetClusterRoleBindings(ctx context.Context) ([]*rbacv1.ClusterRoleBinding, error)
	
	// GetRole returns a specific role
	GetRole(ctx context.Context, workspace logicalcluster.Name, name string) (*rbacv1.Role, error)
	
	// GetClusterRole returns a specific cluster role
	GetClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error)
}

// NewRBACEvaluator creates a new RBAC evaluator with the specified configuration.
func NewRBACEvaluator(config *RBACConfig, roleResolver RoleResolver, policyEngine *PolicyEngine, cache *PermissionCache) *RBACEvaluator {
	if config == nil {
		config = DefaultRBACConfig()
	}
	
	evaluator := &RBACEvaluator{
		config:           config,
		policyEngine:     policyEngine,
		cache:            cache,
		roleResolver:     roleResolver,
		bindingCache:     make(map[string]*RoleBindingEntry),
		policyCache:      make(map[string]*PolicyDecision),
		inheritanceGraph: make(map[logicalcluster.Name][]logicalcluster.Name),
		lastCacheUpdate:  time.Now(),
	}
	
	// Start background cache refresh if configured
	if config.CacheRefreshInterval > 0 {
		go evaluator.refreshCacheLoop()
	}
	
	return evaluator
}

// Evaluate performs comprehensive RBAC evaluation for the given request.
// It considers roles, inheritance, policies, and caching for optimal performance.
func (e *RBACEvaluator) Evaluate(ctx context.Context, userInfo user.Info, attributes authorizer.Attributes, workspace logicalcluster.Name) (*PolicyDecision, error) {
	startTime := time.Now()
	
	// Generate cache key for this evaluation
	cacheKey := e.generateCacheKey(userInfo, attributes, workspace)
	
	// Check cache first
	if cached, found := e.getCachedDecision(cacheKey); found && !e.isCacheExpired(cached) {
		if e.config.EnablePolicyAudit {
			klog.V(4).InfoS("RBAC evaluation cache hit",
				"user", userInfo.GetName(),
				"workspace", workspace,
				"resource", attributes.GetResource(),
				"verb", attributes.GetVerb(),
				"duration", time.Since(startTime))
		}
		return cached, nil
	}
	
	// Perform full evaluation
	decision, err := e.evaluateRequest(ctx, userInfo, attributes, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate RBAC request: %w", err)
	}
	
	// Cache the decision
	e.cacheDecision(cacheKey, decision)
	
	if e.config.EnablePolicyAudit {
		klog.InfoS("RBAC evaluation completed",
			"user", userInfo.GetName(),
			"workspace", workspace,
			"resource", attributes.GetResource(),
			"verb", attributes.GetVerb(),
			"decision", decision.Decision,
			"reason", decision.Reason,
			"roles", decision.RolesApplied,
			"inheritancePath", decision.InheritancePath,
			"duration", time.Since(startTime))
	}
	
	return decision, nil
}

// evaluateRequest performs the core RBAC evaluation logic.
func (e *RBACEvaluator) evaluateRequest(ctx context.Context, userInfo user.Info, attributes authorizer.Attributes, workspace logicalcluster.Name) (*PolicyDecision, error) {
	decision := &PolicyDecision{
		UserInfo:     userInfo,
		Attributes:   attributes,
		Workspace:    workspace,
		EvaluatedAt:  time.Now(),
		ExpiresAt:    time.Now().Add(e.config.CacheConfig.TTL),
	}
	
	// Step 1: Resolve roles for the user in the workspace
	roles, err := e.resolveUserRoles(ctx, userInfo, workspace)
	if err != nil {
		decision.Decision = authorizer.DecisionDeny
		decision.Reason = fmt.Sprintf("failed to resolve user roles: %v", err)
		return decision, nil
	}
	decision.RolesApplied = roles
	
	// Step 2: Check workspace inheritance if enabled
	var inheritancePath []logicalcluster.Name
	if e.config.InheritanceEnabled {
		inheritancePath = e.resolveInheritancePath(workspace)
		decision.InheritancePath = inheritancePath
		
		// Collect inherited roles
		for _, ancestorWorkspace := range inheritancePath {
			inheritedRoles, err := e.resolveUserRoles(ctx, userInfo, ancestorWorkspace)
			if err != nil {
				klog.V(2).InfoS("failed to resolve inherited roles, skipping",
					"workspace", ancestorWorkspace,
					"error", err)
				continue
			}
			roles = append(roles, inheritedRoles...)
		}
	}
	
	// Step 3: Deduplicate roles
	roles = e.deduplicateRoles(roles)
	decision.RolesApplied = roles
	
	// Step 4: Evaluate permissions against roles
	hasPermission, reason, err := e.evaluatePermissions(ctx, roles, attributes, workspace)
	if err != nil {
		decision.Decision = authorizer.DecisionDeny
		decision.Reason = fmt.Sprintf("permission evaluation failed: %v", err)
		return decision, nil
	}
	
	// Step 5: Apply policy engine rules if available
	if e.policyEngine != nil {
		policyResult, err := e.policyEngine.EvaluatePolicy(ctx, &PolicyRequest{
			UserInfo:        userInfo,
			Attributes:      attributes,
			Workspace:       workspace,
			Roles:          roles,
			InheritancePath: inheritancePath,
		})
		if err != nil {
			klog.V(2).InfoS("policy engine evaluation failed, proceeding with RBAC result",
				"error", err)
		} else {
			// Policy engine can override RBAC decision
			if policyResult.Override {
				hasPermission = policyResult.Allow
				reason = fmt.Sprintf("policy override: %s", policyResult.Reason)
			}
		}
	}
	
	// Set final decision
	if hasPermission {
		decision.Decision = authorizer.DecisionAllow
		decision.Reason = reason
	} else {
		decision.Decision = authorizer.DecisionDeny
		decision.Reason = reason
	}
	
	return decision, nil
}

// resolveUserRoles resolves all roles for a user in a specific workspace.
func (e *RBACEvaluator) resolveUserRoles(ctx context.Context, userInfo user.Info, workspace logicalcluster.Name) ([]string, error) {
	roles := make([]string, 0)
	
	// Get role bindings for the workspace
	bindings, err := e.roleResolver.GetRoleBindings(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to get role bindings: %w", err)
	}
	
	// Check each binding
	for _, binding := range bindings {
		if e.isUserMatchedByBinding(userInfo, binding) {
			roles = append(roles, binding.RoleRef.Name)
		}
	}
	
	// Get cluster role bindings
	clusterBindings, err := e.roleResolver.GetClusterRoleBindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster role bindings: %w", err)
	}
	
	// Check each cluster binding
	for _, binding := range clusterBindings {
		if e.isUserMatchedByClusterBinding(userInfo, binding) {
			roles = append(roles, binding.RoleRef.Name)
		}
	}
	
	return roles, nil
}

// isUserMatchedByBinding checks if a user matches a role binding.
func (e *RBACEvaluator) isUserMatchedByBinding(userInfo user.Info, binding *rbacv1.RoleBinding) bool {
	return e.checkSubjectMatch(userInfo, binding.Subjects)
}

// isUserMatchedByClusterBinding checks if a user matches a cluster role binding.
func (e *RBACEvaluator) isUserMatchedByClusterBinding(userInfo user.Info, binding *rbacv1.ClusterRoleBinding) bool {
	return e.checkSubjectMatch(userInfo, binding.Subjects)
}

// checkSubjectMatch checks if a user matches any of the subjects.
func (e *RBACEvaluator) checkSubjectMatch(userInfo user.Info, subjects []rbacv1.Subject) bool {
	userGroups := sets.NewString(userInfo.GetGroups()...)
	
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			if subject.Name == userInfo.GetName() {
				return true
			}
		case rbacv1.GroupKind:
			if userGroups.Has(subject.Name) {
				return true
			}
		case rbacv1.ServiceAccountKind:
			// Handle service account matching
			if e.matchesServiceAccount(userInfo, subject) {
				return true
			}
		}
	}
	
	return false
}

// matchesServiceAccount checks if userInfo matches a service account subject.
func (e *RBACEvaluator) matchesServiceAccount(userInfo user.Info, subject rbacv1.Subject) bool {
	username := userInfo.GetName()
	expectedSAName := fmt.Sprintf("system:serviceaccount:%s:%s", subject.Namespace, subject.Name)
	return username == expectedSAName
}

// evaluatePermissions evaluates if the given roles provide the required permissions.
func (e *RBACEvaluator) evaluatePermissions(ctx context.Context, roles []string, attributes authorizer.Attributes, workspace logicalcluster.Name) (bool, string, error) {
	for _, roleName := range roles {
		// Try to get role first
		role, err := e.roleResolver.GetRole(ctx, workspace, roleName)
		if err == nil && role != nil {
			if e.roleHasPermission(role.Rules, attributes) {
				return true, fmt.Sprintf("allowed by role %s in workspace %s", roleName, workspace), nil
			}
			continue
		}
		
		// Try cluster role
		clusterRole, err := e.roleResolver.GetClusterRole(ctx, roleName)
		if err == nil && clusterRole != nil {
			if e.roleHasPermission(clusterRole.Rules, attributes) {
				return true, fmt.Sprintf("allowed by cluster role %s", roleName), nil
			}
		}
	}
	
	return false, fmt.Sprintf("no role grants permission for %s %s", attributes.GetVerb(), attributes.GetResource()), nil
}

// roleHasPermission checks if a role's rules grant the required permission.
func (e *RBACEvaluator) roleHasPermission(rules []rbacv1.PolicyRule, attributes authorizer.Attributes) bool {
	for _, rule := range rules {
		if e.ruleMatchesAttributes(rule, attributes) {
			return true
		}
	}
	return false
}

// ruleMatchesAttributes checks if a policy rule matches the request attributes.
func (e *RBACEvaluator) ruleMatchesAttributes(rule rbacv1.PolicyRule, attributes authorizer.Attributes) bool {
	// Check verbs
	if !e.matchesVerbs(rule.Verbs, attributes.GetVerb()) {
		return false
	}
	
	// Check resources
	if !e.matchesResources(rule.Resources, attributes.GetResource()) {
		return false
	}
	
	// Check API groups
	if !e.matchesAPIGroups(rule.APIGroups, attributes.GetAPIGroup()) {
		return false
	}
	
	// Check resource names if specified
	if len(rule.ResourceNames) > 0 && attributes.GetName() != "" {
		if !e.matchesResourceNames(rule.ResourceNames, attributes.GetName()) {
			return false
		}
	}
	
	return true
}

// Helper methods for rule matching
func (e *RBACEvaluator) matchesVerbs(ruleVerbs []string, verb string) bool {
	for _, ruleVerb := range ruleVerbs {
		if ruleVerb == "*" || ruleVerb == verb {
			return true
		}
	}
	return false
}

func (e *RBACEvaluator) matchesResources(ruleResources []string, resource string) bool {
	for _, ruleResource := range ruleResources {
		if ruleResource == "*" || ruleResource == resource {
			return true
		}
		// Handle subresources
		if strings.Contains(ruleResource, "/") && strings.HasPrefix(resource, strings.Split(ruleResource, "/")[0]) {
			return true
		}
	}
	return false
}

func (e *RBACEvaluator) matchesAPIGroups(ruleAPIGroups []string, apiGroup string) bool {
	for _, ruleAPIGroup := range ruleAPIGroups {
		if ruleAPIGroup == "*" || ruleAPIGroup == apiGroup {
			return true
		}
	}
	return false
}

func (e *RBACEvaluator) matchesResourceNames(ruleResourceNames []string, resourceName string) bool {
	for _, ruleResourceName := range ruleResourceNames {
		if ruleResourceName == resourceName {
			return true
		}
	}
	return false
}

// Cache management methods
func (e *RBACEvaluator) generateCacheKey(userInfo user.Info, attributes authorizer.Attributes, workspace logicalcluster.Name) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		userInfo.GetName(),
		strings.Join(userInfo.GetGroups(), ","),
		attributes.GetVerb(),
		attributes.GetResource(),
		attributes.GetAPIGroup(),
		workspace)
}

func (e *RBACEvaluator) getCachedDecision(key string) (*PolicyDecision, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	decision, found := e.policyCache[key]
	return decision, found
}

func (e *RBACEvaluator) isCacheExpired(decision *PolicyDecision) bool {
	return time.Now().After(decision.ExpiresAt)
}

func (e *RBACEvaluator) cacheDecision(key string, decision *PolicyDecision) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.policyCache[key] = decision
}

func (e *RBACEvaluator) resolveInheritancePath(workspace logicalcluster.Name) []logicalcluster.Name {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	path := make([]logicalcluster.Name, 0)
	current := workspace
	
	for depth := 0; depth < e.config.MaxInheritanceDepth; depth++ {
		parents, exists := e.inheritanceGraph[current]
		if !exists || len(parents) == 0 {
			break
		}
		
		// Take the first parent (could be extended to handle multiple inheritance)
		parent := parents[0]
		path = append(path, parent)
		current = parent
	}
	
	return path
}

func (e *RBACEvaluator) deduplicateRoles(roles []string) []string {
	seen := sets.NewString()
	result := make([]string, 0, len(roles))
	
	for _, role := range roles {
		if !seen.Has(role) {
			seen.Insert(role)
			result = append(result, role)
		}
	}
	
	sort.Strings(result)
	return result
}

// InvalidateCache invalidates the RBAC evaluation cache.
func (e *RBACEvaluator) InvalidateCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.policyCache = make(map[string]*PolicyDecision)
	e.bindingCache = make(map[string]*RoleBindingEntry)
	e.cacheInvalidations++
	
	klog.V(2).InfoS("RBAC cache invalidated", "invalidationCount", e.cacheInvalidations)
}

// refreshCacheLoop runs in the background to refresh cached data periodically.
func (e *RBACEvaluator) refreshCacheLoop() {
	ticker := time.NewTicker(e.config.CacheRefreshInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		e.refreshCache()
	}
}

func (e *RBACEvaluator) refreshCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	now := time.Now()
	
	// Clean expired policy decisions
	for key, decision := range e.policyCache {
		if now.After(decision.ExpiresAt) {
			delete(e.policyCache, key)
		}
	}
	
	// Clean expired binding entries
	for key, binding := range e.bindingCache {
		if now.After(binding.ExpiresAt) {
			delete(e.bindingCache, key)
		}
	}
	
	e.lastCacheUpdate = now
	klog.V(4).InfoS("RBAC cache refreshed",
		"policyDecisions", len(e.policyCache),
		"bindingEntries", len(e.bindingCache))
}

// GetCacheStats returns cache statistics for monitoring.
func (e *RBACEvaluator) GetCacheStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return map[string]interface{}{
		"policyDecisions":     len(e.policyCache),
		"bindingEntries":      len(e.bindingCache),
		"lastUpdate":          e.lastCacheUpdate,
		"cacheInvalidations":  e.cacheInvalidations,
		"inheritanceGraphSize": len(e.inheritanceGraph),
	}
}