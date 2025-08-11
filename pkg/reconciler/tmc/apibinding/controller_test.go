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

package apibinding

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
)

func TestIsTMCAPIBinding(t *testing.T) {
	tests := map[string]struct {
		binding *apisv1alpha2.APIBinding
		want    bool
	}{
		"cluster registration binding": {
			binding: &apisv1alpha2.APIBinding{
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: ClusterRegistrationAPIExport,
						},
					},
				},
			},
			want: true,
		},
		"workload placement binding": {
			binding: &apisv1alpha2.APIBinding{
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: WorkloadPlacementAPIExport,
						},
					},
				},
			},
			want: true,
		},
		"generic tmc binding": {
			binding: &apisv1alpha2.APIBinding{
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: "tmc-custom-api",
						},
					},
				},
			},
			want: true,
		},
		"non-tmc binding": {
			binding: &apisv1alpha2.APIBinding{
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: "kubernetes.default",
						},
					},
				},
			},
			want: false,
		},
		"binding without export reference": {
			binding: &apisv1alpha2.APIBinding{
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{},
				},
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isTMCAPIBinding(tc.binding)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsTMCAPIExport(t *testing.T) {
	tests := map[string]struct {
		export *apisv1alpha2.APIExport
		want   bool
	}{
		"cluster registration export": {
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: ClusterRegistrationAPIExport,
				},
			},
			want: true,
		},
		"workload placement export": {
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: WorkloadPlacementAPIExport,
				},
			},
			want: true,
		},
		"generic tmc export": {
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tmc-custom-export",
				},
			},
			want: true,
		},
		"non-tmc export": {
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubernetes.default",
				},
			},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isTMCAPIExport(tc.export)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetAPIBindingCondition(t *testing.T) {
	now := metav1.NewTime(time.Now())
	
	tests := map[string]struct {
		initialConditions []metav1.Condition
		conditionType     string
		status           metav1.ConditionStatus
		reason           string
		message          string
		wantConditions   []metav1.Condition
	}{
		"add new condition": {
			initialConditions: []metav1.Condition{},
			conditionType:     apisv1alpha2.APIBindingInitialBindingCompleted,
			status:           metav1.ConditionTrue,
			reason:           "TMCAPIExportReady",
			message:          "TMC APIExport is ready",
			wantConditions: []metav1.Condition{
				{
					Type:               apisv1alpha2.APIBindingInitialBindingCompleted,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: now,
					Reason:             "TMCAPIExportReady",
					Message:            "TMC APIExport is ready",
				},
			},
		},
		"update existing condition": {
			initialConditions: []metav1.Condition{
				{
					Type:               apisv1alpha2.APIBindingInitialBindingCompleted,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
					Reason:             "TMCAPIExportNotReady",
					Message:            "TMC APIExport is not ready",
				},
			},
			conditionType: apisv1alpha2.APIBindingInitialBindingCompleted,
			status:       metav1.ConditionTrue,
			reason:       "TMCAPIExportReady",
			message:      "TMC APIExport is ready",
			wantConditions: []metav1.Condition{
				{
					Type:               apisv1alpha2.APIBindingInitialBindingCompleted,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: now,
					Reason:             "TMCAPIExportReady",
					Message:            "TMC APIExport is ready",
				},
			},
		},
		"no change to existing condition": {
			initialConditions: []metav1.Condition{
				{
					Type:               apisv1alpha2.APIBindingInitialBindingCompleted,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
					Reason:             "TMCAPIExportReady",
					Message:            "TMC APIExport is ready",
				},
			},
			conditionType: apisv1alpha2.APIBindingInitialBindingCompleted,
			status:       metav1.ConditionTrue,
			reason:       "TMCAPIExportReady",
			message:      "TMC APIExport is ready",
			wantConditions: []metav1.Condition{
				{
					Type:               apisv1alpha2.APIBindingInitialBindingCompleted,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
					Reason:             "TMCAPIExportReady",
					Message:            "TMC APIExport is ready",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conditions := tc.initialConditions
			setAPIBindingCondition(&conditions, tc.conditionType, tc.status, tc.reason, tc.message, now)
			
			require.Len(t, conditions, len(tc.wantConditions))
			for i, wantCondition := range tc.wantConditions {
				gotCondition := conditions[i]
				assert.Equal(t, wantCondition.Type, gotCondition.Type)
				assert.Equal(t, wantCondition.Status, gotCondition.Status)
				assert.Equal(t, wantCondition.Reason, gotCondition.Reason)
				assert.Equal(t, wantCondition.Message, gotCondition.Message)
				
				// Only check transition time for new conditions
				if name == "add new condition" || name == "update existing condition" {
					assert.Equal(t, wantCondition.LastTransitionTime, gotCondition.LastTransitionTime)
				}
			}
		})
	}
}

func TestUpdateAPIBindingStatus(t *testing.T) {
	ctx := context.Background()
	
	tests := map[string]struct {
		binding         *apisv1alpha2.APIBinding
		export          *apisv1alpha2.APIExport
		wantRequeue     bool
		wantError       bool
	}{
		"export ready - binding should be ready": {
			binding: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: ClusterRegistrationAPIExport,
						},
					},
				},
			},
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: ClusterRegistrationAPIExport,
				},
				Status: apisv1alpha2.APIExportStatus{
					Conditions: []metav1.Condition{
						{
							Type:   apisv1alpha2.APIExportValid,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
		},
		"export not ready - binding should not be ready": {
			binding: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: WorkloadPlacementAPIExport,
						},
					},
				},
			},
			export: &apisv1alpha2.APIExport{
				ObjectMeta: metav1.ObjectMeta{
					Name: WorkloadPlacementAPIExport,
				},
				Status: apisv1alpha2.APIExportStatus{
					Conditions: []metav1.Condition{
						{
							Type:   apisv1alpha2.APIExportValid,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &controller{}
			
			gotRequeue, err := c.updateAPIBindingStatus(ctx, tc.binding, tc.export)
			
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			assert.Equal(t, tc.wantRequeue, gotRequeue)
			
			// Verify that conditions were set
			assert.NotEmpty(t, tc.binding.Status.Conditions)
			
			// Check for the expected condition type
			foundCondition := false
			for _, condition := range tc.binding.Status.Conditions {
				if condition.Type == apisv1alpha2.APIBindingInitialBindingCompleted {
					foundCondition = true
					break
				}
			}
			assert.True(t, foundCondition, "Expected to find InitialBindingCompleted condition")
		})
	}
}

