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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/features"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var (
	kubeconfig = ""
	workers    = 2
)

func main() {
	// Add feature gates
	fs := pflag.NewFlagSet("", pflag.ExitOnError)
	fs.Var(features.NewFlagValue(), "feature-gates", "A set of key=value pairs that describe feature gates for alpha/experimental features.")
	fs.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "Path to kubeconfig file for KCP connection")
	fs.IntVar(&workers, "workers", workers, "Number of worker goroutines for controllers")

	cmd := &cobra.Command{
		Use:   "tmc-controller",
		Short: "TMC (Transparent Multi-Cluster) controller manages workload placement and cluster synchronization",
		Long: `The TMC controller is responsible for:
- Managing cluster registrations and health monitoring
- Making placement decisions for workloads across registered clusters  
- Synchronizing workload state between control plane and clusters
- Providing status aggregation and lifecycle management

The controller integrates with KCP's workspace system to provide
multi-tenant, workspace-aware multi-cluster management.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context())
		},
		SilenceUsage: true,
	}

	fs.AddGoFlagSet(flag.CommandLine)
	cmd.Flags().AddFlagSet(fs)

	// Set up signal handling for graceful shutdown
	ctx := setupSignalHandler()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	logger := klog.FromContext(ctx)

	// Check if TMC feature flag is enabled
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCFeature) {
		return fmt.Errorf("TMC feature flag is not enabled. Use --feature-gates=TMCFeature=true to enable")
	}

	logger.Info("Starting TMC controller", "version", "v0.1.0", "build", "dev")

	// TODO: In the 03b-controller-config PR, add:
	// - Options validation and completion
	// - Configuration parsing
	// - Command line flag handling

	// Initialize and start controllers
	if err := startControllers(ctx); err != nil {
		return fmt.Errorf("failed to start controllers: %w", err)
	}

	logger.Info("TMC controller initialized successfully")
	
	// Keep running until context is cancelled
	<-ctx.Done()
	logger.Info("TMC controller shutting down")
	
	return nil
}

// startControllers initializes and starts all TMC controllers
func startControllers(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	
	// Create configuration for KCP connection
	config, err := createKCPConfig()
	if err != nil {
		return fmt.Errorf("failed to create KCP config: %w", err)
	}
	
	// Create KCP cluster client
	kcpClusterClient, err := kcpclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create KCP cluster client: %w", err)
	}
	
	logger.Info("Created KCP cluster client successfully")
	
	// Create example cluster configs for demonstration
	// In a real deployment, these would come from configuration files or environment variables
	clusterConfigs := createExampleClusterConfigs(config)
	
	// Create the cluster registration controller
	clusterController, err := controller.NewClusterRegistrationController(
		kcpClusterClient,
		clusterConfigs,
		logicalcluster.Name("root:tmc"), // Example workspace
		30*time.Second,                  // Resync period
		workers,                         // Worker count
	)
	if err != nil {
		return fmt.Errorf("failed to create cluster registration controller: %w", err)
	}
	
	logger.Info("Created ClusterRegistration controller successfully")
	
	// Start the cluster registration controller
	go func() {
		logger.Info("Starting ClusterRegistration controller")
		if err := clusterController.Start(ctx); err != nil {
			logger.Error(err, "ClusterRegistration controller failed")
		}
	}()
	
	logger.Info("TMC controllers started successfully")
	
	return nil
}

// createKCPConfig creates a Kubernetes client configuration for KCP connection
func createKCPConfig() (*rest.Config, error) {
	var config *rest.Config
	var err error
	
	if kubeconfig != "" {
		// Use provided kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfig, err)
		}
	} else {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to default kubeconfig location
			config, err = clientcmd.BuildConfigFromFlags("", "")
			if err != nil {
				return nil, fmt.Errorf("failed to get config (tried in-cluster and default kubeconfig): %w", err)
			}
		}
	}
	
	// Set reasonable defaults for KCP connection
	config.QPS = 50
	config.Burst = 100
	
	return config, nil
}

// createExampleClusterConfigs creates example cluster configurations for demonstration
// In a real deployment, these would be loaded from configuration files or environment variables
func createExampleClusterConfigs(kcpConfig *rest.Config) map[string]*rest.Config {
	configs := make(map[string]*rest.Config)
	
	// For demonstration, we'll use the same config as a "cluster"
	// In real usage, these would be configs for different physical Kubernetes clusters
	configs["example-cluster"] = &rest.Config{
		Host:        kcpConfig.Host,
		BearerToken: kcpConfig.BearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   kcpConfig.TLSClientConfig.Insecure,
			CAFile:     kcpConfig.TLSClientConfig.CAFile,
			CertFile:   kcpConfig.TLSClientConfig.CertFile,
			KeyFile:    kcpConfig.TLSClientConfig.KeyFile,
			CAData:     kcpConfig.TLSClientConfig.CAData,
			CertData:   kcpConfig.TLSClientConfig.CertData,
			KeyData:    kcpConfig.TLSClientConfig.KeyData,
			ServerName: kcpConfig.TLSClientConfig.ServerName,
		},
		QPS:   20,
		Burst: 40,
	}
	
	return configs
}

// setupSignalHandler registers signal handlers and returns a context that is cancelled on signal
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()
	return ctx
}