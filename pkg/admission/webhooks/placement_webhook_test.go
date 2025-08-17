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

package webhooks

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewPlacementWebhook(t *testing.T) {
	webhook, err := NewPlacementWebhook(nil)
	require.NoError(t, err)
	require.NotNil(t, webhook)

	// Verify it implements the expected interfaces
	require.Implements(t, (*admission.MutationInterface)(nil), webhook)
	require.Implements(t, (*admission.ValidationInterface)(nil), webhook)
	require.Implements(t, (*admission.InitializationValidator)(nil), webhook)
}

func TestPlacementWebhook_Admit(t *testing.T) {
	tests := map[string]struct {
		placement       *unstructured.Unstructured
		operation       admission.Operation
		clusterName     string
		expectMutation  bool
		expectedLabels  map[string]string
		expectedAnnotations map[string]string
		expectError     bool
	}{
		"create placement with defaults": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			},
			operation:      admission.Create,
			clusterName:    "root:test",
			expectMutation: true,
			expectedLabels: map[string]string{
				TMCWorkspaceLabel:              "root:test",
				TMCPlacementConstraintsLabel:   "none",
			},
			expectedAnnotations: map[string]string{
				TMCPlacementStrategyAnnotation: StrategySpread,
				TMCPlacementPriorityAnnotation: strconv.Itoa(DefaultPriority),
				TMCPlacementStatusAnnotation:   "pending",
			},
		},
		"create placement with existing labels and annotations": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"labels": map[string]interface{}{
							"existing": "label",
							TMCPlacementConstraintsLabel: "existing-constraint",
						},
						"annotations": map[string]interface{}{
							"existing": "annotation",
							TMCPlacementStrategyAnnotation: StrategyBinpack,
							TMCPlacementPriorityAnnotation: "500",
						},
					},
					"spec": map[string]interface{}{},
				},
			},
			operation:      admission.Create,
			clusterName:    "root:test",
			expectMutation: true,
			expectedLabels: map[string]string{
				"existing":                      "label",
				TMCWorkspaceLabel:               "root:test",
				TMCPlacementConstraintsLabel:    "existing-constraint",
			},
			expectedAnnotations: map[string]string{
				"existing":                      "annotation",
				TMCPlacementStrategyAnnotation:  StrategyBinpack,
				TMCPlacementPriorityAnnotation:  "500",
				TMCPlacementStatusAnnotation:    "pending",
			},
		},
		"update placement status": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"annotations": map[string]interface{}{
							TMCPlacementStatusAnnotation: "pending",
						},
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
					},
				},
			},
			operation:      admission.Update,
			clusterName:    "root:test",
			expectMutation: true,
			expectedAnnotations: map[string]string{
				TMCPlacementStatusAnnotation: "updating",
			},
		},
		"non-workloadplacement resource ignored": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
					},
				},
			},
			operation:      admission.Create,
			clusterName:    "root:test",
			expectMutation: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			webhook := &placementWebhook{
				Handler: admission.NewHandler(admission.Create, admission.Update),
			}

			// Create admission attributes
			attrs := admission.NewAttributesRecord(
				tc.placement,
				nil, // old object
				schema.GroupVersionKind{Group: "workload.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"},
				tc.placement.GetNamespace(),
				tc.placement.GetName(),
				schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"},
				"", // subresource
				tc.operation,
				nil, // options
				false, // dry run
				&user.DefaultInfo{Name: "test-user"},
			)

			// Set cluster context
			ctx := genericapirequest.WithCluster(context.Background(), genericapirequest.Cluster{Name: logicalcluster.Name(tc.clusterName)})

			// Call Admit
			err := webhook.Admit(ctx, attrs, nil)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.expectMutation {
				// Check labels
				if tc.expectedLabels != nil {
					labels := tc.placement.GetLabels()
					for key, expectedValue := range tc.expectedLabels {
						assert.Equal(t, expectedValue, labels[key], "Label %s should match", key)
					}
				}

				// Check annotations
				if tc.expectedAnnotations != nil {
					annotations := tc.placement.GetAnnotations()
					for key, expectedValue := range tc.expectedAnnotations {
						assert.Equal(t, expectedValue, annotations[key], "Annotation %s should match", key)
					}
				}

				// Check spec mutations for create operations
				if tc.operation == admission.Create {
					spec, found, err := unstructured.NestedMap(tc.placement.Object, "spec")
					require.NoError(t, err)
					require.True(t, found)

					// Should have default strategy
					strategy, found, err := unstructured.NestedString(spec, "strategy")
					require.NoError(t, err)
					if found {
						assert.Equal(t, StrategySpread, strategy)
					}

					// Should have default replicas
					replicas, found, err := unstructured.NestedInt64(spec, "replicas")
					require.NoError(t, err)
					if found {
						assert.Equal(t, int64(1), replicas)
					}
				}
			}
		})
	}
}

