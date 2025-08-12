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

package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
)

const (
	// CollectorName is the name of this collector component
	CollectorName = "placement-analysis-collector"

	// DefaultCollectionInterval is the default interval for collecting placement data
	DefaultCollectionInterval = 30 * time.Second

	// MaxDataPoints is the maximum number of data points to keep in memory
	MaxDataPoints = 1000
)

// PlacementData represents collected placement analysis data
type PlacementData struct {
	// Timestamp when this data was collected
	Timestamp time.Time `json:"timestamp"`

	// ClusterName is the logical cluster where the placement occurred
	ClusterName logicalcluster.Name `json:"clusterName"`

	// WorkspaceName is the workspace containing the placement
	WorkspaceName string `json:"workspaceName"`

	// PlacementName is the name of the placement being analyzed
	PlacementName string `json:"placementName"`

	// PlacementNamespace is the namespace of the placement
	PlacementNamespace string `json:"placementNamespace"`

	// ResourceCount is the number of resources managed by this placement
	ResourceCount int64 `json:"resourceCount"`

	// TargetClusters are the clusters where resources are placed
	TargetClusters []string `json:"targetClusters"`

	// HealthStatus represents the overall health of the placement
	HealthStatus string `json:"healthStatus"`

	// LastPlacementTime is when the placement was last updated
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`

	// ResourceUtilization contains resource usage metrics
	ResourceUtilization ResourceUtilization `json:"resourceUtilization"`
}

// ResourceUtilization represents resource usage metrics
type ResourceUtilization struct {
	// CPURequest is the total CPU requested
	CPURequest int64 `json:"cpuRequest"`

	// MemoryRequest is the total memory requested
	MemoryRequest int64 `json:"memoryRequest"`

	// StorageRequest is the total storage requested
	StorageRequest int64 `json:"storageRequest"`

	// PodCount is the number of pods managed
	PodCount int32 `json:"podCount"`
}

// CollectorOptions configures the placement data collector
type CollectorOptions struct {
	// CollectionInterval is how often to collect placement data
	CollectionInterval time.Duration

	// MaxDataPoints is the maximum number of data points to retain
	MaxDataPoints int

	// EnableMetrics controls whether metrics collection is enabled
	EnableMetrics bool

	// MetricsNamespace is the namespace for metrics
	MetricsNamespace string
}

// Collector collects and manages placement analysis data
type Collector struct {
	// logger is the structured logger for this collector
	logger logr.Logger

	// options contains the collector configuration
	options CollectorOptions

	// dataStore holds the collected placement data
	dataStore *DataStore

	// metricsCollector handles metrics collection
	metricsCollector *MetricsCollector

	// stopCh is used to signal shutdown
	stopCh chan struct{}

	// started indicates if the collector is running
	started bool

	// mutex protects concurrent access
	mutex sync.RWMutex
}

// DataStore manages the in-memory storage of placement data
type DataStore struct {
	// data holds the placement data points
	data []PlacementData

	// maxSize is the maximum number of data points to keep
	maxSize int

	// mutex protects concurrent access to data
	mutex sync.RWMutex
}

// NewCollector creates a new placement analysis data collector
func NewCollector(options CollectorOptions) (*Collector, error) {
	if options.CollectionInterval == 0 {
		options.CollectionInterval = DefaultCollectionInterval
	}
	if options.MaxDataPoints == 0 {
		options.MaxDataPoints = MaxDataPoints
	}
	if options.MetricsNamespace == "" {
		options.MetricsNamespace = "tmc_placement_analysis"
	}

	logger := klog.FromContext(context.TODO()).WithName(CollectorName)

	dataStore := &DataStore{
		data:    make([]PlacementData, 0, options.MaxDataPoints),
		maxSize: options.MaxDataPoints,
	}

	var metricsCollector *MetricsCollector
	if options.EnableMetrics {
		var err error
		metricsCollector, err = NewMetricsCollector(options.MetricsNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics collector: %w", err)
		}
	}

	return &Collector{
		logger:           logger,
		options:          options,
		dataStore:        dataStore,
		metricsCollector: metricsCollector,
		stopCh:           make(chan struct{}),
	}, nil
}

// CollectPlacementData collects placement data for a specific placement
func (c *Collector) CollectPlacementData(ctx context.Context, clusterName logicalcluster.Name, workspaceName, placementName, placementNamespace string) error {
	logger := logging.WithReconcileContext(c.logger, ctx)

	data := PlacementData{
		Timestamp:          time.Now(),
		ClusterName:        clusterName,
		WorkspaceName:      workspaceName,
		PlacementName:      placementName,
		PlacementNamespace: placementNamespace,
		HealthStatus:       "Unknown",
	}

	c.dataStore.Add(data)

	if c.metricsCollector != nil {
		c.metricsCollector.RecordPlacementData(data)
	}

	logger.V(4).Info("Collected placement data", "placement", placementName)
	return nil
}

// Add adds a data point to the data store
func (ds *DataStore) Add(data PlacementData) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.data = append(ds.data, data)
	if len(ds.data) > ds.maxSize {
		ds.data = ds.data[1:]
	}
}

// Size returns the current number of data points stored
func (ds *DataStore) Size() int {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return len(ds.data)
}

