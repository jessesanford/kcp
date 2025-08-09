/*
Copyright 2025 The KCP Authors.

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

package workqueue

import (
	"sync"
	"time"
)

// queueMetrics tracks performance metrics for TMC workqueues.
type queueMetrics struct {
	name               string
	totalProcessed     int64
	totalFailed        int64
	totalRetried       int64
	totalAdded         int64
	totalRateLimited   int64
	processingDurations []time.Duration
	lastProcessedAt    time.Time
	startTime          time.Time
	mu                 sync.RWMutex
}

// newQueueMetrics creates a new queue metrics tracker.
func newQueueMetrics(name string) *queueMetrics {
	return &queueMetrics{
		name:                name,
		processingDurations: make([]time.Duration, 0, 1000), // Keep last 1000 durations
		startTime:           time.Now(),
	}
}

// recordAdd records that an item was added to the queue.
func (m *queueMetrics) recordAdd(item *WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalAdded++
}

// recordGet records that an item was retrieved from the queue.
func (m *queueMetrics) recordGet(item *WorkItem) {
	// This is called when items are dequeued, no specific metrics needed here
}

// recordDone records that an item was successfully processed.
func (m *queueMetrics) recordDone(item *WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalProcessed++
	m.lastProcessedAt = time.Now()
	
	// Record processing duration if available
	if !item.LastAttemptAt.IsZero() {
		duration := m.lastProcessedAt.Sub(item.LastAttemptAt)
		if len(m.processingDurations) >= 1000 {
			// Remove oldest entry
			copy(m.processingDurations[0:], m.processingDurations[1:])
			m.processingDurations = m.processingDurations[:len(m.processingDurations)-1]
		}
		m.processingDurations = append(m.processingDurations, duration)
	}
}

// recordFailed records that an item failed processing.
func (m *queueMetrics) recordFailed(item *WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalFailed++
}

// recordRetried records that an item was retried.
func (m *queueMetrics) recordRetried(item *WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRetried++
}

// recordRateLimited records that an item was added with rate limiting.
func (m *queueMetrics) recordRateLimited(item *WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRateLimited++
	m.totalAdded++
}

// recordForget records that an item was forgotten (removed from rate limiting).
func (m *queueMetrics) recordForget(item *WorkItem) {
	// No specific metrics needed for forget operations
}

// recordShutdown records that the queue was shut down.
func (m *queueMetrics) recordShutdown() {
	// Could record shutdown time if needed
}

// getMetrics returns a snapshot of current metrics.
func (m *queueMetrics) getMetrics(queueLength, itemsInMemory int) WorkQueueMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := WorkQueueMetrics{
		QueueLength:       queueLength,
		TotalProcessed:    m.totalProcessed,
		TotalFailed:       m.totalFailed,
		TotalRetried:      m.totalRetried,
		LastProcessedAt:   m.lastProcessedAt,
		ActiveWorkers:     0, // Would need to be set by the worker pool
	}

	// Calculate average processing duration
	if len(m.processingDurations) > 0 {
		var total time.Duration
		for _, duration := range m.processingDurations {
			total += duration
		}
		metrics.ProcessingDuration = total / time.Duration(len(m.processingDurations))
	}

	// Calculate average retries
	if m.totalProcessed > 0 {
		metrics.AverageRetries = float64(m.totalRetried) / float64(m.totalProcessed)
	}

	// Calculate worker utilization (simplified)
	if m.totalProcessed > 0 && !m.startTime.IsZero() {
		uptime := time.Since(m.startTime)
		if uptime > 0 {
			totalProcessingTime := time.Duration(len(m.processingDurations)) * metrics.ProcessingDuration
			metrics.WorkerUtilization = float64(totalProcessingTime) / float64(uptime) * 100
			if metrics.WorkerUtilization > 100 {
				metrics.WorkerUtilization = 100 // Cap at 100%
			}
		}
	}

	return metrics
}

// getSuccessRate returns the success rate as a percentage.
func (m *queueMetrics) getSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.totalProcessed + m.totalFailed
	if total == 0 {
		return 0
	}

	return float64(m.totalProcessed) / float64(total) * 100
}

// getThroughput returns items processed per second.
func (m *queueMetrics) getThroughput() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.totalProcessed == 0 || m.startTime.IsZero() {
		return 0
	}

	elapsed := time.Since(m.startTime)
	if elapsed == 0 {
		return 0
	}

	return float64(m.totalProcessed) / elapsed.Seconds()
}

// reset resets all metrics (useful for testing).
func (m *queueMetrics) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalProcessed = 0
	m.totalFailed = 0
	m.totalRetried = 0
	m.totalAdded = 0
	m.totalRateLimited = 0
	m.processingDurations = m.processingDurations[:0]
	m.lastProcessedAt = time.Time{}
	m.startTime = time.Now()
}