func TestObjOrTombstone(t *testing.T) {
	binding := &apisv1alpha2.APIBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
		},
	}

	tests := map[string]struct {
		obj    interface{}
		want   *apisv1alpha2.APIBinding
		panics bool
	}{
		"direct object": {
			obj:    binding,
			want:   binding,
			panics: false,
		},
		"tombstone object": {
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-key",
				Obj: binding,
			},
			want:   binding,
			panics: false,
		},
		"invalid tombstone": {
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-key",
				Obj: "invalid",
			},
			panics: true,
		},
		"invalid object": {
			obj:    "invalid",
			panics: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.panics {
				assert.Panics(t, func() {
					objOrTombstone[*apisv1alpha2.APIBinding](tc.obj)
				})
			} else {
				result := objOrTombstone[*apisv1alpha2.APIBinding](tc.obj)
				assert.Equal(t, tc.want, result)
			}
		})
	}
}

func TestControllerReconcile(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		binding       *apisv1alpha2.APIBinding
		wantRequeue   bool
		wantError     bool
		setupMocks    func(*controller)
	}{
		"successful reconcile with ready export": {
			binding: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: ClusterRegistrationAPIExport,
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			setupMocks: func(c *controller) {
				c.getAPIExport = func(clusterName logicalcluster.Name, name string) (*apisv1alpha2.APIExport, error) {
					return &apisv1alpha2.APIExport{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterRegistrationAPIExport,
						},
						Status: apisv1alpha2.APIExportStatus{
							Conditions: []metav1.Condition{
								{
									Type:   apisv1alpha2.APIExportValid,
									Status: metav1.ConditionTrue,
								},
							},
						},
					}, nil
				}
			},
		},
		"binding being deleted": {
			binding: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-binding",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: ClusterRegistrationAPIExport,
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			setupMocks:  func(c *controller) {},
		},
		"non-tmc binding": {
			binding: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: "kubernetes.default",
						},
					},
				},
			},
			wantRequeue: false,
			wantError:   false,
			setupMocks:  func(c *controller) {},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &controller{}
			tc.setupMocks(c)
			
			gotRequeue, err := c.reconcile(ctx, tc.binding)
			
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			assert.Equal(t, tc.wantRequeue, gotRequeue)
		})
	}
}

