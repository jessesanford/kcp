/*
Copyright 2022 The KCP Authors.

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
	"time"

	"github.com/spf13/pflag"
)

// SyncerOptions contains configuration options for the workload syncer
type SyncerOptions struct {
	// KCP connection options
	KCPKubeconfig string
	KCPContext    string

	// Physical cluster connection options
	ClusterKubeconfig string
	ClusterContext    string

	// Sync target configuration
	SyncTargetName      string
	SyncTargetUID       string
	SyncTargetWorkspace string

	// Operational options
	QPS              float32
	Burst            int
	ResyncPeriod     time.Duration
	Workers          int
	HealthPort       int
	MetricsPort      int

	// TMC integration options
	EnableTMCMetrics bool
	EnableTMCHealth  bool
	EnableTMCTracing bool

	// Feature flags
	EnableProfiling bool
}

// NewSyncerOptions creates a new SyncerOptions with default values
func NewSyncerOptions() *SyncerOptions {
	return &SyncerOptions{
		// Default connection options
		KCPKubeconfig:     "",
		KCPContext:        "",
		ClusterKubeconfig: "",
		ClusterContext:    "",

		// Default sync target options
		SyncTargetName:      "",
		SyncTargetUID:       "",
		SyncTargetWorkspace: "",

		// Default operational options
		QPS:          50.0,
		Burst:        100,
		ResyncPeriod: 10 * time.Hour,
		Workers:      4,
		HealthPort:   8080,
		MetricsPort:  8081,

		// Default TMC integration
		EnableTMCMetrics: true,
		EnableTMCHealth:  true,
		EnableTMCTracing: true,

		// Default feature flags
		EnableProfiling: false,
	}
}

// AddFlags adds command line flags for the syncer options
func (o *SyncerOptions) AddFlags(fs *pflag.FlagSet) {
	// KCP connection flags
	fs.StringVar(&o.KCPKubeconfig, "kcp-kubeconfig", o.KCPKubeconfig,
		"Path to the KCP kubeconfig file")
	fs.StringVar(&o.KCPContext, "kcp-context", o.KCPContext,
		"Context to use in the KCP kubeconfig file")

	// Physical cluster connection flags
	fs.StringVar(&o.ClusterKubeconfig, "cluster-kubeconfig", o.ClusterKubeconfig,
		"Path to the physical cluster kubeconfig file")
	fs.StringVar(&o.ClusterContext, "cluster-context", o.ClusterContext,
		"Context to use in the physical cluster kubeconfig file")

	// Sync target flags
	fs.StringVar(&o.SyncTargetName, "sync-target-name", o.SyncTargetName,
		"Name of the SyncTarget resource")
	fs.StringVar(&o.SyncTargetUID, "sync-target-uid", o.SyncTargetUID,
		"UID of the SyncTarget resource")
	fs.StringVar(&o.SyncTargetWorkspace, "sync-target-workspace", o.SyncTargetWorkspace,
		"Logical cluster workspace containing the SyncTarget")

	// Operational flags
	fs.Float32Var(&o.QPS, "qps", o.QPS,
		"Maximum QPS for client connections")
	fs.IntVar(&o.Burst, "burst", o.Burst,
		"Maximum burst for client connections")
	fs.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod,
		"Period for full resync of resources")
	fs.IntVar(&o.Workers, "workers", o.Workers,
		"Number of worker goroutines")
	fs.IntVar(&o.HealthPort, "health-port", o.HealthPort,
		"Port for health check endpoint")
	fs.IntVar(&o.MetricsPort, "metrics-port", o.MetricsPort,
		"Port for metrics endpoint")

	// TMC integration flags
	fs.BoolVar(&o.EnableTMCMetrics, "enable-tmc-metrics", o.EnableTMCMetrics,
		"Enable TMC metrics collection")
	fs.BoolVar(&o.EnableTMCHealth, "enable-tmc-health", o.EnableTMCHealth,
		"Enable TMC health monitoring")
	fs.BoolVar(&o.EnableTMCTracing, "enable-tmc-tracing", o.EnableTMCTracing,
		"Enable TMC distributed tracing")

	// Feature flags
	fs.BoolVar(&o.EnableProfiling, "enable-profiling", o.EnableProfiling,
		"Enable profiling endpoints")
}

// Validate validates the syncer options
func (o *SyncerOptions) Validate() error {
	if o.SyncTargetName == "" {
		return fmt.Errorf("sync-target-name is required")
	}
	if o.SyncTargetUID == "" {
		return fmt.Errorf("sync-target-uid is required")
	}
	if o.SyncTargetWorkspace == "" {
		return fmt.Errorf("sync-target-workspace is required")
	}
	if o.Workers <= 0 {
		return fmt.Errorf("workers must be positive")
	}
	if o.QPS <= 0 {
		return fmt.Errorf("qps must be positive")
	}
	if o.Burst <= 0 {
		return fmt.Errorf("burst must be positive")
	}
	if o.HealthPort <= 0 || o.HealthPort > 65535 {
		return fmt.Errorf("health-port must be between 1 and 65535")
	}
	if o.MetricsPort <= 0 || o.MetricsPort > 65535 {
		return fmt.Errorf("metrics-port must be between 1 and 65535")
	}
	return nil
}