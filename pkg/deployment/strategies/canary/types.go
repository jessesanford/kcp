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

package canary

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CanaryDeployment represents a canary deployment resource.
type CanaryDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanaryDeploymentSpec   `json:"spec,omitempty"`
	Status CanaryDeploymentStatus `json:"status,omitempty"`
}

// CanaryDeploymentSpec defines the desired state of CanaryDeployment.
type CanaryDeploymentSpec struct {
	// TrafficPercentages defines the progressive traffic split percentages.
	TrafficPercentages []int32 `json:"trafficPercentages"`
	// AnalysisInterval defines how long to wait before analyzing metrics.
	AnalysisInterval metav1.Duration `json:"analysisInterval"`
	// SuccessThreshold defines the success rate threshold for promotion.
	SuccessThreshold float64 `json:"successThreshold"`
}

// CanaryDeploymentStatus defines the observed state of CanaryDeployment.
type CanaryDeploymentStatus struct {
	// State represents the current canary state.
	State CanaryState `json:"state"`
	// CurrentTrafficPercentage shows the current traffic split.
	CurrentTrafficPercentage int32 `json:"currentTrafficPercentage"`
	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}