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

// Package options provides configuration options for the TMC controller.
// These options allow fine-tuning of the external controller behavior including
// KCP connection settings, cluster management parameters, and operational configurations.
package options

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// Options contains basic configuration for the TMC controller.
type Options struct {
	// KCP connection
	KCPKubeconfig string
	Workspace     string

	// Target cluster connections
	ClusterKubeconfigs map[string]string

	// Controller configuration
	ResyncPeriod    time.Duration
	WorkerCount     int
	ShutdownTimeout time.Duration

	// Observability
	MetricsPort int
	HealthPort  int
}

// NewOptions creates default options.
func NewOptions() *Options {
	return &Options{
		ClusterKubeconfigs: make(map[string]string),
		ResyncPeriod:       30 * time.Second,
		WorkerCount:        5,
		ShutdownTimeout:    30 * time.Second,
		MetricsPort:        8080,
		HealthPort:         8081,
	}
}

// AddFlags adds TMC controller flags.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KCPKubeconfig, "kcp-kubeconfig", o.KCPKubeconfig,
		"Path to KCP kubeconfig file")
	fs.StringVar(&o.Workspace, "workspace", o.Workspace,
		"KCP workspace to watch (e.g., root:production)")
	fs.StringToStringVar(&o.ClusterKubeconfigs, "cluster-kubeconfigs", o.ClusterKubeconfigs,
		"Map of cluster names to kubeconfig paths")
	fs.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod,
		"Controller resync period")
	fs.IntVar(&o.WorkerCount, "worker-count", o.WorkerCount,
		"Number of worker threads")
	fs.DurationVar(&o.ShutdownTimeout, "shutdown-timeout", o.ShutdownTimeout,
		"Graceful shutdown timeout")
	fs.IntVar(&o.MetricsPort, "metrics-port", o.MetricsPort,
		"Metrics port (0 to disable)")
	fs.IntVar(&o.HealthPort, "health-port", o.HealthPort,
		"Health port (0 to disable)")
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.KCPKubeconfig == "" {
		return fmt.Errorf("--kcp-kubeconfig is required")
	}
	if o.Workspace == "" {
		return fmt.Errorf("--workspace is required")
	}
	if len(o.ClusterKubeconfigs) == 0 {
		return fmt.Errorf("at least one --cluster-kubeconfigs entry is required")
	}
	if o.WorkerCount <= 0 {
		return fmt.Errorf("--worker-count must be positive")
	}
	return nil
}