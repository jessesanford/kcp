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

	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// HealthChecker validates deployment health
type HealthChecker interface {
	// Check performs health validation
	Check(ctx context.Context, target HealthTarget) (*HealthStatus, error)

	// WaitForReady waits until target is healthy
	WaitForReady(ctx context.Context, target HealthTarget,
		config types.HealthCheckConfig) error

	// RegisterProbe adds a custom health probe
	RegisterProbe(name string, probe HealthProbe) error
}

// HealthTarget identifies what to check
type HealthTarget struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	Selector  map[string]string `json:"selector,omitempty"`
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

// HealthProbe defines a custom health check
type HealthProbe interface {
	// Name returns the probe name
	Name() string

	// Check executes the health probe
	Check(ctx context.Context, target HealthTarget) (bool, string, error)
}