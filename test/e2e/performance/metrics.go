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

package performance

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// MetricsCollector collects and analyzes performance metrics
type MetricsCollector struct {
	metrics        []Metric
	regressions    []RegressionCheck
	baselineFile   string
	outputDir      string
	currentRun     RunMetadata
}

// Metric represents a performance metric measurement
type Metric struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // latency, throughput, memory, etc.
	Value       float64           `json:"value"`
	Unit        string            `json:"unit"`
	Timestamp   time.Time         `json:"timestamp"`
	Labels      map[string]string `json:"labels,omitempty"`
	Percentiles map[string]float64 `json:"percentiles,omitempty"`
}

// RunMetadata contains information about the benchmark run
type RunMetadata struct {
	Timestamp    time.Time         `json:"timestamp"`
	GitCommit    string            `json:"git_commit,omitempty"`
	BenchmarkID  string            `json:"benchmark_id"`
	Environment  map[string]string `json:"environment"`
	GoVersion    string            `json:"go_version"`
	NumCPU       int               `json:"num_cpu"`
	MemoryTotal  uint64            `json:"memory_total"`
}

// RegressionCheck defines a performance regression threshold
type RegressionCheck struct {
	MetricName        string  `json:"metric_name"`
	ThresholdPercent  float64 `json:"threshold_percent"`  // % increase that triggers regression
	BaselineValue     float64 `json:"baseline_value"`
	CurrentValue      float64 `json:"current_value"`
	IsRegression      bool    `json:"is_regression"`
	PercentDifference float64 `json:"percent_difference"`
}

// PerformanceReport contains the full performance analysis
type PerformanceReport struct {
	Metadata    RunMetadata       `json:"metadata"`
	Metrics     []Metric          `json:"metrics"`
	Regressions []RegressionCheck `json:"regressions"`
	Summary     ReportSummary     `json:"summary"`
}

