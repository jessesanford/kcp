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

// Package install installs all TMC API groups, making them available for
// registration with scheme and discovery systems.
package install

import (
	"k8s.io/apimachinery/pkg/runtime"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Install registers the TMC API group and its types in the given scheme.
// This makes the TMC APIs available for use by controllers, clients, and
// other components that need to work with TMC resources.
//
// Parameters:
//   - scheme: The runtime scheme to register the APIs with
//
// This function should be called during application initialization to ensure
// that TMC APIs are properly registered before any controllers or clients
// attempt to use them.
func Install(scheme *runtime.Scheme) {
	tmcv1alpha1.AddToScheme(scheme)
}
