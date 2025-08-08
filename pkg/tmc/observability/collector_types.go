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

	"github.com/kcp-dev/logicalcluster/v3"
)

// MetricsSource represents a source of metrics data from clusters.
// This interface is the foundation for all metric collection implementations.
type MetricsSource interface {
	// GetMetricValue retrieves a specific metric value from a cluster
	GetMetricValue(ctx context.Context, clusterName string, workspace logicalcluster.Name, metricName string) (float64, map[string]string, error)

	// ListClusters returns available clusters in a workspace
	ListClusters(ctx context.Context, workspace logicalcluster.Name) ([]string, error)
}