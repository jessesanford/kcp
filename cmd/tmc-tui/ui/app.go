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

package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/data"
	"github.com/kcp-dev/kcp/cmd/tmc-tui/ui/components"
)

// App represents the main TUI application.
type App struct {
	// Configuration
	config *config.Config

	// tview application
	app *tview.Application

	// Data collector
	collector *data.Collector

	// UI components
	header *components.Header
	footer *components.Footer
	pages  *tview.Pages

	// Views
	dashboard *DashboardView
	clusters  *ClustersView
	syncers   *SyncersView
	metrics   *MetricsView

	// Current view state
	currentView string

	// Refresh ticker
	ticker *time.Ticker
	done   chan struct{}
}

const (
	ViewDashboard = "dashboard"
	ViewClusters  = "clusters"
	ViewSyncers   = "syncers"
	ViewMetrics   = "metrics"
)

// NewApp creates a new TUI application with the provided configuration.
func NewApp(cfg *config.Config) (*App, error) {
	cfg.Validate()

	// Create data collector
	collector, err := data.NewCollector(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create data collector: %w", err)
	}

	app := &App{
		config:      cfg,
		app:         tview.NewApplication(),
		collector:   collector,
		currentView: ViewDashboard,
		done:        make(chan struct{}),
	}

	if err := app.initializeUI(); err != nil {
		return nil, fmt.Errorf("failed to initialize UI: %w", err)
	}

	return app, nil
}

// initializeUI sets up the UI components and layout.
func (a *App) initializeUI() error {
	// Create UI components
	a.header = components.NewHeader(a.config)
	a.footer = components.NewFooter(a.config)
	a.pages = tview.NewPages()

	// Create views
	var err error
	a.dashboard, err = NewDashboardView(a.config, a.collector)
	if err != nil {
		return fmt.Errorf("failed to create dashboard view: %w", err)
	}

	if a.config.Features.ShowClusterView {
		a.clusters, err = NewClustersView(a.config, a.collector)
		if err != nil {
			return fmt.Errorf("failed to create clusters view: %w", err)
		}
	}

	if a.config.Features.ShowSyncerView {
		a.syncers, err = NewSyncersView(a.config, a.collector)
		if err != nil {
			return fmt.Errorf("failed to create syncers view: %w", err)
		}
	}

	if a.config.Features.ShowMetricsView {
		a.metrics, err = NewMetricsView(a.config, a.collector)
		if err != nil {
			return fmt.Errorf("failed to create metrics view: %w", err)
		}
	}

	// Add views to pages
	a.pages.AddPage(ViewDashboard, a.dashboard.GetView(), true, true)
	if a.clusters != nil {
		a.pages.AddPage(ViewClusters, a.clusters.GetView(), true, false)
	}
	if a.syncers != nil {
		a.pages.AddPage(ViewSyncers, a.syncers.GetView(), true, false)
	}
	if a.metrics != nil {
		a.pages.AddPage(ViewMetrics, a.metrics.GetView(), true, false)
	}

	// Create main layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header.GetView(), 3, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.footer.GetView(), 2, 0, false)

	a.app.SetRoot(flex, true)

	// Set up global key handlers
	a.setupKeyHandlers()

	// Update header with current view
	a.updateHeader()

	return nil
}

// setupKeyHandlers configures global keyboard shortcuts.
func (a *App) setupKeyHandlers() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			a.app.Stop()
			return nil
		case tcell.KeyTab:
			a.nextView()
			return nil
		case tcell.KeyBacktab:
			a.prevView()
			return nil
		case tcell.KeyF1:
			if a.config.Features.EnableKeyboardShortcuts {
				a.showHelp()
			}
			return nil
		}

		switch event.Rune() {
		case 'q', 'Q':
			a.app.Stop()
			return nil
		case '1':
			a.switchToView(ViewDashboard)
			return nil
		case '2':
			if a.clusters != nil {
				a.switchToView(ViewClusters)
			}
			return nil
		case '3':
			if a.syncers != nil {
				a.switchToView(ViewSyncers)
			}
			return nil
		case '4':
			if a.metrics != nil {
				a.switchToView(ViewMetrics)
			}
			return nil
		case 'r', 'R':
			a.refreshData()
			return nil
		case 'h', 'H', '?':
			if a.config.Features.EnableKeyboardShortcuts {
				a.showHelp()
			}
			return nil
		}

		return event
	})
}

