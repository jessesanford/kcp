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

package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregate "k8s.io/component-base/featuregate"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// MockWorkspaceAwareMetricsCollector provides a mock implementation for testing
type MockWorkspaceAwareMetricsCollector struct {
	clusters      map[logicalcluster.Name][]string
	clusterMetrics map[string]*MockClusterMetrics
	shouldError   bool
	errorMessage  string
}

// MockClusterMetrics represents metrics from a single cluster
type MockClusterMetrics struct {
	Metrics   map[string]float64
	Labels    map[string]string
	Timestamp time.Time
}

func (m *MockWorkspaceAwareMetricsCollector) ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if clusters, ok := m.clusters[workspace]; ok {
		return clusters, nil
	}
	return []string{}, nil
}

func (m *MockWorkspaceAwareMetricsCollector) CollectClusterMetrics(ctx context.Context, clusterName string, workspace logicalcluster.Name) (*ClusterMetrics, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if metrics, ok := m.clusterMetrics[clusterName]; ok {
		return &ClusterMetrics{
			Metrics:   metrics.Metrics,
			Labels:    metrics.Labels,
			Timestamp: metrics.Timestamp,
		}, nil
	}
	return &ClusterMetrics{
		Metrics:   make(map[string]float64),
		Labels:    make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

func TestNewMetricsAggregator(t *testing.T) {
	mockCollector := &MockWorkspaceAwareMetricsCollector{}
	aggregator := NewMetricsAggregator(mockCollector)
	
	require.NotNil(t, aggregator)
	impl, ok := aggregator.(*MetricsAggregatorImpl)
	require.True(t, ok)
	require.Equal(t, mockCollector, impl.metricsCollector)
}

func TestAggregateMetrics_FeatureFlagDisabled(t *testing.T) {
	// Ensure feature flag is disabled
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation: {Default: false, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	mockCollector := &MockWorkspaceAwareMetricsCollector{}
	aggregator := NewMetricsAggregator(mockCollector)
	
	ctx := context.Background()
	workspace := logicalcluster.Name("test-workspace")
	timeRange := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "TMC metrics aggregation is disabled")
}

func TestAggregateMetrics_AllStrategies(t *testing.T) {
	// Enable feature flags
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation:    {Default: true, PreRelease: featuregate.Alpha},
		features.TMCAdvancedAggregation:   {Default: true, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	workspace := logicalcluster.Name("test-workspace")
	clusters := []string{"cluster1", "cluster2", "cluster3"}
	
	mockCollector := &MockWorkspaceAwareMetricsCollector{
		clusters: map[logicalcluster.Name][]string{
			workspace: clusters,
		},
		clusterMetrics: map[string]*MockClusterMetrics{
			"cluster1": {
				Metrics:   map[string]float64{"cpu.usage": 10.0, "memory.usage": 50.0},
				Labels:    map[string]string{"region": "us-west"},
				Timestamp: time.Now(),
			},
			"cluster2": {
				Metrics:   map[string]float64{"cpu.usage": 20.0, "memory.usage": 60.0},
				Labels:    map[string]string{"region": "us-east"},
				Timestamp: time.Now(),
			},
			"cluster3": {
				Metrics:   map[string]float64{"cpu.usage": 30.0, "memory.usage": 70.0},
				Labels:    map[string]string{"region": "eu-west"},
				Timestamp: time.Now(),
			},
		},
	}

	aggregator := NewMetricsAggregator(mockCollector)
	ctx := context.Background()
	timeRange := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	tests := map[string]struct {
		strategy      AggregationStrategy
		expectedValue float64
		metricName    string
	}{
		"sum aggregation": {
			strategy:      AggregationSum,
			expectedValue: 60.0, // 10 + 20 + 30
			metricName:    "cpu.usage",
		},
		"avg aggregation": {
			strategy:      AggregationAvg,
			expectedValue: 20.0, // (10 + 20 + 30) / 3
			metricName:    "cpu.usage",
		},
		"max aggregation": {
			strategy:      AggregationMax,
			expectedValue: 30.0, // max(10, 20, 30)
			metricName:    "cpu.usage",
		},
		"min aggregation": {
			strategy:      AggregationMin,
			expectedValue: 10.0, // min(10, 20, 30)
			metricName:    "cpu.usage",
		},
		"memory sum": {
			strategy:      AggregationSum,
			expectedValue: 180.0, // 50 + 60 + 70
			metricName:    "memory.usage",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := aggregator.AggregateMetrics(ctx, workspace, tc.metricName, tc.strategy, timeRange)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			assert.Equal(t, tc.expectedValue, result.Value)
			assert.Equal(t, tc.metricName, result.MetricName)
			assert.Equal(t, tc.strategy, result.Strategy)
			assert.Equal(t, workspace, result.Workspace)
			assert.Equal(t, 3, result.ClusterCount)
			assert.Len(t, result.SourceClusters, 3)
			assert.Contains(t, result.SourceClusters, "cluster1")
			assert.Contains(t, result.SourceClusters, "cluster2")
			assert.Contains(t, result.SourceClusters, "cluster3")
			assert.NotNil(t, result.Labels)
		})
	}
}

func TestAggregateMetrics_AdvancedAggregationDisabled(t *testing.T) {
	// Enable basic aggregation but disable advanced
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation:  {Default: true, PreRelease: featuregate.Alpha},
		features.TMCAdvancedAggregation: {Default: false, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	mockCollector := &MockWorkspaceAwareMetricsCollector{
		clusters: map[logicalcluster.Name][]string{
			logicalcluster.Name("test"): {"cluster1"},
		},
		clusterMetrics: map[string]*MockClusterMetrics{
			"cluster1": {
				Metrics:   map[string]float64{"cpu.usage": 10.0},
				Timestamp: time.Now(),
			},
		},
	}

	aggregator := NewMetricsAggregator(mockCollector)
	ctx := context.Background()
	workspace := logicalcluster.Name("test")
	timeRange := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}

	// Sum should work (basic aggregation)
	result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 10.0, result.Value)

	// Advanced strategies should fail
	advancedStrategies := []AggregationStrategy{AggregationAvg, AggregationMax, AggregationMin}
	for _, strategy := range advancedStrategies {
		t.Run(string(strategy), func(t *testing.T) {
			result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", strategy, timeRange)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "advanced aggregation strategies require TMCAdvancedAggregation feature flag")
		})
	}
}

func TestAggregateMetrics_WorkspaceIsolation(t *testing.T) {
	// Enable feature flags
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation: {Default: true, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	workspace1 := logicalcluster.Name("workspace1")
	workspace2 := logicalcluster.Name("workspace2")
	
	mockCollector := &MockWorkspaceAwareMetricsCollector{
		clusters: map[logicalcluster.Name][]string{
			workspace1: {"cluster1", "cluster2"},
			workspace2: {"cluster3", "cluster4"},
		},
		clusterMetrics: map[string]*MockClusterMetrics{
			"cluster1": {
				Metrics:   map[string]float64{"cpu.usage": 10.0},
				Timestamp: time.Now(),
			},
			"cluster2": {
				Metrics:   map[string]float64{"cpu.usage": 20.0},
				Timestamp: time.Now(),
			},
			"cluster3": {
				Metrics:   map[string]float64{"cpu.usage": 100.0},
				Timestamp: time.Now(),
			},
			"cluster4": {
				Metrics:   map[string]float64{"cpu.usage": 200.0},
				Timestamp: time.Now(),
			},
		},
	}

	aggregator := NewMetricsAggregator(mockCollector)
	ctx := context.Background()
	timeRange := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}

	// Test workspace1 isolation
	result1, err := aggregator.AggregateMetrics(ctx, workspace1, "cpu.usage", AggregationSum, timeRange)
	require.NoError(t, err)
	assert.Equal(t, 30.0, result1.Value) // 10 + 20
	assert.Equal(t, 2, result1.ClusterCount)
	assert.Contains(t, result1.SourceClusters, "cluster1")
	assert.Contains(t, result1.SourceClusters, "cluster2")

	// Test workspace2 isolation
	result2, err := aggregator.AggregateMetrics(ctx, workspace2, "cpu.usage", AggregationSum, timeRange)
	require.NoError(t, err)
	assert.Equal(t, 300.0, result2.Value) // 100 + 200
	assert.Equal(t, 2, result2.ClusterCount)
	assert.Contains(t, result2.SourceClusters, "cluster3")
	assert.Contains(t, result2.SourceClusters, "cluster4")
}

func TestAggregateMetrics_ErrorConditions(t *testing.T) {
	// Enable feature flags
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation: {Default: true, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	ctx := context.Background()
	workspace := logicalcluster.Name("test-workspace")
	timeRange := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}

	t.Run("collector error", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			shouldError: true,
		}
		aggregator := NewMetricsAggregator(mockCollector)

		result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
		
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to list clusters")
	})

	t.Run("no clusters in workspace", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
		
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no clusters found in workspace")
	})

	t.Run("no metric values found", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {"cluster1"},
			},
			clusterMetrics: map[string]*MockClusterMetrics{
				"cluster1": {
					Metrics:   map[string]float64{"memory.usage": 50.0}, // different metric
					Timestamp: time.Now(),
				},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
		
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no metric values found for cpu.usage")
	})

	t.Run("unsupported aggregation strategy", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {"cluster1"},
			},
			clusterMetrics: map[string]*MockClusterMetrics{
				"cluster1": {
					Metrics:   map[string]float64{"cpu.usage": 10.0},
					Timestamp: time.Now(),
				},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		// Test with invalid strategy by calling the internal method directly
		impl := aggregator.(*MetricsAggregatorImpl)
		_, err := impl.applyAggregationStrategy(AggregationStrategy("invalid"), []float64{10.0})
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported aggregation strategy")
	})
}

