# TMC Feature Flags

This document describes the TMC (Transparent Multi-Cluster) feature flags available in KCP and how to use them.

## Overview

TMC functionality in KCP is controlled by a hierarchical set of feature flags. All TMC features are disabled by default and must be explicitly enabled using command-line flags.

## Feature Flag Hierarchy

TMC uses a master feature flag with sub-feature flags for fine-grained control:

```
TMCFeature (master)
├── TMCAPIs (API types and exports)
├── TMCControllers (controllers and reconciliation)
└── TMCPlacement (placement engine)
```

### Master Feature Flag

- **`TMCFeature`**: Master feature flag that must be enabled for any TMC functionality
  - **Owner**: @jessesanford
  - **Version**: v0.1
  - **Default**: `false` (disabled)
  - **Stage**: Alpha

### Sub-Feature Flags

All sub-feature flags require the master `TMCFeature` to be enabled:

- **`TMCAPIs`**: Enables TMC API types (ClusterRegistration, WorkloadPlacement) and APIExport functionality
  - **Owner**: @jessesanford  
  - **Version**: v0.1
  - **Default**: `false` (disabled)
  - **Stage**: Alpha

- **`TMCControllers`**: Enables TMC controllers for cluster registration and workload placement management
  - **Owner**: @jessesanford
  - **Version**: v0.1  
  - **Default**: `false` (disabled)
  - **Stage**: Alpha

- **`TMCPlacement`**: Enables TMC placement engine for advanced workload placement strategies
  - **Owner**: @jessesanford
  - **Version**: v0.1
  - **Default**: `false` (disabled)
  - **Stage**: Alpha

## Usage

### Enabling TMC Features

Use the `--feature-gates` flag when starting KCP:

```bash
# Enable all TMC functionality
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true

# Enable only TMC APIs (requires TMCFeature)
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true

# Enable TMC and controllers only
kcp start --feature-gates=TMCFeature=true,TMCControllers=true
```

### Checking Feature Status

You can verify which features are enabled by checking the KCP server logs or using the feature gate utilities.

## Programming Interface

TMC feature flags can be checked programmatically using utility functions:

```go
import "github.com/kcp-dev/kcp/pkg/features"

// Check if master TMC feature is enabled
if features.TMCEnabled() {
    // TMC functionality is available
}

// Check if TMC APIs are enabled
if features.TMCAPIsEnabled() {
    // TMC API types are available
    // ClusterRegistration and WorkloadPlacement APIs can be used
}

// Check if TMC controllers are enabled  
if features.TMCControllersEnabled() {
    // TMC controllers will be started
    // Cluster registration and placement reconciliation is active
}

// Check if TMC placement engine is enabled
if features.TMCPlacementEnabled() {
    // Advanced placement strategies are available
    // Placement engine will evaluate workload placement decisions
}

// Check if any TMC feature is enabled
if features.TMCAnyEnabled() {
    // At least one TMC feature is enabled
    // Used for general TMC initialization checks
}
```

## Feature Dependencies

The TMC feature flags have the following dependency relationships:

1. **Master Dependency**: All sub-features require `TMCFeature=true`
   - `TMCAPIsEnabled()` returns `TMCEnabled() && Enabled(TMCAPIs)`
   - `TMCControllersEnabled()` returns `TMCEnabled() && Enabled(TMCControllers)` 
   - `TMCPlacementEnabled()` returns `TMCEnabled() && Enabled(TMCPlacement)`

2. **Independent Sub-features**: Sub-features can be enabled independently of each other
   - You can enable `TMCAPIs` without `TMCControllers`
   - You can enable `TMCPlacement` without `TMCAPIs`

## Use Cases

### Development and Testing

```bash
# Enable only APIs for API schema testing
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true

# Enable APIs and controllers for integration testing
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true
```

### Production Rollout

```bash
# Phase 1: Deploy TMC APIs only
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true

# Phase 2: Add TMC controllers  
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true

# Phase 3: Enable full TMC functionality
kcp start --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true
```

### Feature-Specific Testing

```bash
# Test placement engine only
kcp start --feature-gates=TMCFeature=true,TMCPlacement=true

# Test controller behavior only
kcp start --feature-gates=TMCFeature=true,TMCControllers=true
```

## Migration Path

When migrating TMC functionality:

1. **Start with APIs**: Enable `TMCFeature` and `TMCAPIs` first
2. **Add Controllers**: Enable `TMCControllers` once APIs are stable
3. **Enable Placement**: Enable `TMCPlacement` for advanced placement features
4. **Full Enablement**: Enable all features for complete TMC functionality

## Troubleshooting

### Common Issues

1. **Sub-features not working**: Ensure `TMCFeature=true` is set
2. **Controllers not starting**: Verify `TMCControllers=true` is enabled
3. **APIs not available**: Check that `TMCAPIs=true` is set
4. **Placement not working**: Confirm `TMCPlacement=true` is enabled

### Debugging

Check KCP server logs for feature flag status:

```bash
kubectl logs <kcp-pod> | grep -i "tmc\|feature"
```

### Validation

Use the utility functions to validate feature state programmatically:

```go
// Log current TMC feature status
klog.InfoS("TMC Feature Status",
    "TMCEnabled", features.TMCEnabled(),
    "TMCAPIsEnabled", features.TMCAPIsEnabled(), 
    "TMCControllersEnabled", features.TMCControllersEnabled(),
    "TMCPlacementEnabled", features.TMCPlacementEnabled(),
)
```

## Security Considerations

- TMC features are disabled by default for security
- Enable only the features you need
- Test feature combinations in non-production environments
- Monitor feature flag changes in production

## Future Enhancements

Additional TMC feature flags may be added for:
- TMC networking features
- TMC storage integration  
- TMC security policies
- TMC monitoring and observability

All future TMC features will follow the same hierarchical pattern with the master `TMCFeature` flag as the root dependency.