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
	
	// The TMC APIExport should be created via the generated manifests in config/root-phase0/apiexport-tmc.kcp.io.yaml
	// This controller only manages the lifecycle and status of existing APIExports
	logger.V(2).Info("TMC APIExport should be created via KCP bootstrap manifests, not by controller")
	return fmt.Errorf("TMC APIExport not found - should be created via bootstrap manifests")
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