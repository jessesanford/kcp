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

	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
)

func TestIsTMCAPIExport(t *testing.T) {
	tests := map[string]struct {
		apiExport       *apisv1alpha1.APIExport
		expectProcessed bool
	}{
		"tmc apiexport": {
			apiExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: TMCAPIExportName,
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:tmc",
					},
				},
			},
			expectProcessed: true,
		},
		"non-tmc apiexport": {
			apiExport: &apisv1alpha1.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-api.kcp.io",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:other",
					},
				},
			},
			expectProcessed: false,
		},
	}

	for testName, testCase := range tests {
		testCase := testCase
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Create a minimal controller with mock schema getter
			controller := &Controller{
				getAPIResourceSchema: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error) {
					return &apisv1alpha1.APIResourceSchema{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}, nil
				},
			}

			// Test isTMCAPIExport
			isTMC := controller.isTMCAPIExport(testCase.apiExport)
			require.Equal(t, testCase.expectProcessed, isTMC, "isTMCAPIExport result mismatch")

			if testCase.expectProcessed {
				// Test reconcile - this should not error for basic cases
				ctx := context.Background()
				err := controller.reconcileTMCAPIExport(ctx, testCase.apiExport)
				require.NoError(t, err, "reconcileTMCAPIExport should not return error")
			}
		})
	}
}

func TestReconcileTMCAPIExportSchemaValidation(t *testing.T) {
	controller := &Controller{
		getAPIResourceSchema: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error) {
			// Simulate schema exists for ClusterRegistration but not WorkloadPlacement
			if name == "tmc.kcp.io.v1alpha1.ClusterRegistration" {
				return &apisv1alpha1.APIResourceSchema{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
				}, nil
			}
			return nil, errors.NewNotFound(apisv1alpha1.Resource("apiresourceschema"), name)
		},
	}

	apiExport := &apisv1alpha1.APIExport{
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
	}

	ctx := context.Background()
	err := controller.reconcileTMCAPIExport(ctx, apiExport)
	require.NoError(t, err, "reconcileTMCAPIExport should handle missing schemas gracefully")
}

func TestTMCAPIExportControllerConstants(t *testing.T) {
	require.Equal(t, "tmc-apiexport", ControllerName)
	require.Equal(t, "tmc.kcp.io", TMCAPIExportName)
}