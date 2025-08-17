/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestWorkspaceDiscoverer_DiscoverWorkspaces(t *testing.T) {
	tests := []struct {
		name     string
		opts     DiscoveryOptions
		wantLen  int
		wantErr  bool
	}{
		{
			name: "discover all workspaces",
			opts: DiscoveryOptions{
				IncludeNotReady: true,
			},
			wantLen: 0, // Empty cache
			wantErr: false,
		},
		{
			name: "discover with label selector",
			opts: DiscoveryOptions{
				LabelSelector: labels.Nothing(),
			},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverer := &WorkspaceDiscovererImpl{
				workspaceCache:          cache.NewStore(cache.MetaNamespaceKeyFunc),
				syncTargetCache:         cache.NewStore(cache.MetaNamespaceKeyFunc),
				authChecker:             &mockAuthChecker{},
				workspaceLabelIndex:     make(map[string][]*tenancyv1alpha1.Workspace),
				workspaceHierarchyIndex: make(map[logicalcluster.Name]*hierarchyNode),
			}

			results, err := discoverer.DiscoverWorkspaces(context.Background(), tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverWorkspaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(results) != tt.wantLen {
				t.Errorf("DiscoverWorkspaces() got %v results, want %v", len(results), tt.wantLen)
			}
		})
	}
}

type mockAuthChecker struct{}

func (m *mockAuthChecker) CanAccessWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) (bool, error) {
	return true, nil
}

func (m *mockAuthChecker) GetPermittedWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error) {
	return []logicalcluster.Name{}, nil
}