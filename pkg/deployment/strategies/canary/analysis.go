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
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

// AnalysisDecision represents the outcome of canary metrics analysis.
type AnalysisDecision string

const (
	// AnalysisDecisionContinue indicates the canary should continue to the next stage.
	AnalysisDecisionContinue AnalysisDecision = "Continue"
	// AnalysisDecisionPromote indicates the canary should be promoted immediately.
	AnalysisDecisionPromote AnalysisDecision = "Promote"
	// AnalysisDecisionRollback indicates the canary should be rolled back.
	AnalysisDecisionRollback AnalysisDecision = "Rollback"
)

// AnalysisResult contains the results of canary metrics analysis.
type AnalysisResult struct {
	// Decision is the recommended action based on analysis.
	Decision AnalysisDecision `json:"decision"`
	// Score represents the overall health score (0-100).
	Score float64 `json:"score"`
	// Reasons provides detailed reasoning for the decision.
	Reasons []string `json:"reasons"`
	// MetricDetails contains individual metric analysis results.
	MetricDetails map[string]MetricAnalysis `json:"metricDetails"`
}

// MetricAnalysis contains analysis results for a specific metric.
type MetricAnalysis struct {
	// Current value of the metric.
	Value float64 `json:"value"`
	// Threshold that was compared against.
	Threshold float64 `json:"threshold"`
	// Healthy indicates if the metric passes the threshold.
	Healthy bool `json:"healthy"`
	// Trend indicates the direction of the metric over time.
	Trend MetricTrend `json:"trend"`
}

// MetricTrend represents the direction of a metric over time.
type MetricTrend string

const (
	// TrendImproving indicates the metric is getting better.
	TrendImproving MetricTrend = "Improving"
	// TrendStable indicates the metric is stable.
	TrendStable MetricTrend = "Stable"
	// TrendDeteriorating indicates the metric is getting worse.
	TrendDeteriorating MetricTrend = "Deteriorating"
	// TrendUnknown indicates the trend cannot be determined.
	TrendUnknown MetricTrend = "Unknown"
)

// AnalysisEngine provides metrics analysis capabilities for canary deployments.
type AnalysisEngine interface {
	// AnalyzeMetrics analyzes collected metrics and returns a decision.
	AnalyzeMetrics(ctx context.Context, metrics *Metrics, canary *CanaryDeployment) (*AnalysisResult, error)
}

// DefaultAnalysisEngine implements the AnalysisEngine interface.
type DefaultAnalysisEngine struct {
	// Config holds analysis configuration.
	config AnalysisConfig
}

// AnalysisConfig contains configuration for metrics analysis.
type AnalysisConfig struct {
	// SuccessRateWeight is the weight given to success rate in the overall score.
	SuccessRateWeight float64
	// LatencyWeight is the weight given to latency in the overall score.
	LatencyWeight float64
	// ErrorRateWeight is the weight given to error rate in the overall score.
	ErrorRateWeight float64
	// MinimumDataPoints is the minimum number of data points required for analysis.
	MinimumDataPoints int
	// PromotionScore is the minimum score required for automatic promotion.
	PromotionScore float64
}

// NewDefaultAnalysisEngine creates a new default analysis engine.
func NewDefaultAnalysisEngine() AnalysisEngine {
	return &DefaultAnalysisEngine{
		config: AnalysisConfig{
			SuccessRateWeight: 0.4,
			LatencyWeight:     0.3,
			ErrorRateWeight:   0.3,
			MinimumDataPoints: 10,
			PromotionScore:    85.0,
		},
	}
}

