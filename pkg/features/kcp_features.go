/*
Copyright 2022 The KCP Authors.

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

package features

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	logsapiv1 "k8s.io/component-base/logs/api/v1"
)

const (
	// owner: @sttts @ncdc
	// alpha: v0.8
	// Enables virtual workspaces
	VirtualWorkspaces featuregate.Feature = "VirtualWorkspaces"

	// owner: @sttts @ncdc
	// alpha: v0.7
	// Enables location in virtual workspaces
	LocationInVirtualWorkspaces featuregate.Feature = "LocationInVirtualWorkspaces"

	// owner: @sttts
	// alpha: v0.8
	// kcp enables advanced scheduling features
	AdvancedScheduling featuregate.Feature = "AdvancedScheduling"

	// owner: @sttts
	// alpha: v0.9
	// kcp enables support for sharded servers
	ShardedServer featuregate.Feature = "ShardedServer"

	// Enables cache apis and controllers.
	CacheAPIs featuregate.Feature = "CacheAPIs"

	// owner: @mjudeikis
	// alpha: v0.1
	// Enables VirtualWorkspace urls on APIExport. This enables to use Deprecated APIExport VirtualWorkspace urls.
	// This is a temporary feature to ease the migration to the new VirtualWorkspace urls.
	EnableDeprecatedAPIExportVirtualWorkspacesUrls featuregate.Feature = "EnableDeprecatedAPIExportVirtualWorkspacesUrls"

	// owner: @mjudeikis
	// alpha: v0.1
	// Enables workspace mounts functionality
	WorkspaceMounts featuregate.Feature = "WorkspaceMounts"

	// TMC Feature Flags

	// owner: @jessesanford
	// alpha: v0.1
	// Master feature flag for all TMC functionality. When disabled, all TMC features are disabled.
	TMCFeature featuregate.Feature = "TMCFeature"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC APIs (ClusterRegistration, WorkloadPlacement) and APIExport functionality.
	// This feature provides advanced placement policies, cluster registration, and workload distribution capabilities.
	TMCAPIs featuregate.Feature = "TMCAPIs"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC controllers for cluster registration and workload placement management.
	TMCControllers featuregate.Feature = "TMCControllers"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC placement engine for advanced workload placement strategies.
	TMCPlacement featuregate.Feature = "TMCPlacement"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC metrics aggregation functionality for cross-cluster metric collection.
	// This feature is required for aggregating metrics from multiple TMC clusters.
	TMCMetricsAggregation featuregate.Feature = "TMCMetricsAggregation"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables advanced aggregation strategies beyond simple sum operations.
	// This feature provides more sophisticated metric aggregation algorithms.
	TMCAdvancedAggregation featuregate.Feature = "TMCAdvancedAggregation"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables time series data consolidation for efficient metric storage.
	// This feature optimizes metric storage by consolidating historical data points.
	TMCTimeSeriesConsolidation featuregate.Feature = "TMCTimeSeriesConsolidation"
)

// DefaultFeatureGate exposes the upstream feature gate, but with our gate setting applied.
var DefaultFeatureGate = utilfeature.DefaultFeatureGate

func init() {
	utilruntime.Must(utilfeature.DefaultMutableFeatureGate.Add(defaultGenericControlPlaneFeatureGates))
	// Add Kubernetes logging feature gates that are expected by the logging system
	utilruntime.Must(logsapiv1.AddFeatureGates(utilfeature.DefaultMutableFeatureGate))
}

func KnownFeatures() []string {
	features := make([]string, 0, len(defaultGenericControlPlaneFeatureGates))
	for k := range defaultGenericControlPlaneFeatureGates {
		features = append(features, string(k))
	}
	return features
}

// NewFlagValue returns a wrapper to be used for a pflag flag value.
func NewFlagValue() pflag.Value {
	return &kcpFeatureGate{
		utilfeature.DefaultMutableFeatureGate,
	}
}

type kcpFeatureGate struct {
	featuregate.MutableFeatureGate
}

// Removed - using simple feature gates now

func (f *kcpFeatureGate) String() string {
	pairs := []string{}

	for featureName, spec := range defaultGenericControlPlaneFeatureGates {
		pairs = append(pairs, fmt.Sprintf("%s=%t", featureName, spec.Default))
	}

	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func (f *kcpFeatureGate) Type() string {
	return "mapStringBool"
}

// TMC Feature Flag Utilities

// TMCEnabled returns true if the master TMC feature flag is enabled.
// This is the primary gate that must be enabled for any TMC functionality.
func TMCEnabled() bool {
	return utilfeature.DefaultFeatureGate.Enabled(TMCFeature)
}

// TMCAPIsEnabled returns true if TMC APIs feature flag is enabled.
// This controls whether TMC API types (ClusterRegistration, WorkloadPlacement) are available.
// Requires TMCFeature to be enabled.
func TMCAPIsEnabled() bool {
	return TMCEnabled() && utilfeature.DefaultFeatureGate.Enabled(TMCAPIs)
}

// TMCControllersEnabled returns true if TMC controllers feature flag is enabled.
// This controls whether TMC controllers for cluster registration and workload placement are active.
// Requires TMCFeature to be enabled.
func TMCControllersEnabled() bool {
	return TMCEnabled() && utilfeature.DefaultFeatureGate.Enabled(TMCControllers)
}

// TMCPlacementEnabled returns true if TMC placement engine feature flag is enabled.
// This controls whether advanced workload placement strategies are available.
// Requires TMCFeature to be enabled.
func TMCPlacementEnabled() bool {
	return TMCEnabled() && utilfeature.DefaultFeatureGate.Enabled(TMCPlacement)
}

// TMCAnyEnabled returns true if any TMC feature is enabled.
// This can be used for general TMC-related initialization checks.
func TMCAnyEnabled() bool {
	return TMCEnabled() ||
		utilfeature.DefaultFeatureGate.Enabled(TMCAPIs) ||
		utilfeature.DefaultFeatureGate.Enabled(TMCControllers) ||
		utilfeature.DefaultFeatureGate.Enabled(TMCPlacement)
}

// defaultGenericControlPlaneFeatureGates consists of all known kcp-specific feature keys.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout kcp binaries.
var defaultGenericControlPlaneFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	VirtualWorkspaces: {Default: false, PreRelease: featuregate.Alpha},
	LocationInVirtualWorkspaces: {Default: false, PreRelease: featuregate.Alpha},
	AdvancedScheduling: {Default: false, PreRelease: featuregate.Alpha},
	ShardedServer: {Default: false, PreRelease: featuregate.Alpha},
	CacheAPIs: {Default: false, PreRelease: featuregate.Alpha},
	EnableDeprecatedAPIExportVirtualWorkspacesUrls: {Default: false, PreRelease: featuregate.Alpha},
	WorkspaceMounts: {Default: false, PreRelease: featuregate.Alpha},
	// TMC Feature Flags
	TMCFeature: {Default: false, PreRelease: featuregate.Alpha},
	TMCAPIs: {Default: false, PreRelease: featuregate.Alpha},
	TMCControllers: {Default: false, PreRelease: featuregate.Alpha},
	TMCPlacement: {Default: false, PreRelease: featuregate.Alpha},
	TMCMetricsAggregation: {Default: false, PreRelease: featuregate.Alpha},
	TMCAdvancedAggregation: {Default: false, PreRelease: featuregate.Alpha},
	TMCTimeSeriesConsolidation: {Default: false, PreRelease: featuregate.Alpha},
}
