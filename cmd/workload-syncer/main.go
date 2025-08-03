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

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer"
)

// Options holds the command line options
type Options struct {
	SyncTargetName   string
	SyncTargetUID    string
	WorkspaceCluster string
	KCPKubeconfig    string
	ClusterKubeconfig string
	ResyncPeriod     time.Duration
	Workers          int
	HeartbeatPeriod  time.Duration
	Verbose          int
}

func main() {
	var options Options

	// Define command line flags
	flag.StringVar(&options.SyncTargetName, "sync-target-name", "", "Name of the SyncTarget resource")
	flag.StringVar(&options.SyncTargetUID, "sync-target-uid", "", "UID of the SyncTarget resource")
	flag.StringVar(&options.WorkspaceCluster, "workspace-cluster", "", "Logical cluster where the SyncTarget resides")
	flag.StringVar(&options.KCPKubeconfig, "kcp-kubeconfig", "", "Path to KCP kubeconfig file")
	flag.StringVar(&options.ClusterKubeconfig, "cluster-kubeconfig", "", "Path to target cluster kubeconfig file")
	flag.DurationVar(&options.ResyncPeriod, "resync-period", 30*time.Second, "Resync period for informers")
	flag.IntVar(&options.Workers, "workers", 2, "Number of worker goroutines per resource controller")
	flag.DurationVar(&options.HeartbeatPeriod, "heartbeat-period", 30*time.Second, "Period for sending heartbeats to KCP")
	flag.IntVar(&options.Verbose, "v", 2, "Log verbosity level")

	klog.InitFlags(nil)
	flag.Parse()

	// Set log verbosity
	klog.V(klog.Level(options.Verbose)).Infof("Starting workload syncer with verbosity level %d", options.Verbose)

	// Validate required options
	if err := validateOptions(options); err != nil {
		klog.Errorf("Invalid options: %v", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		klog.Infof("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Create and start syncer
	if err := runSyncer(ctx, options); err != nil {
		klog.Errorf("Syncer failed: %v", err)
		os.Exit(1)
	}

	klog.Info("Workload syncer shutdown complete")
}

// validateOptions validates the command line options
func validateOptions(options Options) error {
	if options.SyncTargetName == "" {
		return fmt.Errorf("--sync-target-name is required")
	}
	if options.SyncTargetUID == "" {
		return fmt.Errorf("--sync-target-uid is required")
	}
	if options.WorkspaceCluster == "" {
		return fmt.Errorf("--workspace-cluster is required")
	}
	if options.KCPKubeconfig == "" {
		return fmt.Errorf("--kcp-kubeconfig is required")
	}
	if options.ClusterKubeconfig == "" {
		return fmt.Errorf("--cluster-kubeconfig is required")
	}
	return nil
}

// runSyncer creates and runs the syncer
func runSyncer(ctx context.Context, options Options) error {
	logger := klog.FromContext(ctx).WithValues("component", "main")
	logger.Info("Starting workload syncer",
		"syncTarget", options.SyncTargetName,
		"workspace", options.WorkspaceCluster,
	)

	// Parse workspace cluster
	workspaceCluster := logicalcluster.Name(options.WorkspaceCluster)

	// Load KCP kubeconfig
	kcpConfig, err := clientcmd.BuildConfigFromFlags("", options.KCPKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load KCP kubeconfig from %q: %w", options.KCPKubeconfig, err)
	}

	// Load cluster kubeconfig
	clusterConfig, err := clientcmd.BuildConfigFromFlags("", options.ClusterKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load cluster kubeconfig from %q: %w", options.ClusterKubeconfig, err)
	}

	// Configure client settings
	configureClientConfig(kcpConfig, "kcp")
	configureClientConfig(clusterConfig, "cluster")

	// Create syncer
	syncerOptions := syncer.SyncerOptions{
		SyncTargetName:   options.SyncTargetName,
		SyncTargetUID:    options.SyncTargetUID,
		WorkspaceCluster: workspaceCluster,
		KCPConfig:        kcpConfig,
		ClusterConfig:    clusterConfig,
		ResyncPeriod:     options.ResyncPeriod,
		Workers:          options.Workers,
		HeartbeatPeriod:  options.HeartbeatPeriod,
	}

	syncerInstance, err := syncer.NewSyncer(syncerOptions)
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	// Start syncer
	if err := syncerInstance.Start(ctx); err != nil {
		return fmt.Errorf("failed to start syncer: %w", err)
	}

	logger.Info("Syncer started successfully")

	// Wait for context cancellation
	<-ctx.Done()

	logger.Info("Stopping syncer...")

	// Stop syncer
	syncerInstance.Stop()

	logger.Info("Syncer stopped successfully")
	return nil
}

// configureClientConfig configures client settings for optimal performance
func configureClientConfig(config *rest.Config, name string) {
	// Set rate limiting
	config.QPS = 50
	config.Burst = 100

	// Set timeouts
	config.Timeout = 30 * time.Second

	// Set user agent
	config.UserAgent = fmt.Sprintf("workload-syncer/%s", name)

	klog.V(3).Infof("Configured %s client: QPS=%.1f, Burst=%d, Timeout=%v",
		name, config.QPS, config.Burst, config.Timeout)
}