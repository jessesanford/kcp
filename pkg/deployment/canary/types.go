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

package canary

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/metrics"
)

// CanaryManager defines the interface for managing canary deployments
// with integrated metrics analysis and traffic management.
type CanaryManager interface {
	// AnalyzeCanary performs metrics-based analysis of a canary deployment
	AnalyzeCanary(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (AnalysisResult, error)
	
	// UpdateTraffic adjusts traffic distribution between stable and canary versions
	UpdateTraffic(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, percentage int) error
	
	// PromoteCanary promotes the canary to the next step or to completion
	PromoteCanary(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error
	
	// RollbackCanary performs rollback to the stable version
	RollbackCanary(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error
}

// StateManager defines the interface for managing canary deployment state transitions.
type StateManager interface {
	// GetCurrentState returns the current state of the canary deployment
	GetCurrentState(canary *deploymentv1alpha1.CanaryDeployment) CanaryState
	
	// TransitionTo attempts to transition the canary to a new state
	TransitionTo(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, newState CanaryState) error
	
	// ShouldTransition checks if the canary should transition to the next state
	ShouldTransition(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (bool, CanaryState, error)
}

// MetricsAnalyzer defines the interface for canary metrics analysis using Wave 1 metrics.
type MetricsAnalyzer interface {
	// AnalyzeMetrics performs analysis of canary metrics against configured thresholds
	AnalyzeMetrics(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) ([]deploymentv1alpha1.AnalysisResult, error)
	
	// QueryMetric queries a specific metric from the metrics system
	QueryMetric(ctx context.Context, query string, labels map[string]string) (float64, error)
	
	// GetHealthScore calculates an overall health score for the canary
	GetHealthScore(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (float64, error)
}

// TrafficManager defines the interface for managing traffic distribution in canary deployments.
type TrafficManager interface {
	// SetTrafficWeight configures traffic distribution between versions
	SetTrafficWeight(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, canaryWeight int) error
	
	// GetCurrentTrafficWeights returns the current traffic distribution
	GetCurrentTrafficWeights(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (canaryWeight, stableWeight int, err error)
	
	// ValidateTrafficConfig validates the traffic configuration for the canary
	ValidateTrafficConfig(canary *deploymentv1alpha1.CanaryDeployment) error
}

// CanaryState represents the state of a canary deployment in the state machine.
type CanaryState struct {
	// Phase represents the current phase of the canary
	Phase deploymentv1alpha1.CanaryPhase
	
	// Step represents the current step in the rollout strategy
	Step int
	
	// StepStartTime represents when the current step began
	StepStartTime metav1.Time
	
	// Message provides context about the current state
	Message string
	
	// LastTransitionTime represents when the last state transition occurred
	LastTransitionTime metav1.Time
}

// AnalysisResult represents the result of a canary analysis.
type AnalysisResult struct {
	// Success indicates whether the analysis passed overall
	Success bool
	
	// Score represents the overall health score (0-100)
	Score float64
	
	// MetricResults contains the results for individual metrics
	MetricResults []deploymentv1alpha1.AnalysisResult
	
	// Message provides details about the analysis result
	Message string
	
	// Timestamp indicates when the analysis was performed
	Timestamp metav1.Time
}

// CanaryConfiguration holds configuration for canary operations.
type CanaryConfiguration struct {
	// MetricsRegistry provides access to the Wave 1 metrics system
	MetricsRegistry *metrics.MetricsRegistry
	
	// DefaultAnalysisInterval specifies the default interval between analyses
	DefaultAnalysisInterval time.Duration
	
	// DefaultStepDuration specifies the default duration for each rollout step
	DefaultStepDuration time.Duration
	
	// DefaultSuccessThreshold specifies the default success threshold percentage
	DefaultSuccessThreshold int
	
	// MaxAnalysisAttempts specifies the maximum number of analysis attempts before failure
	MaxAnalysisAttempts int
	
	// EnableWebhookChecks indicates whether webhook-based checks are enabled
	EnableWebhookChecks bool
}

// DeploymentInfo contains information about the target deployment.
type DeploymentInfo struct {
	// Deployment is the target deployment object
	Deployment *appsv1.Deployment
	
	// CanaryReplicas represents the number of replicas for the canary version
	CanaryReplicas int32
	
	// StableReplicas represents the number of replicas for the stable version
	StableReplicas int32
	
	// CanarySelector contains the label selector for canary pods
	CanarySelector map[string]string
	
	// StableSelector contains the label selector for stable pods
	StableSelector map[string]string
}

// MetricQuery represents a query for metrics analysis.
type MetricQueryInfo struct {
	// Name is the name of the metric query
	Name string
	
	// Query is the actual query string (PromQL format)
	Query string
	
	// Labels contains additional labels for the query
	Labels map[string]string
	
	// ExpectedResult contains the expected result configuration
	ExpectedResult deploymentv1alpha1.MetricQuery
}

// StateTransition represents a state transition in the canary deployment.
type StateTransition struct {
	// From represents the source state
	From CanaryState
	
	// To represents the target state
	To CanaryState
	
	// Reason provides the reason for the transition
	Reason string
	
	// Timestamp indicates when the transition occurred
	Timestamp metav1.Time
}

// CanaryEvent represents an event in the canary deployment lifecycle.
type CanaryEvent struct {
	// Type indicates the type of event (Normal, Warning)
	Type string
	
	// Reason provides the reason for the event
	Reason string
	
	// Message provides details about the event
	Message string
	
	// Timestamp indicates when the event occurred
	Timestamp metav1.Time
	
	// Component indicates which component generated the event
	Component string
}