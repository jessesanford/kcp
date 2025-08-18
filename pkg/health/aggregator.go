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
	"sort"
	"sync"
	"time"
)

// DefaultHealthAggregator implements the HealthAggregator interface.
type DefaultHealthAggregator struct {
	checkers map[string]HealthChecker
	mutex    sync.RWMutex
	config   HealthConfiguration
}

// NewDefaultHealthAggregator creates a new default health aggregator.
func NewDefaultHealthAggregator(config HealthConfiguration) HealthAggregator {
	return &DefaultHealthAggregator{
		checkers: make(map[string]HealthChecker),
		config:   config,
	}
}

// AddChecker adds a health checker to the aggregator.
func (d *DefaultHealthAggregator) AddChecker(checker HealthChecker) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.checkers[checker.Name()] = checker
}

// RemoveChecker removes a health checker from the aggregator.
func (d *DefaultHealthAggregator) RemoveChecker(name string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delete(d.checkers, name)
}

// CheckAll performs health checks on all registered checkers and returns the aggregated status.
func (d *DefaultHealthAggregator) CheckAll(ctx context.Context) SystemHealthStatus {
	d.mutex.RLock()
	checkers := make(map[string]HealthChecker, len(d.checkers))
	for name, checker := range d.checkers {
		checkers[name] = checker
	}
	d.mutex.RUnlock()
	
	components := make(map[string]HealthStatus, len(checkers))
	healthyCount := 0
	totalCount := len(checkers)
	
	// Use a channel to collect results from parallel health checks
	type checkResult struct {
		name   string
		status HealthStatus
	}
	resultCh := make(chan checkResult, totalCount)
	
	// Start health checks in parallel with timeout
	for name, checker := range checkers {
		go func(name string, checker HealthChecker) {
			checkCtx, cancel := context.WithTimeout(ctx, d.config.CheckTimeout)
			defer cancel()
			
			status := checker.Check(checkCtx)
			resultCh <- checkResult{name: name, status: status}
		}(name, checker)
	}
	
	// Collect results
	for i := 0; i < totalCount; i++ {
		select {
		case result := <-resultCh:
			components[result.name] = result.status
			if result.status.Healthy {
				healthyCount++
			}
		case <-ctx.Done():
			// Context cancelled, mark remaining checks as unhealthy
			return SystemHealthStatus{
				Healthy:      false,
				Message:      "Health check timed out",
				Components:   components,
				Timestamp:    time.Now(),
				HealthyCount: healthyCount,
				TotalCount:   totalCount,
			}
		}
	}
	
	// Determine overall system health
	healthy := healthyCount == totalCount
	message := d.generateHealthMessage(healthyCount, totalCount, components)
	
	return SystemHealthStatus{
		Healthy:      healthy,
		Message:      message,
		Components:   components,
		Timestamp:    time.Now(),
		HealthyCount: healthyCount,
		TotalCount:   totalCount,
	}
}

// CheckComponent performs a health check on a specific component.
func (d *DefaultHealthAggregator) CheckComponent(ctx context.Context, name string) (HealthStatus, error) {
	d.mutex.RLock()
	checker, exists := d.checkers[name]
	d.mutex.RUnlock()
	
	if !exists {
		return HealthStatus{}, fmt.Errorf("health checker '%s' not found", name)
	}
	
	checkCtx, cancel := context.WithTimeout(ctx, d.config.CheckTimeout)
	defer cancel()
	
	return checker.Check(checkCtx), nil
}

// generateHealthMessage creates a human-readable health message based on the check results.
func (d *DefaultHealthAggregator) generateHealthMessage(healthyCount, totalCount int, components map[string]HealthStatus) string {
	if healthyCount == totalCount {
		if totalCount == 0 {
			return "No components registered"
		}
		return fmt.Sprintf("All %d components are healthy", totalCount)
	}
	
	unhealthyCount := totalCount - healthyCount
	if unhealthyCount == totalCount {
		return fmt.Sprintf("All %d components are unhealthy", totalCount)
	}
	
	// List unhealthy components
	var unhealthyComponents []string
	for name, status := range components {
		if !status.Healthy {
			unhealthyComponents = append(unhealthyComponents, name)
		}
	}
	
	sort.Strings(unhealthyComponents)
	
	message := fmt.Sprintf("%d/%d components healthy", healthyCount, totalCount)
	if len(unhealthyComponents) <= 3 {
		message += fmt.Sprintf(" (unhealthy: %v)", unhealthyComponents)
	} else {
		message += fmt.Sprintf(" (unhealthy: %v and %d more)", unhealthyComponents[:3], len(unhealthyComponents)-3)
	}
	
	return message
}

