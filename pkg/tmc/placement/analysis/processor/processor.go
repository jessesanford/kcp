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

package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// AnalysisDataProcessor processes analysis data from WorkloadAnalysisRun resources
// and provides functionality for data transformation, validation, and computation.
type AnalysisDataProcessor struct {
	// metrics tracks processing metrics
	metrics *ProcessingMetrics

	// config holds processing configuration
	config *ProcessorConfig

	// mu protects concurrent access to processor state
	mu sync.RWMutex
}

// ProcessorConfig defines configuration for the analysis data processor.
type ProcessorConfig struct {
	// MaxConcurrentProcessing defines maximum concurrent processing jobs
	MaxConcurrentProcessing int

	// ProcessingTimeout defines timeout for individual processing operations
	ProcessingTimeout time.Duration

	// RetryAttempts defines number of retry attempts for failed operations
	RetryAttempts int

	// RetryBackoff defines backoff duration between retries
	RetryBackoff time.Duration
}

// ProcessingMetrics tracks metrics for analysis data processing operations.
type ProcessingMetrics struct {
	// ProcessedAnalyses tracks total number of processed analyses
	ProcessedAnalyses int64

	// FailedAnalyses tracks total number of failed analyses
	FailedAnalyses int64

	// AverageProcessingTime tracks average processing time
	AverageProcessingTime time.Duration

	// LastProcessedTime tracks last processing timestamp
	LastProcessedTime time.Time

	// mu protects concurrent access to metrics
	mu sync.RWMutex
}

// ProcessingResult represents the result of analysis data processing.
type ProcessingResult struct {
	// AnalysisName identifies the analysis that was processed
	AnalysisName string

	// ProcessedAt indicates when processing was completed
	ProcessedAt time.Time

	// Score represents the computed analysis score (0-100)
	Score int32

	// ProcessedMeasurements contains processed measurement data
	ProcessedMeasurements []ProcessedMeasurement

	// Metadata contains additional processing metadata
	Metadata map[string]string

	// Errors contains any processing errors that occurred
	Errors []ProcessingError
}

// ProcessedMeasurement represents a processed analysis measurement.
type ProcessedMeasurement struct {
	// OriginalMeasurement references the original measurement
	OriginalMeasurement *AnalysisMeasurement

	// ProcessedValue contains the processed value
	ProcessedValue string

	// NormalizedValue contains normalized value for comparison
	NormalizedValue float64

	// ProcessingApplied describes transformations applied
	ProcessingApplied []string
}

// ProcessingError represents errors that occur during processing.
type ProcessingError struct {
	// Type categorizes the error
	Type ProcessingErrorType

	// Message provides error details
	Message string

	// Timestamp indicates when error occurred
	Timestamp time.Time

	// Context provides additional error context
	Context map[string]string
}

// ProcessingErrorType represents types of processing errors.
type ProcessingErrorType string

const (
	// ProcessingErrorTypeValidation indicates validation errors
	ProcessingErrorTypeValidation ProcessingErrorType = "Validation"

	// ProcessingErrorTypeTransformation indicates transformation errors
	ProcessingErrorTypeTransformation ProcessingErrorType = "Transformation"

	// ProcessingErrorTypeComputation indicates computation errors
	ProcessingErrorTypeComputation ProcessingErrorType = "Computation"

	// ProcessingErrorTypeTimeout indicates timeout errors
	ProcessingErrorTypeTimeout ProcessingErrorType = "Timeout"

	// ProcessingErrorTypeUnknown indicates unknown errors
	ProcessingErrorTypeUnknown ProcessingErrorType = "Unknown"
)

// NewAnalysisDataProcessor creates a new analysis data processor.
func NewAnalysisDataProcessor(config *ProcessorConfig) *AnalysisDataProcessor {
	if config == nil {
		config = &ProcessorConfig{
			MaxConcurrentProcessing: 10,
			ProcessingTimeout:       5 * time.Minute,
			RetryAttempts:          3,
			RetryBackoff:           30 * time.Second,
		}
	}

	return &AnalysisDataProcessor{
		metrics: &ProcessingMetrics{},
		config:  config,
	}
}

// ProcessAnalysis processes a WorkloadAnalysisRun and returns processed results.
func (p *AnalysisDataProcessor) ProcessAnalysis(ctx context.Context, analysis *WorkloadAnalysisRun) (*ProcessingResult, error) {
	startTime := time.Now()
	
	// Validate input analysis first
	if err := p.validateAnalysis(analysis); err != nil {
		return nil, fmt.Errorf("analysis validation failed: %w", err)
	}
	
	klog.V(2).InfoS("Starting analysis processing",
		"analysis", analysis.Name,
		"namespace", analysis.Namespace,
		"phase", analysis.Status.Phase)

	result := &ProcessingResult{
		AnalysisName:          analysis.Name,
		ProcessedAt:           time.Now(),
		ProcessedMeasurements: make([]ProcessedMeasurement, 0),
		Metadata:              make(map[string]string),
		Errors:                make([]ProcessingError, 0),
	}

	// Process each analysis result with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, p.config.ProcessingTimeout)
	defer cancel()

	for _, analysisResult := range analysis.Status.AnalysisResults {
		if err := p.processAnalysisResult(timeoutCtx, &analysisResult, result); err != nil {
			result.Errors = append(result.Errors, ProcessingError{
				Type:      ProcessingErrorTypeComputation,
				Message:   fmt.Sprintf("failed to process analysis result %s: %v", analysisResult.Name, err),
				Timestamp: time.Now(),
			})
			continue
		}
	}

	// Calculate overall score using simple averaging for now
	if score, err := p.calculateSimpleScore(result.ProcessedMeasurements); err == nil {
		result.Score = score
	} else {
		result.Errors = append(result.Errors, ProcessingError{
			Type:      ProcessingErrorTypeComputation,
			Message:   fmt.Sprintf("failed to calculate overall score: %v", err),
			Timestamp: time.Now(),
		})
	}

	// Update processing metrics
	p.updateMetrics(time.Since(startTime), len(result.Errors) == 0)

	klog.V(2).InfoS("Completed analysis processing",
		"analysis", analysis.Name,
		"score", result.Score,
		"measurements", len(result.ProcessedMeasurements),
		"errors", len(result.Errors),
		"duration", time.Since(startTime))

	return result, nil
}

