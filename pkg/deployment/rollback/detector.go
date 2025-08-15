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
	"time"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// FailureDetector monitors deployment health and triggers rollbacks when failures are detected.
type FailureDetector interface {
	// DetectFailure checks if a deployment has failed and should trigger a rollback.
	DetectFailure(ctx context.Context, deployment *DeploymentState) (*FailureAnalysis, error)

	// ShouldRollback determines if conditions warrant an automatic rollback.
	ShouldRollback(ctx context.Context, analysis *FailureAnalysis) bool

	// RegisterFailureThreshold sets failure thresholds for different failure types.
	RegisterFailureThreshold(failureType string, threshold FailureThreshold) error
}

// DeploymentState represents the current state of a deployment for failure analysis.
type DeploymentState struct {
	Name               string
	Namespace          string
	LogicalCluster     logicalcluster.Name
	CurrentRevision    int64
	PreviousRevision   int64
	DesiredReplicas    int32
	ReadyReplicas      int32
	AvailableReplicas  int32
	FailedReplicas     int32
	LastUpdateTime     metav1.Time
	Conditions         []metav1.Condition
	HealthCheckResults []HealthCheckResult
}

// FailureAnalysis contains the analysis results from failure detection.
type FailureAnalysis struct {
	DeploymentName      string
	LogicalCluster      logicalcluster.Name
	FailureTypes        []FailureType
	Severity            FailureSeverity
	FailureReasons      []string
	RollbackRecommended bool
	ConfidenceLevel     float64
	DetectionTime       metav1.Time
}

// FailureType represents different types of deployment failures.
type FailureType string

const (
	FailureTypeAvailability FailureType = "Availability"
	FailureTypePerformance  FailureType = "Performance"
	FailureTypeHealth       FailureType = "Health"
	FailureTypeDependency   FailureType = "Dependency"
	FailureTypeResource     FailureType = "Resource"
)

// FailureSeverity indicates the severity level of detected failures.
type FailureSeverity string

const (
	SeverityCritical FailureSeverity = "Critical"
	SeverityHigh     FailureSeverity = "High"
	SeverityMedium   FailureSeverity = "Medium"
	SeverityLow      FailureSeverity = "Low"
)

// FailureThreshold defines thresholds for triggering rollbacks.
type FailureThreshold struct {
	MaxFailureRate     float64
	MaxFailureDuration time.Duration
	MinHealthyReplicas int32
	MinConfidenceLevel float64
}

// HealthCheckResult represents results from health checks.
type HealthCheckResult struct {
	CheckName    string
	Success      bool
	Message      string
	Timestamp    metav1.Time
	ResponseTime time.Duration
}

// detector implements the FailureDetector interface.
type detector struct {
	logger     logr.Logger
	thresholds map[string]FailureThreshold
}

// NewFailureDetector creates a new failure detector instance.
func NewFailureDetector() FailureDetector {
	return &detector{
		logger:     klog.Background(),
		thresholds: make(map[string]FailureThreshold),
	}
}

