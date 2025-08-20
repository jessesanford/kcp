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
	"sync/atomic"
	"time"
)

// ConnectionHealthCheck monitors connection health
type ConnectionHealthCheck struct {
	name        string
	isConnected atomic.Bool
	lastError   atomic.Value
}

// NewConnectionHealthCheck creates a connection health checker
func NewConnectionHealthCheck(name string) *ConnectionHealthCheck {
	return &ConnectionHealthCheck{name: name}
}

func (c *ConnectionHealthCheck) Name() string {
	return c.name
}

func (c *ConnectionHealthCheck) Check(ctx context.Context) *ComponentHealth {
	health := &ComponentHealth{
		Name:      c.name,
		LastCheck: time.Now(),
		Metrics:   make(map[string]interface{}),
	}
	
	if c.isConnected.Load() {
		health.Status = HealthStatusHealthy
		health.Message = "Connected"
	} else {
		health.Status = HealthStatusUnhealthy
		health.Message = "Disconnected"
		if err := c.getLastError(); err != nil {
			health.Error = err
		}
	}
	
	return health
}

func (c *ConnectionHealthCheck) SetConnected(connected bool) {
	c.isConnected.Store(connected)
}

func (c *ConnectionHealthCheck) SetLastError(err error) {
	c.lastError.Store(err)
}

func (c *ConnectionHealthCheck) getLastError() error {
	if err := c.lastError.Load(); err != nil {
		if e, ok := err.(error); ok {
			return e
		}
	}
	return nil
}

// SyncEngineHealthCheck monitors sync engine health
type SyncEngineHealthCheck struct {
	name       string
	queueDepth atomic.Int64
	syncRate   atomic.Value
	errorRate  atomic.Value
}

// NewSyncEngineHealthCheck creates a sync engine health checker
func NewSyncEngineHealthCheck(name string) *SyncEngineHealthCheck {
	check := &SyncEngineHealthCheck{name: name}
	check.syncRate.Store(float64(0))
	check.errorRate.Store(float64(0))
	return check
}

func (s *SyncEngineHealthCheck) Name() string {
	return s.name
}

func (s *SyncEngineHealthCheck) Check(ctx context.Context) *ComponentHealth {
	health := &ComponentHealth{
		Name:      s.name,
		LastCheck: time.Now(),
		Metrics:   make(map[string]interface{}),
	}
	
	depth := s.queueDepth.Load()
	rate := s.getSyncRate()
	errRate := s.getErrorRate()
	
	health.Metrics["queueDepth"] = depth
	health.Metrics["syncRate"] = rate
	health.Metrics["errorRate"] = errRate
	
	if errRate > 0.5 {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("High error rate: %.2f%%", errRate*100)
	} else if depth > 1000 {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("Queue depth high: %d", depth)
	} else {
		health.Status = HealthStatusHealthy
		health.Message = fmt.Sprintf("Sync rate: %.2f/sec", rate)
	}
	
	return health
}

func (s *SyncEngineHealthCheck) UpdateMetrics(queueDepth int64, syncRate, errorRate float64) {
	s.queueDepth.Store(queueDepth)
	s.syncRate.Store(syncRate)
	s.errorRate.Store(errorRate)
}

func (s *SyncEngineHealthCheck) getSyncRate() float64 {
	if rate := s.syncRate.Load(); rate != nil {
		if r, ok := rate.(float64); ok {
			return r
		}
	}
	return 0.0
}

func (s *SyncEngineHealthCheck) getErrorRate() float64 {
	if rate := s.errorRate.Load(); rate != nil {
		if r, ok := rate.(float64); ok {
			return r
		}
	}
	return 0.0
}

// ResourceHealthCheck monitors system resources
type ResourceHealthCheck struct {
	name string
}

// NewResourceHealthCheck creates a resource health checker
func NewResourceHealthCheck(name string) *ResourceHealthCheck {
	return &ResourceHealthCheck{name: name}
}

func (r *ResourceHealthCheck) Name() string {
	return r.name
}

func (r *ResourceHealthCheck) Check(ctx context.Context) *ComponentHealth {
	health := &ComponentHealth{
		Name:      r.name,
		LastCheck: time.Now(),
		Metrics:   make(map[string]interface{}),
	}
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	allocMB := float64(memStats.Alloc) / 1024 / 1024
	goroutines := runtime.NumGoroutine()
	
	health.Metrics["allocatedMemoryMB"] = allocMB
	health.Metrics["goroutines"] = goroutines
	
	if allocMB > 1000 {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("High memory usage: %.1f MB", allocMB)
	} else if goroutines > 10000 {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("High goroutine count: %d", goroutines)
	} else {
		health.Status = HealthStatusHealthy
		health.Message = fmt.Sprintf("Memory: %.1f MB, Goroutines: %d", allocMB, goroutines)
	}
	
	return health
}