func TestControllerFilterFunctions(t *testing.T) {
	tmcBinding := &apisv1alpha2.APIBinding{
		Spec: apisv1alpha2.APIBindingSpec{
			Reference: apisv1alpha2.BindingReference{
				Export: &apisv1alpha2.ExportBindingReference{
					Name: ClusterRegistrationAPIExport,
				},
			},
		},
	}

	nonTMCBinding := &apisv1alpha2.APIBinding{
		Spec: apisv1alpha2.APIBindingSpec{
			Reference: apisv1alpha2.BindingReference{
				Export: &apisv1alpha2.ExportBindingReference{
					Name: "kubernetes.default",
				},
			},
		},
	}

	tmcExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: WorkloadPlacementAPIExport,
		},
	}

	nonTMCExport := &apisv1alpha2.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes.default",
		},
	}

	tests := map[string]struct {
		obj          interface{}
		filterFunc   func(interface{}) bool
		wantFiltered bool
	}{
		"TMC APIBinding passes filter": {
			obj:          tmcBinding,
			filterFunc:   func(obj interface{}) bool { return isTMCAPIBinding(obj.(*apisv1alpha2.APIBinding)) },
			wantFiltered: true,
		},
		"non-TMC APIBinding rejected by filter": {
			obj:          nonTMCBinding,
			filterFunc:   func(obj interface{}) bool { return isTMCAPIBinding(obj.(*apisv1alpha2.APIBinding)) },
			wantFiltered: false,
		},
		"TMC APIExport passes filter": {
			obj:          tmcExport,
			filterFunc:   func(obj interface{}) bool { return isTMCAPIExport(obj.(*apisv1alpha2.APIExport)) },
			wantFiltered: true,
		},
		"non-TMC APIExport rejected by filter": {
			obj:          nonTMCExport,
			filterFunc:   func(obj interface{}) bool { return isTMCAPIExport(obj.(*apisv1alpha2.APIExport)) },
			wantFiltered: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.filterFunc(tc.obj)
			assert.Equal(t, tc.wantFiltered, result)
		})
	}
}

func TestIndexerFunctions(t *testing.T) {
	tests := map[string]struct {
		obj           interface{}
		indexerFunc   func(interface{}) ([]string, error)
		wantKeys      []string
		wantError     bool
	}{
		"TMC APIBinding by export index": {
			obj: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: ClusterRegistrationAPIExport,
						},
					},
				},
			},
			indexerFunc: IndexTMCAPIBindingsByExportFunc,
			wantKeys:    []string{"root:test//" + ClusterRegistrationAPIExport},
			wantError:   false,
		},
		"non-TMC APIBinding by export index": {
			obj: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: "kubernetes.default",
						},
					},
				},
			},
			indexerFunc: IndexTMCAPIBindingsByExportFunc,
			wantKeys:    []string{},
			wantError:   false,
		},
		"TMC APIBinding by workspace index": {
			obj: &apisv1alpha2.APIBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
					Annotations: map[string]string{
						logicalcluster.AnnotationKey: "root:test",
					},
				},
				Spec: apisv1alpha2.APIBindingSpec{
					Reference: apisv1alpha2.BindingReference{
						Export: &apisv1alpha2.ExportBindingReference{
							Name: WorkloadPlacementAPIExport,
						},
					},
				},
			},
			indexerFunc: IndexTMCAPIBindingsByWorkspaceFunc,
			wantKeys:    []string{"root:test"},
			wantError:   false,
		},
		"invalid object type": {
			obj:         "invalid",
			indexerFunc: IndexTMCAPIBindingsByExportFunc,
			wantKeys:    []string{},
			wantError:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotKeys, err := tc.indexerFunc(tc.obj)
			
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tc.wantKeys, gotKeys)
			}
		})
	}
}