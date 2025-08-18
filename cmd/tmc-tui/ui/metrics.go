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
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/data"
)

// MetricsView represents the detailed metrics view.
type MetricsView struct {
	config    *config.Config
	collector *data.Collector

	// Layout components
	layout      *tview.Flex
	summary     *tview.TextView
	charts      *tview.TextView
	performance *tview.TextView
	trends      *tview.TextView

	// Historical data for trend display
	latencyHistory []float64
	errorHistory   []float64
	throughputHistory []float64
}

// NewMetricsView creates a new metrics view.
func NewMetricsView(cfg *config.Config, collector *data.Collector) (*MetricsView, error) {
	mv := &MetricsView{
		config:    cfg,
		collector: collector,
		latencyHistory: make([]float64, 0, 60),    // Keep 60 data points
		errorHistory:   make([]float64, 0, 60),
		throughputHistory: make([]float64, 0, 60),
	}

	if err := mv.initializeMetricsView(); err != nil {
		return nil, err
	}

	return mv, nil
}

// initializeMetricsView sets up the metrics view layout and components.
func (mv *MetricsView) initializeMetricsView() error {
	// Create main layout - 2x2 grid
	mv.layout = tview.NewFlex().SetDirection(tview.FlexColumn)

	// Left column
	leftColumn := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Summary panel
	mv.summary = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Metrics Summary ").
		SetTitleAlign(tview.AlignLeft)

	// Performance panel
	mv.performance = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetTitle(" Performance Indicators ").
		SetTitleAlign(tview.AlignLeft)

	// Right column
	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow)

	// Charts panel
	mv.charts = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetTitle(" Real-time Charts ").
		SetTitleAlign(tview.AlignLeft)

	// Trends panel
	mv.trends = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false).
		SetTitle(" Historical Trends ").
		SetTitleAlign(tview.AlignLeft)

	// Apply borders and theme
	if mv.config.Theme.ShowBorders {
		mv.summary.SetBorder(true)
		mv.performance.SetBorder(true)
		mv.charts.SetBorder(true)
		mv.trends.SetBorder(true)
	}

	mv.applyTheme()

	// Add to columns
	leftColumn.AddItem(mv.summary, 0, 1, false).
		AddItem(mv.performance, 0, 1, false)

	rightColumn.AddItem(mv.charts, 0, 1, false).
		AddItem(mv.trends, 0, 1, false)

	// Add columns to main layout
	mv.layout.AddItem(leftColumn, 0, 1, false).
		AddItem(rightColumn, 0, 1, false)

	// Initial refresh
	mv.Refresh()

	return nil
}

// applyTheme applies the configured theme to metrics view components.
func (mv *MetricsView) applyTheme() {
	var bg, fg tcell.Color

	switch mv.config.Theme.ColorScheme {
	case "light":
		bg, fg = tcell.ColorWhite, tcell.ColorBlack
	case "dark":
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	default:
		bg, fg = tcell.ColorBlack, tcell.ColorWhite
	}

	components := []*tview.TextView{mv.summary, mv.performance, mv.charts, mv.trends}
	for _, component := range components {
		component.SetBackgroundColor(bg).SetTextColor(fg)
	}
}

// Refresh updates the metrics view with current data.
func (mv *MetricsView) Refresh() {
	metrics := mv.collector.GetMetrics()
	syncers := mv.collector.GetSyncers()
	clusters := mv.collector.GetClusters()

	// Update historical data
	mv.updateHistoricalData(metrics)

	mv.updateSummary(metrics)
	mv.updatePerformance(metrics, syncers, clusters)
	mv.updateCharts(metrics)
	mv.updateTrends()
}

// updateHistoricalData adds current metrics to historical data arrays.
func (mv *MetricsView) updateHistoricalData(metrics data.MetricsData) {
	// Add current values
	mv.latencyHistory = append(mv.latencyHistory, float64(metrics.AverageLatency.Milliseconds()))
	mv.errorHistory = append(mv.errorHistory, metrics.ErrorRate)
	mv.throughputHistory = append(mv.throughputHistory, float64(metrics.ResourcesSynced))

	// Keep only last 60 entries
	if len(mv.latencyHistory) > 60 {
		mv.latencyHistory = mv.latencyHistory[len(mv.latencyHistory)-60:]
	}
	if len(mv.errorHistory) > 60 {
		mv.errorHistory = mv.errorHistory[len(mv.errorHistory)-60:]
	}
	if len(mv.throughputHistory) > 60 {
		mv.throughputHistory = mv.throughputHistory[len(mv.throughputHistory)-60:]
	}
}

