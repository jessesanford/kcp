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
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/data"
)

// DashboardView represents the main dashboard view showing system overview.
type DashboardView struct {
	config    *config.Config
	collector *data.Collector

	// Layout components
	layout     *tview.Flex
	systemInfo *tview.TextView
	metrics    *tview.TextView
	clusters   *tview.TextView
	syncers    *tview.TextView
	events     *tview.TextView
}

// NewDashboardView creates a new dashboard view.
func NewDashboardView(cfg *config.Config, collector *data.Collector) (*DashboardView, error) {
	dv := &DashboardView{
		config:    cfg,
		collector: collector,
	}

	if err := dv.initializeDashboard(); err != nil {
		return nil, err
	}

	return dv, nil
}

// initializeDashboard sets up the dashboard layout and components.
func (dv *DashboardView) initializeDashboard() error {
	// Create main layout - split into sections
	dv.layout = tview.NewFlex().SetDirection(tview.FlexColumn)

	// Left column
	leftColumn := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// System information panel
	dv.systemInfo = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" System Status ").
		SetTitleAlign(tview.AlignLeft)

	// Metrics overview panel
	dv.metrics = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Key Metrics ").
		SetTitleAlign(tview.AlignLeft)

	// Add borders if configured
	if dv.config.Theme.ShowBorders {
		dv.systemInfo.SetBorder(true)
		dv.metrics.SetBorder(true)
	}

	// Apply theme colors
	dv.applyTheme()

	// Add to left column
	leftColumn.AddItem(dv.systemInfo, 0, 1, false).
		AddItem(dv.metrics, 0, 1, false)

	// Right column
	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow)

	// Cluster summary panel
	dv.clusters = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Cluster Summary ").
		SetTitleAlign(tview.AlignLeft)

	// Syncer summary panel  
	dv.syncers = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Syncer Summary ").
		SetTitleAlign(tview.AlignLeft)

	// Events panel
	dv.events = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Recent Events ").
		SetTitleAlign(tview.AlignLeft)

	if dv.config.Theme.ShowBorders {
		dv.clusters.SetBorder(true)
		dv.syncers.SetBorder(true)
		dv.events.SetBorder(true)
	}

	// Add to right column
	rightColumn.AddItem(dv.clusters, 0, 1, false).
		AddItem(dv.syncers, 0, 1, false).
		AddItem(dv.events, 0, 1, false)

	// Add columns to main layout
	dv.layout.AddItem(leftColumn, 0, 1, false).
		AddItem(rightColumn, 0, 1, false)

	// Initial refresh
	dv.Refresh()

	return nil
}

// applyTheme applies the configured theme to dashboard components.
func (dv *DashboardView) applyTheme() {
	var bg, fg tcell.Color

	switch dv.config.Theme.ColorScheme {
	case "light":
		bg, fg = tcell.ColorWhite, tcell.ColorBlack
	case "dark":
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	default:
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	}

	components := []*tview.TextView{dv.systemInfo, dv.metrics, dv.clusters, dv.syncers, dv.events}
	for _, component := range components {
		if component != nil {
			component.SetBackgroundColor(bg).SetTextColor(fg)
		}
	}
}

// Refresh updates all dashboard panels with current data.
func (dv *DashboardView) Refresh() {
	dashboardData := dv.collector.GetDashboardData()

	dv.updateSystemInfo(dashboardData)
	dv.updateMetrics(dashboardData.Metrics)
	dv.updateClusterSummary(dashboardData.Clusters)
	dv.updateSyncerSummary(dashboardData.Syncers)
	dv.updateEvents(dashboardData.RecentEvents)
}

// updateSystemInfo updates the system information panel.
func (dv *DashboardView) updateSystemInfo(data data.DashboardData) {
	uptimeStr := dv.formatDuration(data.Uptime)
	lastRefreshStr := data.LastRefresh.Format("15:04:05")

	statusColor := "green"
	if data.SystemStatus != "Healthy" {
		statusColor = "red"
	}

	content := fmt.Sprintf(`[blue::b]TMC System Status[-::-]

[yellow]Status:[-] [%s]%s[-]
[yellow]Uptime:[-] %s
[yellow]Last Refresh:[-] %s

[blue::b]Component Health[-::-]
[yellow]Total Clusters:[-] %d
[yellow]Healthy Clusters:[-] %d
[yellow]Total Syncers:[-] %d
[yellow]Active Syncers:[-] %d`,
		statusColor, data.SystemStatus,
		uptimeStr,
		lastRefreshStr,
		data.Metrics.TotalClusters,
		data.Metrics.HealthyClusters,
		data.Metrics.TotalSyncers,
		data.Metrics.ActiveSyncers,
	)

	dv.systemInfo.SetText(content)
}

