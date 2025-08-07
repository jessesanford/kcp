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

// tmc-controller implements an external TMC (Transparent Multi-Cluster) controller that consumes
// TMC APIs from KCP via APIBinding and manages workload placement on physical clusters.
// 
// This controller is designed to run outside of KCP and connects to KCP workspaces via standard
// Kubernetes client libraries, demonstrating the external controller pattern for KCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/tmc-controller/options"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
)

func main() {
	opts := options.NewOptions()
	
	// Setup command line flags
	fs := flag.NewFlagSet("tmc-controller", flag.ExitOnError)
	opts.AddFlags(fs.FlagSet)
	klog.InitFlags(fs.FlagSet)
	
	if err := fs.Parse(os.Args[1:]); err != nil {
		klog.Fatalf("Error parsing flags: %v", err)
	}

	// Complete and validate options
	if err := opts.Complete(); err != nil {
		klog.Fatalf("Error completing options: %v", err)
	}
	if err := opts.Validate(); err != nil {
		klog.Fatalf("Invalid options: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), 
		syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	klog.InfoS("Starting TMC controller", 
		"workspace", opts.Workspace,
		"clusters", len(opts.ClusterKubeconfigs),
		"workers", opts.WorkerCount)

	// Build KCP client config for workspace
	kcpConfig, err := buildKCPConfig(opts.KCPKubeconfig, opts.Workspace)
	if err != nil {
		klog.Fatalf("Error building KCP config: %v", err)
	}

	// Build target cluster configs
	clusterConfigs, err := buildClusterConfigs(opts.ClusterKubeconfigs)
	if err != nil {
		klog.Fatalf("Error building cluster configs: %v", err)
	}

	// Start health and metrics servers
	if err := startHealthServer(ctx, opts.HealthPort); err != nil {
		klog.Fatalf("Error starting health server: %v", err)
	}
	
	if err := startMetricsServer(ctx, opts.MetricsPort); err != nil {
		klog.Fatalf("Error starting metrics server: %v", err)
	}

	if opts.PProfPort > 0 {
		if err := startPProfServer(ctx, opts.PProfPort); err != nil {
			klog.Fatalf("Error starting pprof server: %v", err)
		}
	}

	// Create TMC controller manager
	mgr, err := controller.NewManager(ctx, &controller.Config{
		KCPConfig:                    kcpConfig,
		ClusterConfigs:               clusterConfigs,
		Workspace:                    opts.Workspace,
		ResyncPeriod:                 opts.ResyncPeriod,
		WorkerCount:                  opts.WorkerCount,
		ClusterHealthCheckInterval:   opts.ClusterHealthCheckInterval,
		PlacementDecisionTimeout:     opts.PlacementDecisionTimeout,
		MaxConcurrentPlacements:      opts.MaxConcurrentPlacements,
		EnablePlacementController:    opts.EnablePlacementController,
		EnableClusterHealthChecking:  opts.EnableClusterHealthChecking,
		EnableWorkloadSynchronization: opts.EnableWorkloadSynchronization,
	})
	if err != nil {
		klog.Fatalf("Error creating TMC controller manager: %v", err)
	}

	klog.InfoS("TMC controller configuration complete, starting controllers")

	// Start the controller manager
	if err := mgr.Start(ctx); err != nil {
		klog.Fatalf("Error starting TMC controller: %v", err)
	}

	<-ctx.Done()
	klog.InfoS("Shutting down TMC controller")
}

// buildKCPConfig creates a REST config for connecting to KCP in the specified workspace
func buildKCPConfig(kubeconfig, workspace string) (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build KCP config: %w", err)
	}

	// Configure the config for the specific workspace
	// The logical cluster path will be added by the KCP client libraries
	config.UserAgent = "tmc-controller/v1alpha1"
	
	return config, nil
}

// buildClusterConfigs creates REST configs for all target physical clusters
func buildClusterConfigs(clusterKubeconfigs map[string]string) (map[string]*rest.Config, error) {
	configs := make(map[string]*rest.Config, len(clusterKubeconfigs))
	
	for name, kubeconfig := range clusterKubeconfigs {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build cluster config for %s: %w", name, err)
		}
		
		config.UserAgent = fmt.Sprintf("tmc-controller/v1alpha1 cluster/%s", name)
		configs[name] = config
	}
	
	return configs, nil
}

// startHealthServer starts the health check server
func startHealthServer(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	
	// Basic health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	
	// Ready check - more sophisticated check could be added
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.ErrorS(err, "Error shutting down health server")
		}
	}()

	go func() {
		klog.InfoS("Starting health server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "Health server error")
		}
	}()

	return nil
}

// startMetricsServer starts the metrics server
func startMetricsServer(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	
	// Basic metrics endpoint - Prometheus metrics would be added here
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "# TMC Controller Metrics\n")
		fmt.Fprint(w, "# TYPE tmc_controller_info gauge\n")
		fmt.Fprint(w, "tmc_controller_info{version=\"v1alpha1\"} 1\n")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.ErrorS(err, "Error shutting down metrics server")
		}
	}()

	go func() {
		klog.InfoS("Starting metrics server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "Metrics server error")
		}
	}()

	return nil
}

// startPProfServer starts the pprof server for debugging
func startPProfServer(ctx context.Context, port int) error {
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.ErrorS(err, "Error shutting down pprof server")
		}
	}()

	go func() {
		klog.InfoS("Starting pprof server", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "PProf server error")
		}
	}()

	return nil
}