func TestPlacementWebhook_Validate(t *testing.T) {
	tests := map[string]struct {
		placement   *unstructured.Unstructured
		expectError bool
		errorContains string
	}{
		"valid placement with all fields": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"annotations": map[string]interface{}{
							TMCPlacementPriorityAnnotation: "500",
						},
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"replicas": int64(3),
						"targetClusters": []interface{}{
							map[string]interface{}{
								"name": "cluster-1",
							},
							map[string]interface{}{
								"name": "cluster-2",
							},
						},
						"constraints": map[string]interface{}{
							"zone": "us-west-2",
							"env":  "production",
						},
						"affinity": map[string]interface{}{
							"nodeAffinity": []interface{}{
								map[string]interface{}{
									"key":      "kubernetes.io/arch",
									"operator": "In",
									"values":   []interface{}{"amd64"},
								},
							},
							"clusterAffinity": []interface{}{
								map[string]interface{}{
									"key":      "location",
									"operator": "NotIn",
									"values":   []interface{}{"us-east-1"},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		"missing spec": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
				},
			},
			expectError:   true,
			errorContains: "spec is required",
		},
		"invalid strategy": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": "InvalidStrategy",
					},
				},
			},
			expectError:   true,
			errorContains: "Unsupported value",
		},
		"empty strategy": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": "",
					},
				},
			},
			expectError:   true,
			errorContains: "strategy is required",
		},
		"invalid replicas - negative": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"replicas": int64(-1),
					},
				},
			},
			expectError:   true,
			errorContains: "must be non-negative",
		},
		"invalid replicas - zero": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"replicas": int64(0),
					},
				},
			},
			expectError:   true,
			errorContains: "must be greater than 0",
		},
		"too many target clusters": {
			placement: func() *unstructured.Unstructured {
				clusters := make([]interface{}, MaxTargetClusters+1)
				for i := 0; i <= MaxTargetClusters; i++ {
					clusters[i] = map[string]interface{}{
						"name": "cluster-" + strconv.Itoa(i),
					}
				}
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.kcp.io/v1alpha1",
						"kind":       "WorkloadPlacement",
						"metadata": map[string]interface{}{
							"name":      "test-placement",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"strategy":       StrategySpread,
							"targetClusters": clusters,
						},
					},
				}
			}(),
			expectError:   true,
			errorContains: "Too many",
		},
		"duplicate target clusters": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"targetClusters": []interface{}{
							map[string]interface{}{"name": "cluster-1"},
							map[string]interface{}{"name": "cluster-1"},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "Duplicate value",
		},
		"invalid cluster name": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"targetClusters": []interface{}{
							map[string]interface{}{"name": "INVALID_CLUSTER_NAME!"},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "must be a valid DNS name",
		},
		"empty target clusters list": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy":       StrategySpread,
						"targetClusters": []interface{}{},
					},
				},
			},
			expectError:   true,
			errorContains: "cannot be empty if specified",
		},
		"too many constraints": {
			placement: func() *unstructured.Unstructured {
				constraints := make(map[string]interface{})
				for i := 0; i <= MaxConstraintPairs; i++ {
					constraints["key"+strconv.Itoa(i)] = "value" + strconv.Itoa(i)
				}
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.kcp.io/v1alpha1",
						"kind":       "WorkloadPlacement",
						"metadata": map[string]interface{}{
							"name":      "test-placement",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"strategy":    StrategySpread,
							"constraints": constraints,
						},
					},
				}
			}(),
			expectError:   true,
			errorContains: "Too many",
		},
		"invalid priority - too low": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"annotations": map[string]interface{}{
							TMCPlacementPriorityAnnotation: "0",
						},
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
					},
				},
			},
			expectError:   true,
			errorContains: "must be between",
		},
		"invalid priority - too high": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"annotations": map[string]interface{}{
							TMCPlacementPriorityAnnotation: "1001",
						},
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
					},
				},
			},
			expectError:   true,
			errorContains: "must be between",
		},
		"invalid priority - not a number": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
						"annotations": map[string]interface{}{
							TMCPlacementPriorityAnnotation: "not-a-number",
						},
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
					},
				},
			},
			expectError:   true,
			errorContains: "must be a valid integer",
		},
		"too many node affinity rules": {
			placement: func() *unstructured.Unstructured {
				rules := make([]interface{}, MaxAffinityRules+1)
				for i := 0; i <= MaxAffinityRules; i++ {
					rules[i] = map[string]interface{}{
						"key":      "key" + strconv.Itoa(i),
						"operator": "In",
						"values":   []interface{}{"value"},
					}
				}
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "workload.kcp.io/v1alpha1",
						"kind":       "WorkloadPlacement",
						"metadata": map[string]interface{}{
							"name":      "test-placement",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"strategy": StrategySpread,
							"affinity": map[string]interface{}{
								"nodeAffinity": rules,
							},
						},
					},
				}
			}(),
			expectError:   true,
			errorContains: "Too many",
		},
		"missing affinity key": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "workload.kcp.io/v1alpha1",
					"kind":       "WorkloadPlacement",
					"metadata": map[string]interface{}{
						"name":      "test-placement",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"strategy": StrategySpread,
						"affinity": map[string]interface{}{
							"nodeAffinity": []interface{}{
								map[string]interface{}{
									"operator": "In",
									"values":   []interface{}{"value"},
								},
							},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "affinity key is required",
		},
		"non-workloadplacement resource ignored": {
			placement: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "nginx",
							},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			webhook := &placementWebhook{
				Handler: admission.NewHandler(admission.Create, admission.Update),
			}

			// Determine GVK/GVR based on the object
			var gvk schema.GroupVersionKind
			var gvr schema.GroupVersionResource
			
			if kind := tc.placement.GetKind(); kind == "Pod" {
				gvk = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
				gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
			} else {
				gvk = schema.GroupVersionKind{Group: "workload.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"}
				gvr = schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"}
			}

			// Create admission attributes
			attrs := admission.NewAttributesRecord(
				tc.placement,
				nil, // old object
				gvk,
				tc.placement.GetNamespace(),
				tc.placement.GetName(),
				gvr,
				"", // subresource
				admission.Create,
				nil, // options
				false, // dry run
				&user.DefaultInfo{Name: "test-user"},
			)

			// Call Validate
			err := webhook.Validate(context.Background(), attrs, nil)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPlacementWebhook_ValidateInitialization(t *testing.T) {
	webhook := &placementWebhook{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}

	err := webhook.ValidateInitialization()
	require.NoError(t, err)
}

func TestRegisterPlacementWebhook(t *testing.T) {
	plugins := admission.NewPlugins()
	
	// Check that plugin is not registered initially
	initialPluginNames := plugins.Registered()
	initialFound := false
	for _, name := range initialPluginNames {
		if name == PlacementPluginName {
			initialFound = true
			break
		}
	}
	assert.False(t, initialFound, "PlacementWebhook should not be registered initially")

	// Register the plugin
	RegisterPlacementWebhook(plugins)

	// Verify the plugin was registered
	pluginNames := plugins.Registered()
	found := false
	for _, name := range pluginNames {
		if name == PlacementPluginName {
			found = true
			break
		}
	}
	assert.True(t, found, "PlacementWebhook should be registered after registration")
	
	// Verify we have more plugins after registration
	assert.Greater(t, len(pluginNames), len(initialPluginNames), "Should have more plugins after registration")
}

// Helper function to create test placement objects
func createTestPlacement(name, namespace string, spec map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "workload.kcp.io/v1alpha1",
			"kind":       "WorkloadPlacement",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
}

func TestPlacementWebhook_AdmitInvalidObject(t *testing.T) {
	webhook := &placementWebhook{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}

	// Test with invalid object type
	attrs := admission.NewAttributesRecord(
		&runtime.Unknown{}, // Invalid type
		nil,
		schema.GroupVersionKind{Group: "workload.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"},
		"default",
		"test",
		schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"},
		"",
		admission.Create,
		nil,
		false,
		&user.DefaultInfo{Name: "test-user"},
	)

	ctx := genericapirequest.WithCluster(context.Background(), genericapirequest.Cluster{Name: logicalcluster.Name("root:test")})
	err := webhook.Admit(ctx, attrs, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected type")
}

func TestPlacementWebhook_ValidateInvalidObject(t *testing.T) {
	webhook := &placementWebhook{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}

	// Test with invalid object type
	attrs := admission.NewAttributesRecord(
		&runtime.Unknown{}, // Invalid type
		nil,
		schema.GroupVersionKind{Group: "workload.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"},
		"default",
		"test",
		schema.GroupVersionResource{Group: "workload.kcp.io", Version: "v1alpha1", Resource: "workloadplacements"},
		"",
		admission.Create,
		nil,
		false,
		&user.DefaultInfo{Name: "test-user"},
	)

	err := webhook.Validate(context.Background(), attrs, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected type")
}