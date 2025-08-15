// Copyright 2024 The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduler

import "context"

// ClusterTarget represents a target cluster for placement.
type ClusterTarget struct {
	Name      string
	Available Resources
	Capacity  Resources
	Taints    []Taint
	Labels    map[string]string
}

// Resources represents cluster resources.
type Resources struct {
	CPU    int64
	Memory int64
}

// Taint represents a cluster taint.
type Taint struct {
	Key    string
	Value  string
	Effect string
}

// Workload represents a workload to be placed.
type Workload struct {
	Name string
	Spec WorkloadSpec
}

// WorkloadSpec defines the workload specification.
type WorkloadSpec struct {
	Replicas    int32
	Resources   ResourceRequirements
	Tolerations []Toleration
	Affinity    *Affinity
}

// ResourceRequirements specifies workload resource requirements.
type ResourceRequirements struct {
	CPU    int64
	Memory int64
}

// Toleration represents a toleration for taints.
type Toleration struct {
	Key    string
	Value  string
	Effect string
}

// Affinity represents affinity and anti-affinity rules.
type Affinity struct {
	NodeAffinity *NodeAffinity
	AntiAffinity *AntiAffinity
}

// NodeAffinity defines node affinity rules.
type NodeAffinity struct {
	RequiredDuringScheduling  []string
	PreferredDuringScheduling []WeightedPreference
}

// WeightedPreference represents a weighted preference.
type WeightedPreference struct {
	Weight     int32
	Preference string
}

// AntiAffinity defines anti-affinity rules.
type AntiAffinity struct {
	RequiredDuringScheduling []string
}

// PlacementDecision represents the result of scheduling.
type PlacementDecision struct {
	WorkloadName string
	Clusters     []string
	Strategy     string
}

// ScoredTarget represents a scored cluster target.
type ScoredTarget struct {
	Target  ClusterTarget
	Score   float64
	Reason  string
	Details map[string]float64
}

// Algorithm defines the interface for scheduling algorithms.
type Algorithm interface {
	Score(ctx context.Context, workload Workload, targets []ClusterTarget) ([]ScoredTarget, error)
	GetName() string
}

// hasToleration checks if a workload tolerates a taint.
func hasToleration(tolerations []Toleration, taint Taint) bool {
	for _, toleration := range tolerations {
		if toleration.Key == taint.Key &&
			toleration.Value == taint.Value &&
			toleration.Effect == taint.Effect {
			return true
		}
	}
	return false
}