// DetectFailure analyzes deployment state and detects failures.
func (d *detector) DetectFailure(ctx context.Context, deployment *DeploymentState) (*FailureAnalysis, error) {
	logger := d.logger.WithValues("deployment", deployment.Name, "cluster", deployment.LogicalCluster)
	logger.V(2).Info("detecting deployment failures")

	analysis := &FailureAnalysis{
		DeploymentName:  deployment.Name,
		LogicalCluster:  deployment.LogicalCluster,
		FailureTypes:    []FailureType{},
		FailureReasons:  []string{},
		DetectionTime:   metav1.Now(),
		ConfidenceLevel: 0.0,
	}

	// Check availability failures
	if deployment.AvailableReplicas < deployment.DesiredReplicas/2 {
		analysis.FailureTypes = append(analysis.FailureTypes, FailureTypeAvailability)
		analysis.FailureReasons = append(analysis.FailureReasons,
			fmt.Sprintf("Available replicas (%d) below 50%% of desired (%d)",
				deployment.AvailableReplicas, deployment.DesiredReplicas))
		analysis.ConfidenceLevel += 0.3
	}

	// Check failed replicas
	if deployment.FailedReplicas > 0 {
		analysis.FailureTypes = append(analysis.FailureTypes, FailureTypeHealth)
		analysis.FailureReasons = append(analysis.FailureReasons,
			fmt.Sprintf("%d replicas in failed state", deployment.FailedReplicas))
		analysis.ConfidenceLevel += 0.4
	}

	// Check health check failures
	failedHealthChecks := 0
	for _, result := range deployment.HealthCheckResults {
		if !result.Success {
			failedHealthChecks++
		}
	}

	if failedHealthChecks > len(deployment.HealthCheckResults)/2 {
		analysis.FailureTypes = append(analysis.FailureTypes, FailureTypeHealth)
		analysis.FailureReasons = append(analysis.FailureReasons,
			fmt.Sprintf("%d of %d health checks failing", failedHealthChecks, len(deployment.HealthCheckResults)))
		analysis.ConfidenceLevel += 0.3
	}

	// Determine severity based on failure types and confidence
	analysis.Severity = d.calculateSeverity(analysis)
	analysis.RollbackRecommended = d.shouldRecommendRollback(analysis)

	logger.V(2).Info("failure detection completed",
		"failureTypes", analysis.FailureTypes,
		"severity", analysis.Severity,
		"confidence", analysis.ConfidenceLevel,
		"rollbackRecommended", analysis.RollbackRecommended)

	return analysis, nil
}

// ShouldRollback determines if automatic rollback should be triggered.
func (d *detector) ShouldRollback(ctx context.Context, analysis *FailureAnalysis) bool {
	logger := d.logger.WithValues("deployment", analysis.DeploymentName, "cluster", analysis.LogicalCluster)

	if !analysis.RollbackRecommended {
		return false
	}

	// Check if severity is high enough
	if analysis.Severity != SeverityCritical && analysis.Severity != SeverityHigh {
		logger.V(2).Info("severity not high enough for automatic rollback", "severity", analysis.Severity)
		return false
	}

	// Check confidence level
	if analysis.ConfidenceLevel < 0.7 {
		logger.V(2).Info("confidence level too low for automatic rollback", "confidence", analysis.ConfidenceLevel)
		return false
	}

	logger.Info("automatic rollback recommended",
		"severity", analysis.Severity,
		"confidence", analysis.ConfidenceLevel)

	return true
}

// RegisterFailureThreshold registers failure thresholds for different failure types.
func (d *detector) RegisterFailureThreshold(failureType string, threshold FailureThreshold) error {
	d.thresholds[failureType] = threshold
	d.logger.V(2).Info("registered failure threshold", "type", failureType, "threshold", threshold)
	return nil
}

// calculateSeverity determines the severity level based on failure analysis.
func (d *detector) calculateSeverity(analysis *FailureAnalysis) FailureSeverity {
	failureTypeSet := sets.NewString()
	for _, ft := range analysis.FailureTypes {
		failureTypeSet.Insert(string(ft))
	}

	// Critical if multiple failure types or availability issues
	if failureTypeSet.Len() >= 2 || failureTypeSet.Has(string(FailureTypeAvailability)) {
		if analysis.ConfidenceLevel >= 0.8 {
			return SeverityCritical
		}
		return SeverityHigh
	}

	// High if health or dependency failures
	if failureTypeSet.Has(string(FailureTypeHealth)) || failureTypeSet.Has(string(FailureTypeDependency)) {
		if analysis.ConfidenceLevel >= 0.7 {
			return SeverityHigh
		}
		return SeverityMedium
	}

	// Medium for performance or resource issues
	if failureTypeSet.Has(string(FailureTypePerformance)) || failureTypeSet.Has(string(FailureTypeResource)) {
		return SeverityMedium
	}

	return SeverityLow
}

// shouldRecommendRollback determines if rollback should be recommended based on analysis.
func (d *detector) shouldRecommendRollback(analysis *FailureAnalysis) bool {
	if len(analysis.FailureTypes) == 0 {
		return false
	}

	// Recommend rollback for critical or high severity failures with high confidence
	if (analysis.Severity == SeverityCritical || analysis.Severity == SeverityHigh) &&
		analysis.ConfidenceLevel >= 0.6 {
		return true
	}

	return false
}