// updateMetrics updates the key metrics panel.
func (dv *DashboardView) updateMetrics(metrics data.MetricsData) {
	avgLatencyMs := metrics.AverageLatency.Milliseconds()
	
	latencyColor := "green"
	if avgLatencyMs > 500 {
		latencyColor = "red"
	} else if avgLatencyMs > 200 {
		latencyColor = "yellow"
	}

	errorRateColor := "green"
	if metrics.ErrorRate > 10.0 {
		errorRateColor = "red"
	} else if metrics.ErrorRate > 5.0 {
		errorRateColor = "yellow"
	}

	content := fmt.Sprintf(`[blue::b]Performance Metrics[-::-]

[yellow]Average Latency:[-] [%s]%dms[-]
[yellow]Error Rate:[-] [%s]%.1f/min[-]
[yellow]Resources Synced:[-] %d

[blue::b]Capacity[-::-]
[yellow]Cluster Utilization:[-] %.1f%%
[yellow]Syncer Utilization:[-] %.1f%%

[blue::b]Throughput[-::-]
[yellow]Ops/sec:[-] %.1f
[yellow]Data Transfer:[-] %s/s`,
		latencyColor, avgLatencyMs,
		errorRateColor, metrics.ErrorRate,
		metrics.ResourcesSynced,
		float64(metrics.HealthyClusters)/float64(metrics.TotalClusters)*100,
		float64(metrics.ActiveSyncers)/float64(metrics.TotalSyncers)*100,
		float64(metrics.ResourcesSynced)/60.0, // Assuming per minute
		"2.3MB", // Mock data transfer rate
	)

	dv.metrics.SetText(content)
}

// updateClusterSummary updates the cluster summary panel.
func (dv *DashboardView) updateClusterSummary(clusters []data.ClusterData) {
	content := "[blue::b]Cluster Overview[-::-]\n\n"

	if len(clusters) == 0 {
		content += "[red]No clusters found[-]"
	} else {
		for _, cluster := range clusters {
			healthColor := "green"
			if cluster.HealthStatus != "Healthy" {
				healthColor = "red"
			}

			ageStr := dv.formatAge(cluster.LastSeen)
			content += fmt.Sprintf("[yellow]%s[-]\n  Status: [%s]%s[-] | Nodes: %d | Pods: %d\n  Location: %s | Age: %s\n\n",
				cluster.Name,
				healthColor, cluster.HealthStatus,
				cluster.NodeCount,
				cluster.PodCount,
				cluster.Location,
				ageStr,
			)
		}
	}

	dv.clusters.SetText(content)
}

// updateSyncerSummary updates the syncer summary panel.
func (dv *DashboardView) updateSyncerSummary(syncers []data.SyncerData) {
	content := "[blue::b]Syncer Overview[-::-]\n\n"

	if len(syncers) == 0 {
		content += "[red]No syncers found[-]"
	} else {
		for _, syncer := range syncers {
			statusColor := "green"
			if syncer.Status != "Active" {
				statusColor = "red"
			}

			queueColor := "green"
			if syncer.QueueDepth > 20 {
				queueColor = "red"
			} else if syncer.QueueDepth > 10 {
				queueColor = "yellow"
			}

			ageStr := dv.formatAge(syncer.LastSync)
			content += fmt.Sprintf("[yellow]%s[-]\n  Status: [%s]%s[-] | Queue: [%s]%d[-] | Errors: %.1f/min\n  Target: %s | Last Sync: %s\n\n",
				syncer.Name,
				statusColor, syncer.Status,
				queueColor, syncer.QueueDepth,
				syncer.ErrorRate,
				syncer.TargetCluster,
				ageStr,
			)
		}
	}

	dv.syncers.SetText(content)
}

// updateEvents updates the recent events panel.
func (dv *DashboardView) updateEvents(events []data.EventData) {
	content := "[blue::b]Recent Activity[-::-]\n\n"

	if len(events) == 0 {
		content += "[yellow]No recent events[-]"
	} else {
		for _, event := range events {
			severityColor := "green"
			switch event.Severity {
			case "High":
				severityColor = "red"
			case "Medium":
				severityColor = "yellow"
			case "Low":
				severityColor = "green"
			}

			typeColor := "white"
			switch event.Type {
			case "Error":
				typeColor = "red"
			case "Warning":
				typeColor = "yellow"
			case "Info":
				typeColor = "blue"
			}

			ageStr := dv.formatAge(event.Timestamp)
			content += fmt.Sprintf("[%s]%s[-] [%s]%s[-] %s\n  [gray]%s ago - %s[-]\n\n",
				typeColor, event.Type,
				severityColor, event.Component,
				event.Message,
				ageStr,
				event.Timestamp.Format("15:04:05"),
			)
		}
	}

	dv.events.SetText(content)
}

// formatDuration formats a duration into a human-readable string.
func (dv *DashboardView) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// formatAge formats a time into a human-readable age string.
func (dv *DashboardView) formatAge(t time.Time) string {
	return dv.formatDuration(time.Since(t))
}

// GetView returns the tview primitive for this dashboard view.
func (dv *DashboardView) GetView() tview.Primitive {
	return dv.layout
}