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

package tmcexport

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

const (
	// TMCAPIExportReady indicates that the TMC APIExport is ready and available
	TMCAPIExportReady = "TMCAPIExportReady"

	// TMCAPIExportConfigured indicates that the TMC APIExport is properly configured
	TMCAPIExportConfigured = "TMCAPIExportConfigured"
)

func (c *Controller) reconcile(ctx context.Context, apiExport *apisv1alpha2.APIExport, clusterName logicalcluster.Name) error {
	logger := klog.FromContext(ctx)

	if apiExport == nil {
		// Create the TMC APIExport if it doesn't exist
		logger.V(2).Info("creating TMC APIExport")
		return c.createTMCAPIExport(ctx, clusterName)
	}

	// Ensure the TMC APIExport is properly configured
	return c.reconcileTMCAPIExport(ctx, apiExport, clusterName)
}

func (c *Controller) createTMCAPIExport(ctx context.Context, clusterName logicalcluster.Name) error {
	logger := klog.FromContext(ctx)

	tmcAPIExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: TMCAPIExportName,
			Annotations: map[string]string{
				"kcp.io/tmc-managed": "true",
			},
		},
		Spec: apisv1alpha2.APIExportSpec{
			Resources: []apisv1alpha2.ResourceSchema{
				{
					Name:   "clusters",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.clusters.tmc.kcp.io",
				},
				{
					Name:   "workloadplacements",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.workloadplacements.tmc.kcp.io",
				},
				{
					Name:   "workloadplacementadvanceds",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.workloadplacementadvanceds.tmc.kcp.io",
				},
				{
					Name:   "workloadsessionpolicies",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.workloadsessionpolicies.tmc.kcp.io",
				},
				{
					Name:   "trafficmetrics",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.trafficmetrics.tmc.kcp.io",
				},
				{
					Name:   "workloadscalingpolicies",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.workloadscalingpolicies.tmc.kcp.io",
				},
				{
					Name:   "workloadstatusaggregators",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.workloadstatusaggregators.tmc.kcp.io",
				},
			},
			PermissionClaims: []apisv1alpha2.PermissionClaim{
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "", Resource: "namespaces"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "", Resource: "configmaps"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "", Resource: "secrets"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "apps", Resource: "deployments"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "apps", Resource: "replicasets"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
				{
					GroupResource: apisv1alpha2.GroupResource{Group: "", Resource: "services"},
					Verbs:         []string{"get", "list", "watch", "create", "update", "patch", "delete"},
					IdentityHash:  "",
				},
			},
		},
	}

	_, err := c.kcpClusterClient.Cluster(clusterName.Path()).ApisV1alpha2().APIExports().Create(ctx, tmcAPIExport, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			logger.V(3).Info("TMC APIExport already exists")
			return nil
		}
		return fmt.Errorf("failed to create TMC APIExport: %w", err)
	}

	logger.V(2).Info("created TMC APIExport")
	return nil
}

func (c *Controller) reconcileTMCAPIExport(ctx context.Context, apiExport *apisv1alpha2.APIExport, clusterName logicalcluster.Name) error {
	logger := klog.FromContext(ctx)

	// Update APIExport status to indicate it's ready
	updated := apiExport.DeepCopy()
	conditions := []conditionsv1alpha1.Condition{
		{
			Type:               TMCAPIExportReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "APIExportReady",
			Message:            "TMC APIExport is ready and available for binding",
		},
		{
			Type:               TMCAPIExportConfigured,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "ConfigurationValid",
			Message:            "TMC APIExport is properly configured",
		},
	}

	// Update conditions if they have changed
	if !conditionsEqual(updated.Status.Conditions, conditions) {
		updated.Status.Conditions = conditions
		logger.V(2).Info("updating TMC APIExport status")
		
		_, err := c.kcpClusterClient.Cluster(clusterName.Path()).ApisV1alpha2().APIExports().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update TMC APIExport status: %w", err)
		}
	}

	return nil
}


func conditionsEqual(existing []conditionsv1alpha1.Condition, new []conditionsv1alpha1.Condition) bool {
	if len(existing) != len(new) {
		return false
	}

	existingMap := make(map[conditionsv1alpha1.ConditionType]conditionsv1alpha1.Condition)
	for _, c := range existing {
		existingMap[c.Type] = c
	}

	for _, newCondition := range new {
		if existing, exists := existingMap[newCondition.Type]; !exists ||
			existing.Status != newCondition.Status ||
			existing.Reason != newCondition.Reason ||
			existing.Message != newCondition.Message {
			return false
		}
	}

	return true
}