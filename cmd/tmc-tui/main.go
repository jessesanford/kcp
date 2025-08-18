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
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
	"github.com/kcp-dev/kcp/cmd/tmc-tui/ui"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := newRootCommand().ExecuteContext(ctx); err != nil {
		klog.ErrorS(err, "tmc-tui failed")
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var (
		configPath     string
		refreshRate    time.Duration
		verbosity      int
	)

	cmd := &cobra.Command{
		Use:     "tmc-tui",
		Short:   "Terminal UI dashboard for TMC (Transparent Multi-Cluster)",
		Long: `A terminal-based dashboard for monitoring TMC components including
clusters, syncers, and metrics. Provides real-time status updates
and performance monitoring in an interactive interface.`,
		Version: fmt.Sprintf("%s (%s)", version, commit),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Configure logging
			klog.InitFlags(nil)
			if err := cmd.Flags().Set("v", fmt.Sprintf("%d", verbosity)); err != nil {
				return fmt.Errorf("failed to set verbosity: %w", err)
			}

			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Override refresh rate if specified
			if cmd.Flags().Changed("refresh-rate") {
				cfg.RefreshRate = refreshRate
			}

			klog.V(1).Infof("Starting TMC TUI (version: %s, refresh: %v)", version, cfg.RefreshRate)

			// Create and start the TUI application
			app, err := ui.NewApp(cfg)
			if err != nil {
				return fmt.Errorf("failed to create TUI app: %w", err)
			}

			return app.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file (optional)")
	cmd.Flags().DurationVarP(&refreshRate, "refresh-rate", "r", 5*time.Second, "Data refresh rate")
	cmd.Flags().IntVarP(&verbosity, "verbose", "v", 1, "Log verbosity level (0-4)")

	return cmd
}