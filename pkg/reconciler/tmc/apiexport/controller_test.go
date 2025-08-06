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

package apiexport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpfakeclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	apisv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha1"
)

func TestTMCAPIExportController(t *testing.T) {
	tests := map[string]struct {
		apiExport       *apisv1alpha1.APIExport
		schemas         []*apisv1alpha1.APIResourceSchema
		workspace       string
		expectProcessed bool
	}{
		"tmc apiexport processing": {
			apiExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:tmc",
					},
				},
				Spec: apisv1alpha1.APIExportSpec{
					LatestResourceSchemas: []string{
						"tmc.kcp.io.v1alpha1.ClusterRegistration",
						"tmc.kcp.io.v1alpha1.WorkloadPlacement",
					},
				},
			},
			schemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tmc.kcp.io.v1alpha1.ClusterRegistration",
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "root:tmc",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tmc.kcp.io.v1alpha1.WorkloadPlacement",
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "root:tmc",
						},
					},
				},
			},
			workspace:       "root:tmc",
			expectProcessed: true,
		},
		"non-tmc apiexport ignored": {
			apiExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-api.kcp.io",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:other",
					},
				},
			},
			workspace:       "root:other",
			expectProcessed: false,
		},
		"tmc apiexport missing schemas": {
			apiExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:tmc",
					},
				},
				Spec: apisv1alpha1.APIExportSpec{
					LatestResourceSchemas: []string{
						"tmc.kcp.io.v1alpha1.ClusterRegistration",
						// Missing WorkloadPlacement schema
					},
				},
			},
			schemas: []*apisv1alpha1.APIResourceSchema{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tmc.kcp.io.v1alpha1.ClusterRegistration",
						Annotations: map[string]string{
							logicalcluster.AnnotationKey: "root:tmc",
						},
					},
				},
			},
			workspace:       "root:tmc",
			expectProcessed: true, // Still processes but logs missing schema
		},
	}

	for testName, testCase := range tests {
		testCase := testCase
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Create fake client
			objs := []runtime.Object{testCase.apiExport}
			for _, schema := range testCase.schemas {
				objs = append(objs, schema)
			}
			kcpClusterClient := kcpfakeclient.NewSimpleClientset(objs...)

			// Create informers
			apiExportInformer := apisv1alpha1informers.NewAPIExportClusterInformer(
				kcpClusterClient,
				time.Minute*10,
				cache.Indexers{},
			)
			apiResourceSchemaInformer := apisv1alpha1informers.NewAPIResourceSchemaClusterInformer(
				kcpClusterClient,
				time.Minute*10,
				cache.Indexers{},
			)

			// Create controller
			controller, err := NewController(
				kcpClusterClient,
				apiExportInformer,
				apiResourceSchemaInformer,
			)
			require.NoError(t, err, "failed to create controller")

			// Add objects to informer stores
			clusterName := logicalcluster.Name(testCase.workspace)
			err = apiExportInformer.Informer().GetStore().Add(testCase.apiExport)
			require.NoError(t, err)

			for _, schema := range testCase.schemas {
				err = apiResourceSchemaInformer.Informer().GetStore().Add(schema)
				require.NoError(t, err)
			}

			// Test isTMCAPIExport
			isTMC := controller.isTMCAPIExport(testCase.apiExport)
			require.Equal(t, testCase.expectProcessed, isTMC, "isTMCAPIExport result mismatch")

			if testCase.expectProcessed {
				// Test reconcile
				ctx := context.Background()
				err = controller.reconcileTMCAPIExport(ctx, testCase.apiExport)
				require.NoError(t, err, "reconcileTMCAPIExport should not return error")
			}
		})
	}
}

func TestTMCAPIExportControllerName(t *testing.T) {
	require.Equal(t, "tmc-apiexport", ControllerName)
	require.Equal(t, "tmc.kcp.io", TMCAPIExportName)
}