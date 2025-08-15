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

package canary

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// DataPoint represents a single metric measurement at a point in time.
type DataPoint struct {
	// Timestamp when the measurement was taken.
	Timestamp time.Time `json:"timestamp"`
	// Value of the metric at this timestamp.
	Value float64 `json:"value"`
}

// TimeSeries represents a time series of metric data points.
type TimeSeries struct {
	// MetricName is the name of the metric.
	MetricName string `json:"metricName"`
	// DataPoints contains the time-ordered data points.
	DataPoints []DataPoint `json:"dataPoints"`
	// Unit specifies the unit of measurement.
	Unit string `json:"unit"`
}

// Metrics contains all collected metrics for canary analysis.
type Metrics struct {
	// RequestCount tracks the total number of requests.
	RequestCount TimeSeries `json:"requestCount"`
	// SuccessRate tracks the percentage of successful requests.
	SuccessRate TimeSeries `json:"successRate"`
	// ErrorRate tracks the percentage of failed requests.
	ErrorRate TimeSeries `json:"errorRate"`
	// Latency tracks response time measurements.
	Latency TimeSeries `json:"latency"`
	// CustomMetrics allows for additional application-specific metrics.
	CustomMetrics map[string]TimeSeries `json:"customMetrics,omitempty"`
}

// MetricsCollector provides metrics collection capabilities.
type MetricsCollector interface {
	// CollectMetrics gathers current metrics for the canary deployment.
	CollectMetrics(ctx context.Context, canary *CanaryDeployment) (*Metrics, error)
	
	// StartCollection begins continuous metrics collection.
	StartCollection(ctx context.Context, canary *CanaryDeployment, interval time.Duration) error
	
	// StopCollection stops continuous metrics collection.
	StopCollection(ctx context.Context, canary *CanaryDeployment) error
}

// DefaultMetricsCollector implements MetricsCollector using Prometheus-style metrics.
type DefaultMetricsCollector struct {
	// config holds collector configuration.
	config MetricsConfig
}

// MetricsConfig contains configuration for metrics collection.
type MetricsConfig struct {
	// CollectionInterval defines how often to collect metrics.
	CollectionInterval time.Duration
	// RetentionDuration defines how long to keep metrics.
	RetentionDuration time.Duration
	// MaxDataPoints defines the maximum number of data points per metric.
	MaxDataPoints int
}

// NewDefaultMetricsCollector creates a new default metrics collector.
func NewDefaultMetricsCollector() MetricsCollector {
	return &DefaultMetricsCollector{
		config: MetricsConfig{
			CollectionInterval: time.Minute,
			RetentionDuration:  time.Hour * 24,
			MaxDataPoints:      1440, // 24 hours of minute-level data
		},
	}
}

// CollectMetrics implements MetricsCollector.CollectMetrics.
func (c *DefaultMetricsCollector) CollectMetrics(ctx context.Context, canary *CanaryDeployment) (*Metrics, error) {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	logger.V(2).Info("Collecting canary metrics")

	now := time.Now()
	
	// In a real implementation, this would query actual metrics backends
	// (Prometheus, CloudWatch, DataDog, etc.)
	metrics := &Metrics{
		RequestCount: TimeSeries{
			MetricName: "request_count",
			Unit:       "requests",
			DataPoints: c.generateRequestCountData(now),
		},
		SuccessRate: TimeSeries{
			MetricName: "success_rate",
			Unit:       "percentage",
			DataPoints: c.generateSuccessRateData(now, canary),
		},
		ErrorRate: TimeSeries{
			MetricName: "error_rate", 
			Unit:       "percentage",
			DataPoints: c.generateErrorRateData(now, canary),
		},
		Latency: TimeSeries{
			MetricName: "response_time_p95",
			Unit:       "milliseconds",
			DataPoints: c.generateLatencyData(now),
		},
		CustomMetrics: make(map[string]TimeSeries),
	}

	// Collect any custom metrics defined for this canary
	if err := c.collectCustomMetrics(ctx, canary, metrics); err != nil {
		logger.V(1).Info("Warning: failed to collect custom metrics", "error", err)
		// Continue with basic metrics even if custom metrics fail
	}

	logger.V(2).Info("Metrics collection completed", 
		"requestCount", len(metrics.RequestCount.DataPoints),
		"successRate", len(metrics.SuccessRate.DataPoints),
		"errorRate", len(metrics.ErrorRate.DataPoints),
		"latency", len(metrics.Latency.DataPoints),
	)

	return metrics, nil
}

