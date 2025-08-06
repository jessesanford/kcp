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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

const (
	// TMCAPIExportReady indicates the TMC APIExport is ready.
	TMCAPIExportReady = "TMCAPIExportReady"
	// TMCResourceSchemasReady indicates TMC resource schemas are ready.
	TMCResourceSchemasReady = "TMCResourceSchemasReady"
)

// ReconcileOptions contains options for APIExport reconciliation.
type ReconcileOptions struct {
	// CreateMissingSchemas indicates whether to create missing APIResourceSchemas.
	CreateMissingSchemas bool
	// ValidatePermissions indicates whether to validate permission claims.
	ValidatePermissions bool
}

// reconcileAPIExport performs the main reconciliation logic for the TMC APIExport.
func (c *Controller) reconcileAPIExport(ctx context.Context, cluster logicalcluster.Name, opts ReconcileOptions) error {
	klog.V(2).InfoS("Reconciling TMC APIExport", "cluster", cluster)

	// Get the current APIExport
	apiExport, err := c.apiExportLister.Cluster(cluster).Get(TMCAPIExportName)
	if apierrors.IsNotFound(err) {
		// Create the APIExport if it doesn't exist
		return c.createTMCAPIExport(ctx, cluster, opts)
	}
	if err != nil {
		return fmt.Errorf("failed to get TMC APIExport: %w", err)
	}

	// Update the APIExport if needed
	return c.updateTMCAPIExport(ctx, cluster, apiExport, opts)
}

// createTMCAPIExport creates a new TMC APIExport.
func (c *Controller) createTMCAPIExport(ctx context.Context, cluster logicalcluster.Name, opts ReconcileOptions) error {
	klog.InfoS("Creating TMC APIExport", "cluster", cluster)

	// Ensure required APIResourceSchemas exist
	if opts.CreateMissingSchemas {
		if err := c.ensureAPIResourceSchemas(ctx, cluster); err != nil {
			return fmt.Errorf("failed to ensure APIResourceSchemas: %w", err)
		}
	}

	// Get the resource schemas to include
	resourceSchemas, err := c.getRequiredResourceSchemas(cluster)
	if err != nil {
		return fmt.Errorf("failed to get required resource schemas: %w", err)
	}

	// Create the APIExport
	apiExport := &apisv1alpha1.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: TMCAPIExportName,
		},
		Spec: apisv1alpha1.APIExportSpec{
			LatestResourceSchemas: resourceSchemas,
			PermissionClaims:      c.getRequiredPermissionClaims(),
		},
	}

	// Set initial conditions
	c.setAPIExportConditions(apiExport, len(resourceSchemas) > 0)

	_, err = c.kcpClusterClient.ApisV1alpha1().APIExports().
		Cluster(cluster.Path()).
		Create(ctx, apiExport, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create TMC APIExport: %w", err)
	}

	klog.InfoS("Successfully created TMC APIExport", "cluster", cluster, "schemas", len(resourceSchemas))
	return nil
}

// updateTMCAPIExport updates an existing TMC APIExport.
func (c *Controller) updateTMCAPIExport(ctx context.Context, cluster logicalcluster.Name, apiExport *apisv1alpha1.APIExport, opts ReconcileOptions) error {
	klog.V(4).InfoS("Updating TMC APIExport", "cluster", cluster)

	// Get required resource schemas
	requiredSchemas, err := c.getRequiredResourceSchemas(cluster)
	if err != nil {
		return fmt.Errorf("failed to get required resource schemas: %w", err)
	}

	// Get required permission claims
	requiredPermissions := c.getRequiredPermissionClaims()

	// Check if update is needed
	needsUpdate := false
	updatedExport := apiExport.DeepCopy()

	// Update resource schemas
	if !c.schemaSetsEqual(updatedExport.Spec.LatestResourceSchemas, requiredSchemas) {
		updatedExport.Spec.LatestResourceSchemas = requiredSchemas
		needsUpdate = true
		klog.V(2).InfoS("Updating resource schemas", "cluster", cluster, "current", len(apiExport.Spec.LatestResourceSchemas), "required", len(requiredSchemas))
	}

	// Update permission claims
	if !c.permissionClaimsEqual(updatedExport.Spec.PermissionClaims, requiredPermissions) {
		updatedExport.Spec.PermissionClaims = requiredPermissions
		needsUpdate = true
		klog.V(2).InfoS("Updating permission claims", "cluster", cluster)
	}

	// Update conditions
	c.setAPIExportConditions(updatedExport, len(requiredSchemas) > 0)

	if needsUpdate {
		_, err = c.commit.WithContext(ctx).Cluster(cluster.Path()).Update(updatedExport, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update TMC APIExport: %w", err)
		}
		klog.InfoS("Successfully updated TMC APIExport", "cluster", cluster)
	}

	return nil
}

