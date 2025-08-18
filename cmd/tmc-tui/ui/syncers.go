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
	"github.com/kcp-dev/kcp/cmd/tmc-tui/ui/components"
)

// SyncersView represents the syncer status view showing detailed syncer information.
type SyncersView struct {
	config    *config.Config
	collector *data.Collector

	// Layout components
	layout *tview.Flex
	table  *components.TableWidget
	detail *tview.TextView
}

// NewSyncersView creates a new syncers view.
func NewSyncersView(cfg *config.Config, collector *data.Collector) (*SyncersView, error) {
	sv := &SyncersView{
		config:    cfg,
		collector: collector,
	}

	if err := sv.initializeSyncersView(); err != nil {
		return nil, err
	}

	return sv, nil
}

// initializeSyncersView sets up the syncers view layout and components.
func (sv *SyncersView) initializeSyncersView() error {
	// Create main layout
	sv.layout = tview.NewFlex().SetDirection(tview.FlexRow)

	// Create table for syncer list
	sv.table = components.NewTableWidget(sv.config, "Syncers")
	
	// Set column headers
	headers := []string{"Name", "Status", "Queue", "Errors/min", "Latency", "Target", "Last Sync"}
	sv.table.SetHeaders(headers)
	
	// Set column widths
	sv.table.SetColumnWidths([]int{20, 10, 8, 12, 10, 20, 15})

	// Create detail panel
	sv.detail = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Syncer Details ").
		SetTitleAlign(tview.AlignLeft)

	if sv.config.Theme.ShowBorders {
		sv.detail.SetBorder(true)
	}

	// Apply theme
	sv.applyTheme()

	// Set up selection handler
	sv.table.SetSelectionChangedFunc(func(row, column int) {
		sv.updateDetails(row)
	})

	// Add components to layout
	tableHeight := 12
	if sv.config.Theme.CompactMode {
		tableHeight = 8
	}
	
	sv.layout.AddItem(sv.table.GetView(), tableHeight, 0, true).
		AddItem(sv.detail, 0, 1, false)

	// Initial refresh
	sv.Refresh()

	return nil
}

// applyTheme applies the configured theme to syncers view components.
func (sv *SyncersView) applyTheme() {
	var bg, fg tcell.Color

	switch sv.config.Theme.ColorScheme {
	case "light":
		bg, fg = tcell.ColorWhite, tcell.ColorBlack
	case "dark":
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	default:
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	}

	sv.detail.SetBackgroundColor(bg).SetTextColor(fg)
}

// Refresh updates the syncers view with current data.
func (sv *SyncersView) Refresh() {
	syncers := sv.collector.GetSyncers()
	
	// Clear existing data (keep headers)
	sv.table.Clear()

	// Add syncer data to table
	for _, syncer := range syncers {
		lastSyncStr := sv.formatAge(syncer.LastSync)
		latencyStr := fmt.Sprintf("%dms", syncer.SyncLatency.Milliseconds())
		
		rowData := []string{
			syncer.Name,
			syncer.Status,
			fmt.Sprintf("%d", syncer.QueueDepth),
			fmt.Sprintf("%.1f", syncer.ErrorRate),
			latencyStr,
			syncer.TargetCluster,
			lastSyncStr,
		}
		
		row := sv.table.AddRow(rowData)
		
		// Apply status-based coloring
		sv.colorizeSyncerRow(row, syncer)
	}

	// Update details for selected row
	selectedRow := sv.table.GetSelectedRow()
	sv.updateDetails(selectedRow)
}

// colorizeSyncerRow applies appropriate colors based on syncer status.
func (sv *SyncersView) colorizeSyncerRow(row int, syncer data.SyncerData) {
	// Status column
	statusColor := tcell.ColorGreen
	if syncer.Status != "Active" {
		statusColor = tcell.ColorRed
	}
	sv.table.SetCellColor(row, 1, statusColor)

	// Queue depth column
	queueColor := tcell.ColorGreen
	if syncer.QueueDepth > 20 {
		queueColor = tcell.ColorRed
	} else if syncer.QueueDepth > 10 {
		queueColor = tcell.ColorYellow
	}
	sv.table.SetCellColor(row, 2, queueColor)

	// Error rate column
	errorColor := tcell.ColorGreen
	if syncer.ErrorRate > 10.0 {
		errorColor = tcell.ColorRed
	} else if syncer.ErrorRate > 5.0 {
		errorColor = tcell.ColorYellow
	}
	sv.table.SetCellColor(row, 3, errorColor)

	// Latency column
	latencyColor := tcell.ColorGreen
	if syncer.SyncLatency > 500*time.Millisecond {
		latencyColor = tcell.ColorRed
	} else if syncer.SyncLatency > 200*time.Millisecond {
		latencyColor = tcell.ColorYellow
	}
	sv.table.SetCellColor(row, 4, latencyColor)

	// Last sync column
	age := time.Since(syncer.LastSync)
	ageColor := tcell.ColorWhite
	if age > 5*time.Minute {
		ageColor = tcell.ColorRed
	} else if age > 2*time.Minute {
		ageColor = tcell.ColorYellow
	}
	sv.table.SetCellColor(row, 6, ageColor)
}

