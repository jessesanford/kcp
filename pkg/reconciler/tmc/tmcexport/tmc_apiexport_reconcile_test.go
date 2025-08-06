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

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpfakeclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	apisinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestController_reconcileAPIExport(t *testing.T) {
	tests := map[string]struct {
		cluster         logicalcluster.Name
		existingExport  *apisv1alpha1.APIExport
		existingSchemas []*apisv1alpha1.APIResourceSchema
		opts            ReconcileOptions
		expectCreate    bool
		expectUpdate    bool
		wantError      bool
	}{
		"creates APIExport when none exists": {
			cluster:         "root:test",
			existingExport:  nil,
			existingSchemas: []*apisv1alpha1.APIResourceSchema{},
			opts: ReconcileOptions{
				CreateMissingSchemas: false,
				ValidatePermissions:  true,
			},
			expectCreate: true,
			expectUpdate: false,
			wantError:   false,
		},
		"updates existing APIExport with new schemas": {
			cluster: "root:test",
			existingExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
				},
				Spec: apisv1alpha1.APIExportSpec{
					LatestResourceSchemas: []string{},
					PermissionClaims:      []apisv1alpha1.PermissionClaim{},
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
			opts: ReconcileOptions{
				CreateMissingSchemas: false,
				ValidatePermissions:  true,
			},
			expectCreate: false,
			expectUpdate: true,
			wantError:   false,
		},
		"creates missing schemas when requested": {
			cluster:         "root:test",
			existingExport:  nil,
			existingSchemas: []*apisv1alpha1.APIResourceSchema{},
			opts: ReconcileOptions{
				CreateMissingSchemas: true,
				ValidatePermissions:  true,
			},
			expectCreate: true,
			expectUpdate: false,
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

			// Track initial action count
			initialActions := len(client.Actions())

			// Test reconcileAPIExport
			err = controller.reconcileAPIExport(ctx, tc.cluster, tc.opts)
			if (err != nil) != tc.wantError {
				t.Errorf("reconcileAPIExport() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if tc.wantError {
				return
			}

			// Check actions taken
			finalActions := len(client.Actions())
			actionsOccurred := finalActions > initialActions

			if tc.expectCreate || tc.expectUpdate {
				if !actionsOccurred {
					t.Errorf("Expected actions to occur, but no actions were taken")
				}
			}

			// Verify APIExport exists
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

			// Verify basic permission claims are set
			if len(apiExport.Spec.PermissionClaims) == 0 {
				t.Error("Expected permission claims to be set")
			}
		})
	}
}

func TestController_createTMCAPIExport(t *testing.T) {
	tests := map[string]struct {
		cluster         logicalcluster.Name
		existingSchemas []*apisv1alpha1.APIResourceSchema
		opts            ReconcileOptions
		expectedSchemas int
		wantError      bool
	}{
		"creates APIExport with existing schemas": {
			cluster: "root:test",
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
			opts: ReconcileOptions{
				CreateMissingSchemas: false,
				ValidatePermissions:  true,
			},
			expectedSchemas: 1,
			wantError:      false,
		},
		"creates APIExport and missing schemas": {
			cluster:         "root:test",
			existingSchemas: []*apisv1alpha1.APIResourceSchema{},
			opts: ReconcileOptions{
				CreateMissingSchemas: true,
				ValidatePermissions:  true,
			},
			expectedSchemas: 2, // ClusterRegistration + WorkloadPlacement
			wantError:      false,
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

			// Test createTMCAPIExport
			err = controller.createTMCAPIExport(ctx, tc.cluster, tc.opts)
			if (err != nil) != tc.wantError {
				t.Errorf("createTMCAPIExport() error = %v, wantError %v", err, tc.wantError)
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

			// Verify conditions
			if !conditions.IsTrue(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady) {
				t.Error("Expected VirtualWorkspaceURLsReady condition to be True")
			}

			// Check if schemas were created when requested
			if tc.opts.CreateMissingSchemas {
				schemas, err := client.ApisV1alpha1().APIResourceSchemas().
					Cluster(tc.cluster.Path()).List(ctx, metav1.ListOptions{})
				if err != nil {
					t.Fatalf("Failed to list APIResourceSchemas: %v", err)
				}

				tmcSchemaCount := 0
				for _, schema := range schemas.Items {
					if schema.Spec.Group == tmcv1alpha1.GroupName {
						tmcSchemaCount++
					}
				}

				if tmcSchemaCount != tc.expectedSchemas {
					t.Errorf("Expected %d TMC schemas, got %d", tc.expectedSchemas, tmcSchemaCount)
				}
			}
		})
	}
}

