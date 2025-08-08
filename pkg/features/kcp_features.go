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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	genericfeatures "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/version"
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

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC (Transparent Multi-Cluster) APIs and controllers for workload placement across clusters.
	// This feature provides advanced placement policies, cluster registration, and workload distribution capabilities.
	TMCAPIs featuregate.Feature = "TMCAPIs"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC metrics aggregation across clusters with basic sum strategy.
	TMCMetricsAggregation featuregate.Feature = "TMCMetricsAggregation"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables advanced TMC aggregation strategies (avg, max, min) beyond basic sum.
	TMCAdvancedAggregation featuregate.Feature = "TMCAdvancedAggregation"
)

// DefaultFeatureGate exposes the upstream feature gate, but with our gate setting applied.
var DefaultFeatureGate = utilfeature.DefaultFeatureGate

func init() {
	utilruntime.Must(utilfeature.DefaultMutableFeatureGate.AddVersioned(defaultVersionedGenericControlPlaneFeatureGates))
}

// defaultVersionedGenericControlPlaneFeatureGates consists of all known kcp-specific feature keys.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout kcp binaries.
var defaultVersionedGenericControlPlaneFeatureGates = map[featuregate.Feature]featuregate.VersionedSpecs{
	VirtualWorkspaces: {
		{Version: version.MustParse("1.24"), Default: false, PreRelease: featuregate.Alpha},
		{Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Beta},
	},
	LocationInVirtualWorkspaces: {
		{Version: version.MustParse("1.24"), Default: false, PreRelease: featuregate.Alpha},
		{Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Beta},
	},
	AdvancedScheduling: {
		{Version: version.MustParse("1.24"), Default: false, PreRelease: featuregate.Alpha},
	},
	ShardedServer: {
		{Version: version.MustParse("1.27"), Default: false, PreRelease: featuregate.Alpha},
	},
	CacheAPIs: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	EnableDeprecatedAPIExportVirtualWorkspacesUrls: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCAPIs: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCMetricsAggregation: {
		{Version: version.MustParse("0.1"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCAdvancedAggregation: {
		{Version: version.MustParse("0.1"), Default: false, PreRelease: featuregate.Alpha},
	},
	// inherited features from generic apiserver, relisted here to get a conflict if it is changed
	// unintentionally on either side:
	genericfeatures.APIResponseCompression: {
		{Version: version.MustParse("1.8"), Default: false, PreRelease: featuregate.Alpha},
		{Version: version.MustParse("1.16"), Default: true, PreRelease: featuregate.Beta},
	},

	genericfeatures.OpenAPIEnums: {
		{Version: version.MustParse("1.23"), Default: false, PreRelease: featuregate.Alpha},
		{Version: version.MustParse("1.24"), Default: true, PreRelease: featuregate.Beta},
	},
}