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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestSyncTargetConditions(t *testing.T) {
	t.Run("GetCondition returns nil for non-existent condition", func(t *testing.T) {
		st := &SyncTarget{}
		condition := st.GetCondition(SyncTargetReady)
		assert.Nil(t, condition)
	})

	t.Run("SetCondition adds new condition", func(t *testing.T) {
		st := &SyncTarget{}
		condition := conditionsv1alpha1.Condition{
			Type:    SyncTargetReady,
			Status:  corev1.ConditionTrue,
			Reason:  "AllComponentsReady",
			Message: "All syncer components are ready",
		}

		st.SetCondition(condition)

		assert.Len(t, st.Status.Conditions, 1)
		retrieved := st.GetCondition(SyncTargetReady)
		require.NotNil(t, retrieved)
		assert.Equal(t, SyncTargetReady, retrieved.Type)
		assert.Equal(t, corev1.ConditionTrue, retrieved.Status)
	})

	t.Run("SetCondition updates existing condition", func(t *testing.T) {
		st := &SyncTarget{
			Status: SyncTargetStatus{
				Conditions: conditionsv1alpha1.Conditions{
					{
						Type:    SyncTargetReady,
						Status:  corev1.ConditionFalse,
						Reason:  "NotReady",
						Message: "Components are not ready",
					},
				},
			},
		}

		updatedCondition := conditionsv1alpha1.Condition{
			Type:    SyncTargetReady,
			Status:  corev1.ConditionTrue,
			Reason:  "AllReady",
			Message: "All components are ready",
		}

		st.SetCondition(updatedCondition)

		assert.Len(t, st.Status.Conditions, 1)
		retrieved := st.GetCondition(SyncTargetReady)
		require.NotNil(t, retrieved)
		assert.Equal(t, corev1.ConditionTrue, retrieved.Status)
		assert.Equal(t, "AllReady", retrieved.Reason)
	})

	t.Run("SetCondition preserves other conditions", func(t *testing.T) {
		st := &SyncTarget{
			Status: SyncTargetStatus{
				Conditions: conditionsv1alpha1.Conditions{
					{
						Type:   SyncTargetSyncerReady,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   SyncTargetClusterReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		}

		newCondition := conditionsv1alpha1.Condition{
			Type:   SyncTargetReady,
			Status: corev1.ConditionTrue,
		}

		st.SetCondition(newCondition)

		assert.Len(t, st.Status.Conditions, 3)
		assert.NotNil(t, st.GetCondition(SyncTargetSyncerReady))
		assert.NotNil(t, st.GetCondition(SyncTargetClusterReady))
		assert.NotNil(t, st.GetCondition(SyncTargetReady))
	})
}

func TestSyncTargetConditionHelpers(t *testing.T) {
	tests := map[string]struct {
		conditions []conditionsv1alpha1.Condition
		testCases  map[string]func(*testing.T, *SyncTarget)
	}{
		"empty conditions": {
			conditions: []conditionsv1alpha1.Condition{},
			testCases: map[string]func(*testing.T, *SyncTarget){
				"HasCondition returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.HasCondition(SyncTargetReady))
				},
				"IsConditionTrue returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsConditionTrue(SyncTargetReady))
				},
				"IsConditionFalse returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsConditionFalse(SyncTargetReady))
				},
				"IsReady returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsReady())
				},
			},
		},
		"condition true": {
			conditions: []conditionsv1alpha1.Condition{
				{
					Type:   SyncTargetReady,
					Status: corev1.ConditionTrue,
				},
			},
			testCases: map[string]func(*testing.T, *SyncTarget){
				"HasCondition returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.HasCondition(SyncTargetReady))
				},
				"IsConditionTrue returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.IsConditionTrue(SyncTargetReady))
				},
				"IsConditionFalse returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsConditionFalse(SyncTargetReady))
				},
				"IsReady returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.IsReady())
				},
			},
		},
		"condition false": {
			conditions: []conditionsv1alpha1.Condition{
				{
					Type:   SyncTargetReady,
					Status: corev1.ConditionFalse,
				},
			},
			testCases: map[string]func(*testing.T, *SyncTarget){
				"HasCondition returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.HasCondition(SyncTargetReady))
				},
				"IsConditionTrue returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsConditionTrue(SyncTargetReady))
				},
				"IsConditionFalse returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.IsConditionFalse(SyncTargetReady))
				},
				"IsReady returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsReady())
				},
			},
		},
		"multiple conditions": {
			conditions: []conditionsv1alpha1.Condition{
				{
					Type:   SyncTargetReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   SyncTargetSyncerReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   SyncTargetClusterReady,
					Status: corev1.ConditionFalse,
				},
			},
			testCases: map[string]func(*testing.T, *SyncTarget){
				"IsReady returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.IsReady())
				},
				"IsSyncerReady returns true": func(t *testing.T, st *SyncTarget) {
					assert.True(t, st.IsSyncerReady())
				},
				"IsClusterReady returns false": func(t *testing.T, st *SyncTarget) {
					assert.False(t, st.IsClusterReady())
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			st := &SyncTarget{
				Status: SyncTargetStatus{
					Conditions: tc.conditions,
				},
			}

			for testName, testFunc := range tc.testCases {
				t.Run(testName, func(t *testing.T) {
					testFunc(t, st)
				})
			}
		})
	}
}

