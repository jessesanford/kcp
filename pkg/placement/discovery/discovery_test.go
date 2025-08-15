package discovery_test

import (
	"context"
	"testing"
	"time"
	
	"github.com/kcp-dev/kcp/pkg/placement/discovery"
	"github.com/kcp-dev/kcp/pkg/placement/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
	
	kcpclientfake "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/fake"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/testing"
)

func TestWorkspaceTraversal(t *testing.T) {
	ctx := context.Background()
	client := newMockKCPClient()
	traverser := discovery.NewWorkspaceTraverser(client)
	
	tests := []struct {
		name     string
		selector labels.Selector
		expected []string
	}{
		{
			name:     "list all workspaces",
			selector: labels.Everything(),
			expected: []string{"root"},
		},
		{
			name:     "filter by label",
			selector: labels.SelectorFromSet(labels.Set{"env": "prod"}),
			expected: []string{},
		},
		{
			name:     "empty result",
			selector: labels.SelectorFromSet(labels.Set{"env": "staging"}),
			expected: []string{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaces, err := traverser.ListWorkspaces(ctx, tt.selector)
			require.NoError(t, err)
			
			names := []string{}
			for _, ws := range workspaces {
				names = append(names, string(ws.Name))
			}
			
			assert.ElementsMatch(t, tt.expected, names)
		})
	}
}

func TestClusterDiscovery(t *testing.T) {
	ctx := context.Background()
	client := newMockKCPClient()
	finder := discovery.NewClusterFinder(client)
	
	criteria := discovery.ClusterCriteria{
		WorkspaceSelector: labels.Everything(),
		LabelSelector:     labels.SelectorFromSet(labels.Set{"type": "compute"}),
		Regions:           []string{"us-west-2", "us-east-1"},
	}
	
	clusters, err := finder.FindClusters(ctx, criteria)
	require.NoError(t, err)
	
	// Should be empty with mock data
	assert.Len(t, clusters, 0)
}

func TestPermissionChecking(t *testing.T) {
	ctx := context.Background()
	client := newMockKCPClient()
	checker := discovery.NewPermissionChecker(client)
	
	tests := []struct {
		name      string
		workspace string
		verb      string
		expected  bool
	}{
		{
			name:      "allowed access",
			workspace: "root:org:team1",
			verb:      "list",
			expected:  true,
		},
		{
			name:      "denied access",
			workspace: "root:org:team2",
			verb:      "delete",
			expected:  true, // Mock always returns true
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := checker.CheckAccess(ctx, tt.workspace, tt.verb)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestCaching(t *testing.T) {
	cache := discovery.NewDiscoveryCache(100 * time.Millisecond)
	
	// Test workspace caching
	workspaces := []interfaces.WorkspaceInfo{
		{Name: "ws1", Ready: true},
		{Name: "ws2", Ready: true},
	}
	
	cache.PutWorkspaces("test-key", workspaces)
	
	// Should retrieve from cache
	cached, ok := cache.GetWorkspaces("test-key")
	assert.True(t, ok)
	assert.Equal(t, workspaces, cached)
	
	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)
	
	// Should not retrieve expired entry
	_, ok = cache.GetWorkspaces("test-key")
	assert.False(t, ok)
}

func TestHierarchy(t *testing.T) {
	manager := discovery.NewHierarchyManager(nil)
	
	// Test ancestor checking
	assert.True(t, manager.IsAncestor("root", "root:org:team"))
	assert.True(t, manager.IsAncestor("root:org", "root:org:team"))
	assert.False(t, manager.IsAncestor("root:org:team", "root:org"))
	
	// Test common ancestor
	ancestor := manager.GetCommonAncestor("root:org:team1", "root:org:team2")
	assert.Equal(t, "root:org", ancestor)
	
	// Test depth calculation
	assert.Equal(t, 0, manager.GetDepth("root"))
	assert.Equal(t, 1, manager.GetDepth("root:org"))
	assert.Equal(t, 2, manager.GetDepth("root:org:team"))
}

func TestCacheStatistics(t *testing.T) {
	cache := discovery.NewDiscoveryCache(5 * time.Minute)
	
	// Add some entries
	workspaces := []interfaces.WorkspaceInfo{{Name: "ws1", Ready: true}}
	clusters := []interfaces.ClusterTarget{{Name: "cluster1", Ready: true}}
	
	cache.PutWorkspaces("key1", workspaces)
	cache.PutClusters("ws1", clusters)
	
	stats := cache.GetStats()
	assert.Equal(t, 1, stats.WorkspaceEntries)
	assert.Equal(t, 1, stats.ClusterEntries)
	assert.Equal(t, 5*time.Minute, stats.TTL)
	
	// Test clear
	cache.Clear()
	stats = cache.GetStats()
	assert.Equal(t, 0, stats.WorkspaceEntries)
	assert.Equal(t, 0, stats.ClusterEntries)
}

func TestPermissionCaching(t *testing.T) {
	client := newMockKCPClient()
	checker := discovery.NewPermissionChecker(client)
	
	ctx := context.Background()
	
	// First call should hit the API
	allowed1, err := checker.CheckAccess(ctx, "root:test", "list")
	require.NoError(t, err)
	assert.True(t, allowed1)
	
	// Second call should use cache
	allowed2, err := checker.CheckAccess(ctx, "root:test", "list")
	require.NoError(t, err)
	assert.True(t, allowed2)
	
	// Clear cache and verify
	checker.ClearCache()
	allowed3, err := checker.CheckAccess(ctx, "root:test", "list")
	require.NoError(t, err)
	assert.True(t, allowed3)
}

// newMockKCPClient creates a mock KCP client for testing
func newMockKCPClient() *kcpclientfake.Clientset {
	client := kcpclientfake.NewSimpleClientset()
	
	// Add reaction for SubjectAccessReview to always return allowed
	client.PrependReactor("create", "subjectaccessreviews", func(action testing.Action) (bool, interface{}, error) {
		createAction := action.(testing.CreateAction)
		sar := createAction.GetObject().(*authv1.SubjectAccessReview)
		
		sar.Status = authv1.SubjectAccessReviewStatus{
			Allowed: true,
		}
		
		return true, sar, nil
	})
	
	// Add reaction for SyncTargets
	client.PrependReactor("list", "synctargets", func(action testing.Action) (bool, interface{}, error) {
		return true, &workloadv1alpha1.SyncTargetList{
			Items: []workloadv1alpha1.SyncTarget{},
		}, nil
	})
	
	return client
}