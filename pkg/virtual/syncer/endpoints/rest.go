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

package endpoints

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

// RESTHandler handles REST operations for virtual workspace resources.
// This is a stub implementation that will be completed in a follow-up PR.
type RESTHandler struct {
	clusterClient   cluster.ClusterInterface
	informerFactory kcpinformers.SharedInformerFactory
	transformer     *ResourceTransformer
	scheme          *runtime.Scheme
}

// RESTConfig provides configuration for creating a RESTHandler
type RESTConfig struct {
	ClusterClient   cluster.ClusterInterface
	InformerFactory kcpinformers.SharedInformerFactory
	Transformer     *ResourceTransformer
	Scheme          *runtime.Scheme
}

// NewRESTHandler creates a new REST handler for virtual workspace operations.
// This is a stub implementation that will be completed in a follow-up PR.
func NewRESTHandler(config *RESTConfig) *RESTHandler {
	return &RESTHandler{
		clusterClient:   config.ClusterClient,
		informerFactory: config.InformerFactory,
		transformer:     config.Transformer,
		scheme:          config.Scheme,
	}
}

// Start initializes the REST handler and starts background processes
// This is a stub implementation that will be completed in a follow-up PR.
func (h *RESTHandler) Start(ctx context.Context) error {
	// TODO: Implement in follow-up PR
	return nil
}

// ServeHTTP implements the http.Handler interface for REST operations
// This is a stub implementation that will be completed in a follow-up PR.
func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement full REST operations in follow-up PR
	http.Error(w, "REST operations not yet implemented", http.StatusNotImplemented)
}