// ensureAPIResourceSchemas ensures that all required TMC APIResourceSchemas exist.
func (c *Controller) ensureAPIResourceSchemas(ctx context.Context, cluster logicalcluster.Name) error {
	klog.V(2).InfoS("Ensuring TMC APIResourceSchemas", "cluster", cluster)

	requiredSchemas := c.getExpectedAPIResourceSchemas()

	for _, schemaSpec := range requiredSchemas {
		schemaName := fmt.Sprintf("%s.%s.%s", schemaSpec.Names.Plural, schemaSpec.Version, schemaSpec.Group)
		
		// Check if schema already exists
		_, err := c.apiResourceSchemaLister.Cluster(cluster).Get(schemaName)
		if err == nil {
			continue // Schema already exists
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get APIResourceSchema %s: %w", schemaName, err)
		}

		// Create the schema
		schema := &apisv1alpha1.APIResourceSchema{
			ObjectMeta: metav1.ObjectMeta{
				Name: schemaName,
			},
			Spec: schemaSpec,
		}

		_, err = c.kcpClusterClient.ApisV1alpha1().APIResourceSchemas().
			Cluster(cluster.Path()).
			Create(ctx, schema, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create APIResourceSchema %s: %w", schemaName, err)
		}

		klog.InfoS("Created APIResourceSchema", "cluster", cluster, "schema", schemaName)
	}

	return nil
}

// getRequiredResourceSchemas returns the list of resource schema names that should be included in the TMC APIExport.
func (c *Controller) getRequiredResourceSchemas(cluster logicalcluster.Name) ([]string, error) {
	schemas := []string{}

	// List all APIResourceSchemas for TMC
	allSchemas, err := c.apiResourceSchemaLister.Cluster(cluster).List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list APIResourceSchemas: %w", err)
	}

	for _, schema := range allSchemas {
		if isTMCSchema(schema) {
			schemas = append(schemas, schema.Name)
		}
	}

	klog.V(4).InfoS("Found TMC resource schemas", "cluster", cluster, "count", len(schemas))
	return schemas, nil
}

// getRequiredPermissionClaims returns the permission claims required by the TMC APIExport.
func (c *Controller) getRequiredPermissionClaims() []apisv1alpha1.PermissionClaim {
	return []apisv1alpha1.PermissionClaim{
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "coordination.k8s.io",
				Resource: "leases",
			},
			All: true,
		},
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "",
				Resource: "events",
			},
			All: true,
		},
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "apps",
				Resource: "deployments",
			},
			All: true,
		},
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "",
				Resource: "services",
			},
			All: true,
		},
	}
}

// getExpectedAPIResourceSchemas returns the expected APIResourceSchema specifications for TMC.
func (c *Controller) getExpectedAPIResourceSchemas() []apisv1alpha1.APIResourceSchemaSpec {
	return []apisv1alpha1.APIResourceSchemaSpec{
		{
			Group:   tmcv1alpha1.GroupName,
			Version: "v1alpha1",
			Names: apisv1alpha1.APIResourceSchemaNames{
				Plural:   "clusterregistrations",
				Singular: "clusterregistration",
				Kind:     "ClusterRegistration",
			},
			Scope: apisv1alpha1.ClusterScope,
		},
		{
			Group:   tmcv1alpha1.GroupName,
			Version: "v1alpha1",
			Names: apisv1alpha1.APIResourceSchemaNames{
				Plural:   "workloadplacements",
				Singular: "workloadplacement",
				Kind:     "WorkloadPlacement",
			},
			Scope: apisv1alpha1.ClusterScope,
		},
	}
}

// setAPIExportConditions sets the appropriate conditions on the TMC APIExport.
func (c *Controller) setAPIExportConditions(apiExport *apisv1alpha1.APIExport, hasResourceSchemas bool) {
	// Set TMCResourceSchemasReady condition
	if hasResourceSchemas {
		conditions.MarkTrue(apiExport, TMCResourceSchemasReady)
	} else {
		conditions.MarkFalse(apiExport, TMCResourceSchemasReady, "NoResourceSchemas", 
			conditionsv1alpha1.ConditionSeverityError, "No TMC resource schemas found")
	}

	// Set TMCAPIExportReady condition based on other conditions
	if conditions.IsTrue(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady) && 
		conditions.IsTrue(apiExport, TMCResourceSchemasReady) {
		conditions.MarkTrue(apiExport, TMCAPIExportReady)
	} else {
		conditions.MarkFalse(apiExport, TMCAPIExportReady, "NotReady", 
			conditionsv1alpha1.ConditionSeverityError, "TMC APIExport is not ready")
	}

	// Ensure VirtualWorkspaceURLsReady is set if not present
	if conditions.Get(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady) == nil {
		conditions.MarkTrue(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady)
	}
}