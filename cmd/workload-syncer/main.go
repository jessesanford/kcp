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
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/workload-syncer/options"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	// Normalize flag names with dash instead of underscore
	pflag.CommandLine.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(name)
	})
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	command := newSyncerCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newSyncerCommand() *cobra.Command {
	opts := options.NewSyncerOptions()

	cmd := &cobra.Command{
		Use:   "workload-syncer",
		Short: "Synchronize workload resources between KCP and physical clusters",
		Long: `The workload syncer is responsible for bidirectional synchronization of workload 
resources between KCP logical clusters and physical Kubernetes clusters. It integrates with 
the TMC (Transparent Multi-Cluster) infrastructure for robust error handling, metrics, 
and health monitoring.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
			go func() {
				<-sigChan
				klog.Info("Received shutdown signal, shutting down gracefully...")
				cancel()
			}()

			return runSyncer(ctx, opts)
		},
	}

	opts.AddFlags(cmd.Flags())
	
	return cmd
}

func runSyncer(ctx context.Context, opts *options.SyncerOptions) error {
	klog.Infof("Starting KCP Workload Syncer with options: %+v", opts)

	// Build KCP client config
	kcpConfig, err := buildConfig(opts.KCPKubeconfig, opts.KCPContext)
	if err != nil {
		return fmt.Errorf("failed to build KCP client config: %w", err)
	}

	// Build physical cluster config
	clusterConfig, err := buildConfig(opts.ClusterKubeconfig, opts.ClusterContext)
	if err != nil {
		return fmt.Errorf("failed to build cluster client config: %w", err)
	}

	// Validate options
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Create syncer
	syncerOpts := syncer.SyncerOptions{
		KCPConfig:     kcpConfig,
		ClusterConfig: clusterConfig,
		SyncerOpts:    opts,
	}
	
	syncerInstance, err := syncer.NewSyncer(ctx, syncerOpts)
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	// Start the syncer
	klog.Info("Starting syncer...")
	if err := syncerInstance.Start(ctx); err != nil {
		return fmt.Errorf("failed to start syncer: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
	
	klog.Info("Shutting down syncer...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	return syncerInstance.Stop(shutdownCtx)
}

func buildConfig(kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		// Use in-cluster config
		return rest.InClusterConfig()
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig

	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	return clientConfig.ClientConfig()
}