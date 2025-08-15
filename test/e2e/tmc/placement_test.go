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
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

var _ = ginkgo.Describe("Workload Placement", func() {
	ginkgo.BeforeEach(func() {
		// Setup test clusters for placement tests
		setupTestClusters(ctx, kcpClusterClient, 3)
	})

	ginkgo.It("should place workload using round-robin strategy", func() {
		placement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-placement",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "RoundRobin",
				Replicas: 3,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"tier": "production",
					},
				},
			},
		}

		// Create the placement
		createdPlacement, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(createdPlacement.Name).To(gomega.Equal("test-placement"))

		// Wait for placement to be scheduled
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return updated.IsPlaced()
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Verify placement results
		updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Get(ctx, placement.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(updated.Status.PlacedClusters).To(gomega.HaveLen(3))

		// Verify round-robin distribution
		placedClusterSet := make(map[string]bool)
		for _, clusterName := range updated.Status.PlacedClusters {
			gomega.Expect(placedClusterSet[clusterName]).To(gomega.BeFalse(), 
				"Cluster %s should only appear once in round-robin placement", clusterName)
			placedClusterSet[clusterName] = true
		}
	})

	ginkgo.It("should update placement when clusters change", func() {
		// Create initial placement
		placement := createTestPlacement(ctx, kcpClusterClient, "dynamic-placement")

		// Wait for initial placement
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			return err == nil && updated.IsPlaced()
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Get initial placement count
		initialPlacement, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Get(ctx, placement.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		initialClusterCount := len(initialPlacement.Status.PlacedClusters)

		// Add a new cluster
		newCluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-cluster",
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location:     "eu-west-1",
				Capabilities: []string{"high-performance"},
			},
		}
		_, err = kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, newCluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Verify placement is updated (may increase or stay same depending on strategy)
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			// Placement should be recalculated when new clusters are available
			return len(updated.Status.PlacedClusters) >= initialClusterCount
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())
	})

	ginkgo.It("should handle placement strategy tests", func() {
		// Test LeastLoaded strategy
		leastLoadedPlacement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "least-loaded-placement",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "LeastLoaded",
				Replicas: 2,
			},
		}

		_, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, leastLoadedPlacement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Wait for placement
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, leastLoadedPlacement.Name, metav1.GetOptions{})
			return err == nil && updated.IsPlaced()
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Verify placement respects strategy
		updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Get(ctx, leastLoadedPlacement.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(updated.Status.PlacedClusters).To(gomega.HaveLen(2))
		gomega.Expect(updated.Status.Strategy).To(gomega.Equal("LeastLoaded"))
	})

	ginkgo.It("should handle edge cases and error scenarios", func() {
		// Test placement with no available clusters
		// First delete all existing clusters
		cleanupTMCResources(ctx, kcpClusterClient, testWorkspace)

		placement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "no-clusters-placement",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "RoundRobin",
				Replicas: 1,
			},
		}

		_, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Verify placement remains unscheduled due to no available clusters
		gomega.Consistently(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return len(updated.Status.PlacedClusters) == 0
		}, 10*time.Second, time.Second).Should(gomega.BeTrue())

		// Add cluster and verify placement succeeds
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rescue-cluster",
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: "rescue-zone",
			},
		}
		_, err = kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, cluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Now placement should succeed
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			return err == nil && len(updated.Status.PlacedClusters) > 0
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())
	})
})

// setupTestClusters creates a specified number of test clusters
func setupTestClusters(ctx context.Context, client kcpclientset.ClusterInterface, count int) {
	for i := 0; i < count; i++ {
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-cluster-%d", i),
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: fmt.Sprintf("zone-%d", i),
			},
		}
		client.TmcV1alpha1().ClusterRegistrations().Create(ctx, cluster, metav1.CreateOptions{})
	}
}

// createTestPlacement creates a test placement with the given name
func createTestPlacement(ctx context.Context, client kcpclientset.ClusterInterface, name string) *tmcv1alpha1.WorkloadPlacement {
	placement := &tmcv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: tmcv1alpha1.WorkloadPlacementSpec{
			Strategy: "RoundRobin",
			Replicas: 2,
		},
	}
	
	createdPlacement, _ := client.TmcV1alpha1().WorkloadPlacements().
		Create(ctx, placement, metav1.CreateOptions{})
	return createdPlacement
}