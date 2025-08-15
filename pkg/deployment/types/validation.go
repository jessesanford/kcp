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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
)

// NOTE: These validation methods extend the types defined in the core-types split (14a).
// They are separated to keep the core types lean while providing comprehensive validation.

// Validate validates the deployment strategy configuration
func (ds *DeploymentStrategy) Validate() error {
	if ds == nil {
		return errors.New("deployment strategy cannot be nil")
	}

	// Validate strategy type
	switch ds.Type {
	case CanaryStrategyType:
		if ds.Canary == nil {
			return errors.New("canary configuration required for Canary strategy type")
		}
		if err := ds.Canary.Validate(); err != nil {
			return fmt.Errorf("invalid canary configuration: %w", err)
		}
	case BlueGreenStrategyType:
		if ds.BlueGreen == nil {
			return errors.New("blue-green configuration required for BlueGreen strategy type")
		}
		if err := ds.BlueGreen.Validate(); err != nil {
			return fmt.Errorf("invalid blue-green configuration: %w", err)
		}
	case RollingUpdateStrategyType:
		if ds.RollingUpdate != nil {
			if err := ds.RollingUpdate.Validate(); err != nil {
				return fmt.Errorf("invalid rolling update configuration: %w", err)
			}
		}
	case RecreateStrategyType:
		// No additional validation needed
	default:
		return fmt.Errorf("unknown strategy type: %s", ds.Type)
	}

	// Validate health check if present
	if ds.HealthCheck != nil {
		if err := ds.HealthCheck.Validate(); err != nil {
			return fmt.Errorf("invalid health check configuration: %w", err)
		}
	}

	return nil
}

// Validate validates the canary strategy configuration
func (cs *CanaryStrategy) Validate() error {
	if cs == nil {
		return errors.New("canary strategy cannot be nil")
	}

	if len(cs.Steps) == 0 {
		return errors.New("canary strategy must have at least one step")
	}

	var totalWeight int32
	for i, step := range cs.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid canary step %d: %w", i, err)
		}
		
		// Check for weight progression
		if step.Weight <= totalWeight && i > 0 {
			return fmt.Errorf("canary step %d weight (%d) must be greater than previous total (%d)", 
				i, step.Weight, totalWeight)
		}
		totalWeight = step.Weight
	}

	// Final step should reach 100%
	if totalWeight != 100 {
		return fmt.Errorf("final canary step should reach 100%% weight, got %d%%", totalWeight)
	}

	// Validate analysis if present
	if cs.Analysis != nil {
		if err := cs.Analysis.Validate(); err != nil {
			return fmt.Errorf("invalid analysis configuration: %w", err)
		}
	}

	// Validate traffic routing if present
	if cs.TrafficRouting != nil {
		if err := cs.TrafficRouting.Validate(); err != nil {
			return fmt.Errorf("invalid traffic routing configuration: %w", err)
		}
	}

	return nil
}

// Validate validates a canary step
func (cs *CanaryStep) Validate() error {
	if cs.Weight < 0 || cs.Weight > 100 {
		return fmt.Errorf("canary step weight must be between 0 and 100, got %d", cs.Weight)
	}

	if cs.Replicas != nil && *cs.Replicas < 0 {
		return fmt.Errorf("canary step replicas cannot be negative")
	}

	if cs.Pause != nil && cs.Pause.Duration < 0 {
		return fmt.Errorf("canary step pause duration cannot be negative")
	}

	return nil
}

// Validate validates the blue-green strategy configuration
func (bgs *BlueGreenStrategy) Validate() error {
	if bgs == nil {
		return errors.New("blue-green strategy cannot be nil")
	}

	if bgs.ActiveService == "" {
		return errors.New("active service name is required")
	}

	if bgs.PreviewService == "" {
		return errors.New("preview service name is required")
	}

	if bgs.AutoPromotionSeconds < 0 {
		return errors.New("auto promotion seconds cannot be negative")
	}

	if bgs.ScaleDownDelaySeconds < 0 {
		return errors.New("scale down delay seconds cannot be negative")
	}

	// Validate analysis configurations if present
	if bgs.PrePromotionAnalysis != nil {
		if err := bgs.PrePromotionAnalysis.Validate(); err != nil {
			return fmt.Errorf("invalid pre-promotion analysis: %w", err)
		}
	}

	if bgs.PostPromotionAnalysis != nil {
		if err := bgs.PostPromotionAnalysis.Validate(); err != nil {
			return fmt.Errorf("invalid post-promotion analysis: %w", err)
		}
	}

	return nil
}

// Validate validates the rolling update strategy configuration
func (rus *RollingUpdateStrategy) Validate() error {
	if rus == nil {
		return nil // Rolling update with defaults is valid
	}

	if rus.MaxUnavailable != nil {
		if err := validateIntOrString(rus.MaxUnavailable); err != nil {
			return fmt.Errorf("invalid maxUnavailable: %w", err)
		}
	}

	if rus.MaxSurge != nil {
		if err := validateIntOrString(rus.MaxSurge); err != nil {
			return fmt.Errorf("invalid maxSurge: %w", err)
		}
	}

	return nil
}

// Validate validates the analysis configuration
func (ac *AnalysisConfig) Validate() error {
	if ac == nil {
		return errors.New("analysis config cannot be nil")
	}

	if len(ac.Metrics) == 0 {
		return errors.New("at least one metric is required for analysis")
	}

	for i, metric := range ac.Metrics {
		if err := metric.Validate(); err != nil {
			return fmt.Errorf("invalid metric %d: %w", i, err)
		}
	}

	if ac.Interval.Duration <= 0 {
		return errors.New("analysis interval must be positive")
	}

	// Validate CEL expression if present
	if ac.SuccessCondition != "" {
		if err := validateCELExpression(ac.SuccessCondition); err != nil {
			return fmt.Errorf("invalid success condition expression: %w", err)
		}
	}

	return nil
}

