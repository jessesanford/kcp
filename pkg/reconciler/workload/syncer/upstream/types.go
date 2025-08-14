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

package upstream

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"
)

const (
	// ControllerName is the name of the upstream sync controller
	ControllerName = "kcp-upstream-syncer"

	// DefaultSyncInterval is the default interval for upstream synchronization
	DefaultSyncInterval = 30 * time.Second

	// DefaultNumWorkers is the default number of sync workers
	DefaultNumWorkers = 2

	// MaxRetries is the maximum number of retries for sync operations
	MaxRetries = 5
)

// SyncTargetKey represents a unique key for a SyncTarget across logical clusters
type SyncTargetKey struct {
	Cluster logicalcluster.Name
	Name    string
}

// String returns the string representation of the SyncTarget key
func (k SyncTargetKey) String() string {
	return k.Cluster.String() + "/" + k.Name
}

// WorkItem represents a work item for the upstream sync queue
type WorkItem struct {
	Key          SyncTargetKey
	Action       WorkAction
	EnqueuedTime time.Time
	Retries      int
}

// WorkAction represents the type of action to perform on a SyncTarget
type WorkAction string

const (
	// ActionSync indicates a normal sync operation
	ActionSync WorkAction = "sync"
	
	// ActionDelete indicates the SyncTarget was deleted
	ActionDelete WorkAction = "delete"
	
	// ActionReconcile indicates a reconciliation is needed
	ActionReconcile WorkAction = "reconcile"
)

// RateLimiter provides rate limiting configuration for sync operations
type RateLimiter struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// DefaultRateLimiter returns the default rate limiter configuration
func DefaultRateLimiter() workqueue.RateLimiter {
	return workqueue.DefaultControllerRateLimiter()
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success   bool
	Error     error
	Timestamp time.Time
	Duration  time.Duration
}

// SyncTargetStatus represents the current status of a SyncTarget sync operation
type SyncTargetStatus struct {
	LastSync      *SyncResult
	SyncCount     int64
	ErrorCount    int64
	LastErrorTime *time.Time
}