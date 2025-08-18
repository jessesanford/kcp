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

package rollback

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TriggerManager manages automatic rollback triggers.
type TriggerManager struct {
	dynamicClient dynamic.Interface
	cluster       logicalcluster.Name
	config        *EngineConfig
	
	// Active triggers and their cooldown state
	activeTriggers map[string]*RollbackTrigger
	cooldowns      map[string]time.Time
	mu            sync.RWMutex
	
	// Channels for trigger events
	triggerEvents chan TriggerEvent
	stopCh        chan struct{}
	
	// Metrics tracking
	healthMetrics map[string]*HealthMetrics
	errorMetrics  map[string]*ErrorMetrics
}

// TriggerEvent represents a trigger activation event.
type TriggerEvent struct {
	TriggerName   string
	DeploymentRef corev1.ObjectReference
	Reason        string
	Timestamp     time.Time
	Severity      TriggerSeverity
}

// TriggerSeverity indicates the severity of a trigger event.
type TriggerSeverity string

const (
	TriggerSeverityLow      TriggerSeverity = "Low"
	TriggerSeverityMedium   TriggerSeverity = "Medium"
	TriggerSeverityHigh     TriggerSeverity = "High"
	TriggerSeverityCritical TriggerSeverity = "Critical"
)

// HealthMetrics tracks health-related metrics for a deployment.
type HealthMetrics struct {
	ConsecutiveFailures int
	LastHealthCheck     time.Time
	HealthCheckPassing  bool
	ReadyReplicas       int32
	DesiredReplicas     int32
}

// ErrorMetrics tracks error-related metrics for a deployment.
type ErrorMetrics struct {
	ErrorRate          float64
	TotalRequests      int64
	FailedRequests     int64
	LastErrorTime      time.Time
	EvaluationWindow   time.Duration
	WindowStartTime    time.Time
}

// NewTriggerManager creates a new trigger manager.
func NewTriggerManager(client dynamic.Interface, cluster logicalcluster.Name, config *EngineConfig) *TriggerManager {
	return &TriggerManager{
		dynamicClient:  client,
		cluster:        cluster,
		config:         config,
		activeTriggers: make(map[string]*RollbackTrigger),
		cooldowns:      make(map[string]time.Time),
		triggerEvents:  make(chan TriggerEvent, 100),
		stopCh:         make(chan struct{}),
		healthMetrics:  make(map[string]*HealthMetrics),
		errorMetrics:   make(map[string]*ErrorMetrics),
	}
}

// Start begins monitoring for trigger conditions.
func (tm *TriggerManager) Start(ctx context.Context) error {
	if !tm.config.EnableAutomaticTriggers {
		klog.InfoS("Automatic triggers disabled")
		return nil
	}

	klog.InfoS("Starting trigger manager", "triggerCount", len(tm.config.Triggers))
	
	// Load configured triggers
	tm.loadTriggers()
	
	// Start monitoring goroutines
	go tm.monitorHealthChecks(ctx)
	go tm.monitorErrorRates(ctx)
	go tm.monitorTimeouts(ctx)
	go tm.processTriggerEvents(ctx)
	
	klog.InfoS("Trigger manager started successfully")
	return nil
}

// Stop stops the trigger manager.
func (tm *TriggerManager) Stop() {
	klog.InfoS("Stopping trigger manager")
	close(tm.stopCh)
}

// EvaluateTriggers checks all triggers for a specific deployment.
func (tm *TriggerManager) EvaluateTriggers(ctx context.Context, deploymentRef corev1.ObjectReference) []TriggerEvent {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	var events []TriggerEvent
	
	for _, trigger := range tm.activeTriggers {
		if !trigger.Enabled {
			continue
		}
		
		// Check cooldown
		if tm.isInCooldown(trigger.Name) {
			continue
		}
		
		// Evaluate trigger condition
		if event := tm.evaluateTrigger(ctx, trigger, deploymentRef); event != nil {
			events = append(events, *event)
		}
	}
	
	return events
}

