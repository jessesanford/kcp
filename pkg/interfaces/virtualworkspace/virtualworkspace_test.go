package virtualworkspace_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace"
	"github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace/router"
)

// Ensure interfaces can be implemented
type testVirtualWorkspace struct{}

var _ virtualworkspace.VirtualWorkspace = &testVirtualWorkspace{}

func (vw *testVirtualWorkspace) Initialize(ctx context.Context, config *virtualworkspace.VirtualWorkspaceConfig) error {
	return nil
}

func (vw *testVirtualWorkspace) Start(ctx context.Context) error                          { return nil }
func (vw *testVirtualWorkspace) Stop() error                                             { return nil }
func (vw *testVirtualWorkspace) GetURL() string                                          { return "" }
func (vw *testVirtualWorkspace) GetAPIGroups() []virtualworkspace.APIGroupInfo          { return nil }
func (vw *testVirtualWorkspace) HandleRequest(w http.ResponseWriter, r *http.Request) error { return nil }
func (vw *testVirtualWorkspace) RegisterLocation(location router.LocationInfo) error { return nil }
func (vw *testVirtualWorkspace) UnregisterLocation(name string) error                    { return nil }

func TestVirtualWorkspaceInterface(t *testing.T) {
	var vw virtualworkspace.VirtualWorkspace = &testVirtualWorkspace{}
	if vw == nil {
		t.Fatal("Failed to implement VirtualWorkspace interface")
	}
}