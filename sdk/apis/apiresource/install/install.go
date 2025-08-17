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

// Package install registers the apiresource.kcp.io API group and its types with the runtime scheme.
package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	apiresourcev1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apiresource/v1alpha1"
)

// Install registers all API versions of the apiresource.kcp.io API group with the given scheme.
// This function should be called during scheme initialization to ensure all apiresource types
// are available for use.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(apiresourcev1alpha1.AddToScheme(scheme))
}
