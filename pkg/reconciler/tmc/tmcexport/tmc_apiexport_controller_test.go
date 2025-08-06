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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpfakeclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	apisinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestController_ensureTMCAPIExport(t *testing.T) {
	tests := map[string]struct {
		cluster            logicalcluster.Name
		existingSchemas    []*apisv1alpha1.APIResourceSchema
		expectedSchemaCount int
		wantError          bool
	}{
		"creates APIExport with no existing schemas": {
			cluster:            "root:test",
			existingSchemas:    []*apisv1alpha1.APIResourceSchema{},
			expectedSchemaCount: 0,
			wantError:          false,
		},
		"creates APIExport with existing TMC schemas": {
			cluster: "root:test",
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
						Names: apisv1alpha1.APIResourceSchemaNames{
							Plural: "clusterregistrations",
							Kind:   "ClusterRegistration",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workloadplacements.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
						Names: apisv1alpha1.APIResourceSchemaNames{
							Plural: "workloadplacements",
							Kind:   "WorkloadPlacement",
						},
					},
				},
			},
			expectedSchemaCount: 2,
			wantError:          false,
		},
		"ignores non-TMC schemas": {
			cluster: "root:test",
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workspaces.v1alpha1.tenancy.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   "tenancy.kcp.io",
						Version: "v1alpha1",
						Names: apisv1alpha1.APIResourceSchemaNames{
							Plural: "workspaces",
							Kind:   "Workspace",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
						Names: apisv1alpha1.APIResourceSchemaNames{
							Plural: "clusterregistrations",
							Kind:   "ClusterRegistration",
						},
					},
				},
			},
			expectedSchemaCount: 1,
			wantError:          false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client with existing objects
			objs := make([]runtime.Object, len(tc.existingSchemas))
			for i, schema := range tc.existingSchemas {
				objs[i] = schema
			}
			client := kcpfakeclient.NewSimpleClientset(objs...)

			// Create informers
			informerFactory := apisinformers.NewSharedInformerFactoryWithOptions(client, time.Minute)
			apiExportInformer := informerFactory.Apis().V1alpha1().APIExports()
			apiResourceSchemaInformer := informerFactory.Apis().V1alpha1().APIResourceSchemas()

			// Add existing schemas to informer
			for _, schema := range tc.existingSchemas {
				err := apiResourceSchemaInformer.Informer().GetStore().Add(schema)
				if err != nil {
					t.Fatalf("Failed to add schema to informer: %v", err)
				}
			}

			// Create controller
			controller, err := NewController(
				client.Cluster(),
				apiExportInformer,
				apiResourceSchemaInformer,
			)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			// Test ensureTMCAPIExport
			err = controller.ensureTMCAPIExport(ctx, tc.cluster)
			if (err != nil) != tc.wantError {
				t.Errorf("ensureTMCAPIExport() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if tc.wantError {
				return
			}

			// Verify APIExport was created
			apiExports, err := client.ApisV1alpha1().APIExports().
				Cluster(tc.cluster.Path()).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("Failed to list APIExports: %v", err)
			}

			if len(apiExports.Items) != 1 {
				t.Errorf("Expected 1 APIExport, got %d", len(apiExports.Items))
				return
			}

			apiExport := &apiExports.Items[0]
			if apiExport.Name != TMCAPIExportName {
				t.Errorf("Expected APIExport name %q, got %q", TMCAPIExportName, apiExport.Name)
			}

			// Verify resource schemas
			if len(apiExport.Spec.LatestResourceSchemas) != tc.expectedSchemaCount {
				t.Errorf("Expected %d resource schemas, got %d", 
					tc.expectedSchemaCount, len(apiExport.Spec.LatestResourceSchemas))
			}

			// Verify permission claims exist
			if len(apiExport.Spec.PermissionClaims) == 0 {
				t.Error("Expected permission claims, got none")
			}

			// Verify conditions are set
			if !conditions.IsTrue(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady) {
				t.Error("Expected VirtualWorkspaceURLsReady condition to be True")
			}
		})
	}
}