// updateDetails updates the detail panel with information for the selected syncer.
func (sv *SyncersView) updateDetails(row int) {
	syncers := sv.collector.GetSyncers()
	
	// Adjust for header row
	if row <= 0 || row > len(syncers) {
		sv.detail.SetText("[yellow]Select a syncer to view details[-]")
		return
	}
	
	syncer := syncers[row-1] // Adjust for header row
	
	statusColor := "green"
	if syncer.Status != "Active" {
		statusColor = "red"
	}
	
	connectedColor := "green"
	connectedStatus := "Connected"
	if !syncer.Connected {
		connectedColor = "red"
		connectedStatus = "Disconnected"
	}

	ageStr := sv.formatAge(syncer.LastSync)
	successRate := float64(syncer.TotalSyncs-syncer.TotalErrors) / float64(syncer.TotalSyncs) * 100
	
	queueStatus := sv.getQueueStatus(syncer.QueueDepth)
	latencyStatus := sv.getLatencyStatus(syncer.SyncLatency)
	
	content := fmt.Sprintf(`[blue::b]%s[-::-]

[blue::b]Status Information[-::-]
[yellow]Status:[-] [%s]%s[-]
[yellow]Connection:[-] [%s]%s[-]
[yellow]Target Cluster:[-] %s
[yellow]Last Sync:[-] %s (%s ago)

[blue::b]Performance Metrics[-::-]
[yellow]Queue Depth:[-] %d %s
[yellow]Sync Latency:[-] %dms %s
[yellow]Error Rate:[-] %.1f/min
[yellow]Success Rate:[-] %.1f%%

[blue::b]Statistics[-::-]
[yellow]Total Syncs:[-] %d
[yellow]Total Errors:[-] %d
[yellow]Uptime:[-] %s
[yellow]Avg Throughput:[-] %.1f ops/min

[blue::b]Health Status[-::-]
%s`,
		syncer.Name,
		statusColor, syncer.Status,
		connectedColor, connectedStatus,
		syncer.TargetCluster,
		syncer.LastSync.Format("15:04:05 Jan 2"),
		ageStr,
		syncer.QueueDepth, queueStatus,
		syncer.SyncLatency.Milliseconds(), latencyStatus,
		syncer.ErrorRate,
		successRate,
		syncer.TotalSyncs,
		syncer.TotalErrors,
		"4h 23m", // Mock uptime
		float64(syncer.TotalSyncs)/240, // Assuming 4 hours uptime
		sv.getSyncerHealthStatus(syncer),
	)

	sv.detail.SetText(content)
}

// getQueueStatus returns a status indicator for queue depth.
func (sv *SyncersView) getQueueStatus(queueDepth int) string {
	if queueDepth > 20 {
		return "[red](High)[-]"
	} else if queueDepth > 10 {
		return "[yellow](Medium)[-]"
	}
	return "[green](Normal)[-]"
}

// getLatencyStatus returns a status indicator for sync latency.
func (sv *SyncersView) getLatencyStatus(latency time.Duration) string {
	if latency > 500*time.Millisecond {
		return "[red](High)[-]"
	} else if latency > 200*time.Millisecond {
		return "[yellow](Medium)[-]"
	}
	return "[green](Good)[-]"
}

// getSyncerHealthStatus returns overall health status based on syncer metrics.
func (sv *SyncersView) getSyncerHealthStatus(syncer data.SyncerData) string {
	var issues []string

	if !syncer.Connected {
		issues = append(issues, "[red]⚠[-] Connection lost to target cluster")
	}
	
	if syncer.QueueDepth > 20 {
		issues = append(issues, "[red]⚠[-] Work queue is backing up")
	}
	
	if syncer.ErrorRate > 10.0 {
		issues = append(issues, "[red]⚠[-] High error rate detected")
	}
	
	if syncer.SyncLatency > 500*time.Millisecond {
		issues = append(issues, "[yellow]⚠[-] Sync latency is elevated")
	}
	
	if time.Since(syncer.LastSync) > 5*time.Minute {
		issues = append(issues, "[red]⚠[-] Syncer has been inactive")
	}

	if len(issues) == 0 {
		return "[green]✓[-] All systems operational\n[green]✓[-] Performance within normal range\n[green]✓[-] Connection stable"
	}

	result := ""
	for _, issue := range issues {
		if result != "" {
			result += "\n"
		}
		result += issue
	}
	
	return result
}

// formatAge formats a time into a human-readable age string.
func (sv *SyncersView) formatAge(t time.Time) string {
	d := time.Since(t)
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

// GetView returns the tview primitive for this syncers view.
func (sv *SyncersView) GetView() tview.Primitive {
	return sv.layout
}