// StartCollection implements MetricsCollector.StartCollection.
func (c *DefaultMetricsCollector) StartCollection(ctx context.Context, canary *CanaryDeployment, interval time.Duration) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	logger.V(2).Info("Starting continuous metrics collection", "interval", interval)
	
	// In a real implementation, this would start a background goroutine
	// to continuously collect metrics at the specified interval
	
	return nil
}

// StopCollection implements MetricsCollector.StopCollection.
func (c *DefaultMetricsCollector) StopCollection(ctx context.Context, canary *CanaryDeployment) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	logger.V(2).Info("Stopping metrics collection")
	
	// In a real implementation, this would stop the background collection goroutine
	
	return nil
}

// generateRequestCountData generates sample request count data.
func (c *DefaultMetricsCollector) generateRequestCountData(baseTime time.Time) []DataPoint {
	var points []DataPoint
	for i := 0; i < 15; i++ {
		points = append(points, DataPoint{
			Timestamp: baseTime.Add(-time.Duration(i) * time.Minute),
			Value:     float64(100 + i*10), // Simulated increasing request count
		})
	}
	return points
}

// generateSuccessRateData generates sample success rate data.
func (c *DefaultMetricsCollector) generateSuccessRateData(baseTime time.Time, canary *CanaryDeployment) []DataPoint {
	var points []DataPoint
	baseRate := 0.95 // 95% success rate baseline
	
	for i := 0; i < 15; i++ {
		// Add some variance around the baseline
		variance := (float64(i%3) - 1) * 0.01 // +/- 1% variance
		value := baseRate + variance
		if value > 1.0 {
			value = 1.0
		}
		if value < 0.0 {
			value = 0.0
		}
		
		points = append(points, DataPoint{
			Timestamp: baseTime.Add(-time.Duration(i) * time.Minute),
			Value:     value,
		})
	}
	return points
}

// generateErrorRateData generates sample error rate data.
func (c *DefaultMetricsCollector) generateErrorRateData(baseTime time.Time, canary *CanaryDeployment) []DataPoint {
	var points []DataPoint
	baseRate := 0.02 // 2% error rate baseline
	
	for i := 0; i < 15; i++ {
		// Add some variance around the baseline
		variance := (float64(i%3) - 1) * 0.005 // +/- 0.5% variance
		value := baseRate + variance
		if value > 1.0 {
			value = 1.0
		}
		if value < 0.0 {
			value = 0.0
		}
		
		points = append(points, DataPoint{
			Timestamp: baseTime.Add(-time.Duration(i) * time.Minute),
			Value:     value,
		})
	}
	return points
}

// generateLatencyData generates sample latency data.
func (c *DefaultMetricsCollector) generateLatencyData(baseTime time.Time) []DataPoint {
	var points []DataPoint
	baseLatency := 250.0 // 250ms baseline
	
	for i := 0; i < 15; i++ {
		// Add some variance around the baseline
		variance := (float64(i%5) - 2) * 25 // +/- 50ms variance
		value := baseLatency + variance
		if value < 0 {
			value = 0
		}
		
		points = append(points, DataPoint{
			Timestamp: baseTime.Add(-time.Duration(i) * time.Minute),
			Value:     value,
		})
	}
	return points
}

// collectCustomMetrics collects any custom metrics defined for the canary.
func (c *DefaultMetricsCollector) collectCustomMetrics(ctx context.Context, canary *CanaryDeployment, metrics *Metrics) error {
	logger := klog.FromContext(ctx)
	
	// In a real implementation, this would:
	// 1. Read custom metric definitions from canary spec or annotations
	// 2. Query the appropriate metrics backends for these custom metrics
	// 3. Add the results to metrics.CustomMetrics
	
	// For now, we'll add a sample custom metric
	metrics.CustomMetrics["cpu_utilization"] = TimeSeries{
		MetricName: "cpu_utilization",
		Unit:       "percentage",
		DataPoints: []DataPoint{
			{
				Timestamp: time.Now(),
				Value:     45.0, // 45% CPU utilization
			},
		},
	}
	
	logger.V(3).Info("Custom metrics collection completed")
	return nil
}