// ReportSummary provides a high-level summary of the performance results
type ReportSummary struct {
	TotalMetrics      int     `json:"total_metrics"`
	RegressionsFound  int     `json:"regressions_found"`
	AvgLatency        float64 `json:"avg_latency_ns"`
	MaxLatency        float64 `json:"max_latency_ns"`
	MinLatency        float64 `json:"min_latency_ns"`
	TotalThroughput   float64 `json:"total_throughput_ops_per_sec"`
	MemoryUsageMB     float64 `json:"memory_usage_mb"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(benchmarkID, outputDir string) *MetricsCollector {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return &MetricsCollector{
		metrics:      make([]Metric, 0),
		regressions:  make([]RegressionCheck, 0),
		baselineFile: filepath.Join(outputDir, "baseline_metrics.json"),
		outputDir:    outputDir,
		currentRun: RunMetadata{
			Timestamp:   time.Now(),
			BenchmarkID: benchmarkID,
			GoVersion:   runtime.Version(),
			NumCPU:      runtime.NumCPU(),
			MemoryTotal: memStats.Sys,
			Environment: map[string]string{
				"GOOS":   runtime.GOOS,
				"GOARCH": runtime.GOARCH,
			},
		},
	}
}

// RecordMetric records a performance metric
func (mc *MetricsCollector) RecordMetric(name, metricType, unit string, value float64, labels map[string]string) {
	metric := Metric{
		Name:      name,
		Type:      metricType,
		Value:     value,
		Unit:      unit,
		Timestamp: time.Now(),
		Labels:    labels,
	}
	
	mc.metrics = append(mc.metrics, metric)
}

// RecordLatencyMetrics records latency metrics with percentiles
func (mc *MetricsCollector) RecordLatencyMetrics(name string, durations []time.Duration, labels map[string]string) {
	if len(durations) == 0 {
		return
	}

	// Convert to float64 slice for calculations
	values := make([]float64, len(durations))
	for i, d := range durations {
		values[i] = float64(d.Nanoseconds())
	}

	sort.Float64s(values)

	// Calculate percentiles
	percentiles := map[string]float64{
		"p50":  calculatePercentile(values, 50),
		"p90":  calculatePercentile(values, 90),
		"p95":  calculatePercentile(values, 95),
		"p99":  calculatePercentile(values, 99),
		"p99.9": calculatePercentile(values, 99.9),
	}

	// Calculate average
	var sum float64
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))

	metric := Metric{
		Name:        name + "_latency",
		Type:        "latency",
		Value:       avg,
		Unit:        "ns",
		Timestamp:   time.Now(),
		Labels:      labels,
		Percentiles: percentiles,
	}
	
	mc.metrics = append(mc.metrics, metric)
}

// RecordMemoryMetrics records current memory usage metrics
func (mc *MetricsCollector) RecordMemoryMetrics(labels map[string]string) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := []struct {
		name  string
		value float64
		unit  string
	}{
		{"heap_alloc", float64(memStats.HeapAlloc), "bytes"},
		{"heap_sys", float64(memStats.HeapSys), "bytes"},
		{"heap_objects", float64(memStats.HeapObjects), "count"},
		{"gc_cycles", float64(memStats.NumGC), "count"},
		{"gc_pause_total", float64(memStats.PauseTotalNs), "ns"},
	}

	for _, m := range metrics {
		mc.RecordMetric(m.name, "memory", m.unit, m.value, labels)
	}
}

// CheckRegression compares current metrics against baseline for regression detection
func (mc *MetricsCollector) CheckRegression(metricName string, currentValue, baselineValue, thresholdPercent float64) {
	if baselineValue == 0 {
		return // Cannot calculate regression without baseline
	}

	percentDiff := ((currentValue - baselineValue) / baselineValue) * 100
	isRegression := percentDiff > thresholdPercent

	regression := RegressionCheck{
		MetricName:        metricName,
		ThresholdPercent:  thresholdPercent,
		BaselineValue:     baselineValue,
		CurrentValue:      currentValue,
		IsRegression:      isRegression,
		PercentDifference: percentDiff,
	}

	mc.regressions = append(mc.regressions, regression)
}

// LoadBaseline loads baseline metrics from file for regression comparison
func (mc *MetricsCollector) LoadBaseline() (map[string]float64, error) {
	baseline := make(map[string]float64)
	
	if _, err := os.Stat(mc.baselineFile); os.IsNotExist(err) {
		return baseline, nil // No baseline file exists yet
	}

	data, err := os.ReadFile(mc.baselineFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var report PerformanceReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline data: %w", err)
	}

	// Extract baseline values
	for _, metric := range report.Metrics {
		baseline[metric.Name] = metric.Value
	}

	return baseline, nil
}

// SaveBaseline saves current metrics as baseline for future comparisons
func (mc *MetricsCollector) SaveBaseline() error {
	report := mc.GenerateReport()
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline data: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(mc.baselineFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(mc.baselineFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write baseline file: %w", err)
	}

	return nil
}

// AnalyzePerformance performs comprehensive performance analysis including regression detection
func (mc *MetricsCollector) AnalyzePerformance() error {
	baseline, err := mc.LoadBaseline()
	if err != nil {
		return fmt.Errorf("failed to load baseline: %w", err)
	}

	// Check for regressions
	for _, metric := range mc.metrics {
		if baselineValue, exists := baseline[metric.Name]; exists {
			// Define threshold based on metric type
			threshold := 20.0 // 20% increase by default
			if metric.Type == "latency" {
				threshold = 15.0 // More sensitive to latency increases
			} else if metric.Type == "throughput" {
				threshold = -10.0 // Throughput decrease is a regression
			}
			
			mc.CheckRegression(metric.Name, metric.Value, baselineValue, threshold)
		}
	}

	return nil
}

// GenerateReport creates a comprehensive performance report
func (mc *MetricsCollector) GenerateReport() *PerformanceReport {
	summary := mc.calculateSummary()
	
	return &PerformanceReport{
		Metadata:    mc.currentRun,
		Metrics:     mc.metrics,
		Regressions: mc.regressions,
		Summary:     summary,
	}
}

// SaveReport saves the performance report to a file
func (mc *MetricsCollector) SaveReport(filename string) error {
	report := mc.GenerateReport()
	
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	reportPath := filepath.Join(mc.outputDir, filename)
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// PrintSummary prints a human-readable summary to stdout
func (mc *MetricsCollector) PrintSummary() {
	report := mc.GenerateReport()
	
	fmt.Println("=== Performance Benchmark Summary ===")
	fmt.Printf("Benchmark ID: %s\n", report.Metadata.BenchmarkID)
	fmt.Printf("Timestamp: %s\n", report.Metadata.Timestamp.Format(time.RFC3339))
	fmt.Printf("Go Version: %s\n", report.Metadata.GoVersion)
	fmt.Printf("CPU Count: %d\n", report.Metadata.NumCPU)
	fmt.Printf("Memory Total: %.2f MB\n", float64(report.Metadata.MemoryTotal)/1024/1024)
	fmt.Println()
	
	fmt.Printf("Total Metrics: %d\n", report.Summary.TotalMetrics)
	fmt.Printf("Average Latency: %.2f μs\n", report.Summary.AvgLatency/1000)
	fmt.Printf("Max Latency: %.2f μs\n", report.Summary.MaxLatency/1000)
	fmt.Printf("Total Throughput: %.2f ops/sec\n", report.Summary.TotalThroughput)
	fmt.Printf("Memory Usage: %.2f MB\n", report.Summary.MemoryUsageMB)
	fmt.Println()
	
	if report.Summary.RegressionsFound > 0 {
		fmt.Printf("⚠️  REGRESSIONS FOUND: %d\n", report.Summary.RegressionsFound)
		for _, regression := range report.Regressions {
			if regression.IsRegression {
				fmt.Printf("  - %s: %.2f%% increase (%.2f -> %.2f)\n", 
					regression.MetricName, 
					regression.PercentDifference,
					regression.BaselineValue,
					regression.CurrentValue)
			}
		}
	} else {
		fmt.Println("✅ No performance regressions detected")
	}
	fmt.Println()
}

// calculateSummary computes summary statistics from collected metrics
func (mc *MetricsCollector) calculateSummary() ReportSummary {
	summary := ReportSummary{
		TotalMetrics: len(mc.metrics),
	}
	
	var latencySum, maxLatency, minLatency, throughputSum, memorySum float64
	var latencyCount, throughputCount, memoryCount int
	
	minLatency = math.Inf(1)
	
	for _, metric := range mc.metrics {
		switch metric.Type {
		case "latency":
			latencySum += metric.Value
			latencyCount++
			if metric.Value > maxLatency {
				maxLatency = metric.Value
			}
			if metric.Value < minLatency {
				minLatency = metric.Value
			}
		case "throughput":
			throughputSum += metric.Value
			throughputCount++
		case "memory":
			if metric.Unit == "bytes" {
				memorySum += metric.Value / 1024 / 1024 // Convert to MB
				memoryCount++
			}
		}
	}
	
	if latencyCount > 0 {
		summary.AvgLatency = latencySum / float64(latencyCount)
		summary.MaxLatency = maxLatency
		summary.MinLatency = minLatency
	}
	
	if throughputCount > 0 {
		summary.TotalThroughput = throughputSum
	}
	
	if memoryCount > 0 {
		summary.MemoryUsageMB = memorySum / float64(memoryCount)
	}
	
	summary.RegressionsFound = 0
	for _, regression := range mc.regressions {
		if regression.IsRegression {
			summary.RegressionsFound++
		}
	}
	
	return summary
}

// calculatePercentile calculates the given percentile from a sorted slice of values
func calculatePercentile(sortedValues []float64, percentile float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	
	if percentile == 0 {
		return sortedValues[0]
	}
	if percentile == 100 {
		return sortedValues[len(sortedValues)-1]
	}
	
	index := (percentile / 100.0) * float64(len(sortedValues)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if lower == upper {
		return sortedValues[lower]
	}
	
	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}