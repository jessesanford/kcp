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

package downstream

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Operation   string         // create, update, delete, noop
	Success     bool
	Error       error
	RetryAfter  *time.Duration
	Conflicts   []string
	ChangedFields []string
}

// DownstreamConfig holds downstream syncer configuration
type DownstreamConfig struct {
	ConflictRetries      int
	UpdateStrategy       string // replace, merge, strategic-merge
	PreserveFields       []string
	IgnoreFields         []string
	DeletionPropagation  metav1.DeletionPropagation
	ConflictRetryDelay   time.Duration
}

// ResourceState tracks downstream resource state
type ResourceState struct {
	GVR              schema.GroupVersionResource
	Namespace        string
	Name             string
	ResourceVersion  string
	Generation       int64
	LastSyncTime     metav1.Time
	Hash             string
	ConflictCount    int
	LastConflictTime *metav1.Time
}

// ConflictType represents different types of sync conflicts
type ConflictType string

const (
	ConflictTypeResourceVersion ConflictType = "ResourceVersion"
	ConflictTypeGeneration      ConflictType = "Generation"
	ConflictTypeFieldConflict   ConflictType = "FieldConflict"
	ConflictTypeDeletion        ConflictType = "Deletion"
)

// SyncConflict represents a conflict during synchronization
type SyncConflict struct {
	Type        ConflictType
	Field       string
	UpstreamValue    interface{}
	DownstreamValue  interface{}
	Resolvable  bool
	Resolution  string
}

// DefaultDownstreamConfig returns default configuration for downstream syncer
func DefaultDownstreamConfig() *DownstreamConfig {
	return &DownstreamConfig{
		ConflictRetries:     3,
		UpdateStrategy:      "strategic-merge",
		PreserveFields:      []string{"status", "metadata.resourceVersion", "metadata.uid", "metadata.creationTimestamp"},
		IgnoreFields:        []string{"metadata.managedFields"},
		DeletionPropagation: metav1.DeletePropagationBackground,
		ConflictRetryDelay:  time.Second * 5,
	}
}