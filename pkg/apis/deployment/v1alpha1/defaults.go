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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// RegisterDefaults adds defaulters functions to the given scheme.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&CanaryDeployment{}, func(obj interface{}) { SetObjectDefaults_CanaryDeployment(obj.(*CanaryDeployment)) })
	return nil
}

// SetObjectDefaults_CanaryDeployment sets default values for CanaryDeployment.
func SetObjectDefaults_CanaryDeployment(obj *CanaryDeployment) {
	SetDefaults_CanaryDeploymentSpec(&obj.Spec)
}

// SetDefaults_CanaryDeploymentSpec sets default values for CanaryDeploymentSpec.
func SetDefaults_CanaryDeploymentSpec(obj *CanaryDeploymentSpec) {
	// Set default progress deadline to 30 minutes
	if obj.ProgressDeadlineSeconds == nil {
		obj.ProgressDeadlineSeconds = ptr.To[int32](1800)
	}

	// Set defaults for strategy
	SetDefaults_CanaryStrategy(&obj.Strategy)

	// Set defaults for analysis
	SetDefaults_CanaryAnalysis(&obj.Analysis)
}

// SetDefaults_CanaryStrategy sets default values for CanaryStrategy.
func SetDefaults_CanaryStrategy(obj *CanaryStrategy) {
	// Set default rollout steps
	if len(obj.Steps) == 0 {
		obj.Steps = []int{10, 25, 50, 100}
	}

	// Set default step duration to 5 minutes
	if obj.StepDuration == nil {
		obj.StepDuration = &metav1.Duration{Duration: metav1.Duration{Duration: 5 * 60 * 1000000000}} // 5 minutes in nanoseconds
	}

	// Set default auto-promotion to true
	if obj.AutoPromotion == nil {
		obj.AutoPromotion = ptr.To(true)
	}

	// Set default max unavailable to 25%
	if obj.MaxUnavailable == nil {
		obj.MaxUnavailable = &intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "25%",
		}
	}
}

// SetDefaults_CanaryAnalysis sets default values for CanaryAnalysis.
func SetDefaults_CanaryAnalysis(obj *CanaryAnalysis) {
	// Set default analysis interval to 1 minute
	if obj.Interval == nil {
		obj.Interval = &metav1.Duration{Duration: metav1.Duration{Duration: 60 * 1000000000}} // 1 minute in nanoseconds
	}

	// Set default success threshold to 95%
	if obj.Threshold == nil {
		obj.Threshold = ptr.To(95)
	}

	// Set default weights for metric queries
	for i := range obj.MetricQueries {
		if obj.MetricQueries[i].Weight == nil {
			obj.MetricQueries[i].Weight = ptr.To(10)
		}
	}

	// Set default timeout for webhook checks
	for i := range obj.Webhooks {
		if obj.Webhooks[i].TimeoutSeconds == nil {
			obj.Webhooks[i].TimeoutSeconds = ptr.To[int32](30)
		}
	}
}