// RegisterTrigger adds a new trigger to the active set.
func (tm *TriggerManager) RegisterTrigger(trigger *RollbackTrigger) error {
	if trigger.Name == "" {
		return fmt.Errorf("trigger name cannot be empty")
	}
	
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	tm.activeTriggers[trigger.Name] = trigger
	klog.InfoS("Registered rollback trigger", "name", trigger.Name, "type", trigger.Type)
	return nil
}

// UnregisterTrigger removes a trigger from the active set.
func (tm *TriggerManager) UnregisterTrigger(triggerName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	delete(tm.activeTriggers, triggerName)
	delete(tm.cooldowns, triggerName)
	klog.InfoS("Unregistered rollback trigger", "name", triggerName)
}

// GetTriggerEvents returns a channel for receiving trigger events.
func (tm *TriggerManager) GetTriggerEvents() <-chan TriggerEvent {
	return tm.triggerEvents
}

// loadTriggers loads triggers from configuration.
func (tm *TriggerManager) loadTriggers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	for _, trigger := range tm.config.Triggers {
		tm.activeTriggers[trigger.Name] = &trigger
		klog.V(2).InfoS("Loaded trigger from config", "name", trigger.Name, "type", trigger.Type)
	}
}

// monitorHealthChecks continuously monitors deployment health.
func (tm *TriggerManager) monitorHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.checkDeploymentHealth(ctx)
		}
	}
}

// monitorErrorRates continuously monitors error rates.
func (tm *TriggerManager) monitorErrorRates(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.checkErrorRates(ctx)
		}
	}
}

// monitorTimeouts monitors for deployment timeouts.
func (tm *TriggerManager) monitorTimeouts(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.checkTimeouts(ctx)
		}
	}
}

// processTriggerEvents processes trigger events and initiates rollbacks.
func (tm *TriggerManager) processTriggerEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case event := <-tm.triggerEvents:
			tm.handleTriggerEvent(ctx, event)
		}
	}
}

// evaluateTrigger evaluates a specific trigger condition.
func (tm *TriggerManager) evaluateTrigger(ctx context.Context, trigger *RollbackTrigger, deploymentRef corev1.ObjectReference) *TriggerEvent {
	switch trigger.Type {
	case TriggerTypeHealthCheck:
		return tm.evaluateHealthCheckTrigger(ctx, trigger, deploymentRef)
	case TriggerTypeErrorRate:
		return tm.evaluateErrorRateTrigger(ctx, trigger, deploymentRef)
	case TriggerTypeTimeout:
		return tm.evaluateTimeoutTrigger(ctx, trigger, deploymentRef)
	case TriggerTypeSLO:
		return tm.evaluateSLOTrigger(ctx, trigger, deploymentRef)
	default:
		klog.V(2).InfoS("Unknown trigger type", "type", trigger.Type, "name", trigger.Name)
		return nil
	}
}

// evaluateHealthCheckTrigger evaluates health check failures.
func (tm *TriggerManager) evaluateHealthCheckTrigger(ctx context.Context, trigger *RollbackTrigger, deploymentRef corev1.ObjectReference) *TriggerEvent {
	key := tm.getDeploymentKey(deploymentRef)
	metrics, exists := tm.healthMetrics[key]
	if !exists {
		return nil
	}
	
	threshold := int32(1) // default
	if trigger.Conditions.HealthCheckFailures != nil {
		threshold = *trigger.Conditions.HealthCheckFailures
	}
	
	if metrics.ConsecutiveFailures >= int(threshold) {
		return &TriggerEvent{
			TriggerName:   trigger.Name,
			DeploymentRef: deploymentRef,
			Reason:        fmt.Sprintf("Health check failed %d consecutive times (threshold: %d)", metrics.ConsecutiveFailures, threshold),
			Timestamp:     time.Now(),
			Severity:      TriggerSeverityHigh,
		}
	}
	
	return nil
}

