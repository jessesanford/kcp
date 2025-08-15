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

package tmc

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	basecontroller "github.com/kcp-dev/kcp/pkg/controller"
	tmccontroller "github.com/kcp-dev/kcp/pkg/tmc/controller"
	"github.com/kcp-dev/kcp/pkg/features"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// TMCController manages TMC resources using the base controller framework.
// It coordinates cluster registration and workload placement following KCP patterns
// with proper workspace isolation and feature flag support.
type TMCController struct {
	// Core dependencies
	client   client.Client
	scheme   *runtime.Scheme
	logger   logr.Logger
	
	// Child controllers for TMC-specific functionality
	clusterController   tmccontroller.Controller
	placementController tmccontroller.Controller
	
	// Configuration
	workspace logicalcluster.Name
	
	// Status tracking
	started bool
}

// TMCControllerConfig holds configuration for creating a TMC controller.
type TMCControllerConfig struct {
	// Client for Kubernetes API operations
	Client client.Client
	
	// KCPClusterClient for KCP cluster-aware operations
	KCPClusterClient kcpclientset.ClusterInterface
	
	// Scheme for object serialization
	Scheme *runtime.Scheme
	
	// InformerFactory for shared informers
	InformerFactory kcpinformers.SharedInformerFactory
	
	// Workspace for logical cluster isolation
	Workspace logicalcluster.Name
	
	// Logger for structured logging
	Logger logr.Logger
}

// NewTMCController creates a new TMC controller with feature flag validation.
// This controller orchestrates both cluster registration and workload placement
// using the base controller framework for consistent KCP integration.
//
// The controller will only start if the TMCAlpha feature gate is enabled,
// ensuring TMC functionality is properly gated during development.
func NewTMCController(config *TMCControllerConfig) (*TMCController, error) {
	if config == nil {
		return nil, fmt.Errorf("TMCControllerConfig cannot be nil")
	}

	// Validate feature gate - TMC functionality must be explicitly enabled
	if !features.IsEnabled(tmcv1alpha1.TMCFeatureGate) {
		return nil, fmt.Errorf("TMC feature gate %q is not enabled", tmcv1alpha1.TMCFeatureGate)
	}

	if config.Workspace.Empty() {
		return nil, fmt.Errorf("workspace cannot be empty - workspace isolation is required for TMC")
	}

	logger := config.Logger.WithName("tmc-controller")

	// Create base controller factory for consistent controller creation
	factory := basecontroller.NewBaseControllerFactory(
		config.KCPClusterClient,
		config.Scheme,
		nil, // Metrics will be added in later phases
	)

	// Create cluster registration controller
	clusterReconciler := NewClusterReconciler(config.Client, logger)
	clusterCtrl := factory.CreateController(
		"tmc-cluster-registration",
		config.Workspace,
		config.InformerFactory,
		clusterReconciler,
	)

	// Create workload placement controller
	placementReconciler := NewPlacementReconciler(config.Client, logger)
	placementCtrl := factory.CreateController(
		"tmc-workload-placement", 
		config.Workspace,
		config.InformerFactory,
		placementReconciler,
	)

	return &TMCController{
		client:              config.Client,
		scheme:              config.Scheme,
		logger:              logger,
		clusterController:   clusterCtrl,
		placementController: placementCtrl,
		workspace:           config.Workspace,
	}, nil
}

// Start implements Controller.Start and begins TMC reconciliation loops.
// This method validates the feature gate again at runtime to ensure
// TMC functionality remains properly controlled.
func (c *TMCController) Start(ctx context.Context) error {
	// Double-check feature gate at start time in case it was disabled
	if !features.IsEnabled(tmcv1alpha1.TMCFeatureGate) {
		return fmt.Errorf("cannot start TMC controller: feature gate %q is disabled", tmcv1alpha1.TMCFeatureGate)
	}

	if c.started {
		return fmt.Errorf("TMC controller already started")
	}

	c.logger.Info("Starting TMC controller", "workspace", c.workspace)

	// Start child controllers concurrently
	errChan := make(chan error, 2)

	// Start cluster registration controller
	go func() {
		if err := c.clusterController.Start(ctx); err != nil {
			errChan <- fmt.Errorf("cluster controller failed to start: %w", err)
		}
	}()

	// Start workload placement controller  
	go func() {
		if err := c.placementController.Start(ctx); err != nil {
			errChan <- fmt.Errorf("placement controller failed to start: %w", err)
		}
	}()

	c.started = true
	c.logger.Info("TMC controller started successfully", 
		"cluster-controller", c.clusterController.Name(),
		"placement-controller", c.placementController.Name())

	// Wait for context cancellation
	<-ctx.Done()

	c.logger.Info("TMC controller shutting down")
	return nil
}

// Stop implements Controller.Stop and gracefully shuts down the TMC controller.
// This ensures all child controllers are properly stopped before returning.
func (c *TMCController) Stop() error {
	if !c.started {
		return nil
	}

	c.logger.Info("Stopping TMC controller")

	// Stop child controllers
	var stopErr error
	
	if err := c.clusterController.Stop(); err != nil {
		stopErr = fmt.Errorf("failed to stop cluster controller: %w", err)
		c.logger.Error(err, "Error stopping cluster controller")
	}

	if err := c.placementController.Stop(); err != nil {
		if stopErr == nil {
			stopErr = fmt.Errorf("failed to stop placement controller: %w", err)
		}
		c.logger.Error(err, "Error stopping placement controller")
	}

	c.started = false
	c.logger.Info("TMC controller stopped")
	return stopErr
}

// Name implements Controller.Name and returns the controller identifier.
func (c *TMCController) Name() string {
	return "tmc-controller"
}

// Ready implements Controller.Ready and reports controller readiness.
// The TMC controller is ready when both child controllers are ready.
func (c *TMCController) Ready() bool {
	if !c.started {
		return false
	}

	// Check if both child controllers are ready
	return c.clusterController.Ready() && c.placementController.Ready()
}

// GetWorkspace returns the logical cluster workspace for this controller.
// This is useful for debugging and metrics collection.
func (c *TMCController) GetWorkspace() logicalcluster.Name {
	return c.workspace
}