// AnalyzeMetrics performs comprehensive analysis of canary metrics.
func (e *DefaultAnalysisEngine) AnalyzeMetrics(ctx context.Context, metrics *Metrics, canary *CanaryDeployment) (*AnalysisResult, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Starting canary metrics analysis", "canaryName", canary.Name)

	// Validate sufficient data points
	if !e.hasSufficientData(metrics) {
		return &AnalysisResult{
			Decision: AnalysisDecisionContinue,
			Score:    0,
			Reasons:  []string{"Insufficient data points for analysis"},
			MetricDetails: make(map[string]MetricAnalysis),
		}, nil
	}

	result := &AnalysisResult{
		MetricDetails: make(map[string]MetricAnalysis),
		Reasons:       []string{},
	}

	// Analyze individual metrics
	successRateAnalysis := e.analyzeSuccessRate(metrics, canary)
	result.MetricDetails["success_rate"] = successRateAnalysis

	latencyAnalysis := e.analyzeLatency(metrics, canary)
	result.MetricDetails["latency"] = latencyAnalysis

	errorRateAnalysis := e.analyzeErrorRate(metrics, canary)
	result.MetricDetails["error_rate"] = errorRateAnalysis

	// Calculate overall score
	result.Score = e.calculateOverallScore(successRateAnalysis, latencyAnalysis, errorRateAnalysis)

	// Make decision based on analysis
	result.Decision, result.Reasons = e.makeDecision(result.Score, canary, result.MetricDetails)

	logger.V(2).Info("Analysis completed",
		"score", result.Score,
		"decision", result.Decision,
		"reasons", result.Reasons,
	)

	return result, nil
}

// hasSufficientData checks if there are enough data points for reliable analysis.
func (e *DefaultAnalysisEngine) hasSufficientData(metrics *Metrics) bool {
	return len(metrics.RequestCount.DataPoints) >= e.config.MinimumDataPoints
}

// analyzeSuccessRate analyzes the success rate metric.
func (e *DefaultAnalysisEngine) analyzeSuccessRate(metrics *Metrics, canary *CanaryDeployment) MetricAnalysis {
	if len(metrics.SuccessRate.DataPoints) == 0 {
		return MetricAnalysis{
			Value:     0,
			Threshold: canary.Spec.SuccessThreshold,
			Healthy:   false,
			Trend:     TrendUnknown,
		}
	}

	current := metrics.SuccessRate.DataPoints[len(metrics.SuccessRate.DataPoints)-1].Value
	healthy := current >= canary.Spec.SuccessThreshold
	trend := e.calculateTrend(metrics.SuccessRate.DataPoints)

	return MetricAnalysis{
		Value:     current,
		Threshold: canary.Spec.SuccessThreshold,
		Healthy:   healthy,
		Trend:     trend,
	}
}

// analyzeLatency analyzes the latency metric.
func (e *DefaultAnalysisEngine) analyzeLatency(metrics *Metrics, canary *CanaryDeployment) MetricAnalysis {
	if len(metrics.Latency.DataPoints) == 0 {
		return MetricAnalysis{
			Value:     0,
			Threshold: 1000, // Default 1000ms threshold
			Healthy:   false,
			Trend:     TrendUnknown,
		}
	}

	current := metrics.Latency.DataPoints[len(metrics.Latency.DataPoints)-1].Value
	threshold := 1000.0 // 1 second default threshold
	healthy := current <= threshold
	trend := e.calculateTrend(metrics.Latency.DataPoints)

	return MetricAnalysis{
		Value:     current,
		Threshold: threshold,
		Healthy:   healthy,
		Trend:     trend,
	}
}

// analyzeErrorRate analyzes the error rate metric.
func (e *DefaultAnalysisEngine) analyzeErrorRate(metrics *Metrics, canary *CanaryDeployment) MetricAnalysis {
	if len(metrics.ErrorRate.DataPoints) == 0 {
		return MetricAnalysis{
			Value:     0,
			Threshold: canary.Spec.RollbackThreshold,
			Healthy:   true,
			Trend:     TrendUnknown,
		}
	}

	current := metrics.ErrorRate.DataPoints[len(metrics.ErrorRate.DataPoints)-1].Value
	healthy := current <= canary.Spec.RollbackThreshold
	trend := e.calculateTrend(metrics.ErrorRate.DataPoints)

	return MetricAnalysis{
		Value:     current,
		Threshold: canary.Spec.RollbackThreshold,
		Healthy:   healthy,
		Trend:     trend,
	}
}