// evaluateErrorRateTrigger evaluates error rate thresholds.
func (tm *TriggerManager) evaluateErrorRateTrigger(ctx context.Context, trigger *RollbackTrigger, deploymentRef corev1.ObjectReference) *TriggerEvent {
	key := tm.getDeploymentKey(deploymentRef)
	metrics, exists := tm.errorMetrics[key]
	if !exists {
		return nil
	}
	
	threshold := 5.0 // default 5%
	if trigger.Conditions.ErrorRateThreshold != nil {
		threshold = *trigger.Conditions.ErrorRateThreshold
	}
	
	if metrics.ErrorRate > threshold {
		severity := TriggerSeverityMedium
		if metrics.ErrorRate > threshold*2 {
			severity = TriggerSeverityHigh
		}
		if metrics.ErrorRate > threshold*5 {
			severity = TriggerSeverityCritical
		}
		
		return &TriggerEvent{
			TriggerName:   trigger.Name,
			DeploymentRef: deploymentRef,
			Reason:        fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", metrics.ErrorRate, threshold),
			Timestamp:     time.Now(),
			Severity:      severity,
		}
	}
	
	return nil
}

// evaluateTimeoutTrigger evaluates deployment timeouts.
func (tm *TriggerManager) evaluateTimeoutTrigger(ctx context.Context, trigger *RollbackTrigger, deploymentRef corev1.ObjectReference) *TriggerEvent {
	// Check if deployment has been in progressing state too long
	deployment, err := tm.getDeployment(ctx, deploymentRef)
	if err != nil {
		return nil
	}
	
	// Look for Progressing condition
	conditions, exists, err := unstructured.NestedSlice(deployment.Object, "status", "conditions")
	if err != nil || !exists {
		return nil
	}
	
	timeout := 10 * time.Minute // default
	if trigger.Conditions.TimeoutDuration != nil {
		timeout = trigger.Conditions.TimeoutDuration.Duration
	}
	
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		
		condType, _, _ := unstructured.NestedString(condMap, "type")
		condStatus, _, _ := unstructured.NestedString(condMap, "status")
		lastTransition, _, _ := unstructured.NestedString(condMap, "lastTransitionTime")
		
		if condType == "Progressing" && condStatus == "True" {
			transitionTime, err := time.Parse(time.RFC3339, lastTransition)
			if err == nil && time.Since(transitionTime) > timeout {
				return &TriggerEvent{
					TriggerName:   trigger.Name,
					DeploymentRef: deploymentRef,
					Reason:        fmt.Sprintf("Deployment has been progressing for %v (timeout: %v)", time.Since(transitionTime), timeout),
					Timestamp:     time.Now(),
					Severity:      TriggerSeverityHigh,
				}
			}
		}
	}
	
	return nil
}

// evaluateSLOTrigger evaluates SLO violations.
func (tm *TriggerManager) evaluateSLOTrigger(ctx context.Context, trigger *RollbackTrigger, deploymentRef corev1.ObjectReference) *TriggerEvent {
	// This would integrate with SLO monitoring system
	// For now, return nil as placeholder
	klog.V(2).InfoS("SLO trigger evaluation not implemented", "trigger", trigger.Name)
	return nil
}

// checkDeploymentHealth checks the health of all deployments.
func (tm *TriggerManager) checkDeploymentHealth(ctx context.Context) {
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	
	deployments, err := tm.dynamicClient.Resource(deploymentGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "Failed to list deployments for health check")
		return
	}
	
	for _, deployment := range deployments.Items {
		tm.updateHealthMetrics(deployment)
	}
}

