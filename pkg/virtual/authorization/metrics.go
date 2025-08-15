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

package authorization

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	// authorizationRequestsTotal counts total authorization requests
	authorizationRequestsTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_authorization_requests_total",
			Help: "Total number of authorization requests handled",
		},
		[]string{"workspace", "user", "result"},
	)

	// authorizationRequestDuration measures authorization request duration
	authorizationRequestDuration = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Name:    "kcp_virtual_authorization_request_duration_seconds",
			Help:    "Duration of authorization requests in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"workspace", "operation"},
	)

	// authorizationCacheHits counts cache hits/misses
	authorizationCacheHits = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_authorization_cache_hits_total",
			Help: "Total number of authorization cache hits",
		},
		[]string{"workspace", "hit_type"},
	)

	// authorizationDenials tracks authorization denials
	authorizationDenials = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name: "kcp_virtual_authorization_denials_total",
			Help: "Total number of authorization denials",
		},
		[]string{"workspace", "user", "resource", "verb"},
	)
)

// init registers metrics
func init() {
	metrics.MustRegister(
		authorizationRequestsTotal,
		authorizationRequestDuration,
		authorizationCacheHits,
		authorizationDenials,
	)
}

// RecordAuthorizationRequest records metrics for an authorization request
func RecordAuthorizationRequest(workspace, user string, duration time.Duration, allowed bool, err error) {
	result := "allowed"
	if err != nil {
		result = "error"
	} else if !allowed {
		result = "denied"
	}

	authorizationRequestsTotal.WithLabelValues(workspace, user, result).Inc()
	authorizationRequestDuration.WithLabelValues(workspace, "authorize").Observe(duration.Seconds())

	if !allowed && err == nil {
		// Only record denial metric for actual denials, not errors
		authorizationDenials.WithLabelValues(workspace, user, "unknown", "unknown").Inc()
	}
}

// RecordCacheHit records an authorization cache hit or miss
func RecordCacheHit(workspace string, hit bool) {
	hitType := "miss"
	if hit {
		hitType = "hit"
	}
	authorizationCacheHits.WithLabelValues(workspace, hitType).Inc()
}

// RecordDenial records an authorization denial
func RecordDenial(workspace, user, resource, verb string) {
	authorizationDenials.WithLabelValues(workspace, user, resource, verb).Inc()
}