func TestAggregateMetrics_EdgeCases(t *testing.T) {
	// Enable feature flags
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCMetricsAggregation:  {Default: true, PreRelease: featuregate.Alpha},
		features.TMCAdvancedAggregation: {Default: true, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	ctx := context.Background()
	workspace := logicalcluster.Name("test-workspace")
	timeRange := TimeRange{Start: time.Now().Add(-time.Hour), End: time.Now()}

	t.Run("single cluster", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {"cluster1"},
			},
			clusterMetrics: map[string]*MockClusterMetrics{
				"cluster1": {
					Metrics:   map[string]float64{"cpu.usage": 42.0},
					Labels:    map[string]string{"env": "test"},
					Timestamp: time.Now(),
				},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		strategies := []AggregationStrategy{AggregationSum, AggregationAvg, AggregationMax, AggregationMin}
		for _, strategy := range strategies {
			t.Run(string(strategy), func(t *testing.T) {
				result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", strategy, timeRange)
				
				require.NoError(t, err)
				assert.Equal(t, 42.0, result.Value) // All strategies should return same value for single cluster
				assert.Equal(t, 1, result.ClusterCount)
				assert.Equal(t, []string{"cluster1"}, result.SourceClusters)
				assert.Equal(t, "test", result.Labels["env"])
			})
		}
	})

	t.Run("zero values", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {"cluster1", "cluster2"},
			},
			clusterMetrics: map[string]*MockClusterMetrics{
				"cluster1": {
					Metrics:   map[string]float64{"cpu.usage": 0.0},
					Timestamp: time.Now(),
				},
				"cluster2": {
					Metrics:   map[string]float64{"cpu.usage": 0.0},
					Timestamp: time.Now(),
				},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
		
		require.NoError(t, err)
		assert.Equal(t, 0.0, result.Value)
		assert.Equal(t, 2, result.ClusterCount)
	})

	t.Run("negative values", func(t *testing.T) {
		mockCollector := &MockWorkspaceAwareMetricsCollector{
			clusters: map[logicalcluster.Name][]string{
				workspace: {"cluster1", "cluster2"},
			},
			clusterMetrics: map[string]*MockClusterMetrics{
				"cluster1": {
					Metrics:   map[string]float64{"cpu.usage": -10.0},
					Timestamp: time.Now(),
				},
				"cluster2": {
					Metrics:   map[string]float64{"cpu.usage": 5.0},
					Timestamp: time.Now(),
				},
			},
		}
		aggregator := NewMetricsAggregator(mockCollector)

		result, err := aggregator.AggregateMetrics(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
		
		require.NoError(t, err)
		assert.Equal(t, -5.0, result.Value) // -10 + 5
		assert.Equal(t, 2, result.ClusterCount)
	})
}

