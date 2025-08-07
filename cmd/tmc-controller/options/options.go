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

package options

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// Options contains configuration for TMC controller
type Options struct {
	// KCP connection
	KCPKubeconfig string
	Workspace     string

	// Target cluster connections  
	ClusterKubeconfigs map[string]string

	// Controller configuration
	ResyncPeriod    time.Duration
	WorkerCount     int
	MetricsPort     int
	HealthPort      int
	PProfPort       int

	// Logging
	LogLevel int

	// TMC-specific configuration
	ClusterHealthCheckInterval time.Duration
	PlacementDecisionTimeout   time.Duration
	MaxConcurrentPlacements    int

	// Feature gates
	EnablePlacementController    bool
	EnableClusterHealthChecking  bool
	EnableWorkloadSynchronization bool
}

// NewOptions creates default options for the TMC controller
func NewOptions() *Options {
	return &Options{
		ClusterKubeconfigs:           make(map[string]string),
		ResyncPeriod:                 30 * time.Second,
		WorkerCount:                  5,
		MetricsPort:                  8080,
		HealthPort:                   8081,
		PProfPort:                    8082,
		LogLevel:                     2,
		ClusterHealthCheckInterval:   30 * time.Second,
		PlacementDecisionTimeout:     5 * time.Minute,
		MaxConcurrentPlacements:      10,
		EnablePlacementController:    true,
		EnableClusterHealthChecking:  true,
		EnableWorkloadSynchronization: false, // Phase 3 feature
	}
}

// AddFlags adds flags to the flagset
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	// KCP connection flags
	fs.StringVar(&o.KCPKubeconfig, "kcp-kubeconfig", o.KCPKubeconfig,
		"Path to KCP kubeconfig file")
	fs.StringVar(&o.Workspace, "workspace", o.Workspace,
		"KCP workspace to watch (e.g., root:my-workspace)")

	// Target cluster connection flags
	fs.StringToStringVar(&o.ClusterKubeconfigs, "cluster-kubeconfigs", o.ClusterKubeconfigs,
		"Map of cluster names to kubeconfig paths (e.g., cluster1=/path/to/config)")

	// Controller configuration flags
	fs.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod,
		"Resync period for controllers")
	fs.IntVar(&o.WorkerCount, "worker-count", o.WorkerCount,
		"Number of worker threads for each controller")
	fs.IntVar(&o.MetricsPort, "metrics-port", o.MetricsPort,
		"Port for metrics endpoint")
	fs.IntVar(&o.HealthPort, "health-port", o.HealthPort,
		"Port for health endpoint")
	fs.IntVar(&o.PProfPort, "pprof-port", o.PProfPort,
		"Port for pprof endpoint (0 to disable)")

	// Logging flags
	fs.IntVar(&o.LogLevel, "log-level", o.LogLevel,
		"Log verbosity level")

	// TMC-specific configuration flags
	fs.DurationVar(&o.ClusterHealthCheckInterval, "cluster-health-check-interval", o.ClusterHealthCheckInterval,
		"Interval for cluster health checks")
	fs.DurationVar(&o.PlacementDecisionTimeout, "placement-decision-timeout", o.PlacementDecisionTimeout,
		"Timeout for placement decisions")
	fs.IntVar(&o.MaxConcurrentPlacements, "max-concurrent-placements", o.MaxConcurrentPlacements,
		"Maximum number of concurrent placement operations")

	// Feature gate flags
	fs.BoolVar(&o.EnablePlacementController, "enable-placement-controller", o.EnablePlacementController,
		"Enable workload placement controller")
	fs.BoolVar(&o.EnableClusterHealthChecking, "enable-cluster-health-checking", o.EnableClusterHealthChecking,
		"Enable cluster health checking")
	fs.BoolVar(&o.EnableWorkloadSynchronization, "enable-workload-synchronization", o.EnableWorkloadSynchronization,
		"Enable workload synchronization (Phase 3 feature)")
}

// Validate validates the options
func (o *Options) Validate() error {
	var errors []string

	if o.KCPKubeconfig == "" {
		errors = append(errors, "--kcp-kubeconfig is required")
	}
	if o.Workspace == "" {
		errors = append(errors, "--workspace is required")
	}
	if len(o.ClusterKubeconfigs) == 0 {
		errors = append(errors, "at least one --cluster-kubeconfigs entry is required")
	}

	// Validate port ranges
	if o.MetricsPort <= 0 || o.MetricsPort > 65535 {
		errors = append(errors, "--metrics-port must be between 1 and 65535")
	}
	if o.HealthPort <= 0 || o.HealthPort > 65535 {
		errors = append(errors, "--health-port must be between 1 and 65535")
	}
	if o.PProfPort < 0 || o.PProfPort > 65535 {
		errors = append(errors, "--pprof-port must be between 0 and 65535")
	}

	// Validate worker count
	if o.WorkerCount <= 0 {
		errors = append(errors, "--worker-count must be positive")
	}

	// Validate TMC-specific settings
	if o.ClusterHealthCheckInterval <= 0 {
		errors = append(errors, "--cluster-health-check-interval must be positive")
	}
	if o.PlacementDecisionTimeout <= 0 {
		errors = append(errors, "--placement-decision-timeout must be positive")
	}
	if o.MaxConcurrentPlacements <= 0 {
		errors = append(errors, "--max-concurrent-placements must be positive")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, ", "))
	}

	return nil
}

// Complete fills in any fields not set that are required to have valid data
func (o *Options) Complete() error {
	// Set defaults for derived values if needed
	return nil
}