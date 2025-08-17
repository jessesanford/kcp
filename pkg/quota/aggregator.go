/*
Copyright 2025 The KCP Authors.

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

package quota

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

const (
	// DefaultCollectionInterval defines how often metrics are collected
	DefaultCollectionInterval = 15 * time.Second
	
	// DefaultTrendWindow defines the window for trend analysis
	DefaultTrendWindow = 24 * time.Hour
	
	// MaxSyncTargets limits the number of sync targets monitored
	MaxSyncTargets = 100
	
	// DataRetentionPeriod defines how long historical data is kept
	DataRetentionPeriod = 7 * 24 * time.Hour
)

// AggregatorResourceName represents a resource type for quota management (defined locally for aggregator)
// Note: This duplicates types from manager.go but ensures aggregator can work independently
type AggregatorResourceName string

const (
	// Core compute resources
	AggregatorResourceCPU              AggregatorResourceName = "cpu"
	AggregatorResourceMemory           AggregatorResourceName = "memory" 
	AggregatorResourceStorage          AggregatorResourceName = "storage"
	AggregatorResourceEphemeralStorage AggregatorResourceName = "ephemeral-storage"
	
	// Object count resources
	AggregatorResourcePods        AggregatorResourceName = "pods"
	AggregatorResourceServices    AggregatorResourceName = "services"
	AggregatorResourceDeployments AggregatorResourceName = "deployments"
)

// SyncTargetMetrics represents resource metrics from a single SyncTarget
type SyncTargetMetrics struct {
	// SyncTarget name
	SyncTargetName string `json:"syncTargetName"`
	
	// Cluster name associated with the SyncTarget
	ClusterName string `json:"clusterName"`
	
	// Resource usage by type
	Usage map[AggregatorResourceName]resource.Quantity `json:"usage"`
	
	// Resource capacity by type
	Capacity map[AggregatorResourceName]resource.Quantity `json:"capacity"`
	
	// Health status of the SyncTarget
	Healthy bool `json:"healthy"`
	
	// Last update timestamp
	LastUpdated metav1.Time `json:"lastUpdated"`
	
	// Collection errors
	Errors []string `json:"errors,omitempty"`
}

// AggregatedMetrics represents aggregated resource metrics across all SyncTargets
type AggregatedMetrics struct {
	// Total usage across all clusters
	TotalUsage map[AggregatorResourceName]resource.Quantity `json:"totalUsage"`
	
	// Total capacity across all clusters
	TotalCapacity map[AggregatorResourceName]resource.Quantity `json:"totalCapacity"`
	
	// Available resources (capacity - usage)
	TotalAvailable map[AggregatorResourceName]resource.Quantity `json:"totalAvailable"`
	
	// Utilization percentages by resource type
	Utilization map[AggregatorResourceName]float64 `json:"utilization"`
	
	// Number of healthy clusters
	HealthyClusters int `json:"healthyClusters"`
	
	// Number of total clusters
	TotalClusters int `json:"totalClusters"`
	
	// Aggregation timestamp
	AggregatedAt metav1.Time `json:"aggregatedAt"`
	
	// Per-cluster breakdown
	ClusterMetrics []SyncTargetMetrics `json:"clusterMetrics,omitempty"`
}

// TrendPoint represents a single point in resource usage trend
type TrendPoint struct {
	Timestamp metav1.Time                          `json:"timestamp"`
	Usage     map[AggregatorResourceName]resource.Quantity `json:"usage"`
}

// TrendAnalysis represents trend analysis for resource usage
type TrendAnalysis struct {
	// Resource name
	Resource AggregatorResourceName `json:"resource"`
	
	// Historical data points
	DataPoints []TrendPoint `json:"dataPoints"`
	
	// Trend direction (positive = increasing, negative = decreasing)
	TrendDirection float64 `json:"trendDirection"`
	
	// Predicted usage at next collection
	PredictedUsage resource.Quantity `json:"predictedUsage"`
	
	// Confidence level (0.0 to 1.0)
	Confidence float64 `json:"confidence"`
	
	// Anomaly detection status
	AnomalyDetected bool `json:"anomalyDetected"`
}

// CollectionStats tracks aggregator performance statistics
type CollectionStats struct {
	// Total collections performed
	TotalCollections int64 `json:"totalCollections"`
	
	// Failed collections
	FailedCollections int64 `json:"failedCollections"`
	
	// Average collection time
	AverageCollectionTime time.Duration `json:"averageCollectionTime"`
	
	// Last collection time
	LastCollectionTime metav1.Time `json:"lastCollectionTime"`
	
	// Number of sync targets monitored
	SyncTargetsMonitored int `json:"syncTargetsMonitored"`
}

// ResourceCollector defines the interface for collecting resource metrics from SyncTargets
type ResourceCollector interface {
	// CollectMetrics collects resource usage from a specific SyncTarget
	CollectMetrics(ctx context.Context, syncTargetName string) (*SyncTargetMetrics, error)
	
	// ListSyncTargets returns available SyncTargets
	ListSyncTargets(ctx context.Context, workspace logicalcluster.Name) ([]string, error)
	
	// GetSyncTargetHealth checks health of a SyncTarget
	GetSyncTargetHealth(ctx context.Context, syncTargetName string) (bool, error)
}

// ResourceAggregator aggregates resource usage across multiple SyncTargets and clusters
type ResourceAggregator struct {
	// Configuration
	collectionInterval time.Duration
	trendWindow       time.Duration
	maxSyncTargets    int
	
	// Dependencies
	collector ResourceCollector
	
	// State management
	mutex sync.RWMutex
	
	// Current aggregated metrics
	currentMetrics *AggregatedMetrics
	
	// Historical data for trend analysis
	historicalData map[AggregatorResourceName][]TrendPoint
	
	// Per-workspace metrics
	workspaceMetrics map[logicalcluster.Name]*AggregatedMetrics
	
	// Performance statistics
	stats *CollectionStats
	
	// Event callbacks
	onMetricsUpdated  func(*AggregatedMetrics)
	onAnomalyDetected func(AggregatorResourceName, TrendAnalysis)
	onCollectionError func(string, error)
	
	// Control channels
	stopCh   chan struct{}
	updateCh chan struct{}
}

// NewResourceAggregator creates a new resource aggregator
func NewResourceAggregator(collector ResourceCollector, opts ...AggregatorOption) *ResourceAggregator {
	ra := &ResourceAggregator{
		collectionInterval: DefaultCollectionInterval,
		trendWindow:       DefaultTrendWindow,
		maxSyncTargets:    MaxSyncTargets,
		collector:         collector,
		historicalData:    make(map[AggregatorResourceName][]TrendPoint),
		workspaceMetrics:  make(map[logicalcluster.Name]*AggregatedMetrics),
		stats: &CollectionStats{
			LastCollectionTime: metav1.Now(),
		},
		stopCh:   make(chan struct{}),
		updateCh: make(chan struct{}, 1),
	}
	
	for _, opt := range opts {
		opt(ra)
	}
	
	return ra
}

// AggregatorOption configures a ResourceAggregator
type AggregatorOption func(*ResourceAggregator)

// WithCollectionInterval sets the collection interval
func WithCollectionInterval(interval time.Duration) AggregatorOption {
	return func(ra *ResourceAggregator) {
		ra.collectionInterval = interval
	}
}

// WithTrendWindow sets the trend analysis window
func WithTrendWindow(window time.Duration) AggregatorOption {
	return func(ra *ResourceAggregator) {
		ra.trendWindow = window
	}
}

// WithMetricsCallback sets a callback for metrics updates
func WithMetricsCallback(callback func(*AggregatedMetrics)) AggregatorOption {
	return func(ra *ResourceAggregator) {
		ra.onMetricsUpdated = callback
	}
}

// WithAnomalyCallback sets a callback for anomaly detection
func WithAnomalyCallback(callback func(AggregatorResourceName, TrendAnalysis)) AggregatorOption {
	return func(ra *ResourceAggregator) {
		ra.onAnomalyDetected = callback
	}
}

// Start begins the resource aggregation process
func (ra *ResourceAggregator) Start(ctx context.Context) error {
	klog.Info("Starting resource aggregator")
	
	// Start collection loop
	go wait.UntilWithContext(ctx, ra.collectMetrics, ra.collectionInterval)
	
	// Start trend analysis loop
	go wait.UntilWithContext(ctx, ra.analyzeTrends, time.Minute)
	
	// Start cleanup loop
	go wait.UntilWithContext(ctx, ra.cleanupHistoricalData, time.Hour)
	
	// Handle manual update requests
	go ra.handleUpdateRequests(ctx)
	
	<-ctx.Done()
	close(ra.stopCh)
	klog.Info("Resource aggregator stopped")
	return nil
}

// GetAggregatedMetrics returns the current aggregated metrics
func (ra *ResourceAggregator) GetAggregatedMetrics() *AggregatedMetrics {
	ra.mutex.RLock()
	defer ra.mutex.RUnlock()
	
	if ra.currentMetrics == nil {
		return &AggregatedMetrics{
			TotalUsage:     make(map[AggregatorResourceName]resource.Quantity),
			TotalCapacity:  make(map[AggregatorResourceName]resource.Quantity),
			TotalAvailable: make(map[AggregatorResourceName]resource.Quantity),
			Utilization:    make(map[AggregatorResourceName]float64),
			AggregatedAt:   metav1.Now(),
		}
	}
	
	// Return a deep copy
	result := *ra.currentMetrics
	return &result
}

// GetWorkspaceMetrics returns aggregated metrics for a specific workspace
func (ra *ResourceAggregator) GetWorkspaceMetrics(workspace logicalcluster.Name) *AggregatedMetrics {
	ra.mutex.RLock()
	defer ra.mutex.RUnlock()
	
	metrics, exists := ra.workspaceMetrics[workspace]
	if !exists {
		return &AggregatedMetrics{
			TotalUsage:     make(map[AggregatorResourceName]resource.Quantity),
			TotalCapacity:  make(map[AggregatorResourceName]resource.Quantity),
			TotalAvailable: make(map[AggregatorResourceName]resource.Quantity),
			Utilization:    make(map[AggregatorResourceName]float64),
			AggregatedAt:   metav1.Now(),
		}
	}
	
	result := *metrics
	return &result
}

// GetTrendAnalysis returns trend analysis for a specific resource
func (ra *ResourceAggregator) GetTrendAnalysis(resource AggregatorResourceName) *TrendAnalysis {
	ra.mutex.RLock()
	defer ra.mutex.RUnlock()
	
	dataPoints, exists := ra.historicalData[resource]
	if !exists || len(dataPoints) < 2 {
		return &TrendAnalysis{
			Resource:    resource,
			DataPoints:  []TrendPoint{},
			Confidence:  0.0,
		}
	}
	
	return ra.calculateTrend(resource, dataPoints)
}

// GetCollectionStats returns performance statistics
func (ra *ResourceAggregator) GetCollectionStats() *CollectionStats {
	ra.mutex.RLock()
	defer ra.mutex.RUnlock()
	
	statsCopy := *ra.stats
	return &statsCopy
}

// TriggerCollection manually triggers a metrics collection
func (ra *ResourceAggregator) TriggerCollection() {
	select {
	case ra.updateCh <- struct{}{}:
	default:
		// Channel is full, collection already pending
	}
}

// collectMetrics collects and aggregates metrics from all SyncTargets
func (ra *ResourceAggregator) collectMetrics(ctx context.Context) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		ra.updateCollectionStats(duration, nil)
	}()
	
	klog.V(4).Info("Starting metrics collection cycle")
	
	// Get all workspaces and their SyncTargets
	workspaceMetrics := make(map[logicalcluster.Name][]*SyncTargetMetrics)
	
	// For this implementation, we'll simulate collecting from a root workspace
	// In a real implementation, this would iterate through all workspaces
	rootWorkspace := logicalcluster.Name("root")
	
	syncTargets, err := ra.collector.ListSyncTargets(ctx, rootWorkspace)
	if err != nil {
		klog.Errorf("Failed to list sync targets: %v", err)
		ra.updateCollectionStats(time.Since(start), err)
		return
	}
	
	if len(syncTargets) > ra.maxSyncTargets {
		syncTargets = syncTargets[:ra.maxSyncTargets]
		klog.Warningf("Limiting collection to %d sync targets", ra.maxSyncTargets)
	}
	
	var allMetrics []*SyncTargetMetrics
	var wg sync.WaitGroup
	metricsCh := make(chan *SyncTargetMetrics, len(syncTargets))
	
	// Collect metrics from each SyncTarget concurrently
	for _, syncTargetName := range syncTargets {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			
			metrics, err := ra.collector.CollectMetrics(ctx, name)
			if err != nil {
				klog.Errorf("Failed to collect metrics from sync target %s: %v", name, err)
				if ra.onCollectionError != nil {
					ra.onCollectionError(name, err)
				}
				return
			}
			
			metricsCh <- metrics
		}(syncTargetName)
	}
	
	wg.Wait()
	close(metricsCh)
	
	// Collect results
	for metrics := range metricsCh {
		allMetrics = append(allMetrics, metrics)
	}
	
	workspaceMetrics[rootWorkspace] = allMetrics
	
	// Aggregate metrics
	globalMetrics := ra.aggregateMetrics(allMetrics)
	
	// Update state
	ra.mutex.Lock()
	ra.currentMetrics = globalMetrics
	ra.workspaceMetrics[rootWorkspace] = globalMetrics
	ra.mutex.Unlock()
	
	// Update historical data
	ra.updateHistoricalData(globalMetrics)
	
	// Trigger callbacks
	if ra.onMetricsUpdated != nil {
		ra.onMetricsUpdated(globalMetrics)
	}
	
	klog.V(4).Infof("Completed metrics collection: %d sync targets, %d healthy clusters", 
		len(allMetrics), globalMetrics.HealthyClusters)
}

// aggregateMetrics combines metrics from multiple SyncTargets
func (ra *ResourceAggregator) aggregateMetrics(syncTargetMetrics []*SyncTargetMetrics) *AggregatedMetrics {
	totalUsage := make(map[AggregatorResourceName]resource.Quantity)
	totalCapacity := make(map[AggregatorResourceName]resource.Quantity)
	healthyClusters := 0
	
	// Initialize with zero values for all known resource types
	for _, resName := range []AggregatorResourceName{
		AggregatorResourceCPU, AggregatorResourceMemory, AggregatorResourceStorage, AggregatorResourceEphemeralStorage,
		AggregatorResourcePods, AggregatorResourceServices, AggregatorResourceDeployments,
	} {
		totalUsage[resName] = resource.Quantity{}
		totalCapacity[resName] = resource.Quantity{}
	}
	
	// Aggregate across all sync targets
	for _, metrics := range syncTargetMetrics {
		if metrics.Healthy {
			healthyClusters++
		}
		
		for resName, usage := range metrics.Usage {
			current := totalUsage[resName]
			current.Add(usage)
			totalUsage[resName] = current
		}
		
		for resName, capacity := range metrics.Capacity {
			current := totalCapacity[resName]
			current.Add(capacity)
			totalCapacity[resName] = current
		}
	}
	
	// Calculate available resources and utilization
	totalAvailable := make(map[AggregatorResourceName]resource.Quantity)
	utilization := make(map[AggregatorResourceName]float64)
	
	for resName, capacity := range totalCapacity {
		usage := totalUsage[resName]
		
		available := capacity.DeepCopy()
		available.Sub(usage)
		totalAvailable[resName] = available
		
		if !capacity.IsZero() {
			util := float64(usage.MilliValue()) / float64(capacity.MilliValue()) * 100
			utilization[resName] = math.Min(util, 100.0)
		} else {
			utilization[resName] = 0.0
		}
	}
	
	// Create cluster metrics slice for detailed view
	var clusterMetrics []SyncTargetMetrics
	for _, metrics := range syncTargetMetrics {
		clusterMetrics = append(clusterMetrics, *metrics)
	}
	
	return &AggregatedMetrics{
		TotalUsage:      totalUsage,
		TotalCapacity:   totalCapacity,
		TotalAvailable:  totalAvailable,
		Utilization:     utilization,
		HealthyClusters: healthyClusters,
		TotalClusters:   len(syncTargetMetrics),
		AggregatedAt:    metav1.Now(),
		ClusterMetrics:  clusterMetrics,
	}
}

// updateHistoricalData adds current metrics to historical trend data
func (ra *ResourceAggregator) updateHistoricalData(metrics *AggregatedMetrics) {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()
	
	timestamp := metav1.Now()
	
	for resName, usage := range metrics.TotalUsage {
		dataPoint := TrendPoint{
			Timestamp: timestamp,
			Usage:     map[AggregatorResourceName]resource.Quantity{resName: usage},
		}
		
		ra.historicalData[resName] = append(ra.historicalData[resName], dataPoint)
	}
}

// analyzeTrends performs trend analysis on historical data
func (ra *ResourceAggregator) analyzeTrends(ctx context.Context) {
	ra.mutex.RLock()
	historicalDataCopy := make(map[AggregatorResourceName][]TrendPoint)
	for resName, points := range ra.historicalData {
		historicalDataCopy[resName] = append([]TrendPoint{}, points...)
	}
	ra.mutex.RUnlock()
	
	for resName, dataPoints := range historicalDataCopy {
		if len(dataPoints) < 5 {
			continue // Need at least 5 points for meaningful analysis
		}
		
		trend := ra.calculateTrend(resName, dataPoints)
		
		// Check for anomalies
		if trend.AnomalyDetected && ra.onAnomalyDetected != nil {
			ra.onAnomalyDetected(resName, *trend)
		}
	}
}

// calculateTrend performs statistical analysis on trend data
func (ra *ResourceAggregator) calculateTrend(resourceName AggregatorResourceName, dataPoints []TrendPoint) *TrendAnalysis {
	if len(dataPoints) < 2 {
		return &TrendAnalysis{
			Resource:   resourceName,
			DataPoints: dataPoints,
			Confidence: 0.0,
		}
	}
	
	// Sort data points by timestamp
	sort.Slice(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(&dataPoints[j].Timestamp)
	})
	
	// Simple linear regression for trend direction
	n := float64(len(dataPoints))
	var sumX, sumY, sumXY, sumXX float64
	
	for i, point := range dataPoints {
		x := float64(i) // Time index
		quantity := point.Usage[resourceName]
		y := float64(quantity.MilliValue())
		
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	
	// Calculate slope (trend direction)
	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	if math.IsNaN(slope) {
		slope = 0
	}
	
	// Calculate predicted next value
	nextX := n
	intercept := (sumY - slope*sumX) / n
	predictedMilliValue := int64(slope*nextX + intercept)
	predictedUsage := resource.NewMilliQuantity(predictedMilliValue, resource.DecimalSI)
	
	// Simple anomaly detection based on recent deviation
	anomaly := false
	if len(dataPoints) >= 3 {
		recent := dataPoints[len(dataPoints)-1]
		previous := dataPoints[len(dataPoints)-2]
		
		recentQuantity := recent.Usage[resourceName]
		previousQuantity := previous.Usage[resourceName]
		recentValue := float64(recentQuantity.MilliValue())
		previousValue := float64(previousQuantity.MilliValue())
		
		// Detect sudden spikes (> 50% change)
		if previousValue > 0 {
			changeRatio := math.Abs(recentValue-previousValue) / previousValue
			anomaly = changeRatio > 0.5
		}
	}
	
	// Confidence based on data consistency
	confidence := math.Min(float64(len(dataPoints))/10.0, 1.0)
	
	return &TrendAnalysis{
		Resource:        resourceName,
		DataPoints:      dataPoints,
		TrendDirection:  slope,
		PredictedUsage:  *predictedUsage,
		Confidence:      confidence,
		AnomalyDetected: anomaly,
	}
}

// cleanupHistoricalData removes old data points beyond retention period
func (ra *ResourceAggregator) cleanupHistoricalData(ctx context.Context) {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()
	
	cutoff := time.Now().Add(-DataRetentionPeriod)
	cutoffTime := metav1.NewTime(cutoff)
	
	for resName, dataPoints := range ra.historicalData {
		var filteredPoints []TrendPoint
		for _, point := range dataPoints {
			if point.Timestamp.After(cutoffTime.Time) {
				filteredPoints = append(filteredPoints, point)
			}
		}
		ra.historicalData[resName] = filteredPoints
	}
}

// handleUpdateRequests handles manual collection triggers
func (ra *ResourceAggregator) handleUpdateRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ra.updateCh:
			ra.collectMetrics(ctx)
		}
	}
}

// updateCollectionStats updates performance statistics
func (ra *ResourceAggregator) updateCollectionStats(duration time.Duration, err error) {
	ra.mutex.Lock()
	defer ra.mutex.Unlock()
	
	ra.stats.TotalCollections++
	ra.stats.LastCollectionTime = metav1.Now()
	
	if err != nil {
		ra.stats.FailedCollections++
	}
	
	// Update rolling average
	if ra.stats.AverageCollectionTime == 0 {
		ra.stats.AverageCollectionTime = duration
	} else {
		alpha := 0.1 // Smoothing factor
		ra.stats.AverageCollectionTime = time.Duration(
			float64(ra.stats.AverageCollectionTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
}

// mockResourceCollector implements ResourceCollector for testing/demo purposes
type mockResourceCollector struct{}

// NewMockResourceCollector creates a mock collector for testing
func NewMockResourceCollector() ResourceCollector {
	return &mockResourceCollector{}
}

func (m *mockResourceCollector) CollectMetrics(ctx context.Context, syncTargetName string) (*SyncTargetMetrics, error) {
	// Simulate metrics collection
	return &SyncTargetMetrics{
		SyncTargetName: syncTargetName,
		ClusterName:    fmt.Sprintf("cluster-%s", syncTargetName),
		Usage: map[AggregatorResourceName]resource.Quantity{
			AggregatorResourceCPU:    resource.MustParse("2"),
			AggregatorResourceMemory: resource.MustParse("4Gi"),
		},
		Capacity: map[AggregatorResourceName]resource.Quantity{
			AggregatorResourceCPU:    resource.MustParse("8"),
			AggregatorResourceMemory: resource.MustParse("16Gi"),
		},
		Healthy:     true,
		LastUpdated: metav1.Now(),
	}, nil
}

func (m *mockResourceCollector) ListSyncTargets(ctx context.Context, workspace logicalcluster.Name) ([]string, error) {
	return []string{"sync-target-1", "sync-target-2"}, nil
}

func (m *mockResourceCollector) GetSyncTargetHealth(ctx context.Context, syncTargetName string) (bool, error) {
	return true, nil
}