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
	"k8s.io/component-base/featuregate"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

func TestNewTimeSeriesConsolidator(t *testing.T) {
	mockSource := &mockMetricsSource{}
	config := DefaultConsolidationConfig()

	consolidator := NewTimeSeriesConsolidator(mockSource, config)

	assert.NotNil(t, consolidator)
	assert.Equal(t, mockSource, consolidator.metricsSource)
	assert.Equal(t, config, consolidator.config)
}

func TestDefaultConsolidationConfig(t *testing.T) {
	config := DefaultConsolidationConfig()

	assert.Equal(t, 1000, config.MaxDataPoints)
	assert.Equal(t, ConsolidationAverage, config.ConsolidationFunction)
	assert.Equal(t, time.Minute, config.Tolerance)
}

func TestConsolidateTimeSeries(t *testing.T) {
	tests := map[string]struct {
		clusters      []string
		featureFlags  map[featuregate.Feature]bool
		config        ConsolidationConfig
		timeRange     TimeRange
		wantError     bool
		errorContains string
		wantPoints    int
	}{
		"successful consolidation": {
			clusters: []string{"cluster1", "cluster2"},
			featureFlags: map[featuregate.Feature]bool{
				features.TMCTimeSeriesConsolidation: true,
			},
			config: ConsolidationConfig{
				MaxDataPoints:         10,
				ConsolidationFunction: ConsolidationAverage,
				Tolerance:             time.Minute,
			},
			timeRange: TimeRange{
				Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC),
				Step:  time.Minute * 5,
			},
			wantError:  false,
			wantPoints: 10,
		},
		"feature disabled": {
			clusters: []string{"cluster1"},
			featureFlags: map[featuregate.Feature]bool{
				features.TMCTimeSeriesConsolidation: false,
			},
			wantError:     true,
			errorContains: "TMC time series consolidation is disabled",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup feature flags
			originalGate := utilfeature.DefaultMutableFeatureGate
			defer func() {
				utilfeature.DefaultMutableFeatureGate = originalGate
			}()

			testGate := featuregate.NewFeatureGate()
			err := testGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
				features.TMCTimeSeriesConsolidation: {Default: false, PreRelease: featuregate.Alpha},
			})
			require.NoError(t, err)

			for feature, enabled := range tc.featureFlags {
				err := testGate.Set(string(feature), enabled)
				require.NoError(t, err)
			}
			utilfeature.DefaultMutableFeatureGate = testGate

			// Create mock source
			mockSource := &mockMetricsSource{
				clusters: tc.clusters,
			}

			// Create consolidator
			consolidator := NewTimeSeriesConsolidator(mockSource, tc.config)

			// Execute consolidation
			result, err := consolidator.ConsolidateTimeSeries(
				context.Background(),
				logicalcluster.Name("root:test"),
				"cpu_usage",
				AggregationSum,
				tc.timeRange,
			)

			if tc.wantError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, "cpu_usage", result.TimeSeries.MetricName)
				assert.Equal(t, tc.wantPoints, len(result.TimeSeries.Points))
				assert.Greater(t, result.ConsolidatedBy, 0.0)
				assert.Equal(t, logicalcluster.Name("root:test"), result.SourceWorkspace)
			}
		})
	}
}

func TestValidateConsolidationFunction(t *testing.T) {
	tests := map[string]struct {
		function  ConsolidationFunction
		wantError bool
	}{
		"valid average":      {function: ConsolidationAverage, wantError: false},
		"valid max":          {function: ConsolidationMax, wantError: false},
		"valid min":          {function: ConsolidationMin, wantError: false},
		"invalid function":   {function: ConsolidationFunction("invalid"), wantError: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateConsolidationFunction(tc.function)

			if tc.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported consolidation function")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// mockMetricsSource for testing
type mockMetricsSource struct {
	clusters []string
}

func (m *mockMetricsSource) GetMetricValue(ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string) (float64, map[string]string, error) {
	return 10.0, map[string]string{"cluster": clusterName, "metric": metricName}, nil
}

func (m *mockMetricsSource) ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error) {
	return m.clusters, nil
}