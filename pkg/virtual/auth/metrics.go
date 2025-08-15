package auth

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// AuthorizationDecisions tracks authorization decisions by result
	AuthorizationDecisions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_auth_decisions_total",
			Help: "Total number of authorization decisions by result",
		},
		[]string{"decision", "workspace", "verb", "resource"},
	)

	// AuthorizationLatency tracks authorization decision latency
	AuthorizationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "virtual_workspace_auth_latency_seconds",
			Help:    "Authorization decision latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"workspace", "cached"},
	)

	// CacheHits tracks cache hit rate
	CacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_cache_hits_total",
			Help: "Total number of cache hits by cache level",
		},
		[]string{"level", "cache_type"},
	)

	// CacheMisses tracks cache miss rate
	CacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_cache_misses_total",
			Help: "Total number of cache misses by cache level",
		},
		[]string{"level", "cache_type"},
	)

	// CacheEvictions tracks cache evictions
	CacheEvictions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_cache_evictions_total",
			Help: "Total number of cache evictions by reason",
		},
		[]string{"level", "reason"},
	)

	// PolicyEvaluations tracks policy evaluation metrics
	PolicyEvaluations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_policy_evaluations_total",
			Help: "Total number of policy evaluations by result",
		},
		[]string{"effect", "policy_id"},
	)

	// TokenValidations tracks token validation results
	TokenValidations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "virtual_workspace_token_validations_total",
			Help: "Total number of token validations by result",
		},
		[]string{"result", "token_type"},
	)

	// ActiveConnections tracks active connections to virtual workspaces
	ActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "virtual_workspace_active_connections",
			Help: "Number of active connections to virtual workspaces",
		},
		[]string{"workspace"},
	)
)

func init() {
	// Register all metrics
	prometheus.MustRegister(
		AuthorizationDecisions,
		AuthorizationLatency,
		CacheHits,
		CacheMisses,
		CacheEvictions,
		PolicyEvaluations,
		TokenValidations,
		ActiveConnections,
	)
}