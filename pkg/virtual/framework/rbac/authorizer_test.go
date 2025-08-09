/*
Copyright 2023 The KCP Authors.

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

package rbac

import (
	"testing"

	"github.com/stretchr/testify/require"

	kcpfakecorev1 "github.com/kcp-dev/client-go/kubernetes/fake"
	kcprbacinformers "github.com/kcp-dev/client-go/informers/rbac/v1"
	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewSubjectLocator(t *testing.T) {
	tests := map[string]struct {
		cluster   logicalcluster.Name
		wantError bool
	}{
		"creates subject locator for valid cluster": {
			cluster:   "root:test-workspace",
			wantError: false,
		},
		"creates subject locator for empty cluster": {
			cluster:   "",
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := kcpfakecorev1.NewSimpleClientset()
			informers := kcprbacinformers.NewSharedInformerFactory(client.RbacV1(), 0)

			locator := NewSubjectLocator(tc.cluster, informers)
			require.NotNil(t, locator, "subject locator should not be nil")
		})
	}
}