// Validate validates a metric configuration
func (mc *MetricConfig) Validate() error {
	if mc.Name == "" {
		return errors.New("metric name is required")
	}

	if mc.Threshold < 0 {
		return fmt.Errorf("metric threshold cannot be negative")
	}

	// Validate query if present (basic check)
	if mc.Query != "" && len(mc.Query) > 10000 {
		return errors.New("metric query is too long (max 10000 characters)")
	}

	return nil
}

// Validate validates traffic routing configuration
func (tr *TrafficRouting) Validate() error {
	if tr == nil {
		return errors.New("traffic routing cannot be nil")
	}

	configCount := 0
	if tr.Istio != nil {
		configCount++
		if err := tr.Istio.Validate(); err != nil {
			return fmt.Errorf("invalid Istio configuration: %w", err)
		}
	}

	if tr.SMI != nil {
		configCount++
		if tr.SMI.TrafficSplitName == "" {
			return errors.New("SMI traffic split name is required")
		}
	}

	if configCount == 0 {
		return errors.New("at least one traffic routing configuration is required")
	}

	if configCount > 1 {
		return errors.New("only one traffic routing configuration can be specified")
	}

	return nil
}

// Validate validates Istio traffic routing configuration
func (itr *IstioTrafficRouting) Validate() error {
	if itr.VirtualService == "" {
		return errors.New("Istio virtual service name is required")
	}

	// Validate Kubernetes resource names
	if errs := validation.IsDNS1123Subdomain(itr.VirtualService); len(errs) > 0 {
		return fmt.Errorf("invalid virtual service name: %s", strings.Join(errs, ", "))
	}

	if itr.DestinationRule != "" {
		if errs := validation.IsDNS1123Subdomain(itr.DestinationRule); len(errs) > 0 {
			return fmt.Errorf("invalid destination rule name: %s", strings.Join(errs, ", "))
		}
	}

	return nil
}

// Validate validates health check configuration
func (hc *HealthCheckConfig) Validate() error {
	if hc == nil {
		return nil // Health check is optional
	}

	if hc.InitialDelaySeconds < 0 {
		return errors.New("initial delay seconds cannot be negative")
	}

	if hc.PeriodSeconds <= 0 {
		return errors.New("period seconds must be positive")
	}

	if hc.TimeoutSeconds <= 0 {
		return errors.New("timeout seconds must be positive")
	}

	if hc.SuccessThreshold <= 0 {
		return errors.New("success threshold must be positive")
	}

	if hc.FailureThreshold <= 0 {
		return errors.New("failure threshold must be positive")
	}

	// Validate probe configuration
	probeCount := 0
	if hc.HTTPGet != nil {
		probeCount++
		if hc.HTTPGet.Path == "" {
			return errors.New("HTTP probe path is required")
		}
		if hc.HTTPGet.Port <= 0 || hc.HTTPGet.Port > 65535 {
			return fmt.Errorf("HTTP probe port must be between 1 and 65535")
		}
	}

	if hc.TCPSocket != nil {
		probeCount++
		if hc.TCPSocket.Port <= 0 || hc.TCPSocket.Port > 65535 {
			return fmt.Errorf("TCP probe port must be between 1 and 65535")
		}
	}

	if hc.Exec != nil {
		probeCount++
		if len(hc.Exec.Command) == 0 {
			return errors.New("exec probe command is required")
		}
	}

	if probeCount == 0 {
		return errors.New("at least one probe type must be specified")
	}

	if probeCount > 1 {
		return errors.New("only one probe type can be specified")
	}

	return nil
}

// Helper functions

// validateIntOrString validates an IntOrString value
func validateIntOrString(val *intstr.IntOrString) error {
	if val == nil {
		return nil
	}

	if val.Type == intstr.String {
		str := val.StrVal
		if !strings.HasSuffix(str, "%") {
			return fmt.Errorf("string value must be a percentage (e.g., '25%%'), got %s", str)
		}
		// Parse percentage
		percentStr := strings.TrimSuffix(str, "%")
		var percent int
		if _, err := fmt.Sscanf(percentStr, "%d", &percent); err != nil {
			return fmt.Errorf("invalid percentage value: %s", str)
		}
		if percent < 0 || percent > 100 {
			return fmt.Errorf("percentage must be between 0 and 100, got %d", percent)
		}
	} else {
		if val.IntVal < 0 {
			return fmt.Errorf("integer value cannot be negative")
		}
	}

	return nil
}

// validateCELExpression performs basic validation of a CEL expression
func validateCELExpression(expr string) error {
	if expr == "" {
		return errors.New("CEL expression cannot be empty")
	}

	// Basic syntax checks
	if strings.Count(expr, "(") != strings.Count(expr, ")") {
		return errors.New("mismatched parentheses in expression")
	}

	if strings.Count(expr, "[") != strings.Count(expr, "]") {
		return errors.New("mismatched brackets in expression")
	}

	if strings.Count(expr, "{") != strings.Count(expr, "}") {
		return errors.New("mismatched braces in expression")
	}

	// Check for common CEL keywords
	celKeywords := []string{"&&", "||", "!", "==", "!=", "<", ">", "<=", ">=", "in", "has"}
	hasKeyword := false
	for _, keyword := range celKeywords {
		if strings.Contains(expr, keyword) {
			hasKeyword = true
			break
		}
	}

	if !hasKeyword && !strings.Contains(expr, ".") {
		return errors.New("expression appears to be invalid CEL syntax")
	}

	return nil
}