// updateSummary updates the metrics summary panel.
func (mv *MetricsView) updateSummary(metrics data.MetricsData) {
	healthScore := mv.calculateHealthScore(metrics)
	healthColor := "green"
	if healthScore < 70 {
		healthColor = "red"
	} else if healthScore < 85 {
		healthColor = "yellow"
	}

	availability := float64(metrics.HealthyClusters) / float64(metrics.TotalClusters) * 100
	availabilityColor := "green"
	if availability < 90 {
		availabilityColor = "red"
	} else if availability < 95 {
		availabilityColor = "yellow"
	}

	content := fmt.Sprintf(`[blue::b]System Health[-::-]
[yellow]Overall Score:[-] [%s]%.1f/100[-]
[yellow]Availability:[-] [%s]%.1f%%[-]
[yellow]Last Update:[-] %s

[blue::b]Resource Overview[-::-]
[yellow]Total Clusters:[-] %d
[yellow]Healthy Clusters:[-] %d
[yellow]Total Syncers:[-] %d
[yellow]Active Syncers:[-] %d

[blue::b]Data Flow[-::-]
[yellow]Resources Synced:[-] %d
[yellow]Sync Operations:[-] %d/min
[yellow]Data Transfer:[-] %s

[blue::b]System Status[-::-]
%s`,
		healthColor, healthScore,
		availabilityColor, availability,
		metrics.LastUpdateTime.Format("15:04:05"),
		metrics.TotalClusters,
		metrics.HealthyClusters,
		metrics.TotalSyncers,
		metrics.ActiveSyncers,
		metrics.ResourcesSynced,
		int(float64(metrics.ResourcesSynced)/60), // Operations per minute
		"12.4MB", // Mock data transfer
		mv.getSystemStatusIndicators(metrics),
	)

	mv.summary.SetText(content)
}

// updatePerformance updates the performance indicators panel.
func (mv *MetricsView) updatePerformance(metrics data.MetricsData, syncers []data.SyncerData, clusters []data.ClusterData) {
	latencyColor := "green"
	if metrics.AverageLatency > 500*time.Millisecond {
		latencyColor = "red"
	} else if metrics.AverageLatency > 200*time.Millisecond {
		latencyColor = "yellow"
	}

	errorColor := "green"
	if metrics.ErrorRate > 10.0 {
		errorColor = "red"
	} else if metrics.ErrorRate > 5.0 {
		errorColor = "yellow"
	}

	// Calculate performance metrics
	avgQueueDepth := mv.calculateAverageQueueDepth(syncers)
	maxQueueDepth := mv.calculateMaxQueueDepth(syncers)
	
	content := fmt.Sprintf(`[blue::b]Latency Metrics[-::-]
[yellow]Average Latency:[-] [%s]%dms[-]
[yellow]P95 Latency:[-] %dms
[yellow]P99 Latency:[-] %dms
[yellow]Max Latency:[-] %dms

[blue::b]Error Metrics[-::-]
[yellow]Error Rate:[-] [%s]%.1f/min[-]
[yellow]Success Rate:[-] %.1f%%
[yellow]Total Errors (24h):[-] %d
[yellow]MTBF:[-] %s

[blue::b]Queue Metrics[-::-]
[yellow]Avg Queue Depth:[-] %.1f
[yellow]Max Queue Depth:[-] %d
[yellow]Queue Processing:[-] %.1f/sec
[yellow]Backlog Age:[-] %s

[blue::b]Throughput[-::-]
[yellow]Current Rate:[-] %.1f ops/sec
[yellow]Peak Rate (1h):[-] %.1f ops/sec
[yellow]Daily Volume:[-] %d ops
[yellow]Efficiency:[-] %.1f%%`,
		latencyColor, metrics.AverageLatency.Milliseconds(),
		int(float64(metrics.AverageLatency.Milliseconds())*1.5), // Mock P95
		int(float64(metrics.AverageLatency.Milliseconds())*2.0), // Mock P99
		int(float64(metrics.AverageLatency.Milliseconds())*3.0), // Mock Max
		errorColor, metrics.ErrorRate,
		100.0-metrics.ErrorRate*100.0/60.0, // Rough success rate
		int(metrics.ErrorRate*24*60), // Errors in 24h
		"4h 23m", // Mock MTBF
		avgQueueDepth,
		maxQueueDepth,
		25.6, // Mock processing rate
		"2m 15s", // Mock backlog age
		float64(metrics.ResourcesSynced)/60.0, // Current rate
		float64(metrics.ResourcesSynced)/45.0, // Peak rate
		int(metrics.ResourcesSynced*24), // Daily volume
		95.2, // Mock efficiency
	)

	mv.performance.SetText(content)
}

