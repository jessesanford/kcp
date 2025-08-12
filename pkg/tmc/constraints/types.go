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

package constraints

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// EngineOptions configures the constraint engine.
type EngineOptions struct {
	MetricsEnabled bool
	DefaultWeight  int32
}

// EvaluationRequest contains the parameters for constraint evaluation.
type EvaluationRequest struct {
	Workload    WorkloadSpec
	Clusters    []ClusterSpec
	Constraints []tmcv1alpha1.PlacementConstraint
}

// EvaluationResult contains the results of constraint evaluation.
type EvaluationResult struct {
	ClusterEvaluations []*ClusterEvaluation
	Conflicts          []Conflict
	Metrics            *EvaluationMetrics
	Timestamp          metav1.Time
}

// ClusterEvaluation represents evaluation results for a single cluster.
type ClusterEvaluation struct {
	ClusterName       string
	Score             float64
	ConstraintResults []*ConstraintEvaluation
	Violations        []string
	Suitable          bool
}

// ConstraintEvaluation represents evaluation of a single constraint.
type ConstraintEvaluation struct {
	ConstraintName string
	Type           tmcv1alpha1.ConstraintType
	Score          float64
	Satisfied      bool
	Reason         string
	Weight         int32
}

// Conflict represents a constraint conflict that requires resolution.
type Conflict struct {
	Type            ConflictType
	ConstraintNames []string
	Description     string
	Resolution      string
}

// ConflictType categorizes different types of constraint conflicts.
type ConflictType string

const (
	ConflictTypeUnsatisfiable   ConflictType = "Unsatisfiable"
	ConflictTypeOverConstrained ConflictType = "OverConstrained"
	ConflictTypeContradictory   ConflictType = "Contradictory"
)

// EvaluationMetrics tracks constraint engine performance.
type EvaluationMetrics struct {
	TotalEvaluations      int64
	AverageEvaluationTime time.Duration
}

// WorkloadSpec represents a workload for constraint evaluation.
type WorkloadSpec struct {
	Name            string
	Namespace       string
	Labels          map[string]string
	NamespaceLabels map[string]string
	Resources       tmcv1alpha1.ResourceRequirements
}

// ClusterSpec represents a cluster for constraint evaluation.
type ClusterSpec struct {
	Name      string
	Location  string
	Labels    map[string]string
	Resources tmcv1alpha1.ResourceRequirements
	Zones     []string
}