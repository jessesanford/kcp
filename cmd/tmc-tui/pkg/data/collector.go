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

package data

import (
	"context"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
)

// ClusterData represents cluster status information for the TUI.
type ClusterData struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	HealthStatus string    `json:"health_status"`
	LastSeen     time.Time `json:"last_seen"`
	NodeCount    int       `json:"node_count"`
	PodCount     int       `json:"pod_count"`
	Location     string    `json:"location"`
	Version      string    `json:"version"`
}

// SyncerData represents syncer status information for the TUI.
type SyncerData struct {
	Name         string        `json:"name"`
	Status       string        `json:"status"`
	QueueDepth   int           `json:"queue_depth"`
	ErrorRate    float64       `json:"error_rate"`
	LastSync     time.Time     `json:"last_sync"`
	SyncLatency  time.Duration `json:"sync_latency"`
	TotalSyncs   int64         `json:"total_syncs"`
	TotalErrors  int64         `json:"total_errors"`
	Connected    bool          `json:"connected"`
	TargetCluster string       `json:"target_cluster"`
}

// MetricsData represents aggregated metrics for the TUI.
type MetricsData struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalClusters    int       `json:"total_clusters"`
	HealthyClusters  int       `json:"healthy_clusters"`
	TotalSyncers     int       `json:"total_syncers"`
	ActiveSyncers    int       `json:"active_syncers"`
	AverageLatency   time.Duration `json:"average_latency"`
	ErrorRate        float64   `json:"error_rate"`
	ResourcesSynced  int64     `json:"resources_synced"`
	LastUpdateTime   time.Time `json:"last_update_time"`
}

// DashboardData represents the main dashboard overview data.
type DashboardData struct {
	SystemStatus     string        `json:"system_status"`
	Uptime          time.Duration `json:"uptime"`
	Clusters        []ClusterData `json:"clusters"`
	Syncers         []SyncerData  `json:"syncers"`
	Metrics         MetricsData   `json:"metrics"`
	RecentEvents    []EventData   `json:"recent_events"`
	LastRefresh     time.Time     `json:"last_refresh"`
}