func TestApplyAggregationStrategy(t *testing.T) {
	mockCollector := &MockWorkspaceAwareMetricsCollector{}
	aggregator := NewMetricsAggregator(mockCollector)
	impl := aggregator.(*MetricsAggregatorImpl)

	t.Run("empty values", func(t *testing.T) {
		_, err := impl.applyAggregationStrategy(AggregationSum, []float64{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no values to aggregate")
	})

	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	tests := map[string]struct {
		strategy      AggregationStrategy
		expectedValue float64
	}{
		"sum": {
			strategy:      AggregationSum,
			expectedValue: 15.0, // 1+2+3+4+5
		},
		"average": {
			strategy:      AggregationAvg,
			expectedValue: 3.0, // 15/5
		},
		"maximum": {
			strategy:      AggregationMax,
			expectedValue: 5.0,
		},
		"minimum": {
			strategy:      AggregationMin,
			expectedValue: 1.0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := impl.applyAggregationStrategy(tc.strategy, values)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedValue, result)
		})
	}
}
func TestAggregateTimeSeries(t *testing.T) {
	// Enable feature flags
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCTimeSeriesConsolidation: {Default: true, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	workspace := logicalcluster.Name("test-workspace")
	baseTime := time.Now().Truncate(time.Minute)
	
	mockCollector := &MockWorkspaceAwareMetricsCollector{
		clusters: map[logicalcluster.Name][]string{
			workspace: {"cluster1", "cluster2"},
		},
		clusterMetrics: map[string]*MockClusterMetrics{
			"cluster1": {
				Metrics:   map[string]float64{"cpu.usage": 10.0},
				Labels:    map[string]string{"region": "us-west"},
				Timestamp: baseTime,
			},
			"cluster2": {
				Metrics:   map[string]float64{"cpu.usage": 20.0},
				Labels:    map[string]string{"region": "us-east"},
				Timestamp: baseTime.Add(time.Minute),
			},
		},
	}

	aggregator := NewMetricsAggregator(mockCollector)
	ctx := context.Background()
	timeRange := TimeRange{
		Start: baseTime.Add(-time.Hour),
		End:   baseTime.Add(time.Hour),
		Step:  time.Minute,
	}

	result, err := aggregator.AggregateTimeSeries(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "cpu.usage", result.MetricName)
	assert.NotEmpty(t, result.Points)
	assert.NotNil(t, result.Labels)
}

func TestAggregateTimeSeries_FeatureFlagDisabled(t *testing.T) {
	// Disable time series consolidation
	featureGate := featuregate.NewFeatureGate()
	featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		features.TMCTimeSeriesConsolidation: {Default: false, PreRelease: featuregate.Alpha},
	})
	utilfeature.DefaultFeatureGate = featureGate

	mockCollector := &MockWorkspaceAwareMetricsCollector{}
	aggregator := NewMetricsAggregator(mockCollector)
	
	ctx := context.Background()
	workspace := logicalcluster.Name("test-workspace")
	timeRange := TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	result, err := aggregator.AggregateTimeSeries(ctx, workspace, "cpu.usage", AggregationSum, timeRange)
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "TMC time series consolidation is disabled")
}

