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

package engine

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SyncItem represents a work item in the sync queue
type SyncItem struct {
	GVR       schema.GroupVersionResource `json:"gvr"`
	Key       string                      `json:"key"` // namespace/name
	Action    string                      `json:"action"` // add, update, delete, status
	Object    interface{}                 `json:"object"` // The actual object
	Retries   int                         `json:"retries"`
	Timestamp metav1.Time                 `json:"timestamp"`
}

// SyncStatus tracks synchronization state
type SyncStatus struct {
	Connected        bool                                      `json:"connected"`
	LastSyncTime     *metav1.Time                              `json:"lastSyncTime,omitempty"`
	SyncedResources  map[schema.GroupVersionResource]int       `json:"syncedResources"`
	PendingResources map[schema.GroupVersionResource]int       `json:"pendingResources"`
	FailedResources  map[schema.GroupVersionResource]int       `json:"failedResources"`
	ErrorMessage     string                                   `json:"errorMessage,omitempty"`
}

// EngineConfig holds engine configuration
type EngineConfig struct {
	WorkerCount     int           `json:"workerCount"`
	ResyncPeriod    time.Duration `json:"resyncPeriod"`
	MaxRetries      int           `json:"maxRetries"`
	RateLimitPerSec int           `json:"rateLimitPerSec"`
	QueueDepth      int           `json:"queueDepth"`
	EnableProfiling bool          `json:"enableProfiling"`
}

// DefaultEngineConfig returns a default engine configuration
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		WorkerCount:     5,
		ResyncPeriod:    10 * time.Minute,
		MaxRetries:      3,
		RateLimitPerSec: 100,
		QueueDepth:      1000,
		EnableProfiling: false,
	}
}