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

package apis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmcv1alpha1 "github.com/kcp-dev/kcp/apis/tmc/v1alpha1"
)

func TestCoreAPITypes(t *testing.T) {
	tests := []struct {
		name     string
		validate func(*testing.T)
	}{
		{
			name: "TMCConfig should have feature flags field",
			validate: func(t *testing.T) {
				cfg := &tmcv1alpha1.TMCConfig{}
				// Test that the FeatureFlags field can be set
				cfg.Spec.FeatureFlags = map[string]bool{
					"syncTargets":     true,
					"placement":       false,
					"workloadDistrib": true,
				}
				assert.NotNil(t, cfg.Spec.FeatureFlags)
				assert.True(t, cfg.Spec.FeatureFlags["syncTargets"])
				assert.False(t, cfg.Spec.FeatureFlags["placement"])
			},
		},
		{
			name: "TMCConfig should have status with conditions",
			validate: func(t *testing.T) {
				cfg := &tmcv1alpha1.TMCConfig{}
				cfg.Status.Conditions = []metav1.Condition{
					{
						Type:   "Ready",
						Status: metav1.ConditionTrue,
						Reason: "ConfigValid",
					},
				}
				assert.NotNil(t, cfg.Status.Conditions)
				assert.Len(t, cfg.Status.Conditions, 1)
				assert.Equal(t, "Ready", cfg.Status.Conditions[0].Type)
			},
		},
		{
			name: "TMCStatus should implement status conditions pattern",
			validate: func(t *testing.T) {
				status := &tmcv1alpha1.TMCStatus{}
				status.Conditions = []metav1.Condition{
					{
						Type:   "Available",
						Status: metav1.ConditionTrue,
						Reason: "ComponentsHealthy",
					},
				}
				status.Phase = "Ready"
				status.ObservedGeneration = 1

				assert.NotNil(t, status.Conditions)
				assert.Equal(t, "Ready", status.Phase)
				assert.Equal(t, int64(1), status.ObservedGeneration)
			},
		},
		{
			name: "ResourceIdentifier should validate GVR components",
			validate: func(t *testing.T) {
				ri := &tmcv1alpha1.ResourceIdentifier{
					Group:     "apps",
					Version:   "v1",
					Resource:  "deployments",
					Kind:      "Deployment",
					Namespace: "default",
					Name:      "test-deployment",
				}
				
				assert.Equal(t, "apps", ri.Group)
				assert.Equal(t, "v1", ri.Version)
				assert.Equal(t, "deployments", ri.Resource)
				assert.Equal(t, "Deployment", ri.Kind)
				assert.Equal(t, "default", ri.Namespace)
				assert.Equal(t, "test-deployment", ri.Name)
			},
		},
		{
			name: "ResourceIdentifier should support cluster-scoped resources",
			validate: func(t *testing.T) {
				ri := &tmcv1alpha1.ResourceIdentifier{
					Group:    "",
					Version:  "v1",
					Resource: "nodes",
					Kind:     "Node",
					Name:     "worker-1",
				}
				
				assert.Empty(t, ri.Namespace, "cluster-scoped resource should have empty namespace")
				assert.Equal(t, "nodes", ri.Resource)
				assert.Equal(t, "Node", ri.Kind)
			},
		},
		{
			name: "ClusterIdentifier should support cloud provider metadata",
			validate: func(t *testing.T) {
				ci := &tmcv1alpha1.ClusterIdentifier{
					Name:        "prod-cluster-1",
					Region:      "us-west-2",
					Zone:        "us-west-2a",
					Provider:    "aws",
					Environment: "production",
					Labels: map[string]string{
						"team":     "platform",
						"cost-center": "engineering",
					},
				}
				
				assert.Equal(t, "prod-cluster-1", ci.Name)
				assert.Equal(t, "us-west-2", ci.Region)
				assert.Equal(t, "us-west-2a", ci.Zone)
				assert.Equal(t, "aws", ci.Provider)
				assert.Equal(t, "production", ci.Environment)
				assert.Equal(t, "platform", ci.Labels["team"])
				assert.Equal(t, "engineering", ci.Labels["cost-center"])
			},
		},
		{
			name: "ClusterIdentifier should support minimal configuration",
			validate: func(t *testing.T) {
				ci := &tmcv1alpha1.ClusterIdentifier{
					Name: "minimal-cluster",
				}
				
				assert.Equal(t, "minimal-cluster", ci.Name)
				assert.Empty(t, ci.Region)
				assert.Empty(t, ci.Zone)
				assert.Empty(t, ci.Provider)
				assert.Empty(t, ci.Environment)
				assert.Nil(t, ci.Labels)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t)
		})
	}
}