// EventData represents system events for the dashboard.
type EventData struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Component   string    `json:"component"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
}

// Collector manages data collection from various TMC components.
type Collector struct {
	config *config.Config
	cache  *Cache
	mu     sync.RWMutex

	// Connection state
	connected bool
	startTime time.Time

	// Error tracking
	lastError error
	errorCount int
}

// NewCollector creates a new data collector with the provided configuration.
func NewCollector(cfg *config.Config) (*Collector, error) {
	return &Collector{
		config:    cfg,
		cache:     NewCache(cfg.RefreshRate * 2), // Cache TTL is 2x refresh rate
		startTime: time.Now(),
	}, nil
}

// Start initializes the data collector and establishes connections.
func (c *Collector) Start(ctx context.Context) error {
	klog.V(2).Info("Starting TMC data collector")

	// Initialize connections to metrics sources
	if err := c.initializeConnections(ctx); err != nil {
		c.lastError = err
		c.errorCount++
		klog.Errorf("Failed to initialize connections: %v", err)
		// Continue with mock data for development
	} else {
		c.connected = true
		c.lastError = nil
		c.errorCount = 0
	}

	// Perform initial data collection
	c.RefreshAll()

	klog.V(1).Infof("TMC data collector started (connected: %v)", c.connected)
	return nil
}

// Stop gracefully shuts down the data collector.
func (c *Collector) Stop() {
	klog.V(2).Info("Stopping TMC data collector")
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
}

// initializeConnections establishes connections to TMC metrics sources.
func (c *Collector) initializeConnections(ctx context.Context) error {
	// In a real implementation, this would connect to:
	// - KCP metrics registry
	// - Kubernetes clusters
	// - Health monitoring endpoints
	// - Prometheus/metrics endpoints
	
	// For now, we'll simulate connection establishment
	klog.V(3).Info("Initializing connections to TMC metrics sources...")
	
	// Simulate connection delay
	time.Sleep(100 * time.Millisecond)
	
	// For development, return success to enable mock data
	return nil
}

// RefreshAll triggers a refresh of all data types.
func (c *Collector) RefreshAll() {
	klog.V(3).Info("Refreshing all data")
	
	go func() {
		c.refreshClusters()
		c.refreshSyncers()
		c.refreshMetrics()
		c.refreshEvents()
	}()
}

// refreshClusters updates cluster status data.
func (c *Collector) refreshClusters() {
	klog.V(4).Info("Refreshing cluster data")

	// Generate mock cluster data for development
	clusters := c.generateMockClusterData()
	c.cache.Set("clusters", clusters)
}

// refreshSyncers updates syncer status data.
func (c *Collector) refreshSyncers() {
	klog.V(4).Info("Refreshing syncer data")

	// Generate mock syncer data for development
	syncers := c.generateMockSyncerData()
	c.cache.Set("syncers", syncers)
}

// refreshMetrics updates aggregated metrics data.
func (c *Collector) refreshMetrics() {
	klog.V(4).Info("Refreshing metrics data")

	// Generate mock metrics data for development
	metrics := c.generateMockMetricsData()
	c.cache.Set("metrics", metrics)
}

// refreshEvents updates recent events data.
func (c *Collector) refreshEvents() {
	klog.V(4).Info("Refreshing events data")

	// Generate mock events data for development
	events := c.generateMockEventsData()
	c.cache.Set("events", events)
}

// GetClusters returns current cluster data.
func (c *Collector) GetClusters() []ClusterData {
	if data, found := c.cache.Get("clusters"); found {
		if clusters, ok := data.([]ClusterData); ok {
			return clusters
		}
	}
	return []ClusterData{}
}

// GetSyncers returns current syncer data.
func (c *Collector) GetSyncers() []SyncerData {
	if data, found := c.cache.Get("syncers"); found {
		if syncers, ok := data.([]SyncerData); ok {
			return syncers
		}
	}
	return []SyncerData{}
}

// GetMetrics returns current metrics data.
func (c *Collector) GetMetrics() MetricsData {
	if data, found := c.cache.Get("metrics"); found {
		if metrics, ok := data.(MetricsData); ok {
			return metrics
		}
	}
	return MetricsData{}
}

// GetEvents returns recent events data.
func (c *Collector) GetEvents() []EventData {
	if data, found := c.cache.Get("events"); found {
		if events, ok := data.([]EventData); ok {
			return events
		}
	}
	return []EventData{}
}

// GetDashboardData returns aggregated dashboard data.
func (c *Collector) GetDashboardData() DashboardData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := "Healthy"
	if !c.connected || c.errorCount > 0 {
		status = "Degraded"
	}

	return DashboardData{
		SystemStatus: status,
		Uptime:       time.Since(c.startTime),
		Clusters:     c.GetClusters(),
		Syncers:      c.GetSyncers(),
		Metrics:      c.GetMetrics(),
		RecentEvents: c.GetEvents(),
		LastRefresh:  time.Now(),
	}
}

// Mock data generators for development
func (c *Collector) generateMockClusterData() []ClusterData {
	now := time.Now()
	return []ClusterData{
		{
			Name:         "us-west-2-cluster",
			Status:       "Ready",
			HealthStatus: "Healthy",
			LastSeen:     now.Add(-30 * time.Second),
			NodeCount:    5,
			PodCount:     42,
			Location:     "us-west-2",
			Version:      "v1.28.3",
		},
		{
			Name:         "eu-central-1-cluster",
			Status:       "Ready",
			HealthStatus: "Healthy",
			LastSeen:     now.Add(-45 * time.Second),
			NodeCount:    3,
			PodCount:     28,
			Location:     "eu-central-1",
			Version:      "v1.28.3",
		},
		{
			Name:         "ap-southeast-1-cluster",
			Status:       "NotReady",
			HealthStatus: "Unhealthy",
			LastSeen:     now.Add(-5 * time.Minute),
			NodeCount:    4,
			PodCount:     35,
			Location:     "ap-southeast-1",
			Version:      "v1.28.2",
		},
	}
}

func (c *Collector) generateMockSyncerData() []SyncerData {
	now := time.Now()
	return []SyncerData{
		{
			Name:          "us-west-2-syncer",
			Status:        "Active",
			QueueDepth:    5,
			ErrorRate:     0.1,
			LastSync:      now.Add(-10 * time.Second),
			SyncLatency:   150 * time.Millisecond,
			TotalSyncs:    1250,
			TotalErrors:   3,
			Connected:     true,
			TargetCluster: "us-west-2-cluster",
		},
		{
			Name:          "eu-central-1-syncer",
			Status:        "Active",
			QueueDepth:    12,
			ErrorRate:     2.3,
			LastSync:      now.Add(-25 * time.Second),
			SyncLatency:   280 * time.Millisecond,
			TotalSyncs:    980,
			TotalErrors:   12,
			Connected:     true,
			TargetCluster: "eu-central-1-cluster",
		},
		{
			Name:          "ap-southeast-1-syncer",
			Status:        "Error",
			QueueDepth:    45,
			ErrorRate:     15.7,
			LastSync:      now.Add(-3 * time.Minute),
			SyncLatency:   850 * time.Millisecond,
			TotalSyncs:    750,
			TotalErrors:   67,
			Connected:     false,
			TargetCluster: "ap-southeast-1-cluster",
		},
	}
}

func (c *Collector) generateMockMetricsData() MetricsData {
	return MetricsData{
		Timestamp:       time.Now(),
		TotalClusters:   3,
		HealthyClusters: 2,
		TotalSyncers:    3,
		ActiveSyncers:   2,
		AverageLatency:  215 * time.Millisecond,
		ErrorRate:       6.0,
		ResourcesSynced: 2980,
		LastUpdateTime:  time.Now(),
	}
}

func (c *Collector) generateMockEventsData() []EventData {
	now := time.Now()
	return []EventData{
		{
			Timestamp: now.Add(-2 * time.Minute),
			Type:      "Warning",
			Component: "syncer",
			Message:   "ap-southeast-1-syncer connection lost",
			Severity:  "High",
		},
		{
			Timestamp: now.Add(-5 * time.Minute),
			Type:      "Info",
			Component: "cluster",
			Message:   "us-west-2-cluster node added",
			Severity:  "Low",
		},
		{
			Timestamp: now.Add(-8 * time.Minute),
			Type:      "Error",
			Component: "syncer",
			Message:   "High error rate detected in eu-central-1-syncer",
			Severity:  "Medium",
		},
	}
}