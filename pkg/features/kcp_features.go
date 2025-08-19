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

<<<<<<< HEAD
	// TMC Feature Flags

	// owner: @jessesanford
	// alpha: v0.1
	// Master feature flag for all TMC functionality. When disabled, all TMC features are disabled.
	TMCFeature featuregate.Feature = "TMCFeature"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC APIs (ClusterRegistration, WorkloadPlacement) and APIExport functionality.
	TMCAPIs featuregate.Feature = "TMCAPIs"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC controllers for cluster registration and workload placement management.
	TMCControllers featuregate.Feature = "TMCControllers"

	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC placement engine for advanced workload placement strategies.
	TMCPlacement featuregate.Feature = "TMCPlacement"
=======
	// owner: @jessesanford
	// alpha: v0.1
	// Enables TMC (Transparent Multi-Cluster) APIs and controllers for workload placement across clusters.
	// This feature provides advanced placement policies, cluster registration, and workload distribution capabilities.
	TMCAPIs featuregate.Feature = "TMCAPIs"
>>>>>>> origin/feature/tmc-impl4/07-controller-binary-fixed
)

// DefaultFeatureGate exposes the upstream feature gate, but with our gate setting applied.
var DefaultFeatureGate = utilfeature.DefaultFeatureGate

func init() {
	utilruntime.Must(utilfeature.DefaultMutableFeatureGate.AddVersioned(defaultVersionedGenericControlPlaneFeatureGates))
}

<<<<<<< HEAD
func KnownFeatures() []string {
	features := make([]string, 0, len(defaultVersionedGenericControlPlaneFeatureGates))
	for k := range defaultVersionedGenericControlPlaneFeatureGates {
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

func featureSpecAtEmulationVersion(v featuregate.VersionedSpecs, emulationVersion *version.Version) *featuregate.FeatureSpec {
	i := len(v) - 1
	for ; i >= 0; i-- {
		if v[i].Version.GreaterThan(emulationVersion) {
			continue
		}
		return &v[i]
	}
	return &featuregate.FeatureSpec{
		Default:    false,
		PreRelease: featuregate.PreAlpha,
		Version:    version.MajorMinor(0, 0),
	}
}

func (f *kcpFeatureGate) String() string {
	pairs := []string{}
	emulatedVersion := utilfeature.DefaultMutableFeatureGate.EmulationVersion()

	for featureName, versionedSpecs := range defaultVersionedGenericControlPlaneFeatureGates {
		spec := featureSpecAtEmulationVersion(versionedSpecs, emulatedVersion)
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

// defaultGenericControlPlaneFeatureGates consists of all known Kubernetes-specific feature keys
// in the generic control plane code. To add a new feature, define a key for it above and add it
// here. The Version field should be set to whatever is specified in
// https://github.com/kubernetes/kubernetes/blob/master/pkg/features/versioned_kube_features.go.
// For features that are kcp-specific, the Version should be set to whatever go.mod k8s.io
// dependencies version we're currently using.
=======
// defaultVersionedGenericControlPlaneFeatureGates consists of all known kcp-specific feature keys.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout kcp binaries.
>>>>>>> origin/feature/tmc-impl4/07-controller-binary-fixed
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
	// TMC Feature Flags
	TMCFeature: {
		{Version: version.MustParse("0.1"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCAPIs: {
		{Version: version.MustParse("0.1"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCControllers: {
		{Version: version.MustParse("0.1"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCPlacement: {
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