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
	"k8s.io/apimachinery/pkg/util/wait"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

var _ = ginkgo.Describe("Cluster Registration", func() {
	ginkgo.It("should register and monitor cluster health", func() {
		// Create a ClusterRegistration
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-1",
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location:     "us-west-2",
				Capabilities: []string{"gpu", "high-memory"},
				Endpoint:     "https://test-cluster.example.com",
			},
		}

		// Create the cluster
		createdCluster, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, cluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(createdCluster.Name).To(gomega.Equal("test-cluster-1"))

		// Wait for cluster to be ready
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Get(ctx, cluster.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return updated.IsReady()
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Verify status conditions
		updated, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Get(ctx, cluster.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(updated.Status.Conditions).ToNot(gomega.BeEmpty())

		// Find the Ready condition
		var readyCondition *metav1.Condition
		for i := range updated.Status.Conditions {
			if updated.Status.Conditions[i].Type == "Ready" {
				readyCondition = &updated.Status.Conditions[i]
				break
			}
		}
		gomega.Expect(readyCondition).ToNot(gomega.BeNil())
		gomega.Expect(readyCondition.Status).To(gomega.Equal(metav1.ConditionTrue))
	})

	ginkgo.It("should handle multiple clusters", func() {
		clusterNames := []string{}
		
		// Create multiple clusters
		for i := 0; i < 5; i++ {
			clusterName := fmt.Sprintf("cluster-%d", i)
			clusterNames = append(clusterNames, clusterName)
			
			cluster := &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: fmt.Sprintf("region-%d", i%3),
				},
			}
			_, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Create(ctx, cluster, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		}

		// Verify all clusters are registered
		gomega.Eventually(func() int {
			clusterList, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				List(ctx, metav1.ListOptions{})
			if err != nil {
				return 0
			}
			return len(clusterList.Items)
		}, 30*time.Second, time.Second).Should(gomega.Equal(5))

		// Verify cluster locations are set correctly
		clusterList, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			List(ctx, metav1.ListOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		
		locationCounts := make(map[string]int)
		for _, cluster := range clusterList.Items {
			locationCounts[cluster.Spec.Location]++
		}
		gomega.Expect(locationCounts).To(gomega.HaveLen(3)) // 3 regions
		gomega.Expect(locationCounts["region-0"]).To(gomega.Equal(2))
		gomega.Expect(locationCounts["region-1"]).To(gomega.Equal(2))
		gomega.Expect(locationCounts["region-2"]).To(gomega.Equal(1))
	})

	ginkgo.It("should handle cluster deletion and cleanup", func() {
		// Create a cluster to delete
		cluster := &tmcv1alpha1.ClusterRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "delete-test-cluster",
			},
			Spec: tmcv1alpha1.ClusterRegistrationSpec{
				Location: "us-east-1",
			},
		}

		_, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Create(ctx, cluster, metav1.CreateOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Wait for cluster to be ready
		gomega.Eventually(func() bool {
			updated, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Get(ctx, cluster.Name, metav1.GetOptions{})
			return err == nil && updated.IsReady()
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())

		// Delete the cluster
		err = kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
			Delete(ctx, cluster.Name, metav1.DeleteOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Verify cluster is deleted
		gomega.Eventually(func() bool {
			_, err := kcpClusterClient.TmcV1alpha1().ClusterRegistrations().
				Get(ctx, cluster.Name, metav1.GetOptions{})
			return err != nil
		}, 30*time.Second, time.Second).Should(gomega.BeTrue())
	})
})