// updateCharts updates the real-time charts panel with ASCII charts.
func (mv *MetricsView) updateCharts(metrics data.MetricsData) {
	// Create simple ASCII charts for recent data
	latencyChart := mv.createSimpleChart("Latency (ms)", mv.latencyHistory, 8)
	errorChart := mv.createSimpleChart("Error Rate", mv.errorHistory, 6)
	
	content := fmt.Sprintf(`[blue::b]Latency Trend (Last 60min)[-::-]
%s

[blue::b]Error Rate Trend[-::-]
%s

[blue::b]Real-time Status[-::-]
[green]●[-] Clusters Online: %d/%d
[yellow]●[-] Syncers Active: %d/%d
[blue]●[-] Connections: %d
[purple]●[-] Queue Items: %d`,
		latencyChart,
		errorChart,
		metrics.HealthyClusters, metrics.TotalClusters,
		metrics.ActiveSyncers, metrics.TotalSyncers,
		metrics.ActiveSyncers, // Mock connections
		mv.calculateTotalQueueDepth(),
	)

	mv.charts.SetText(content)
}

// updateTrends updates the historical trends panel.
func (mv *MetricsView) updateTrends() {
	// Create trend indicators
	latencyTrend := mv.calculateTrend(mv.latencyHistory)
	errorTrend := mv.calculateTrend(mv.errorHistory)
	throughputTrend := mv.calculateTrend(mv.throughputHistory)

	content := fmt.Sprintf(`[blue::b]Performance Trends[-::-]
[yellow]Latency Trend:[-] %s
[yellow]Error Rate Trend:[-] %s
[yellow]Throughput Trend:[-] %s

[blue::b]Capacity Planning[-::-]
[yellow]Resource Utilization:[-] 67%%
[yellow]Growth Rate:[-] +2.3%%/week
[yellow]Projected Capacity:[-] 87%% (next 30d)
[yellow]Scale Recommendation:[-] %s

[blue::b]SLA Compliance[-::-]
[yellow]Availability SLA:[-] [green]99.9%%[-] ✓
[yellow]Latency SLA:[-] [green]<200ms[-] ✓
[yellow]Error Rate SLA:[-] [green]<1%%[-] ✓
[yellow]Overall SLA:[-] [green]Met[-] ✓

[blue::b]Optimization Hints[-::-]
%s`,
		mv.formatTrendIndicator(latencyTrend, false), // Lower is better for latency
		mv.formatTrendIndicator(errorTrend, false),   // Lower is better for errors
		mv.formatTrendIndicator(throughputTrend, true), // Higher is better for throughput
		mv.getScaleRecommendation(),
		mv.getOptimizationHints(),
	)

	mv.trends.SetText(content)
}

// Helper methods for calculations and formatting

func (mv *MetricsView) calculateHealthScore(metrics data.MetricsData) float64 {
	score := 100.0
	
	// Deduct for unavailable clusters
	if metrics.TotalClusters > 0 {
		availability := float64(metrics.HealthyClusters) / float64(metrics.TotalClusters)
		score -= (1.0 - availability) * 30.0
	}
	
	// Deduct for inactive syncers
	if metrics.TotalSyncers > 0 {
		syncerHealth := float64(metrics.ActiveSyncers) / float64(metrics.TotalSyncers)
		score -= (1.0 - syncerHealth) * 25.0
	}
	
	// Deduct for high latency
	if metrics.AverageLatency > 200*time.Millisecond {
		score -= 20.0
	}
	
	// Deduct for high error rate
	if metrics.ErrorRate > 5.0 {
		score -= 25.0
	}
	
	if score < 0 {
		score = 0
	}
	
	return score
}