// switchToView switches to the specified view if it exists.
func (a *App) switchToView(viewName string) {
	if !a.isViewAvailable(viewName) {
		return
	}

	a.currentView = viewName
	a.pages.SwitchToPage(viewName)
	a.updateHeader()
	a.refreshCurrentView()

	klog.V(3).Infof("Switched to view: %s", viewName)
}

// nextView switches to the next available view.
func (a *App) nextView() {
	views := a.getAvailableViews()
	if len(views) <= 1 {
		return
	}

	currentIndex := -1
	for i, view := range views {
		if view == a.currentView {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(views)
	a.switchToView(views[nextIndex])
}

// prevView switches to the previous available view.
func (a *App) prevView() {
	views := a.getAvailableViews()
	if len(views) <= 1 {
		return
	}

	currentIndex := -1
	for i, view := range views {
		if view == a.currentView {
			currentIndex = i
			break
		}
	}

	prevIndex := (currentIndex - 1 + len(views)) % len(views)
	a.switchToView(views[prevIndex])
}

// getAvailableViews returns a list of available views based on configuration.
func (a *App) getAvailableViews() []string {
	views := []string{ViewDashboard}

	if a.config.Features.ShowClusterView && a.clusters != nil {
		views = append(views, ViewClusters)
	}
	if a.config.Features.ShowSyncerView && a.syncers != nil {
		views = append(views, ViewSyncers)
	}
	if a.config.Features.ShowMetricsView && a.metrics != nil {
		views = append(views, ViewMetrics)
	}

	return views
}

// isViewAvailable checks if the specified view is available.
func (a *App) isViewAvailable(viewName string) bool {
	for _, view := range a.getAvailableViews() {
		if view == viewName {
			return true
		}
	}
	return false
}

// updateHeader updates the header with current view information.
func (a *App) updateHeader() {
	a.header.SetCurrentView(a.currentView)
	a.app.QueueUpdateDraw(func() {})
}

// refreshData manually triggers a data refresh.
func (a *App) refreshData() {
	klog.V(2).Info("Manual data refresh triggered")
	a.collector.RefreshAll()
	a.refreshCurrentView()
}

// refreshCurrentView refreshes the currently active view.
func (a *App) refreshCurrentView() {
	switch a.currentView {
	case ViewDashboard:
		if a.dashboard != nil {
			a.dashboard.Refresh()
		}
	case ViewClusters:
		if a.clusters != nil {
			a.clusters.Refresh()
		}
	case ViewSyncers:
		if a.syncers != nil {
			a.syncers.Refresh()
		}
	case ViewMetrics:
		if a.metrics != nil {
			a.metrics.Refresh()
		}
	}

	a.app.QueueUpdateDraw(func() {})
}

// showHelp displays the help screen with keyboard shortcuts.
func (a *App) showHelp() {
	// This would show a modal with help information
	// For now, we'll log the shortcuts
	klog.Info("Help: 1-4=views, Tab=next, r=refresh, q=quit, ?=help")
}

// startAutoRefresh starts the automatic data refresh ticker.
func (a *App) startAutoRefresh() {
	if !a.config.Features.AutoRefresh {
		return
	}

	a.ticker = time.NewTicker(a.config.RefreshRate)
	go func() {
		for {
			select {
			case <-a.ticker.C:
				a.collector.RefreshAll()
				a.refreshCurrentView()
			case <-a.done:
				return
			}
		}
	}()

	klog.V(2).Infof("Auto-refresh started (interval: %v)", a.config.RefreshRate)
}

// stopAutoRefresh stops the automatic data refresh ticker.
func (a *App) stopAutoRefresh() {
	if a.ticker != nil {
		a.ticker.Stop()
		close(a.done)
		klog.V(2).Info("Auto-refresh stopped")
	}
}

// Run starts the TUI application and blocks until it exits.
func (a *App) Run(ctx context.Context) error {
	// Start data collection
	if err := a.collector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start data collector: %w", err)
	}
	defer a.collector.Stop()

	// Start auto-refresh
	a.startAutoRefresh()
	defer a.stopAutoRefresh()

	// Initial data load
	a.collector.RefreshAll()
	a.refreshCurrentView()

	klog.V(1).Info("Starting TUI application")

	// Run the application
	return a.app.Run()
}