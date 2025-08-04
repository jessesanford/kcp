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

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	kcpclientsetfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestTMCAPIExportController_BasicFunctionality(t *testing.T) {
	cluster1 := logicalcluster.Name("root:org:ws1")

	// Create fake client
	kcpClient := kcpclientsetfake.NewSimpleClientset()

	// Create informers
	kcpInformers := kcpinformers.NewSharedInformerFactory(kcpClient, 0)

	// Create controller
	controller, err := NewController(
		kcpClient.Cluster,
		kcpInformers.Apis().V1alpha2().APIExports(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test creating APIExport when it doesn't exist
	key := cluster1.String() + "/" + TMCAPIExportName
	err = controller.process(ctx, key)
	require.NoError(t, err)

	// Verify APIExport was created
	actions := kcpClient.Actions()
	require.Len(t, actions, 1)
	require.Equal(t, "create", actions[0].GetVerb())
	require.Equal(t, "apiexports", actions[0].GetResource().Resource)
}

func TestTMCAPIExportController_ExistingAPIExport(t *testing.T) {
	cluster1 := logicalcluster.Name("root:org:ws1")

	// Create existing APIExport
	existingAPIExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TMCAPIExportName,
			Annotations: map[string]string{logicalcluster.AnnotationKey: cluster1.String()},
		},
		Spec: apisv1alpha2.APIExportSpec{
			Resources: []apisv1alpha2.ResourceSchema{
				{
					Name:   "clusters",
					Group:  "tmc.kcp.io",
					Schema: "v1alpha1.clusters.tmc.kcp.io",
				},
			},
		},
	}

	// Create fake client with existing APIExport
	kcpClient := kcpclientsetfake.NewSimpleClientset(existingAPIExport)

	// Create informers
	kcpInformers := kcpinformers.NewSharedInformerFactory(kcpClient, 0)

	// Add existing object to informer
	kcpInformers.Apis().V1alpha2().APIExports().Informer().GetStore().Add(existingAPIExport)

	// Create controller
	controller, err := NewController(
		kcpClient.Cluster,
		kcpInformers.Apis().V1alpha2().APIExports(),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test reconciling existing APIExport
	key := cluster1.String() + "/" + TMCAPIExportName
	err = controller.process(ctx, key)
	require.NoError(t, err)

	// Verify status update was attempted
	actions := kcpClient.Actions()
	require.Len(t, actions, 1)
	require.Equal(t, "update", actions[0].GetVerb())
	require.Equal(t, "status", actions[0].GetSubresource())
}

func TestConditionsEqual(t *testing.T) {
	// Test empty conditions
	require.True(t, conditionsEqual(nil, nil))
	require.True(t, conditionsEqual([]conditionsv1alpha1.Condition{}, []conditionsv1alpha1.Condition{}))

	// Test different lengths
	cond1 := []conditionsv1alpha1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Test", Message: "Test"},
	}
	require.False(t, conditionsEqual(cond1, nil))

	// Test same conditions
	cond2 := []conditionsv1alpha1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Test", Message: "Test"},
	}
	require.True(t, conditionsEqual(cond1, cond2))

	// Test different conditions
	cond3 := []conditionsv1alpha1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Test", Message: "Test"},
	}
	require.False(t, conditionsEqual(cond1, cond3))
}