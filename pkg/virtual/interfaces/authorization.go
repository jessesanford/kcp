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

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AuthorizationRequest represents a request for authorization
type AuthorizationRequest struct {
	// User is the user making the request
	User string

	// Groups are the groups the user belongs to
	Groups []string

	// Workspace is the target workspace
	Workspace string

	// Verb is the action being requested (get, list, create, etc.)
	Verb string

	// Resource is the resource being accessed
	Resource schema.GroupVersionResource

	// ResourceName is the specific resource name (if applicable)
	ResourceName string

	// Path is the original request path
	Path string

	// Namespace is the namespace context (if applicable)
	Namespace string
}

// AuthorizationDecision represents the result of an authorization check
type AuthorizationDecision struct {
	// Allowed indicates if the request is authorized
	Allowed bool

	// Reason provides a human-readable reason for the decision
	Reason string

	// EvaluationError holds any error that occurred during evaluation
	EvaluationError error
}

// Permission represents a permission granted to a user
type Permission struct {
	// Workspace is the workspace this permission applies to
	Workspace string

	// Verb is the allowed action
	Verb string

	// Resource is the resource type
	Resource schema.GroupVersionResource

	// ResourceNames are specific resource names (if restricted)
	ResourceNames []string

	// Namespace is the namespace context (if applicable)
	Namespace string
}

// AuthorizationProvider defines the interface for authorization providers
type AuthorizationProvider interface {
	// Start initializes the authorization provider
	Start(ctx context.Context) error

	// Authorize determines if a request is allowed
	Authorize(ctx context.Context, req *AuthorizationRequest) (*AuthorizationDecision, error)

	// GetPermissions returns permissions for a user in a workspace
	GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error)

	// RefreshCache updates cached authorization data
	RefreshCache(ctx context.Context, workspace string) error
}