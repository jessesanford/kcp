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

package interfaces

import (
	"context"

	"github.com/kcp-dev/kcp/pkg/apis/core"
	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// HealthChecker validates deployment health across KCP logical clusters.
// All health checking operations must be performed within the appropriate
// logical cluster context and respect workspace isolation boundaries.
//
// Implementations must be thread-safe and capable of handling concurrent
// health checks across multiple logical clusters and deployments.
//
// Example usage:
//   checker := NewHealthChecker(kcpClient)
//   target := HealthTarget{
//     Name:           "my-app",
//     Namespace:      "default", 
//     LogicalCluster: cluster,
//     Type:           "Deployment",
//   }
//   status, err := checker.Check(ctx, target)
type HealthChecker interface {
	// Check performs health validation within the target's logical cluster
	Check(ctx context.Context, target HealthTarget) (*HealthStatus, error)

	// CheckInCluster performs health validation in a specific logical cluster
	CheckInCluster(ctx context.Context, cluster core.LogicalCluster, target HealthTarget) (*HealthStatus, error)

	// WaitForReady waits until target is healthy within its logical cluster
	WaitForReady(ctx context.Context, target HealthTarget,
		config types.HealthCheckConfig) error

	// RegisterProbe adds a custom health probe (must be thread-safe)
	RegisterProbe(name string, probe HealthProbe) error

	// ListProbes returns available health probes
	ListProbes() []string
}

// HealthTarget identifies what to check within KCP's logical cluster architecture
type HealthTarget struct {
	Name           string                `json:"name"`
	Namespace      string                `json:"namespace"`
	LogicalCluster core.LogicalCluster   `json:"logicalCluster,omitempty"`
	Type           string                `json:"type"`
	Selector       map[string]string     `json:"selector,omitempty"`
}

// HealthStatus represents health check result
type HealthStatus struct {
	Healthy    bool              `json:"healthy"`
	Ready      bool              `json:"ready"`
	Message    string            `json:"message,omitempty"`
	Conditions []HealthCondition `json:"conditions,omitempty"`
}

// HealthCondition is a specific health aspect
type HealthCondition struct {
	Type    string `json:"type"`
	Status  bool   `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthProbe defines a custom health check that can operate across logical clusters.
// Probe implementations must be stateless and thread-safe to support concurrent
// health checking across multiple deployments and logical clusters.
type HealthProbe interface {
	// Name returns the probe name for identification and logging
	Name() string

	// Check executes the health probe within the target's logical cluster context
	Check(ctx context.Context, target HealthTarget) (bool, string, error)

	// CheckInCluster executes the health probe in a specific logical cluster
	CheckInCluster(ctx context.Context, cluster core.LogicalCluster, target HealthTarget) (bool, string, error)
}