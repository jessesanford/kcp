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

package health

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Health status constants
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusDegraded  HealthStatus = "Degraded"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

// ComponentHealth represents component health state
type ComponentHealth struct {
	Name      string                 `json:"name"`
	Status    HealthStatus           `json:"status"`
	LastCheck time.Time              `json:"lastCheck"`
	Message   string                 `json:"message,omitempty"`
	Error     error                  `json:"error,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// HealthReport contains complete health information
type HealthReport struct {
	Status         HealthStatus                    `json:"status"`
	Components     map[string]*ComponentHealth     `json:"components"`
	LastHeartbeat  *metav1.Time                   `json:"lastHeartbeat,omitempty"`
	Uptime         time.Duration                  `json:"uptime"`
	SyncRate       float64                        `json:"syncRate"`
	ErrorRate      float64                        `json:"errorRate"`
	MemoryUsage    int64                          `json:"memoryUsage"`
	GoroutineCount int                            `json:"goroutineCount"`
}

// HealthChecker interface for component health checks
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) *ComponentHealth
}

// Monitor provides health monitoring and heartbeat functionality
type Monitor struct {
	kubeClient      kubernetes.Interface
	workspace       logicalcluster.Name
	targetNamespace string
	targetName      string
	
	components   map[string]HealthChecker
	componentsMu sync.RWMutex
	status       HealthStatus
	startTime    time.Time
	
	// Heartbeat state
	lease           *coordinationv1.Lease
	leaseMutex      sync.RWMutex
	heartbeatConfig HeartbeatConfig
	lastHeartbeat   time.Time
	heartbeatFails  atomic.Int32
	isHealthy       atomic.Bool
	
	// Metrics
	syncCount    atomic.Int64
	errorCount   atomic.Int64
	latencySum   atomic.Int64
	latencyCount atomic.Int64
	
	ctx    context.Context
	cancel context.CancelFunc
}

// HeartbeatConfig configures heartbeat behavior
type HeartbeatConfig struct {
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int
	LeaseNamespace   string
	LeaseName        string
}

// Prometheus metrics
var (
	healthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "syncer_health_status", Help: "Health status of syncer components"},
		[]string{"component"},
	)
	heartbeatTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "syncer_heartbeat_total", Help: "Total heartbeat attempts"},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(healthStatus, heartbeatTotal)
}

// NewMonitor creates a new health monitor with integrated heartbeat
func NewMonitor(kubeClient kubernetes.Interface, workspace logicalcluster.Name, targetNamespace, targetName string) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Monitor{
		kubeClient:      kubeClient,
		workspace:       workspace,
		targetNamespace: targetNamespace,
		targetName:      targetName,
		components:      make(map[string]HealthChecker),
		status:          HealthStatusUnknown,
		startTime:       time.Now(),
		heartbeatConfig: HeartbeatConfig{
			Interval:         10 * time.Second,
			Timeout:          30 * time.Second,
			FailureThreshold: 3,
			LeaseNamespace:   targetNamespace,
			LeaseName:        fmt.Sprintf("%s-heartbeat", targetName),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins health monitoring and heartbeat
func (m *Monitor) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting health monitor with heartbeat")
	
	if err := m.initializeLease(ctx); err != nil {
		return fmt.Errorf("failed to initialize lease: %w", err)
	}
	
	go m.runLoop(ctx)
	
	return nil
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// RegisterComponent registers a component for health checking
func (m *Monitor) RegisterComponent(checker HealthChecker) {
	m.componentsMu.Lock()
	defer m.componentsMu.Unlock()
	m.components[checker.Name()] = checker
}

// runLoop runs the main monitoring and heartbeat loop
func (m *Monitor) runLoop(ctx context.Context) {
	healthTicker := time.NewTicker(30 * time.Second)
	heartbeatTicker := time.NewTicker(m.heartbeatConfig.Interval)
	defer healthTicker.Stop()
	defer heartbeatTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-healthTicker.C:
			m.performHealthChecks(ctx)
		case <-heartbeatTicker.C:
			m.sendHeartbeat(ctx)
		}
	}
}

// initializeLease creates or updates the heartbeat lease
func (m *Monitor) initializeLease(ctx context.Context) error {
	now := metav1.NewTime(time.Now())
	leaseDuration := int32(m.heartbeatConfig.Timeout.Seconds())
	
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.heartbeatConfig.LeaseName,
			Namespace: m.heartbeatConfig.LeaseNamespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("syncer-heartbeat"),
			LeaseDurationSeconds: &leaseDuration,
			RenewTime:            &now,
		},
	}
	
	createdLease, err := m.kubeClient.CoordinationV1().Leases(m.heartbeatConfig.LeaseNamespace).Create(ctx, lease, metav1.CreateOptions{})
	if err == nil {
		m.setLease(createdLease)
		return nil
	}
	
	if apierrors.IsAlreadyExists(err) {
		existingLease, getErr := m.kubeClient.CoordinationV1().Leases(m.heartbeatConfig.LeaseNamespace).Get(ctx, m.heartbeatConfig.LeaseName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}
		
		existingLease.Spec.RenewTime = &now
		updatedLease, updateErr := m.kubeClient.CoordinationV1().Leases(m.heartbeatConfig.LeaseNamespace).Update(ctx, existingLease, metav1.UpdateOptions{})
		if updateErr != nil {
			return updateErr
		}
		
		m.setLease(updatedLease)
		return nil
	}
	
	return err
}

// sendHeartbeat sends a heartbeat by updating the lease
func (m *Monitor) sendHeartbeat(ctx context.Context) {
	heartbeatCtx, cancel := context.WithTimeout(ctx, m.heartbeatConfig.Timeout)
	defer cancel()
	
	m.leaseMutex.RLock()
	currentLease := m.lease
	m.leaseMutex.RUnlock()
	
	if currentLease == nil {
		m.handleHeartbeatFailure()
		return
	}
	
	now := metav1.NewTime(time.Now())
	currentLease.Spec.RenewTime = &now
	
	updatedLease, err := m.kubeClient.CoordinationV1().Leases(m.heartbeatConfig.LeaseNamespace).Update(heartbeatCtx, currentLease, metav1.UpdateOptions{})
	if err != nil {
		m.handleHeartbeatFailure()
		heartbeatTotal.WithLabelValues("failure").Inc()
		return
	}
	
	m.setLease(updatedLease)
	m.lastHeartbeat = time.Now()
	m.heartbeatFails.Store(0)
	m.isHealthy.Store(true)
	heartbeatTotal.WithLabelValues("success").Inc()
}

// handleHeartbeatFailure handles failed heartbeats
func (m *Monitor) handleHeartbeatFailure() {
	fails := m.heartbeatFails.Add(1)
	if int(fails) >= m.heartbeatConfig.FailureThreshold {
		m.isHealthy.Store(false)
	}
}

// performHealthChecks checks all registered components
func (m *Monitor) performHealthChecks(ctx context.Context) {
	m.componentsMu.RLock()
	checkers := make([]HealthChecker, 0, len(m.components))
	for _, checker := range m.components {
		checkers = append(checkers, checker)
	}
	m.componentsMu.RUnlock()
	
	if len(checkers) == 0 {
		m.status = HealthStatusUnknown
		return
	}
	
	results := make(map[string]*ComponentHealth)
	for _, checker := range checkers {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		health := checker.Check(checkCtx)
		cancel()
		
		if health != nil {
			results[checker.Name()] = health
			healthStatus.WithLabelValues(checker.Name()).Set(float64(statusToValue(health.Status)))
		}
	}
	
	m.status = m.aggregateStatus(results)
}

// aggregateStatus determines overall health from components
func (m *Monitor) aggregateStatus(components map[string]*ComponentHealth) HealthStatus {
	if len(components) == 0 {
		return HealthStatusUnknown
	}
	
	unhealthy, degraded := 0, 0
	for _, health := range components {
		if health.Status == HealthStatusUnhealthy {
			unhealthy++
		} else if health.Status == HealthStatusDegraded {
			degraded++
		}
	}
	
	if unhealthy > 0 {
		return HealthStatusUnhealthy
	}
	if degraded > len(components)/2 || degraded > 0 {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}

// GetHealth returns current health status
func (m *Monitor) GetHealth() HealthStatus {
	return m.status
}

// GetReport returns comprehensive health report
func (m *Monitor) GetReport() *HealthReport {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return &HealthReport{
		Status:         m.status,
		LastHeartbeat:  &metav1.Time{Time: m.lastHeartbeat},
		Uptime:         time.Since(m.startTime),
		SyncRate:       m.getSyncRate(),
		ErrorRate:      m.getErrorRate(),
		MemoryUsage:    int64(memStats.Alloc),
		GoroutineCount: runtime.NumGoroutine(),
	}
}

// LivenessProbe implements Kubernetes liveness probe
func (m *Monitor) LivenessProbe() error {
	if m.status == HealthStatusUnhealthy {
		return fmt.Errorf("syncer is unhealthy")
	}
	return nil
}

// ReadinessProbe implements Kubernetes readiness probe
func (m *Monitor) ReadinessProbe() error {
	if m.status != HealthStatusHealthy {
		return fmt.Errorf("syncer is not ready: %s", m.status)
	}
	return nil
}

// RecordSync records a successful sync operation
func (m *Monitor) RecordSync(duration time.Duration) {
	m.syncCount.Add(1)
	m.latencySum.Add(duration.Nanoseconds())
	m.latencyCount.Add(1)
}

// RecordError records a sync error
func (m *Monitor) RecordError() {
	m.errorCount.Add(1)
}

// Helper methods
func (m *Monitor) setLease(lease *coordinationv1.Lease) {
	m.leaseMutex.Lock()
	defer m.leaseMutex.Unlock()
	m.lease = lease
}

func (m *Monitor) getSyncRate() float64 {
	count := m.syncCount.Load()
	elapsed := time.Since(m.startTime).Seconds()
	if elapsed > 0 {
		return float64(count) / elapsed
	}
	return 0.0
}

func (m *Monitor) getErrorRate() float64 {
	syncCount := m.syncCount.Load()
	errorCount := m.errorCount.Load()
	total := syncCount + errorCount
	if total > 0 {
		return float64(errorCount) / float64(total)
	}
	return 0.0
}

func statusToValue(status HealthStatus) int {
	switch status {
	case HealthStatusHealthy:
		return 1
	case HealthStatusDegraded:
		return 2
	case HealthStatusUnhealthy:
		return 3
	default:
		return 0
	}
}