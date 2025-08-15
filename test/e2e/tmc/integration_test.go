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
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

var _ = ginkgo.Describe("TMC Integration", func() {
	ginkgo.It("should handle complete workflow", func() {
		// Step 1: Register clusters
		clusters := registerTestClusters(ctx, kcpClusterClient, []string{
			"production-1", "production-2", "staging-1",
		})
		gomega.Expect(clusters).To(gomega.HaveLen(3))

		// Step 2: Create placement policy
		placement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "production-workload",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "LeastLoaded",
				Replicas: 2,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"env": "production",
					},
				},
			},
		}
		_, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Step 3: Verify placement decisions
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			return err == nil && len(updated.Status.PlacedClusters) == 2
		}, 30*time.Second).Should(gomega.BeTrue())

		// Step 4: Simulate cluster failure
		failedCluster := clusters[0]
		failedCluster.Status.Conditions = []metav1.Condition{
			{
				Type:   "Ready",
				Status: metav1.ConditionFalse,
				Reason: "Unreachable",
				Message: "Cluster is unreachable",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		_, err = kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			UpdateStatus(ctx, failedCluster, metav1.UpdateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Step 5: Verify placement is updated
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			// Should not include failed cluster
			for _, clusterName := range updated.Status.PlacedClusters {
				if clusterName == failedCluster.Name {
					return false
				}
			}
			return true
		}, 30*time.Second).Should(gomega.BeTrue())
	})

	ginkgo.It("should test controller interaction", func() {
		// Test controller reconciliation behavior
		clusterName := "controller-test-cluster"
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location:     "controller-zone",
				Capabilities: []string{"testing"},
			},
		}

		// Create cluster and verify controller processes it
		_, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, cluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Verify controller sets initial status
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Get(ctx, clusterName, metav1.GetOptions{})
			return err == nil && len(updated.Status.Conditions) > 0
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Create placement that should use this cluster
		placement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "controller-test-placement",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "RoundRobin",
				Replicas: 1,
			},
		}

		_, err = kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Verify placement controller places workload on the cluster
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return len(updated.Status.PlacedClusters) == 1 && 
				   updated.Status.PlacedClusters[0] == clusterName
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())
	})

	ginkgo.It("should test performance and scale", func() {
		start := time.Now()

		// Create 20 clusters
		clusterNames := []string{}
		for i := 0; i < 20; i++ {
			clusterName := fmt.Sprintf("perf-cluster-%d", i)
			clusterNames = append(clusterNames, clusterName)

			cluster := &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: fmt.Sprintf("perf-region-%d", i%5),
				},
			}
			_, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Create(ctx, cluster, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		}

		// Create placement for half the clusters
		placement := &tmcv1alpha1.WorkloadPlacement{
			ObjectMeta: metav1.ObjectMeta{
				Name: "perf-placement",
			},
			Spec: tmcv1alpha1.WorkloadPlacementSpec{
				Strategy: "RoundRobin",
				Replicas: 10,
			},
		}
		_, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Create(ctx, placement, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Measure time to complete placement
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
				Get(ctx, placement.Name, metav1.GetOptions{})
			return err == nil && updated.IsPlaced() && len(updated.Status.PlacedClusters) == 10
		}, 60*time.Second).Should(gomega.BeTrue())

		duration := time.Since(start)
		ginkgo.By(fmt.Sprintf("Placement completed in %v", duration))

		// Assert reasonable performance (should complete within 30 seconds)
		gomega.Expect(duration).To(gomega.BeNumerically("<", 30*time.Second))

		// Verify placement distribution
		updated, err := kcpClusterClient.TmcV1alpha1().WorkloadPlacements().
			Get(ctx, placement.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(updated.Status.PlacedClusters).To(gomega.HaveLen(10))

		// Verify no duplicate placements in round-robin
		placedSet := make(map[string]bool)
		for _, clusterName := range updated.Status.PlacedClusters {
			gomega.Expect(placedSet[clusterName]).To(gomega.BeFalse(),
				"Round-robin should not place on same cluster twice")
			placedSet[clusterName] = true
		}
	})

	ginkgo.It("should test feature flag behavior", func() {
		// This test would normally verify feature flag behavior
		// In a real implementation, this would test that TMC functionality
		// is properly disabled when the feature flag is off
		
		// For now, we skip this test as it requires server restart capability
		ginkgo.Skip("Feature flag testing requires server restart capability")
	})
})

// registerTestClusters creates and returns multiple test clusters
func registerTestClusters(ctx context.Context, client kcpclientset.ClusterInterface, names []string) []*tmcv1alpha1.ClusterRegistration {
	clusters := make([]*tmcv1alpha1.ClusterRegistration, 0, len(names))

	for i, name := range names {
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: fmt.Sprintf("zone-%d", i),
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		created, err := client.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, cluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		clusters = append(clusters, created)
	}

	return clusters
}