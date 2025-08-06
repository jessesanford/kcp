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

// tmc-controller is an external controller that manages TMC resources by consuming
// KCP APIs through APIBinding. It provides the foundation for TMC functionality
// including cluster registration, workload placement, and multi-cluster management.
//
// This controller operates outside of KCP and connects to KCP workspaces through
// APIBinding to manage physical Kubernetes clusters and workload placements.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/tmc-controller/options"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
)

func main() {
	// TMC controller startup (feature gates can be added later)

	// Create options and parse flags
	opts := options.NewOptions()
	
	// Add flags to pflag.CommandLine first
	fs := pflag.NewFlagSet("tmc-controller", pflag.ExitOnError)
	opts.AddFlags(fs)
	
	// Add klog flags
	klog.InitFlags(flag.CommandLine)
	fs.AddGoFlagSet(flag.CommandLine)
	
	// Parse flags
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		klog.Fatalf("Invalid options: %v", err)
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), 
		syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()

	klog.InfoS("Starting TMC controller", 
		"version", "v1alpha1",
		"workspace", opts.Workspace)

	// Build KCP client config for workspace connection
	kcpConfig, err := buildKCPConfig(opts)
	if err != nil {
		klog.Fatalf("Error building KCP config: %v", err)
	}

	// Build target cluster configs for physical cluster management
	clusterConfigs, err := buildClusterConfigs(opts)
	if err != nil {
		klog.Fatalf("Error building cluster configs: %v", err)
	}

	// Create TMC controller manager
	mgr, err := controller.NewManager(ctx, &controller.Config{
		KCPConfig:      kcpConfig,
		ClusterConfigs: clusterConfigs,
		Workspace:      opts.Workspace,
		ResyncPeriod:   opts.ResyncPeriod,
		WorkerCount:    opts.WorkerCount,
		MetricsPort:    opts.MetricsPort,
		HealthPort:     opts.HealthPort,
	})
	if err != nil {
		klog.Fatalf("Error creating TMC controller manager: %v", err)
	}

	klog.InfoS("TMC controller manager created successfully", 
		"clusters", len(clusterConfigs),
		"workers", opts.WorkerCount)

	// Start the controller manager
	if err := mgr.Start(ctx); err != nil {
		klog.Fatalf("Error starting TMC controller: %v", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	klog.InfoS("Received shutdown signal, stopping TMC controller")

	// Give controllers time to finish current work
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
	defer shutdownCancel()

	if err := mgr.Shutdown(shutdownCtx); err != nil {
		klog.ErrorS(err, "Error during controller shutdown")
		os.Exit(1)
	}

	klog.InfoS("TMC controller shutdown complete")
}

// buildKCPConfig creates a REST config for connecting to KCP
func buildKCPConfig(opts *options.Options) (*rest.Config, error) {
	if opts.KCPKubeconfig == "" {
		return nil, fmt.Errorf("KCP kubeconfig is required")
	}

	config, err := clientcmd.BuildConfigFromFlags("", opts.KCPKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build KCP config from %s: %w", opts.KCPKubeconfig, err)
	}

	// Configure client for external controller usage
	config.QPS = 50.0
	config.Burst = 100
	config.UserAgent = "tmc-controller/v1alpha1"

	return config, nil
}

// buildClusterConfigs creates REST configs for all target clusters
func buildClusterConfigs(opts *options.Options) (map[string]*rest.Config, error) {
	if len(opts.ClusterKubeconfigs) == 0 {
		return nil, fmt.Errorf("at least one cluster kubeconfig is required")
	}

	clusterConfigs := make(map[string]*rest.Config)

	for clusterName, kubeconfigPath := range opts.ClusterKubeconfigs {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build cluster config for %s from %s: %w", 
				clusterName, kubeconfigPath, err)
		}

		// Configure client for cluster management
		config.QPS = 30.0
		config.Burst = 60
		config.UserAgent = fmt.Sprintf("tmc-controller/v1alpha1 cluster=%s", clusterName)

		clusterConfigs[clusterName] = config

		klog.V(2).InfoS("Configured cluster client", 
			"cluster", clusterName, 
			"kubeconfig", kubeconfigPath)
	}

	return clusterConfigs, nil
}