func TestController_syncAPIExport(t *testing.T) {
	tests := map[string]struct {
		cluster         logicalcluster.Name
		existingExport  *apisv1alpha1.APIExport
		existingSchemas []*apisv1alpha1.APIResourceSchema
		expectUpdate    bool
		wantError      bool
	}{
		"no update needed when export is current": {
			cluster: "root:test",
			existingExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
				},
				Spec: apisv1alpha1.APIExportSpec{
					LatestResourceSchemas: []string{
						"clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					PermissionClaims: []apisv1alpha1.PermissionClaim{
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
					},
				},
			},
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
					},
				},
			},
			expectUpdate: false,
			wantError:   false,
		},
		"updates export when schemas change": {
			cluster: "root:test",
			existingExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
				},
				Spec: apisv1alpha1.APIExportSpec{
					LatestResourceSchemas: []string{},
					PermissionClaims: []apisv1alpha1.PermissionClaim{
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
					},
				},
			},
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workloadplacements.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group:   tmcv1alpha1.GroupName,
						Version: "v1alpha1",
					},
				},
			},
			expectUpdate: true,
			wantError:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client with existing objects
			objs := make([]runtime.Object, 0, len(tc.existingSchemas)+1)
			for _, schema := range tc.existingSchemas {
				objs = append(objs, schema)
			}
			if tc.existingExport != nil {
				objs = append(objs, tc.existingExport)
			}
			client := kcpfakeclient.NewSimpleClientset(objs...)

			// Create informers
			informerFactory := apisinformers.NewSharedInformerFactoryWithOptions(client, time.Minute)
			apiExportInformer := informerFactory.Apis().V1alpha1().APIExports()
			apiResourceSchemaInformer := informerFactory.Apis().V1alpha1().APIResourceSchemas()

			// Add existing objects to informers
			if tc.existingExport != nil {
				err := apiExportInformer.Informer().GetStore().Add(tc.existingExport)
				if err != nil {
					t.Fatalf("Failed to add export to informer: %v", err)
				}
			}
			for _, schema := range tc.existingSchemas {
				err := apiResourceSchemaInformer.Informer().GetStore().Add(schema)
				if err != nil {
					t.Fatalf("Failed to add schema to informer: %v", err)
				}
			}

			// Create controller
			controller, err := NewController(
				client.Cluster(),
				apiExportInformer,
				apiResourceSchemaInformer,
			)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			// Track initial update count
			initialActions := len(client.Actions())

			// Test syncAPIExport
			err = controller.syncAPIExport(ctx, tc.cluster, tc.existingExport)
			if (err != nil) != tc.wantError {
				t.Errorf("syncAPIExport() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if tc.wantError {
				return
			}

			// Check if update occurred
			finalActions := len(client.Actions())
			updateOccurred := finalActions > initialActions

			if updateOccurred != tc.expectUpdate {
				t.Errorf("Expected update = %v, got %v (actions: %d -> %d)", 
					tc.expectUpdate, updateOccurred, initialActions, finalActions)
			}
		})
	}
}

func TestController_getTMCResourceSchemas(t *testing.T) {
	tests := map[string]struct {
		cluster         logicalcluster.Name
		existingSchemas []*apisv1alpha1.APIResourceSchema
		expectedCount   int
	}{
		"finds TMC schemas": {
			cluster: "root:test",
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterregistrations.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group: tmcv1alpha1.GroupName,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workloadplacements.v1alpha1.tmc.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group: tmcv1alpha1.GroupName,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workspaces.v1alpha1.tenancy.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group: "tenancy.kcp.io",
					},
				},
			},
			expectedCount: 2,
		},
		"returns empty list when no TMC schemas exist": {
			cluster: "root:test",
			existingSchemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "workspaces.v1alpha1.tenancy.kcp.io",
					},
					Spec: apisv1alpha1.APIResourceSchemaSpec{
						Group: "tenancy.kcp.io",
					},
				},
			},
			expectedCount: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create fake client with existing objects
			objs := make([]runtime.Object, len(tc.existingSchemas))
			for i, schema := range tc.existingSchemas {
				objs[i] = schema
			}
			client := kcpfakeclient.NewSimpleClientset(objs...)

			// Create informers
			informerFactory := apisinformers.NewSharedInformerFactoryWithOptions(client, time.Minute)
			apiExportInformer := informerFactory.Apis().V1alpha1().APIExports()
			apiResourceSchemaInformer := informerFactory.Apis().V1alpha1().APIResourceSchemas()

			// Add existing schemas to informer
			for _, schema := range tc.existingSchemas {
				err := apiResourceSchemaInformer.Informer().GetStore().Add(schema)
				if err != nil {
					t.Fatalf("Failed to add schema to informer: %v", err)
				}
			}

			// Create controller
			controller, err := NewController(
				client.Cluster(),
				apiExportInformer,
				apiResourceSchemaInformer,
			)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			// Test getTMCResourceSchemas
			schemas := controller.getTMCResourceSchemas(tc.cluster)

			if len(schemas) != tc.expectedCount {
				t.Errorf("Expected %d schemas, got %d", tc.expectedCount, len(schemas))
			}

			// Verify all returned schemas are TMC schemas
			for _, schemaName := range schemas {
				found := false
				for _, existingSchema := range tc.existingSchemas {
					if existingSchema.Name == schemaName && existingSchema.Spec.Group == tmcv1alpha1.GroupName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Returned schema %s is not a valid TMC schema", schemaName)
				}
			}
		})
	}
}

func TestController_schemaSetsEqual(t *testing.T) {
	controller := &Controller{}

	tests := map[string]struct {
		a        []string
		b        []string
		expected bool
	}{
		"equal empty sets": {
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		"equal non-empty sets": {
			a:        []string{"schema1", "schema2"},
			b:        []string{"schema2", "schema1"},
			expected: true,
		},
		"different lengths": {
			a:        []string{"schema1"},
			b:        []string{"schema1", "schema2"},
			expected: false,
		},
		"different content": {
			a:        []string{"schema1", "schema2"},
			b:        []string{"schema1", "schema3"},
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := controller.schemaSetsEqual(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("schemaSetsEqual(%v, %v) = %v, want %v", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestIsTMCSchema(t *testing.T) {
	tests := map[string]struct {
		schema   *apisv1alpha1.APIResourceSchema
		expected bool
	}{
		"TMC schema": {
			schema: &apisv1alpha1.APIResourceSchema{
				Spec: apisv1alpha1.APIResourceSchemaSpec{
					Group: tmcv1alpha1.GroupName,
				},
			},
			expected: true,
		},
		"non-TMC schema": {
			schema: &apisv1alpha1.APIResourceSchema{
				Spec: apisv1alpha1.APIResourceSchemaSpec{
					Group: "tenancy.kcp.io",
				},
			},
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := isTMCSchema(tc.schema)
			if result != tc.expected {
				t.Errorf("isTMCSchema() = %v, want %v", result, tc.expected)
			}
		})
	}
}