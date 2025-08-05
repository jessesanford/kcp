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
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/client-go/util/homedir"
)

// Options contains configuration for the TMC controller
type Options struct {
	// Kubeconfig is the path to the kubeconfig file
	Kubeconfig string

	// MasterURL is the address of the Kubernetes API server
	MasterURL string

	// Namespace restricts the controller to watch resources in a specific namespace.
	// If empty, the controller watches all namespaces.
	Namespace string

	// SyncPeriod is the period for syncing controller state
	SyncPeriod time.Duration

	// WorkerCount is the number of worker goroutines per controller
	WorkerCount int

	// LeaderElection enables leader election for high availability
	LeaderElection bool

	// LeaderElectionNamespace is the namespace for leader election resources
	LeaderElectionNamespace string

	// LeaderElectionID is the name of the leader election resource
	LeaderElectionID string

	// MetricsBindAddress is the address for binding metrics server
	MetricsBindAddress string

	// HealthProbeBindAddress is the address for binding health probe server
	HealthProbeBindAddress string
}

// NewOptions creates new Options with default values
func NewOptions() (*Options, error) {
	// Default kubeconfig location
	var defaultKubeconfig string
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}

	return &Options{
		Kubeconfig:              defaultKubeconfig,
		MasterURL:               "",
		Namespace:               "",
		SyncPeriod:              30 * time.Second,
		WorkerCount:             2,
		LeaderElection:          false,
		LeaderElectionNamespace: "kcp-system",
		LeaderElectionID:        "tmc-controller-leader",
		MetricsBindAddress:      ":8080",
		HealthProbeBindAddress:  ":8081",
	}, nil
}

// AddFlags adds command line flags for the options
func (o *Options) AddFlags(flags *pflag.FlagSet) error {
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig,
		"Path to kubeconfig file. If empty, will use in-cluster configuration.")

	flags.StringVar(&o.MasterURL, "master", o.MasterURL,
		"The address of the Kubernetes API server. Overrides any value in kubeconfig.")

	flags.StringVar(&o.Namespace, "namespace", o.Namespace,
		"Namespace to watch for resources. If empty, watches all namespaces.")

	flags.DurationVar(&o.SyncPeriod, "sync-period", o.SyncPeriod,
		"The period for syncing controller state.")

	flags.IntVar(&o.WorkerCount, "worker-count", o.WorkerCount,
		"The number of worker goroutines per controller.")

	flags.BoolVar(&o.LeaderElection, "enable-leader-election", o.LeaderElection,
		"Enable leader election for high availability. Only one controller instance will be active.")

	flags.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", o.LeaderElectionNamespace,
		"The namespace where leader election resources are stored.")

	flags.StringVar(&o.LeaderElectionID, "leader-election-id", o.LeaderElectionID,
		"The name of the leader election resource.")

	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", o.MetricsBindAddress,
		"The address the metrics endpoint binds to.")

	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", o.HealthProbeBindAddress,
		"The address the health probe endpoint binds to.")

	return nil
}

// Complete fills in missing values with defaults
func (o *Options) Complete() error {
	// If kubeconfig is not set and default doesn't exist, try in-cluster config
	if o.Kubeconfig != "" {
		if _, err := os.Stat(o.Kubeconfig); os.IsNotExist(err) {
			// If specified kubeconfig doesn't exist, clear it to use in-cluster config
			o.Kubeconfig = ""
		}
	}

	return nil
}

// Validate checks that the options are valid
func (o *Options) Validate() error {
	if o.SyncPeriod <= 0 {
		return fmt.Errorf("sync-period must be positive, got %v", o.SyncPeriod)
	}

	if o.WorkerCount <= 0 {
		return fmt.Errorf("worker-count must be positive, got %d", o.WorkerCount)
	}

	if o.WorkerCount > 50 {
		return fmt.Errorf("worker-count must be <= 50 for performance reasons, got %d", o.WorkerCount)
	}

	if o.LeaderElection {
		if o.LeaderElectionNamespace == "" {
			return fmt.Errorf("leader-election-namespace must be specified when leader election is enabled")
		}

		if o.LeaderElectionID == "" {
			return fmt.Errorf("leader-election-id must be specified when leader election is enabled")
		}
	}

	return nil
}

// String returns a string representation of the options
func (o *Options) String() string {
	return fmt.Sprintf("Options{Namespace: %q, SyncPeriod: %v, WorkerCount: %d, LeaderElection: %t}",
		o.Namespace, o.SyncPeriod, o.WorkerCount, o.LeaderElection)
}