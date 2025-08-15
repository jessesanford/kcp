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

package tmc_test

import (
	"context"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

func TestTMC(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "TMC Suite")
}

var _ = ginkgo.Describe("TMC E2E Tests", func() {
	var (
		ctx           context.Context
		server        framework.RunningServer
		kcpClusterClient kcpclientset.ClusterInterface
		testWorkspace string
	)

	ginkgo.BeforeEach(func() {
		ctx = context.Background()

		// Start test server with TMC enabled
		server = kcptesting.SharedKcpServer(ginkgo.GinkgoT())
		require.Eventually(ginkgo.GinkgoT(), func() bool {
			return server.Ready()
		}, 30*time.Second, time.Second)

		// Create cluster-aware client
		config := server.BaseConfig(ginkgo.GinkgoT())
		var err error
		kcpClusterClient, err = kcpclientset.NewForConfig(config)
		require.NoError(ginkgo.GinkgoT(), err)

		// Create test workspace
		testWorkspace = framework.NewOrganizationFixture(ginkgo.GinkgoT(), server, 
			framework.WithNamePrefix("tmc-test-")).Spec.Name
	})

	ginkgo.AfterEach(func() {
		// Cleanup TMC resources
		cleanupTMCResources(ctx, kcpClusterClient, testWorkspace)
	})
})

// cleanupTMCResources removes all TMC resources from the test workspace
func cleanupTMCResources(ctx context.Context, client kcpclientset.ClusterInterface, workspace string) {
	// Cleanup ClusterRegistrations
	clusterList, _ := client.TmcV1alpha1().ClusterRegistrations().List(ctx, metav1.ListOptions{})
	for _, cluster := range clusterList.Items {
		client.TmcV1alpha1().ClusterRegistrations().Delete(ctx, cluster.Name, metav1.DeleteOptions{})
	}

	// Cleanup WorkloadPlacements
	placementList, _ := client.TmcV1alpha1().WorkloadPlacements().List(ctx, metav1.ListOptions{})
	for _, placement := range placementList.Items {
		client.TmcV1alpha1().WorkloadPlacements().Delete(ctx, placement.Name, metav1.DeleteOptions{})
	}
}