func TestConsolidateTimeSeries(t *testing.T) {
	mockCollector := &MockWorkspaceAwareMetricsCollector{}
	aggregator := NewMetricsAggregator(mockCollector)
	
	baseTime := time.Now().Truncate(time.Minute)
	interval := time.Minute

	t.Run("empty time series", func(t *testing.T) {
		result, err := aggregator.ConsolidateTimeSeries([]*TimeSeries{}, interval)
		
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no time series to consolidate")
	})

	t.Run("inconsistent metric names", func(t *testing.T) {
		timeSeries := []*TimeSeries{
			{
				MetricName: "cpu.usage",
				Points: []MetricPoint{
					{Timestamp: baseTime, Value: 10.0},
				},
			},
			{
				MetricName: "memory.usage", // Different metric name
				Points: []MetricPoint{
					{Timestamp: baseTime, Value: 50.0},
				},
			},
		}

		result, err := aggregator.ConsolidateTimeSeries(timeSeries, interval)
		
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "inconsistent metric names")
	})

	t.Run("successful consolidation", func(t *testing.T) {
		timeSeries := []*TimeSeries{
			{
				MetricName: "cpu.usage",
				Labels:     map[string]string{"env": "test"},
				Points: []MetricPoint{
					{Timestamp: baseTime, Value: 10.0},
					{Timestamp: baseTime.Add(time.Minute), Value: 15.0},
				},
			},
			{
				MetricName: "cpu.usage",
				Labels:     map[string]string{"region": "us-west"},
				Points: []MetricPoint{
					{Timestamp: baseTime, Value: 20.0},
					{Timestamp: baseTime.Add(2 * time.Minute), Value: 25.0},
				},
			},
		}

		result, err := aggregator.ConsolidateTimeSeries(timeSeries, interval)
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cpu.usage", result.MetricName)
		assert.NotEmpty(t, result.Labels)
		assert.NotEmpty(t, result.Points)

		// Check that points are consolidated and gaps are filled
		pointMap := make(map[time.Time]MetricPoint)
		for _, point := range result.Points {
			pointMap[point.Timestamp] = point
		}

		// First point should be average of 10.0 and 20.0
		point1, exists := pointMap[baseTime]
		require.True(t, exists)
		assert.Equal(t, 15.0, point1.Value) // (10+20)/2

		// Second point should be 15.0 (only one value)
		point2, exists := pointMap[baseTime.Add(time.Minute)]
		require.True(t, exists)
		assert.Equal(t, 15.0, point2.Value)

		// Third point should be 25.0 (only one value)
		point3, exists := pointMap[baseTime.Add(2*time.Minute)]
		require.True(t, exists)
		assert.Equal(t, 25.0, point3.Value)
	})

	t.Run("gap filling", func(t *testing.T) {
		timeSeries := []*TimeSeries{
			{
				MetricName: "cpu.usage",
				Points: []MetricPoint{
					{Timestamp: baseTime, Value: 10.0},
					{Timestamp: baseTime.Add(3 * time.Minute), Value: 30.0}, // Gap of 2 minutes
				},
			},
		}

		result, err := aggregator.ConsolidateTimeSeries(timeSeries, interval)
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Points, 4) // Original 2 points + 2 gap-filled points

		// Check that gaps are filled with previous values
		pointMap := make(map[time.Time]MetricPoint)
		for _, point := range result.Points {
			pointMap[point.Timestamp] = point
		}

		// Verify original points
		assert.Equal(t, 10.0, pointMap[baseTime].Value)
		assert.Equal(t, 30.0, pointMap[baseTime.Add(3*time.Minute)].Value)

		// Verify gap-filled points
		gapPoint1 := pointMap[baseTime.Add(time.Minute)]
		assert.Equal(t, 10.0, gapPoint1.Value) // Should use previous value
		assert.Equal(t, "true", gapPoint1.Labels["filled"])

		gapPoint2 := pointMap[baseTime.Add(2*time.Minute)]
		assert.Equal(t, 10.0, gapPoint2.Value) // Should use previous value
		assert.Equal(t, "true", gapPoint2.Labels["filled"])
	})
}

