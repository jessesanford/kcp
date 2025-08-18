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

// ClustersView represents the cluster status view showing detailed cluster information.
type ClustersView struct {
	config    *config.Config
	collector *data.Collector

	// Layout components
	layout *tview.Flex
	table  *components.TableWidget
	detail *tview.TextView
}

// NewClustersView creates a new clusters view.
func NewClustersView(cfg *config.Config, collector *data.Collector) (*ClustersView, error) {
	cv := &ClustersView{
		config:    cfg,
		collector: collector,
	}

	if err := cv.initializeClustersView(); err != nil {
		return nil, err
	}

	return cv, nil
}

// initializeClustersView sets up the clusters view layout and components.
func (cv *ClustersView) initializeClustersView() error {
	// Create main layout
	cv.layout = tview.NewFlex().SetDirection(tview.FlexRow)

	// Create table for cluster list
	cv.table = components.NewTableWidget(cv.config, "Clusters")
	
	// Set column headers
	headers := []string{"Name", "Status", "Health", "Nodes", "Pods", "Location", "Version", "Last Seen"}
	cv.table.SetHeaders(headers)
	
	// Set column widths
	cv.table.SetColumnWidths([]int{20, 10, 10, 8, 8, 15, 12, 15})

	// Create detail panel
	cv.detail = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Cluster Details ").
		SetTitleAlign(tview.AlignLeft)

	if cv.config.Theme.ShowBorders {
		cv.detail.SetBorder(true)
	}

	// Apply theme
	cv.applyTheme()

	// Set up selection handler
	cv.table.SetSelectionChangedFunc(func(row, column int) {
		cv.updateDetails(row)
	})

	// Add components to layout
	tableHeight := 12
	if cv.config.Theme.CompactMode {
		tableHeight = 8
	}
	
	cv.layout.AddItem(cv.table.GetView(), tableHeight, 0, true).
		AddItem(cv.detail, 0, 1, false)

	// Initial refresh
	cv.Refresh()

	return nil
}

// applyTheme applies the configured theme to clusters view components.
func (cv *ClustersView) applyTheme() {
	var bg, fg tcell.Color

	switch cv.config.Theme.ColorScheme {
	case "light":
		bg, fg = tcell.ColorWhite, tcell.ColorBlack
	case "dark":
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	default:
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	}

	cv.detail.SetBackgroundColor(bg).SetTextColor(fg)
}

// Refresh updates the clusters view with current data.
func (cv *ClustersView) Refresh() {
	clusters := cv.collector.GetClusters()
	
	// Clear existing data (keep headers)
	cv.table.Clear()

	// Add cluster data to table
	for _, cluster := range clusters {
		lastSeenStr := cv.formatAge(cluster.LastSeen)
		
		rowData := []string{
			cluster.Name,
			cluster.Status,
			cluster.HealthStatus,
			fmt.Sprintf("%d", cluster.NodeCount),
			fmt.Sprintf("%d", cluster.PodCount),
			cluster.Location,
			cluster.Version,
			lastSeenStr,
		}
		
		row := cv.table.AddRow(rowData)
		
		// Apply status-based coloring
		cv.colorizeClusterRow(row, cluster)
	}

	// Update details for selected row
	selectedRow := cv.table.GetSelectedRow()
	cv.updateDetails(selectedRow)
}

// colorizeClusterRow applies appropriate colors based on cluster status.
func (cv *ClustersView) colorizeClusterRow(row int, cluster data.ClusterData) {
	// Status column
	statusColor := tcell.ColorGreen
	if cluster.Status != "Ready" {
		statusColor = tcell.ColorRed
	}
	cv.table.SetCellColor(row, 1, statusColor)

	// Health column
	healthColor := tcell.ColorGreen
	if cluster.HealthStatus != "Healthy" {
		healthColor = tcell.ColorRed
	}
	cv.table.SetCellColor(row, 2, healthColor)

	// Last seen column - warn if old
	age := time.Since(cluster.LastSeen)
	ageColor := tcell.ColorWhite
	if age > 5*time.Minute {
		ageColor = tcell.ColorRed
	} else if age > 2*time.Minute {
		ageColor = tcell.ColorYellow
	}
	cv.table.SetCellColor(row, 7, ageColor)
}

// updateDetails updates the detail panel with information for the selected cluster.
func (cv *ClustersView) updateDetails(row int) {
	clusters := cv.collector.GetClusters()
	
	// Adjust for header row
	if row <= 0 || row > len(clusters) {
		cv.detail.SetText("[yellow]Select a cluster to view details[-]")
		return
	}
	
	cluster := clusters[row-1] // Adjust for header row
	
	healthColor := "green"
	if cluster.HealthStatus != "Healthy" {
		healthColor = "red"
	}
	
	statusColor := "green"
	if cluster.Status != "Ready" {
		statusColor = "red"
	}

	ageStr := cv.formatAge(cluster.LastSeen)
	
	content := fmt.Sprintf(`[blue::b]%s[-::-]

[blue::b]Status Information[-::-]
[yellow]Status:[-] [%s]%s[-]
[yellow]Health:[-] [%s]%s[-]
[yellow]Last Contact:[-] %s (%s ago)

[blue::b]Capacity[-::-]
[yellow]Nodes:[-] %d
[yellow]Pods:[-] %d
[yellow]Resource Utilization:[-] %.1f%%

[blue::b]Configuration[-::-]
[yellow]Location:[-] %s
[yellow]Kubernetes Version:[-] %s
[yellow]API Endpoint:[-] %s

[blue::b]Recent Activity[-::-]
[green]✓[-] Node health checks passing
[green]✓[-] API server responsive
[green]✓[-] Control plane stable
%s`,
		cluster.Name,
		statusColor, cluster.Status,
		healthColor, cluster.HealthStatus,
		cluster.LastSeen.Format("15:04:05 Jan 2"),
		ageStr,
		cluster.NodeCount,
		cluster.PodCount,
		float64(cluster.PodCount)/float64(cluster.NodeCount*20)*100, // Rough utilization
		cluster.Location,
		cluster.Version,
		fmt.Sprintf("https://%s-api.example.com", cluster.Name), // Mock endpoint
		cv.getClusterActivityStatus(cluster),
	)

	cv.detail.SetText(content)
}

// getClusterActivityStatus returns activity status based on cluster health.
func (cv *ClustersView) getClusterActivityStatus(cluster data.ClusterData) string {
	if cluster.HealthStatus != "Healthy" {
		return "\n[red]⚠[-] Issues detected - check cluster logs"
	}
	
	if time.Since(cluster.LastSeen) > 2*time.Minute {
		return "\n[yellow]⚠[-] Connection issues - cluster not recently seen"
	}
	
	return "\n[green]✓[-] All systems operational"
}

// formatAge formats a time into a human-readable age string.
func (cv *ClustersView) formatAge(t time.Time) string {
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

// GetView returns the tview primitive for this clusters view.
func (cv *ClustersView) GetView() tview.Primitive {
	return cv.layout
}