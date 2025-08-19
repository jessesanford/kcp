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
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// TMCControllerOptions contains configuration for the TMC controller.
// It follows KCP patterns for multi-workspace controller configuration.
type TMCControllerOptions struct {
	// KubeConfig is the path to the kubeconfig file for connecting to KCP
	KubeConfig string

	// Workspaces specifies which workspaces the controller should watch.
	// Empty means watch all accessible workspaces.
	Workspaces []string

	// ResyncPeriod is the period between full reconciliation cycles
	ResyncPeriod time.Duration

	// LeaderElection enables leader election for controller high availability
	LeaderElection bool

	// LeaderElectionNamespace is the namespace for leader election resources
	LeaderElectionNamespace string

	// LeaderElectionID is the identifier for leader election
	LeaderElectionID string

	// LogLevel sets the verbosity of logging
	LogLevel int

	// Config is the computed REST config (populated during Complete())
	Config *rest.Config `json:"-"`
}

// NewTMCControllerOptions creates a new TMCControllerOptions with default values.
func NewTMCControllerOptions() *TMCControllerOptions {
	return &TMCControllerOptions{
		KubeConfig:                 "",
		Workspaces:                 []string{},
		ResyncPeriod:               10 * time.Minute,
		LeaderElection:             true,
		LeaderElectionNamespace: "kcp-system",
		LeaderElectionID:        "tmc-controller",
		LogLevel:                2,
	}
}

// AddFlags adds command line flags for all TMCControllerOptions fields.
func (o *TMCControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig,
		"Path to the kubeconfig file to use for KCP API server connections")

	fs.StringSliceVar(&o.Workspaces, "workspaces", o.Workspaces,
		"List of workspaces to watch. If empty, watches all accessible workspaces")

	fs.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod,
		"Period between full reconciliation cycles")

	fs.BoolVar(&o.LeaderElection, "leader-election", o.LeaderElection,
		"Enable leader election for controller high availability")

	fs.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", o.LeaderElectionNamespace,
		"Namespace for leader election resources")

	fs.StringVar(&o.LeaderElectionID, "leader-election-id", o.LeaderElectionID,
		"Identifier for leader election")

	fs.IntVar(&o.LogLevel, "log-level", o.LogLevel,
		"Log level verbosity (0-10)")
}

// Validate validates all option values and returns an error if any are invalid.
func (o *TMCControllerOptions) Validate() error {
	if o.ResyncPeriod <= 0 {
		return fmt.Errorf("resync-period must be positive, got %v", o.ResyncPeriod)
	}

	if o.LogLevel < 0 || o.LogLevel > 10 {
		return fmt.Errorf("log-level must be between 0 and 10, got %d", o.LogLevel)
	}

	if o.LeaderElection && o.LeaderElectionNamespace == "" {
		return fmt.Errorf("leader-election-namespace is required when leader election is enabled")
	}

	if o.LeaderElection && o.LeaderElectionID == "" {
		return fmt.Errorf("leader-election-id is required when leader election is enabled")
	}

	// Validate workspace names if provided
	for _, workspace := range o.Workspaces {
		// Basic validation - ensure workspace path is not empty
		if workspace == "" {
			return fmt.Errorf("workspace path cannot be empty")
		}
		// Validate workspace path format (should be like "root:org:workspace")
		parts := strings.Split(workspace, ":")
		if len(parts) > 4 {
			return fmt.Errorf("invalid workspace path %q: too many segments", workspace)
		}
	}

	return nil
}

// Complete fills in any missing configuration and performs any setup required
// before the options can be used.
func (o *TMCControllerOptions) Complete() error {
	// Set up logging level
	klogFlags := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(klogFlags)
	klogFlags.Set("v", fmt.Sprintf("%d", o.LogLevel))

	var err error

	// Build REST config from kubeconfig
	if o.KubeConfig == "" {
		// Use in-cluster config if no kubeconfig specified
		o.Config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	} else {
		// Load config from kubeconfig file
		o.Config, err = clientcmd.BuildConfigFromFlags("", o.KubeConfig)
		if err != nil {
			return fmt.Errorf("failed to build config from kubeconfig %q: %w", o.KubeConfig, err)
		}
	}

	// Set QPS and Burst for controller workloads
	o.Config.QPS = 100
	o.Config.Burst = 200

	return nil
}