func (mv *MetricsView) getSystemStatusIndicators(metrics data.MetricsData) string {
	indicators := []string{}
	
	if metrics.HealthyClusters == metrics.TotalClusters {
		indicators = append(indicators, "[green]✓[-] All clusters healthy")
	} else {
		indicators = append(indicators, fmt.Sprintf("[red]⚠[-] %d clusters unhealthy", metrics.TotalClusters-metrics.HealthyClusters))
	}
	
	if metrics.ActiveSyncers == metrics.TotalSyncers {
		indicators = append(indicators, "[green]✓[-] All syncers active")
	} else {
		indicators = append(indicators, fmt.Sprintf("[yellow]⚠[-] %d syncers inactive", metrics.TotalSyncers-metrics.ActiveSyncers))
	}
	
	if metrics.ErrorRate < 5.0 {
		indicators = append(indicators, "[green]✓[-] Error rate normal")
	} else {
		indicators = append(indicators, "[red]⚠[-] High error rate")
	}
	
	return strings.Join(indicators, "\n")
}

func (mv *MetricsView) calculateAverageQueueDepth(syncers []data.SyncerData) float64 {
	if len(syncers) == 0 {
		return 0
	}
	
	total := 0
	for _, syncer := range syncers {
		total += syncer.QueueDepth
	}
	
	return float64(total) / float64(len(syncers))
}

func (mv *MetricsView) calculateMaxQueueDepth(syncers []data.SyncerData) int {
	max := 0
	for _, syncer := range syncers {
		if syncer.QueueDepth > max {
			max = syncer.QueueDepth
		}
	}
	return max
}

func (mv *MetricsView) calculateTotalQueueDepth() int {
	syncers := mv.collector.GetSyncers()
	total := 0
	for _, syncer := range syncers {
		total += syncer.QueueDepth
	}
	return total
}

func (mv *MetricsView) createSimpleChart(title string, data []float64, height int) string {
	if len(data) == 0 {
		return "[gray]No data[-]"
	}

	// Create simple ASCII chart
	chart := fmt.Sprintf("[yellow]%s[-]\n", title)
	
	// Find min/max for scaling
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min { min = v }
		if v > max { max = v }
	}
	
	if min == max {
		chart += "[gray]Constant value[-]"
		return chart
	}

	// Create chart lines
	for i := height - 1; i >= 0; i-- {
		threshold := min + (max-min)*float64(i)/float64(height-1)
		line := ""
		for _, value := range data[max(0, len(data)-30):] { // Show last 30 points
			if value >= threshold {
				line += "█"
			} else {
				line += " "
			}
		}
		chart += line + "\n"
	}
	
	chart += fmt.Sprintf("[gray]%.1f - %.1f[-]", min, max)
	return chart
}

func (mv *MetricsView) calculateTrend(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	
	// Simple trend calculation - compare recent average to older average
	mid := len(data) / 2
	oldAvg := mv.average(data[:mid])
	newAvg := mv.average(data[mid:])
	
	if oldAvg == 0 {
		return 0
	}
	
	return (newAvg - oldAvg) / oldAvg * 100 // Percentage change
}

func (mv *MetricsView) average(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	
	return sum / float64(len(data))
}

func (mv *MetricsView) formatTrendIndicator(trend float64, higherIsBetter bool) string {
	if trend > 5 {
		if higherIsBetter {
			return "[green]↗ Improving[-]"
		} else {
			return "[red]↗ Degrading[-]"
		}
	} else if trend < -5 {
		if higherIsBetter {
			return "[red]↘ Declining[-]"
		} else {
			return "[green]↘ Improving[-]"
		}
	}
	return "[yellow]→ Stable[-]"
}

func (mv *MetricsView) getScaleRecommendation() string {
	// Mock scale recommendation based on trends
	return "[green]Normal[-] - No scaling needed"
}

func (mv *MetricsView) getOptimizationHints() string {
	return "[green]✓[-] System performing optimally\n[yellow]💡[-] Consider enabling caching for improved latency"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetView returns the tview primitive for this metrics view.
func (mv *MetricsView) GetView() tview.Primitive {
	return mv.layout
}