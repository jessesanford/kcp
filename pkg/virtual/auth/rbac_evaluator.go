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

package auth

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACEvaluator evaluates Kubernetes RBAC rules for authorization decisions.
// It provides integration with native Kubernetes RBAC by evaluating roles
// and role bindings to determine user permissions.
type RBACEvaluator struct {
	client kubernetes.Interface
}

// NewRBACEvaluator creates a new RBAC evaluator with the given Kubernetes client.
// The evaluator uses the client to fetch RBAC resources and evaluate permissions.
func NewRBACEvaluator(client kubernetes.Interface) *RBACEvaluator {
	return &RBACEvaluator{
		client: client,
	}
}

// Evaluate evaluates RBAC rules for an authorization request.
// It fetches all roles bound to the user and evaluates whether any
// of them grant the requested permission.
func (e *RBACEvaluator) Evaluate(ctx context.Context, req *Request) (*Decision, error) {
	// Get all roles bound to the user
	roles, err := e.getRolesForUser(ctx, req.User, req.Groups, req.Workspace)
	if err != nil {
		return &Decision{
			Allowed:         false,
			Reason:          "Failed to get user roles",
			EvaluationError: err,
		}, nil // Don't return error, return denial decision
	}

	// Evaluate each role to see if any allows the request
	for _, role := range roles {
		if e.roleAllows(role, req) {
			return &Decision{
				Allowed: true,
				Reason:  fmt.Sprintf("Allowed by role: %s", role.Name),
				AuditAnnotations: map[string]string{
					"authorization.rbac.io/decision":  "allow",
					"authorization.rbac.io/role":      role.Name,
					"authorization.rbac.io/namespace": role.Namespace,
				},
			}, nil
		}
	}

	return &Decision{
		Allowed: false,
		Reason:  "No role grants the requested permission",
		AuditAnnotations: map[string]string{
			"authorization.rbac.io/decision": "deny",
			"authorization.rbac.io/reason":   "no-matching-role",
		},
	}, nil
}

// getRolesForUser retrieves all roles bound to a user through role bindings.
// It checks both namespace-scoped roles and cluster roles that apply to the user.
func (e *RBACEvaluator) getRolesForUser(ctx context.Context, user string, groups []string, namespace string) ([]*rbacv1.Role, error) {
	var roles []*rbacv1.Role

	// Get namespace-scoped role bindings
	roleBindings, err := e.client.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list role bindings: %w", err)
	}

	// Process role bindings to find roles for this user
	for _, binding := range roleBindings.Items {
		if e.subjectMatchesUser(binding.Subjects, user, groups) {
			// Get the referenced role
			role, err := e.client.RbacV1().Roles(namespace).Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
			if err != nil {
				// Skip if role not found, don't fail the entire evaluation
				continue
			}
			roles = append(roles, role)
		}
	}

	// Get cluster role bindings that might apply
	clusterRoleBindings, err := e.client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		// Return what we have so far, don't fail completely
		return roles, nil
	}

	// Process cluster role bindings
	for _, binding := range clusterRoleBindings.Items {
		if e.subjectMatchesUser(binding.Subjects, user, groups) {
			// Get the cluster role
			clusterRole, err := e.client.RbacV1().ClusterRoles().Get(ctx, binding.RoleRef.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// Convert cluster role to role format for uniform handling
			roles = append(roles, &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterRole.Name,
					Namespace: namespace, // Use the target namespace
				},
				Rules: clusterRole.Rules,
			})
		}
	}

	return roles, nil
}

// subjectMatchesUser checks if any subject in the list matches the user or their groups.
// It handles both User and Group subjects.
func (e *RBACEvaluator) subjectMatchesUser(subjects []rbacv1.Subject, user string, groups []string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			if subject.Name == user {
				return true
			}
		case rbacv1.GroupKind:
			for _, group := range groups {
				if subject.Name == group {
					return true
				}
			}
		case rbacv1.ServiceAccountKind:
			// For service accounts, match format "system:serviceaccount:namespace:name"
			serviceAccountName := fmt.Sprintf("system:serviceaccount:%s:%s", subject.Namespace, subject.Name)
			if serviceAccountName == user {
				return true
			}
		}
	}
	return false
}

// roleAllows checks if a role allows the requested action.
// It evaluates all policy rules in the role to determine if any match.
func (e *RBACEvaluator) roleAllows(role *rbacv1.Role, req *Request) bool {
	for _, rule := range role.Rules {
		if e.ruleMatches(rule, req) {
			return true
		}
	}
	return false
}

// ruleMatches checks if a policy rule matches the authorization request.
// It validates API groups, resources, and verbs against the request.
func (e *RBACEvaluator) ruleMatches(rule rbacv1.PolicyRule, req *Request) bool {
	// Check API groups - rule must allow the resource's group
	if len(rule.APIGroups) > 0 {
		groupMatches := false
		for _, group := range rule.APIGroups {
			if group == "*" || group == req.Resource.Group {
				groupMatches = true
				break
			}
		}
		if !groupMatches {
			return false
		}
	}

	// Check resources - rule must allow the resource type
	if len(rule.Resources) > 0 {
		resourceMatches := false
		for _, resource := range rule.Resources {
			if resource == "*" || resource == req.Resource.Resource {
				resourceMatches = true
				break
			}
		}
		if !resourceMatches {
			return false
		}
	}

	// Check resource names if specified in both rule and request
	if req.ResourceName != "" && len(rule.ResourceNames) > 0 {
		nameMatches := false
		for _, name := range rule.ResourceNames {
			if name == "*" || name == req.ResourceName {
				nameMatches = true
				break
			}
		}
		if !nameMatches {
			return false
		}
	}

	// Check verbs - rule must allow the requested verb
	for _, verb := range rule.Verbs {
		if verb == "*" || verb == req.Verb {
			return true
		}
	}

	return false
}