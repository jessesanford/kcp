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

package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BaseHealthChecker provides a basic implementation of HealthChecker
// that can be embedded by specific health checkers.
type BaseHealthChecker struct {
	name      string
	checkFunc func(ctx context.Context) HealthStatus
	lastCheck time.Time
	mutex     sync.RWMutex
}

// NewBaseHealthChecker creates a new base health checker with the given name and check function.
func NewBaseHealthChecker(name string, checkFunc func(ctx context.Context) HealthStatus) *BaseHealthChecker {
	return &BaseHealthChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Name returns the name of the health checker.
func (b *BaseHealthChecker) Name() string {
	return b.name
}

// Check performs the health check and updates the last check timestamp.
func (b *BaseHealthChecker) Check(ctx context.Context) HealthStatus {
	defer func() {
		b.mutex.Lock()
		b.lastCheck = time.Now()
		b.mutex.Unlock()
	}()
	
	if b.checkFunc == nil {
		return HealthStatus{
			Healthy:   false,
			Message:   fmt.Sprintf("No health check function defined for %s", b.name),
			Timestamp: time.Now(),
		}
	}
	
	return b.checkFunc(ctx)
}

// LastCheck returns the timestamp of the last health check.
func (b *BaseHealthChecker) LastCheck() time.Time {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.lastCheck
}

// PeriodicHealthChecker wraps a HealthChecker and performs periodic health checks.
type PeriodicHealthChecker struct {
	checker      HealthChecker
	interval     time.Duration
	lastStatus   HealthStatus
	mutex        sync.RWMutex
	stopCh       chan struct{}
	config       HealthConfiguration
	failureCount int
}

// NewPeriodicHealthChecker creates a new periodic health checker.
func NewPeriodicHealthChecker(checker HealthChecker, config HealthConfiguration) *PeriodicHealthChecker {
	return &PeriodicHealthChecker{
		checker:  checker,
		interval: config.CheckInterval,
		stopCh:   make(chan struct{}),
		config:   config,
	}
}

// Start begins periodic health checking in a goroutine.
func (p *PeriodicHealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	
	// Perform initial check
	p.performCheck(ctx)
	
	for {
		select {
		case <-ticker.C:
			p.performCheck(ctx)
		case <-p.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the periodic health checking.
func (p *PeriodicHealthChecker) Stop() {
	close(p.stopCh)
}

// performCheck performs a health check with retry logic and failure tracking.
func (p *PeriodicHealthChecker) performCheck(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, p.config.CheckTimeout)
	defer cancel()
	
	var status HealthStatus
	var success bool
	
	// Retry logic
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		status = p.checker.Check(ctx)
		if status.Healthy {
			success = true
			break
		}
		
		// Wait a bit before retrying (exponential backoff)
		if attempt < p.config.MaxRetries {
			waitTime := time.Duration(attempt+1) * time.Second
			time.Sleep(waitTime)
		}
	}
	
	// Update failure count and status
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if success {
		p.failureCount = 0
	} else {
		p.failureCount++
	}
	
	// Only mark as unhealthy if we exceed failure threshold
	if !success && p.failureCount >= p.config.FailureThreshold {
		status.Healthy = false
		if status.Details == nil {
			status.Details = make(map[string]interface{})
		}
		status.Details["failure_count"] = p.failureCount
		status.Details["failure_threshold"] = p.config.FailureThreshold
	}
	
	p.lastStatus = status
}

// Name returns the name of the underlying checker.
func (p *PeriodicHealthChecker) Name() string {
	return p.checker.Name()
}

// Check returns the last cached health status.
func (p *PeriodicHealthChecker) Check(ctx context.Context) HealthStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.lastStatus
}

// LastCheck returns the timestamp of the last health check.
func (p *PeriodicHealthChecker) LastCheck() time.Time {
	return p.checker.LastCheck()
}

// GetLatestStatus returns the most recent health status without performing a new check.
func (p *PeriodicHealthChecker) GetLatestStatus() HealthStatus {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.lastStatus
}

// FuncHealthChecker is a simple health checker that wraps a function.
type FuncHealthChecker struct {
	*BaseHealthChecker
}

// NewFuncHealthChecker creates a new function-based health checker.
func NewFuncHealthChecker(name string, checkFunc func(ctx context.Context) HealthStatus) HealthChecker {
	return &FuncHealthChecker{
		BaseHealthChecker: NewBaseHealthChecker(name, checkFunc),
	}
}

// StaticHealthChecker always returns the same health status (useful for testing).
type StaticHealthChecker struct {
	name      string
	status    HealthStatus
	lastCheck time.Time
}

// NewStaticHealthChecker creates a new static health checker that always returns the given status.
func NewStaticHealthChecker(name string, healthy bool, message string) HealthChecker {
	return &StaticHealthChecker{
		name: name,
		status: HealthStatus{
			Healthy:   healthy,
			Message:   message,
			Timestamp: time.Now(),
		},
	}
}

// Name returns the name of the static health checker.
func (s *StaticHealthChecker) Name() string {
	return s.name
}

// Check returns the static health status.
func (s *StaticHealthChecker) Check(ctx context.Context) HealthStatus {
	s.lastCheck = time.Now()
	s.status.Timestamp = s.lastCheck
	return s.status
}

// LastCheck returns the timestamp of the last check.
func (s *StaticHealthChecker) LastCheck() time.Time {
	return s.lastCheck
}