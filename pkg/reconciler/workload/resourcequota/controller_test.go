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

package resourcequota

import (
	"testing"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

func TestAddResourceRequests(t *testing.T) {
	tests := map[string]struct {
		usage    corev1.ResourceList
		requests corev1.ResourceList
		expected corev1.ResourceList
	}{
		"add to empty usage": {
			usage: corev1.ResourceList{},
			requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			expected: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("1"),
				corev1.ResourceRequestsMemory: resource.MustParse("1Gi"),
			},
		},
		"add to existing usage": {
			usage: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("2"),
				corev1.ResourceRequestsMemory: resource.MustParse("2Gi"),
			},
			requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			expected: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("3"),
				corev1.ResourceRequestsMemory: resource.MustParse("3Gi"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &Controller{}
			c.addResourceRequests(tc.usage, tc.requests)

			for resource, expectedQuantity := range tc.expected {
				actualQuantity, exists := tc.usage[resource]
				require.True(t, exists, "Expected resource %s not found", resource)
				require.True(t, expectedQuantity.Equal(actualQuantity),
					"Resource %s: expected %s, got %s",
					resource, expectedQuantity.String(), actualQuantity.String())
			}
		})
	}
}

func TestCheckViolations(t *testing.T) {
	tests := map[string]struct {
		spec           workloadv1alpha1.ResourceQuotaSpec
		used           corev1.ResourceList
		wantViolations int
	}{
		"no violations": {
			spec: workloadv1alpha1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsCPU:    resource.MustParse("10"),
					corev1.ResourceRequestsMemory: resource.MustParse("10Gi"),
				},
			},
			used: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("5"),
				corev1.ResourceRequestsMemory: resource.MustParse("5Gi"),
			},
			wantViolations: 0,
		},
		"cpu violation": {
			spec: workloadv1alpha1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsCPU:    resource.MustParse("5"),
					corev1.ResourceRequestsMemory: resource.MustParse("10Gi"),
				},
			},
			used: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("8"),
				corev1.ResourceRequestsMemory: resource.MustParse("5Gi"),
			},
			wantViolations: 1,
		},
		"multiple violations": {
			spec: workloadv1alpha1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsCPU:    resource.MustParse("5"),
					corev1.ResourceRequestsMemory: resource.MustParse("5Gi"),
				},
			},
			used: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("8"),
				corev1.ResourceRequestsMemory: resource.MustParse("7Gi"),
			},
			wantViolations: 2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			violations := CheckViolations(tc.spec, tc.used)
			require.Len(t, violations, tc.wantViolations)
		})
	}
}