// WeightedHealthAggregator is a health aggregator that supports weighted components.
type WeightedHealthAggregator struct {
	checkers map[string]HealthChecker
	weights  map[string]float64
	mutex    sync.RWMutex
	config   HealthConfiguration
}

// NewWeightedHealthAggregator creates a new weighted health aggregator.
func NewWeightedHealthAggregator(config HealthConfiguration) *WeightedHealthAggregator {
	return &WeightedHealthAggregator{
		checkers: make(map[string]HealthChecker),
		weights:  make(map[string]float64),
		config:   config,
	}
}

// AddChecker adds a health checker with equal weight (1.0).
func (w *WeightedHealthAggregator) AddChecker(checker HealthChecker) {
	w.AddWeightedChecker(checker, 1.0)
}

// AddWeightedChecker adds a health checker with a specific weight.
func (w *WeightedHealthAggregator) AddWeightedChecker(checker HealthChecker, weight float64) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.checkers[checker.Name()] = checker
	w.weights[checker.Name()] = weight
}

// RemoveChecker removes a health checker from the aggregator.
func (w *WeightedHealthAggregator) RemoveChecker(name string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	delete(w.checkers, name)
	delete(w.weights, name)
}

// CheckAll performs weighted health checks on all registered checkers.
func (w *WeightedHealthAggregator) CheckAll(ctx context.Context) SystemHealthStatus {
	w.mutex.RLock()
	checkers := make(map[string]HealthChecker, len(w.checkers))
	weights := make(map[string]float64, len(w.weights))
	for name, checker := range w.checkers {
		checkers[name] = checker
		weights[name] = w.weights[name]
	}
	w.mutex.RUnlock()
	
	components := make(map[string]HealthStatus, len(checkers))
	healthyCount := 0
	totalCount := len(checkers)
	totalWeight := 0.0
	healthyWeight := 0.0
	
	// Perform health checks
	for name, checker := range checkers {
		checkCtx, cancel := context.WithTimeout(ctx, w.config.CheckTimeout)
		status := checker.Check(checkCtx)
		cancel()
		
		components[name] = status
		weight := weights[name]
		totalWeight += weight
		
		if status.Healthy {
			healthyCount++
			healthyWeight += weight
		}
	}
	
	// Calculate weighted health percentage
	healthPercentage := 0.0
	if totalWeight > 0 {
		healthPercentage = healthyWeight / totalWeight
	}
	
	// System is healthy if weighted health is above threshold (e.g., 80%)
	healthy := healthPercentage >= 0.8
	
	message := fmt.Sprintf("Weighted health: %.1f%% (%d/%d components, %.1f/%.1f weight)", 
		healthPercentage*100, healthyCount, totalCount, healthyWeight, totalWeight)
	
	return SystemHealthStatus{
		Healthy:      healthy,
		Message:      message,
		Components:   components,
		Timestamp:    time.Now(),
		HealthyCount: healthyCount,
		TotalCount:   totalCount,
	}
}

// CheckComponent performs a health check on a specific component.
func (w *WeightedHealthAggregator) CheckComponent(ctx context.Context, name string) (HealthStatus, error) {
	w.mutex.RLock()
	checker, exists := w.checkers[name]
	w.mutex.RUnlock()
	
	if !exists {
		return HealthStatus{}, fmt.Errorf("health checker '%s' not found", name)
	}
	
	checkCtx, cancel := context.WithTimeout(ctx, w.config.CheckTimeout)
	defer cancel()
	
	return checker.Check(checkCtx), nil
}