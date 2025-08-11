/*
Copyright 2025 The KCP Authors.

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

package cluster

import (
	"context"

	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
)

// convertToCommitterResource converts ClusterRegistration to committer Resource for testing.
func convertToCommitterResource(cluster *tmcv1alpha1.ClusterRegistration) *committer.Resource[tmcv1alpha1.ClusterRegistrationSpec, tmcv1alpha1.ClusterRegistrationStatus] {
	return &committer.Resource[tmcv1alpha1.ClusterRegistrationSpec, tmcv1alpha1.ClusterRegistrationStatus]{
		ObjectMeta: cluster.ObjectMeta,
		Spec:       cluster.Spec,
		Status:     cluster.Status,
	}
}

// getTestContext returns a test context.
func getTestContext() context.Context {
	return context.Background()
}