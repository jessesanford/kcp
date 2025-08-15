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

package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dependency represents a deployment dependency
type Dependency struct {
	// Name of the dependency
	Name string `json:"name"`

	// Type of dependency
	Type DependencyType `json:"type"`

	// Target resource
	Target DependencyTarget `json:"target"`

	// Condition to satisfy
	Condition string `json:"condition,omitempty"`

	// Timeout for dependency resolution
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// DependencyType defines the type of dependency
type DependencyType string

const (
	HardDependency DependencyType = "Hard" // Must be satisfied
	SoftDependency DependencyType = "Soft" // Best effort
)

// DependencyTarget identifies the dependency target
type DependencyTarget struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
}

// DependencyGraph represents deployment dependencies
type DependencyGraph struct {
	Nodes map[string]*DependencyNode `json:"nodes"`
	Edges []DependencyEdge           `json:"edges"`
}

// DependencyNode is a node in the dependency graph
type DependencyNode struct {
	ID        string           `json:"id"`
	Resource  DependencyTarget `json:"resource"`
	Status    DependencyStatus `json:"status"`
	StartTime *metav1.Time     `json:"startTime,omitempty"`
	EndTime   *metav1.Time     `json:"endTime,omitempty"`
}

// DependencyEdge represents a dependency relationship
type DependencyEdge struct {
	From string         `json:"from"`
	To   string         `json:"to"`
	Type DependencyType `json:"type"`
}

// DependencyStatus represents the state of a dependency
type DependencyStatus string

const (
	DependencyPending DependencyStatus = "Pending"
	DependencyReady   DependencyStatus = "Ready"
	DependencyFailed  DependencyStatus = "Failed"
	DependencySkipped DependencyStatus = "Skipped"
)

// DependencyResolverConfig configuration
type DependencyResolverConfig struct {
	// MaxConcurrency limits parallel operations
	MaxConcurrency int `json:"maxConcurrency"`

	// RetryPolicy for failed dependencies
	RetryPolicy RetryPolicy `json:"retryPolicy"`

	// IgnoreSoftFailures continues on soft dependency failures
	IgnoreSoftFailures bool `json:"ignoreSoftFailures"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries int             `json:"maxRetries"`
	Backoff    BackoffStrategy `json:"backoff"`
}

// BackoffStrategy for retries
type BackoffStrategy struct {
	Type     string          `json:"type"` // exponential, linear, fixed
	Interval metav1.Duration `json:"interval"`
	MaxDelay metav1.Duration `json:"maxDelay,omitempty"`
}