// calculateTrend determines the trend direction for a metric.
func (e *DefaultAnalysisEngine) calculateTrend(dataPoints []DataPoint) MetricTrend {
	if len(dataPoints) < 3 {
		return TrendUnknown
	}

	// Calculate simple moving average trend
	recent := dataPoints[len(dataPoints)-3:]
	total := 0.0
	for _, point := range recent {
		total += point.Value
	}
	recentAvg := total / float64(len(recent))

	older := dataPoints[len(dataPoints)-6 : len(dataPoints)-3]
	if len(older) == 0 {
		return TrendUnknown
	}

	total = 0.0
	for _, point := range older {
		total += point.Value
	}
	olderAvg := total / float64(len(older))

	diff := recentAvg - olderAvg
	threshold := 0.05 // 5% change threshold

	if diff > threshold {
		return TrendDeteriorating
	} else if diff < -threshold {
		return TrendImproving
	}
	return TrendStable
}

// calculateOverallScore computes a weighted overall health score.
func (e *DefaultAnalysisEngine) calculateOverallScore(successRate, latency, errorRate MetricAnalysis) float64 {
	var score float64

	// Success rate contribution (higher is better)
	if successRate.Healthy {
		score += e.config.SuccessRateWeight * 100
	} else {
		// Partial credit based on how close to threshold
		ratio := successRate.Value / successRate.Threshold
		if ratio > 1 {
			ratio = 1
		}
		score += e.config.SuccessRateWeight * ratio * 100
	}

	// Latency contribution (lower is better)
	if latency.Healthy {
		score += e.config.LatencyWeight * 100
	} else {
		// Partial credit based on how far from threshold
		ratio := latency.Threshold / latency.Value
		if ratio > 1 {
			ratio = 1
		}
		score += e.config.LatencyWeight * ratio * 100
	}

	// Error rate contribution (lower is better)
	if errorRate.Healthy {
		score += e.config.ErrorRateWeight * 100
	} else {
		// Penalty based on error rate
		ratio := errorRate.Threshold / errorRate.Value
		if ratio > 1 {
			ratio = 1
		}
		score += e.config.ErrorRateWeight * ratio * 100
	}

	return score
}

// makeDecision determines the appropriate action based on analysis results.
func (e *DefaultAnalysisEngine) makeDecision(score float64, canary *CanaryDeployment, details map[string]MetricAnalysis) (AnalysisDecision, []string) {
	var reasons []string

	// Check for immediate rollback conditions
	if details["error_rate"].Value > canary.Spec.RollbackThreshold {
		reasons = append(reasons, fmt.Sprintf("Error rate %.2f%% exceeds rollback threshold %.2f%%", 
			details["error_rate"].Value*100, canary.Spec.RollbackThreshold*100))
		return AnalysisDecisionRollback, reasons
	}

	if details["success_rate"].Value < canary.Spec.SuccessThreshold*0.5 {
		reasons = append(reasons, fmt.Sprintf("Success rate %.2f%% critically below threshold %.2f%%", 
			details["success_rate"].Value*100, canary.Spec.SuccessThreshold*100))
		return AnalysisDecisionRollback, reasons
	}

	// Check for promotion conditions
	if score >= e.config.PromotionScore {
		reasons = append(reasons, fmt.Sprintf("Overall score %.1f meets promotion threshold %.1f", 
			score, e.config.PromotionScore))
		
		// Additional check for trending
		if details["success_rate"].Trend == TrendImproving || details["success_rate"].Trend == TrendStable {
			reasons = append(reasons, "Success rate trend is stable or improving")
			return AnalysisDecisionPromote, reasons
		}
	}

	// Default to continue
	reasons = append(reasons, fmt.Sprintf("Overall score %.1f indicates healthy canary, continuing progression", score))
	return AnalysisDecisionContinue, reasons
}