// processAnalysisResult processes a single AnalysisResult and updates the ProcessingResult.
func (p *AnalysisDataProcessor) processAnalysisResult(ctx context.Context, analysisResult *AnalysisResult, result *ProcessingResult) error {
	for _, measurement := range analysisResult.Measurements {
		processed, err := p.processMeasurement(&measurement)
		if err != nil {
			return fmt.Errorf("failed to process measurement: %w", err)
		}
		
		result.ProcessedMeasurements = append(result.ProcessedMeasurements, *processed)
	}
	
	return nil
}

// processMeasurement processes a single AnalysisMeasurement.
func (p *AnalysisDataProcessor) processMeasurement(measurement *AnalysisMeasurement) (*ProcessedMeasurement, error) {
	processed := &ProcessedMeasurement{
		OriginalMeasurement: measurement,
		ProcessedValue:      measurement.Value,
		ProcessingApplied:   make([]string, 0),
	}

	// Apply data transformations
	if normalized, err := p.normalizeValue(measurement.Value); err == nil {
		processed.NormalizedValue = normalized
		processed.ProcessingApplied = append(processed.ProcessingApplied, "normalization")
	}

	return processed, nil
}

// normalizeValue normalizes measurement values to a standard scale (0.0-1.0).
func (p *AnalysisDataProcessor) normalizeValue(value string) (float64, error) {
	// Implementation would depend on the value format and normalization strategy
	// For now, returning a placeholder normalization
	return 0.5, nil
}

// calculateSimpleScore calculates a simple average score from processed measurements.
func (p *AnalysisDataProcessor) calculateSimpleScore(measurements []ProcessedMeasurement) (int32, error) {
	if len(measurements) == 0 {
		return 0, fmt.Errorf("no measurements provided")
	}

	// Extract normalized values for calculation
	var sum float64
	var count int

	for _, measurement := range measurements {
		if measurement.NormalizedValue >= 0 {
			sum += measurement.NormalizedValue
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid measurement values found")
	}

	// Calculate average and convert to integer score (0-100)
	average := sum / float64(count)
	score := int32(average * 100)

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score, nil
}

// validateAnalysis validates the WorkloadAnalysisRun for processing requirements.
func (p *AnalysisDataProcessor) validateAnalysis(analysis *WorkloadAnalysisRun) error {
	if analysis == nil {
		return fmt.Errorf("analysis cannot be nil")
	}

	if analysis.Name == "" {
		return fmt.Errorf("analysis name cannot be empty")
	}

	if analysis.Status.Phase == "" {
		return fmt.Errorf("analysis phase cannot be empty")
	}

	return nil
}

// updateMetrics updates processing metrics with operation results.
func (p *AnalysisDataProcessor) updateMetrics(duration time.Duration, success bool) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	if success {
		p.metrics.ProcessedAnalyses++
	} else {
		p.metrics.FailedAnalyses++
	}

	// Update average processing time
	total := p.metrics.ProcessedAnalyses + p.metrics.FailedAnalyses
	if total > 0 {
		p.metrics.AverageProcessingTime = time.Duration(
			(int64(p.metrics.AverageProcessingTime)*(total-1) + int64(duration)) / total,
		)
	}

	p.metrics.LastProcessedTime = time.Now()
}

// GetMetrics returns current processing metrics.
func (p *AnalysisDataProcessor) GetMetrics() ProcessingMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return *p.metrics
}

// ProcessBatch processes multiple WorkloadAnalysisRun resources concurrently.
func (p *AnalysisDataProcessor) ProcessBatch(ctx context.Context, analyses []*WorkloadAnalysisRun) ([]*ProcessingResult, error) {
	if len(analyses) == 0 {
		return nil, nil
	}

	results := make([]*ProcessingResult, len(analyses))
	semaphore := make(chan struct{}, p.config.MaxConcurrentProcessing)
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processingErrors []error

	for i, analysis := range analyses {
		wg.Add(1)
		go func(idx int, analysisRun *WorkloadAnalysisRun) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := p.ProcessAnalysis(ctx, analysisRun)
			
			mu.Lock()
			if err != nil {
				processingErrors = append(processingErrors, fmt.Errorf("failed to process analysis %s: %w", analysisRun.Name, err))
			} else {
				results[idx] = result
			}
			mu.Unlock()
		}(i, analysis)
	}

	wg.Wait()

	if len(processingErrors) > 0 {
		return results, fmt.Errorf("batch processing completed with %d errors: %v", len(processingErrors), processingErrors)
	}

	return results, nil
}