func TestController_getRequiredPermissionClaims(t *testing.T) {
	controller := &Controller{}

	claims := controller.getRequiredPermissionClaims()

	if len(claims) == 0 {
		t.Error("Expected permission claims, got none")
	}

	// Check for required claims
	requiredClaims := map[string]string{
		"coordination.k8s.io": "leases",
		"":                    "events",
		"apps":                "deployments",
		"":                    "services",
	}

	foundClaims := make(map[string]string)
	for _, claim := range claims {
		foundClaims[claim.GroupResource.Group] = claim.GroupResource.Resource
	}

	for group, resource := range requiredClaims {
		if foundResource, exists := foundClaims[group]; !exists || foundResource != resource {
			t.Errorf("Missing or incorrect permission claim for %s/%s", group, resource)
		}
	}
}

func TestController_getExpectedAPIResourceSchemas(t *testing.T) {
	controller := &Controller{}

	schemas := controller.getExpectedAPIResourceSchemas()

	if len(schemas) == 0 {
		t.Error("Expected APIResourceSchema specifications, got none")
	}

	// Verify TMC schemas are present
	expectedKinds := []string{"ClusterRegistration", "WorkloadPlacement"}
	foundKinds := make(map[string]bool)

	for _, schema := range schemas {
		if schema.Group != tmcv1alpha1.GroupName {
			t.Errorf("Expected group %s, got %s", tmcv1alpha1.GroupName, schema.Group)
		}
		if schema.Version != "v1alpha1" {
			t.Errorf("Expected version v1alpha1, got %s", schema.Version)
		}
		foundKinds[schema.Names.Kind] = true
	}

	for _, expectedKind := range expectedKinds {
		if !foundKinds[expectedKind] {
			t.Errorf("Expected schema for kind %s not found", expectedKind)
		}
	}
}

func TestController_setAPIExportConditions(t *testing.T) {
	tests := map[string]struct {
		hasResourceSchemas bool
		expectedReady      bool
	}{
		"ready with schemas": {
			hasResourceSchemas: true,
			expectedReady:      true,
		},
		"not ready without schemas": {
			hasResourceSchemas: false,
			expectedReady:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			controller := &Controller{}
			apiExport := &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
				},
			}

			controller.setAPIExportConditions(apiExport, tc.hasResourceSchemas)

			// Check TMCResourceSchemasReady condition
			if tc.hasResourceSchemas {
				if !conditions.IsTrue(apiExport, TMCResourceSchemasReady) {
					t.Error("Expected TMCResourceSchemasReady to be True when schemas exist")
				}
			} else {
				if conditions.IsTrue(apiExport, TMCResourceSchemasReady) {
					t.Error("Expected TMCResourceSchemasReady to be False when no schemas exist")
				}
			}

			// Check VirtualWorkspaceURLsReady condition is set
			if conditions.Get(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady) == nil {
				t.Error("Expected VirtualWorkspaceURLsReady condition to be set")
			}

			// Check TMCAPIExportReady condition
			if tc.expectedReady {
				if !conditions.IsTrue(apiExport, TMCAPIExportReady) {
					t.Error("Expected TMCAPIExportReady to be True")
				}
			} else {
				if conditions.IsTrue(apiExport, TMCAPIExportReady) {
					t.Error("Expected TMCAPIExportReady to be False")
				}
			}
		})
	}
}