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

package metrics

import (
	"sync"
	
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

var (
	upstreamSyncTargets = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kcp_upstream_sync_targets_total",
			Help: "Number of active upstream sync targets",
		},
		[]string{"workspace"},
	)
	
	upstreamResourcesSynced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kcp_upstream_resources_synced_total",
			Help: "Total number of resources synced from upstream",
		},
		[]string{"workspace", "cluster", "resource"},
	)
	
	upstreamConflictsResolved = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kcp_upstream_conflicts_resolved_total",
			Help: "Total number of conflicts resolved",
		},
		[]string{"workspace", "strategy"},
	)
	
	upstreamSyncLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kcp_upstream_sync_latency_seconds",
			Help:    "Latency of upstream sync operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"workspace", "operation"},
	)
	
	registerOnce sync.Once
)

// Register registers all upstream sync metrics
func Register() {
	registerOnce.Do(func() {
		legacyregistry.MustRegister(upstreamSyncTargets)
		legacyregistry.MustRegister(upstreamResourcesSynced)
		legacyregistry.MustRegister(upstreamConflictsResolved)
		legacyregistry.MustRegister(upstreamSyncLatency)
	})
}

// RecordSyncTargets records the number of active sync targets
func RecordSyncTargets(workspace string, count float64) {
	upstreamSyncTargets.WithLabelValues(workspace).Set(count)
}

// RecordResourcesSynced increments the resources synced counter
func RecordResourcesSynced(workspace, cluster, resource string) {
	upstreamResourcesSynced.WithLabelValues(workspace, cluster, resource).Inc()
}

// RecordConflictResolved increments the conflicts resolved counter
func RecordConflictResolved(workspace, strategy string) {
	upstreamConflictsResolved.WithLabelValues(workspace, strategy).Inc()
}

// RecordSyncLatency records sync operation latency
func RecordSyncLatency(workspace, operation string, seconds float64) {
	upstreamSyncLatency.WithLabelValues(workspace, operation).Observe(seconds)
}