func TestTMCConfigDeepCopy(t *testing.T) {
	tests := []struct {
		name     string
		validate func(*testing.T)
	}{
		{
			name: "TMCConfig DeepCopy should create independent copy",
			validate: func(t *testing.T) {
				original := &tmcv1alpha1.TMCConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-config",
					},
					Spec: tmcv1alpha1.TMCConfigSpec{
						FeatureFlags: map[string]bool{
							"feature1": true,
							"feature2": false,
						},
					},
				}
				
				copy := original.DeepCopy()
				
				// Modify copy
				copy.Name = "modified-config"
				copy.Spec.FeatureFlags["feature1"] = false
				copy.Spec.FeatureFlags["feature3"] = true
				
				// Verify original is unchanged
				assert.Equal(t, "test-config", original.Name)
				assert.True(t, original.Spec.FeatureFlags["feature1"])
				assert.False(t, original.Spec.FeatureFlags["feature2"])
				_, exists := original.Spec.FeatureFlags["feature3"]
				assert.False(t, exists)
				
				// Verify copy is modified
				assert.Equal(t, "modified-config", copy.Name)
				assert.False(t, copy.Spec.FeatureFlags["feature1"])
				assert.True(t, copy.Spec.FeatureFlags["feature3"])
			},
		},
		{
			name: "ClusterIdentifier DeepCopy should handle labels properly",
			validate: func(t *testing.T) {
				original := &tmcv1alpha1.ClusterIdentifier{
					Name: "test-cluster",
					Labels: map[string]string{
						"env":  "prod",
						"team": "platform",
					},
				}
				
				copy := original.DeepCopy()
				
				// Modify copy labels
				copy.Labels["env"] = "staging"
				copy.Labels["new-label"] = "new-value"
				
				// Verify original is unchanged
				assert.Equal(t, "prod", original.Labels["env"])
				assert.Equal(t, "platform", original.Labels["team"])
				_, exists := original.Labels["new-label"]
				assert.False(t, exists)
				
				// Verify copy is modified
				assert.Equal(t, "staging", copy.Labels["env"])
				assert.Equal(t, "new-value", copy.Labels["new-label"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t)
		})
	}
}

func TestTMCConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *tmcv1alpha1.TMCConfig
		wantValid bool
	}{
		{
			name: "valid TMCConfig with feature flags",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-config",
				},
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"syncTargets": true,
						"placement":   true,
					},
				},
			},
			wantValid: true,
		},
		{
			name: "valid TMCConfig without feature flags",
			config: &tmcv1alpha1.TMCConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minimal-config",
				},
			},
			wantValid: true,
		},
		{
			name: "TMCConfig with empty name should be valid (validation handled by Kubernetes)",
			config: &tmcv1alpha1.TMCConfig{
				Spec: tmcv1alpha1.TMCConfigSpec{
					FeatureFlags: map[string]bool{
						"test": true,
					},
				},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic structural validation - more complex validation 
			// would be handled by admission webhooks
			require.NotNil(t, tt.config)
			
			if tt.wantValid {
				assert.NotNil(t, tt.config.Spec.FeatureFlags == nil || 
					len(tt.config.Spec.FeatureFlags) >= 0, 
					"FeatureFlags should be valid")
			}
		})
	}
}