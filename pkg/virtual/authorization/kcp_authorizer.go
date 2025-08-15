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

package authorization

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// KCPAuthorizationProvider implements AuthorizationProvider for KCP environments
type KCPAuthorizationProvider struct {
	delegate  authorizer.Authorizer
	workspace logicalcluster.Name
	cache     AuthorizationCache
	filter    *WorkspaceFilter
}

// NewKCPAuthorizationProvider creates a new KCP authorization provider
func NewKCPAuthorizationProvider(
	delegate authorizer.Authorizer,
	kcpClient kcpclient.ClusterInterface,
	workspace logicalcluster.Name,
) (*KCPAuthorizationProvider, error) {
	return &KCPAuthorizationProvider{
		delegate:  delegate,
		workspace: workspace,
		cache:     NewMemoryAuthorizationCache(5*time.Minute, time.Minute),
		filter:    NewWorkspaceFilter(workspace, false),
	}, nil
}

// Start initializes the authorization provider
func (p *KCPAuthorizationProvider) Start(ctx context.Context) error {
	p.cache.(*MemoryAuthorizationCache).Start()
	return nil
}

// Authorize determines if a request is allowed within the workspace context
func (p *KCPAuthorizationProvider) Authorize(ctx context.Context, req *interfaces.AuthorizationRequest) (*interfaces.AuthorizationDecision, error) {
	start := time.Now()
	
	filteredReq, err := p.filter.FilterRequest(ctx, req)
	if err != nil {
		RecordAuthorizationRequest(req.Workspace, req.User, time.Since(start), false, err)
		return &interfaces.AuthorizationDecision{Allowed: false, Reason: err.Error(), EvaluationError: err}, nil
	}
	
	cacheKey := generateCacheKey(req.User, req.Workspace, req.Resource.String(), req.Verb, req.ResourceName)
	if decision, hit := p.cache.GetDecision(cacheKey); hit {
		RecordCacheHit(req.Workspace, true)
		RecordAuthorizationRequest(req.Workspace, req.User, time.Since(start), decision.Allowed, decision.EvaluationError)
		return decision, nil
	}
	
	RecordCacheHit(req.Workspace, false)
	
	if p.delegate != nil {
		attrs := p.buildAuthorizerAttributes(filteredReq)
		authorized, reason, err := p.delegate.Authorize(ctx, attrs)
		decision := &interfaces.AuthorizationDecision{
			Allowed: authorized == authorizer.DecisionAllow,
			Reason: reason,
			EvaluationError: err,
		}
		p.cache.SetDecision(cacheKey, decision, 5*time.Minute)
		RecordAuthorizationRequest(req.Workspace, req.User, time.Since(start), decision.Allowed, decision.EvaluationError)
		return decision, nil
	}
	
	decision := &interfaces.AuthorizationDecision{Allowed: false, Reason: "no authorization delegate configured"}
	RecordAuthorizationRequest(req.Workspace, req.User, time.Since(start), false, nil)
	RecordDenial(req.Workspace, req.User, req.Resource.String(), req.Verb)
	return decision, nil
}

// GetPermissions returns permissions for a user in the workspace
func (p *KCPAuthorizationProvider) GetPermissions(ctx context.Context, workspace, user string) ([]interfaces.Permission, error) {
	return []interfaces.Permission{}, fmt.Errorf("permission extraction not yet implemented")
}

// RefreshCache updates cached authorization data for the workspace
func (p *KCPAuthorizationProvider) RefreshCache(ctx context.Context, workspace string) error {
	p.cache.InvalidateWorkspace(workspace)
	return nil
}

// buildAuthorizerAttributes converts request to authorizer attributes
func (p *KCPAuthorizationProvider) buildAuthorizerAttributes(req *interfaces.AuthorizationRequest) authorizer.Attributes {
	return authorizer.AttributesRecord{
		User:            &requestUser{name: req.User, groups: req.Groups},
		Verb:            req.Verb,
		Namespace:       req.Namespace,
		APIGroup:        req.Resource.Group,
		APIVersion:      req.Resource.Version,
		Resource:        req.Resource.Resource,
		Name:            req.ResourceName,
		ResourceRequest: true,
		Path:            req.Path,
	}
}

// requestUser implements user.Info interface  
type requestUser struct {
	name   string
	groups []string
}

func (u *requestUser) GetName() string                { return u.name }
func (u *requestUser) GetUID() string                 { return "" }
func (u *requestUser) GetGroups() []string            { return u.groups }
func (u *requestUser) GetExtra() map[string][]string  { return nil }