func TestGetTimeSeriesFromCluster(t *testing.T) {
	workspace := logicalcluster.Name("test-workspace")
	baseTime := time.Now()
	
	mockCollector := &MockWorkspaceAwareMetricsCollector{
		clusterMetrics: map[string]*MockClusterMetrics{
			"cluster1": {
				Metrics:   map[string]float64{"cpu.usage": 42.0},
				Labels:    map[string]string{"region": "us-west"},
				Timestamp: baseTime,
			},
		},
	}

	aggregator := NewMetricsAggregator(mockCollector)
	impl := aggregator.(*MetricsAggregatorImpl)
	
	ctx := context.Background()
	timeRange := TimeRange{
		Start: baseTime.Add(-time.Hour),
		End:   baseTime.Add(time.Hour),
	}

	t.Run("successful retrieval", func(t *testing.T) {
		result, err := impl.getTimeSeriesFromCluster(ctx, "cluster1", workspace, "cpu.usage", timeRange)
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cpu.usage", result.MetricName)
		assert.Len(t, result.Points, 1)
		assert.Equal(t, 42.0, result.Points[0].Value)
		assert.Equal(t, baseTime, result.Points[0].Timestamp)
		assert.Equal(t, "cluster1", result.Points[0].Labels["cluster"])
		assert.Equal(t, "us-west", result.Labels["region"])
		assert.Equal(t, "cluster1", result.Labels["cluster"])
	})

	t.Run("metric not found", func(t *testing.T) {
		result, err := impl.getTimeSeriesFromCluster(ctx, "cluster1", workspace, "nonexistent.metric", timeRange)
		
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("cluster not found", func(t *testing.T) {
		result, err := impl.getTimeSeriesFromCluster(ctx, "nonexistent-cluster", workspace, "cpu.usage", timeRange)
		
		require.NoError(t, err)
		assert.Nil(t, result) // Should return nil for missing cluster
	})
}
