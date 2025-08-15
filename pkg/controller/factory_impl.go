// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"k8s.io/apimachinery/pkg/runtime"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Factory creates base controllers with common configuration.
// This follows the factory pattern to allow dependency injection
// and consistent controller configuration across KCP.
type Factory interface {
	// CreateController creates a new controller with the given reconciler
	CreateController(
		name string,
		workspace logicalcluster.Name,
		informerFactory kcpinformers.SharedInformerFactory,
		reconciler Reconciler,
	) BaseController
}

// BaseControllerFactory implements the Factory interface for creating
// base controllers with consistent configuration following KCP patterns.
type BaseControllerFactory struct {
	kcpClusterClient kcpclientset.ClusterInterface
	scheme           *runtime.Scheme
	metrics          *ManagerMetrics
}

// NewBaseControllerFactory creates a new factory for base controllers.
// This provides a consistent way to create controllers with shared dependencies
// following KCP architectural patterns for multi-tenant controller management.
func NewBaseControllerFactory(
	kcpClusterClient kcpclientset.ClusterInterface,
	scheme *runtime.Scheme,
	metrics *ManagerMetrics,
) Factory {
	return &BaseControllerFactory{
		kcpClusterClient: kcpClusterClient,
		scheme:           scheme,
		metrics:          metrics,
	}
}

// CreateController implements Factory.CreateController
func (f *BaseControllerFactory) CreateController(
	name string,
	workspace logicalcluster.Name,
	informerFactory kcpinformers.SharedInformerFactory,
	reconciler Reconciler,
) BaseController {
	config := &BaseControllerConfig{
		Name:             name,
		Workspace:        workspace,
		WorkerCount:      1, // Default to single worker, can be overridden
		Reconciler:       reconciler,
		Metrics:          f.metrics,
		InformerFactory:  informerFactory,
		KcpClusterClient: f.kcpClusterClient,
	}

	return NewBaseController(config)
}