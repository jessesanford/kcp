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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

type EventType string
type Reason string

const (
	EventTypeClusterHealthy   EventType = "ClusterHealthy"
	EventTypeClusterUnhealthy EventType = "ClusterUnhealthy"
	
	ReasonHealthCheckPassed Reason = "HealthCheckPassed"
	ReasonHealthCheckFailed Reason = "HealthCheckFailed"
)

// StubClusterEventRecorder provides minimal event recording for testing.
type StubClusterEventRecorder struct {
	recorder  record.EventRecorder
	workspace logicalcluster.Name
	logger    klog.Logger
}

func NewClusterEventRecorder(recorder record.EventRecorder, workspace logicalcluster.Name, logger klog.Logger) *StubClusterEventRecorder {
	return &StubClusterEventRecorder{
		recorder:  recorder,
		workspace: workspace,
		logger:    logger,
	}
}

func (c *StubClusterEventRecorder) GetWorkspace() logicalcluster.Name {
	return c.workspace
}

func (c *StubClusterEventRecorder) RecordClusterEvent(ctx context.Context, obj runtime.Object, eventType EventType, reason Reason, message string) {
	if c.recorder == nil {
		return
	}

	k8sEventType := corev1.EventTypeNormal
	if eventType == EventTypeClusterUnhealthy {
		k8sEventType = corev1.EventTypeWarning
	}

	c.recorder.Event(obj, k8sEventType, string(reason), message)
}

func (c *StubClusterEventRecorder) RecordClusterHealthEvent(ctx context.Context, obj runtime.Object, healthy bool, message string) {
	if healthy {
		c.RecordClusterEvent(ctx, obj, EventTypeClusterHealthy, ReasonHealthCheckPassed, message)
	} else {
		c.RecordClusterEvent(ctx, obj, EventTypeClusterUnhealthy, ReasonHealthCheckFailed, message)
	}
}

func (c *StubClusterEventRecorder) WithLogger(logger klog.Logger) *StubClusterEventRecorder {
	return &StubClusterEventRecorder{
		recorder:  c.recorder,
		workspace: c.workspace,
		logger:    logger,
	}
}