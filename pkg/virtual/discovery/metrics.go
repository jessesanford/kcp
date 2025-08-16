/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

var (
	// discoveryRequestsTotal counts total discovery requests
	discoveryRequestsTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_discovery_requests_total",
			Help: "Total number of discovery requests handled",
		},
		[]string{"workspace", "result"},
	)

	// discoveryRequestDuration measures discovery request duration
	discoveryRequestDuration = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Name:    "kcp_virtual_discovery_request_duration_seconds",
			Help:    "Duration of discovery requests in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"workspace", "operation"},
	)

	// discoveryCacheHits counts cache hits/misses
	discoveryCacheHits = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_discovery_cache_hits_total",
			Help: "Total number of discovery cache hits",
		},
		[]string{"workspace", "hit_type"},
	)

	// discoveryWatchersActive tracks active watchers
	discoveryWatchersActive = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Name: "kcp_virtual_discovery_watchers_active",
			Help: "Number of active discovery watchers",
		},
		[]string{"workspace"},
	)

	// discoveryConversionsTotal counts resource conversions
	discoveryConversionsTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_discovery_conversions_total",
			Help: "Total number of resource conversions performed",
		},
		[]string{"source_version", "target_version", "result"},
	)

	// discoveryCacheSize tracks cache size
	discoveryCacheSize = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Name: "kcp_virtual_discovery_cache_entries",
			Help: "Number of entries in the discovery cache",
		},
		[]string{"workspace"},
	)
)

// init registers metrics
func init() {
	legacyregistry.MustRegister(
		discoveryRequestsTotal,
		discoveryRequestDuration,
		discoveryCacheHits,
		discoveryWatchersActive,
		discoveryConversionsTotal,
		discoveryCacheSize,
	)
}

// RecordDiscoveryRequest records metrics for a discovery request
func RecordDiscoveryRequest(workspace, operation string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}

	discoveryRequestsTotal.WithLabelValues(workspace, result).Inc()
	discoveryRequestDuration.WithLabelValues(workspace, operation).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit or miss
func RecordCacheHit(workspace string, hit bool) {
	hitType := "miss"
	if hit {
		hitType = "hit"
	}
	
	discoveryCacheHits.WithLabelValues(workspace, hitType).Inc()
}

// UpdateActiveWatchers updates the active watcher count
func UpdateActiveWatchers(workspace string, delta int) {
	discoveryWatchersActive.WithLabelValues(workspace).Add(float64(delta))
}

// RecordConversion records a resource conversion attempt
func RecordConversion(sourceVersion, targetVersion string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	
	discoveryConversionsTotal.WithLabelValues(sourceVersion, targetVersion, result).Inc()
}

// UpdateCacheSize updates the cache size metric for a workspace
func UpdateCacheSize(workspace string, size int) {
	discoveryCacheSize.WithLabelValues(workspace).Set(float64(size))
}