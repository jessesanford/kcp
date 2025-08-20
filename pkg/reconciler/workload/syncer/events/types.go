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

package events

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// EventSyncConfig configures event synchronization behavior.
// It controls filtering, aggregation, and transformation of events
// as they are synced from physical clusters to KCP.
type EventSyncConfig struct {
	// Filtering configuration
	MinimumSeverity     string
	IncludeTypes        []string
	ExcludeTypes        []string
	MaxEventsPerMinute  int

	// Aggregation configuration
	AggregationWindow   time.Duration
	MaxAggregatedEvents int

	// Transformation configuration
	AddLabels      map[string]string
	AddAnnotations map[string]string
}

// SyncedEvent represents an event that has been processed for synchronization.
// It contains the original event plus metadata about the sync operation.
type SyncedEvent struct {
	Event           *corev1.Event
	SourceCluster   string
	TransformedName string
	Aggregated      bool
	Count           int32
}


// AggregationKey uniquely identifies a group of similar events for aggregation.
// Events with the same aggregation key will be grouped together.
type AggregationKey struct {
	Namespace string
	Name      string
	Type      string
	Reason    string
	Message   string
}