func TestGetSetConditions(t *testing.T) {
	t.Run("GetConditions returns empty slice for new SyncTarget", func(t *testing.T) {
		st := &SyncTarget{}
		conditions := st.GetConditions()
		assert.Empty(t, conditions)
	})

	t.Run("GetConditions returns existing conditions", func(t *testing.T) {
		expectedConditions := conditionsv1alpha1.Conditions{
			{
				Type:   SyncTargetReady,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   SyncTargetSyncerReady,
				Status: corev1.ConditionFalse,
			},
		}

		st := &SyncTarget{
			Status: SyncTargetStatus{
				Conditions: expectedConditions,
			},
		}

		conditions := st.GetConditions()
		assert.Equal(t, expectedConditions, conditions)
	})

	t.Run("SetConditions replaces all conditions", func(t *testing.T) {
		st := &SyncTarget{
			Status: SyncTargetStatus{
				Conditions: conditionsv1alpha1.Conditions{
					{
						Type:   SyncTargetReady,
						Status: corev1.ConditionFalse,
					},
				},
			},
		}

		newConditions := conditionsv1alpha1.Conditions{
			{
				Type:   SyncTargetSyncerReady,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   SyncTargetClusterReady,
				Status: corev1.ConditionTrue,
			},
		}

		st.SetConditions(newConditions)

		assert.Equal(t, newConditions, st.Status.Conditions)
		assert.Len(t, st.Status.Conditions, 2)
	})
}

func TestSyncTargetValidation(t *testing.T) {
	validSyncTarget := &SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-synctarget",
		},
		Spec: SyncTargetSpec{
			ClusterRef: ClusterReference{
				Name: "test-cluster",
			},
			Location: "us-west-2",
		},
	}

	t.Run("ValidateCreate succeeds for valid SyncTarget", func(t *testing.T) {
		err := validSyncTarget.ValidateCreate()
		assert.NoError(t, err)
	})

	t.Run("ValidateCreate fails for missing cluster ref", func(t *testing.T) {
		invalidSyncTarget := validSyncTarget.DeepCopy()
		invalidSyncTarget.Spec.ClusterRef.Name = ""

		err := invalidSyncTarget.ValidateCreate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cluster reference name is required")
	})

	t.Run("ValidateUpdate succeeds for valid update", func(t *testing.T) {
		oldSyncTarget := validSyncTarget.DeepCopy()
		newSyncTarget := validSyncTarget.DeepCopy()
		newSyncTarget.Spec.Location = "us-east-1"

		err := newSyncTarget.ValidateUpdate(oldSyncTarget)
		assert.NoError(t, err)
	})

	t.Run("ValidateUpdate fails for immutable field change", func(t *testing.T) {
		oldSyncTarget := validSyncTarget.DeepCopy()
		newSyncTarget := validSyncTarget.DeepCopy()
		newSyncTarget.Spec.ClusterRef.Name = "different-cluster"

		err := newSyncTarget.ValidateUpdate(oldSyncTarget)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cluster reference name is immutable")
	})
}