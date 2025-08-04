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
	"k8s.io/apimachinery/pkg/util/version"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	logsapi "k8s.io/component-base/logs/api/v1"
)

const (
	// Every feature gate should add method here following this template:
	//
	// // owner: @username
	// // alpha: v1.4
	// MyFeature() bool.

	// owner: @mjudeikis
	// alpha: v0.1
	// Enables workspace mounts via frontProxy.
	WorkspaceMounts featuregate.Feature = "WorkspaceMounts"

	// owner: @mjudeikis
	// alpha: v0.1
	// Enables cache apis and controllers.
	CacheAPIs featuregate.Feature = "CacheAPIs"

	// owner: @mjudeikis
	// alpha: v0.1
	// Enables VirtualWorkspace urls on APIExport. This enables to use Deprecated APIExport VirtualWorkspace urls.
	// This is a temporary feature to ease the migration to the new VirtualWorkspace urls.
	EnableDeprecatedAPIExportVirtualWorkspacesUrls featuregate.Feature = "EnableDeprecatedAPIExportVirtualWorkspacesUrls"

	// TMC (Transparent Multi-Cluster) Feature Gates

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables the complete TMC (Transparent Multi-Cluster) system for cross-cluster resource management.
	// This is the master flag that must be enabled for any TMC functionality.
	TransparentMultiCluster featuregate.Feature = "TransparentMultiCluster"

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables TMC placement scheduling capabilities. Requires TransparentMultiCluster=true.
	// Provides intelligent workload placement across multiple clusters based on policies and constraints.
	TMCPlacement featuregate.Feature = "TMCPlacement"

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables TMC bidirectional resource synchronization. Requires TransparentMultiCluster=true.
	// Provides real-time sync of resources and status between KCP and target clusters.
	TMCSynchronization featuregate.Feature = "TMCSynchronization"

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables TMC virtual workspace aggregation. Requires TransparentMultiCluster=true.
	// Provides unified views of resources across multiple clusters through virtual workspaces.
	TMCVirtualWorkspaces featuregate.Feature = "TMCVirtualWorkspaces"

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables TMC workload migration capabilities. Requires TransparentMultiCluster=true.
	// Provides live migration of workloads between clusters with minimal downtime.
	TMCMigration featuregate.Feature = "TMCMigration"

	// owner: @kcp-dev
	// alpha: v0.1
	// Enables TMC cross-cluster status aggregation. Requires TransparentMultiCluster=true.
	// Provides comprehensive status reporting and health monitoring across all clusters.
	TMCStatusAggregation featuregate.Feature = "TMCStatusAggregation"
)

// DefaultFeatureGate exposes the upstream feature gate, but with our gate setting applied.
var DefaultFeatureGate = utilfeature.DefaultFeatureGate

func init() {
	utilruntime.Must(utilfeature.DefaultMutableFeatureGate.AddVersioned(defaultVersionedGenericControlPlaneFeatureGates))
}

func KnownFeatures() []string {
	features := make([]string, 0, len(defaultVersionedGenericControlPlaneFeatureGates))
	for k := range defaultVersionedGenericControlPlaneFeatureGates {
		features = append(features, string(k))
	}
	return features
}

// ValidateTMCFeatureFlags validates that TMC sub-features require TransparentMultiCluster
func ValidateTMCFeatureFlags() error {
	gate := DefaultFeatureGate
	
	// Check if any TMC sub-feature is enabled without the master flag
	tmcSubFeatures := []featuregate.Feature{
		TMCPlacement,
		TMCSynchronization, 
		TMCVirtualWorkspaces,
		TMCMigration,
		TMCStatusAggregation,
	}
	
	masterEnabled := gate.Enabled(TransparentMultiCluster)
	
	for _, feature := range tmcSubFeatures {
		if gate.Enabled(feature) && !masterEnabled {
			return fmt.Errorf("feature flag %s requires TransparentMultiCluster=true", feature)
		}
	}
	
	return nil
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

// defaultGenericControlPlaneFeatureGates consists of all known Kubernetes-specific feature keys
// in the generic control plane code. To add a new feature, define a key for it above and add it
// here. The Version field should be set to whatever is specified in
// https://github.com/kubernetes/kubernetes/blob/master/pkg/features/versioned_kube_features.go.
// For features that are kcp-specific, the Version should be set to whatever go.mod k8s.io
// dependencies version we're currently using.
var defaultVersionedGenericControlPlaneFeatureGates = map[featuregate.Feature]featuregate.VersionedSpecs{
	WorkspaceMounts: {
		{Version: version.MustParse("1.28"), Default: false, PreRelease: featuregate.Alpha},
	},
	CacheAPIs: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	EnableDeprecatedAPIExportVirtualWorkspacesUrls: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},

	// TMC (Transparent Multi-Cluster) Feature Gates
	TransparentMultiCluster: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCPlacement: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCSynchronization: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCVirtualWorkspaces: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCMigration: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
	},
	TMCStatusAggregation: {
		{Version: version.MustParse("1.32"), Default: false, PreRelease: featuregate.Alpha},
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

	logsapi.LoggingBetaOptions: {
		{Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Beta},
	},

	logsapi.ContextualLogging: {
		{Version: version.MustParse("1.26"), Default: true, PreRelease: featuregate.Alpha},
	},
}