// updateHealthMetrics updates health metrics for a deployment.
func (tm *TriggerManager) updateHealthMetrics(deployment unstructured.Unstructured) {
	deploymentRef := corev1.ObjectReference{
		APIVersion: deployment.GetAPIVersion(),
		Kind:       deployment.GetKind(),
		Name:       deployment.GetName(),
		Namespace:  deployment.GetNamespace(),
	}
	
	key := tm.getDeploymentKey(deploymentRef)
	
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	metrics, exists := tm.healthMetrics[key]
	if !exists {
		metrics = &HealthMetrics{
			LastHealthCheck: time.Now(),
		}
		tm.healthMetrics[key] = metrics
	}
	
	// Extract replica counts
	status, exists, _ := unstructured.NestedMap(deployment.Object, "status")
	if !exists {
		return
	}
	
	readyReplicas, _, _ := unstructured.NestedInt64(status, "readyReplicas")
	replicas, _, _ := unstructured.NestedInt64(status, "replicas")
	
	metrics.ReadyReplicas = int32(readyReplicas)
	metrics.DesiredReplicas = int32(replicas)
	metrics.LastHealthCheck = time.Now()
	
	// Determine if health check is passing
	wasHealthy := metrics.HealthCheckPassing
	metrics.HealthCheckPassing = (readyReplicas > 0 && readyReplicas == replicas)
	
	// Update consecutive failures
	if metrics.HealthCheckPassing {
		metrics.ConsecutiveFailures = 0
	} else if !wasHealthy {
		metrics.ConsecutiveFailures++
	} else {
		metrics.ConsecutiveFailures = 1
	}
}

// checkErrorRates checks error rates for deployments.
func (tm *TriggerManager) checkErrorRates(ctx context.Context) {
	// This would integrate with metrics system (Prometheus, etc.)
	// For now, just placeholder logic
	klog.V(2).InfoS("Error rate checking placeholder")
}

// checkTimeouts checks for deployment timeouts.
func (tm *TriggerManager) checkTimeouts(ctx context.Context) {
	// This is handled in evaluateTimeoutTrigger
	klog.V(2).InfoS("Timeout checking placeholder")
}

// handleTriggerEvent handles a trigger event by potentially initiating a rollback.
func (tm *TriggerManager) handleTriggerEvent(ctx context.Context, event TriggerEvent) {
	klog.InfoS("Handling trigger event", "trigger", event.TriggerName, "deployment", event.DeploymentRef.Name, "severity", event.Severity)
	
	// Set cooldown
	tm.setCooldown(event.TriggerName)
	
	// This would integrate with the rollback engine to initiate automatic rollback
	// For now, just log the event
	klog.InfoS("Automatic rollback triggered", 
		"trigger", event.TriggerName,
		"deployment", event.DeploymentRef.Name,
		"reason", event.Reason,
		"severity", event.Severity,
	)
}

// Helper methods

// isInCooldown checks if a trigger is in cooldown period.
func (tm *TriggerManager) isInCooldown(triggerName string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	cooldownEnd, exists := tm.cooldowns[triggerName]
	if !exists {
		return false
	}
	
	return time.Now().Before(cooldownEnd)
}

// setCooldown sets a cooldown period for a trigger.
func (tm *TriggerManager) setCooldown(triggerName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	trigger, exists := tm.activeTriggers[triggerName]
	if !exists {
		return
	}
	
	cooldownDuration := 5 * time.Minute // default
	if trigger.CooldownDuration != nil {
		cooldownDuration = trigger.CooldownDuration.Duration
	}
	
	tm.cooldowns[triggerName] = time.Now().Add(cooldownDuration)
	klog.V(2).InfoS("Set trigger cooldown", "trigger", triggerName, "duration", cooldownDuration)
}

// getDeploymentKey creates a unique key for a deployment.
func (tm *TriggerManager) getDeploymentKey(deploymentRef corev1.ObjectReference) string {
	return fmt.Sprintf("%s/%s", deploymentRef.Namespace, deploymentRef.Name)
}

// getDeployment retrieves a deployment object.
func (tm *TriggerManager) getDeployment(ctx context.Context, deploymentRef corev1.ObjectReference) (*unstructured.Unstructured, error) {
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	
	return tm.dynamicClient.Resource(deploymentGVR).
		Namespace(deploymentRef.Namespace).
		Get(ctx, deploymentRef.Name, metav1.GetOptions{})
}