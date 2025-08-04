# TMC (Transparent Multi-Cluster) Feature Flags

This document describes the feature flags that control TMC (Transparent Multi-Cluster) functionality in KCP. TMC provides advanced multi-cluster resource management capabilities with transparent workload placement, synchronization, and migration.

## Overview

TMC features are controlled through a hierarchical feature flag system based on Kubernetes' standard feature gate mechanism. All TMC functionality is **experimental** (Alpha maturity) and requires explicit enablement.

## Master Feature Flag

### TransparentMultiCluster

- **Status**: Alpha (v1.32+)
- **Default**: `false`
- **Description**: Master flag that enables the complete TMC system for cross-cluster resource management
- **Required**: This flag MUST be enabled for any TMC functionality to work

**Example**:
```bash
kcp start --feature-gates=TransparentMultiCluster=true
```

## Component Feature Flags

All component flags require `TransparentMultiCluster=true` to function. Attempting to enable a component flag without the master flag will result in a validation error at startup.

### TMCPlacement

- **Status**: Alpha (v1.32+) 
- **Default**: `false`
- **Description**: Enables TMC placement scheduling capabilities
- **Features**: Intelligent workload placement across multiple clusters based on policies and constraints
- **Requires**: `TransparentMultiCluster=true`

### TMCSynchronization

- **Status**: Alpha (v1.32+)
- **Default**: `false` 
- **Description**: Enables TMC bidirectional resource synchronization
- **Features**: Real-time sync of resources and status between KCP and target clusters
- **Requires**: `TransparentMultiCluster=true`

### TMCVirtualWorkspaces

- **Status**: Alpha (v1.32+)
- **Default**: `false`
- **Description**: Enables TMC virtual workspace aggregation
- **Features**: Unified views of resources across multiple clusters through virtual workspaces
- **Requires**: `TransparentMultiCluster=true`

### TMCMigration

- **Status**: Alpha (v1.32+) 
- **Default**: `false`
- **Description**: Enables TMC workload migration capabilities
- **Features**: Live migration of workloads between clusters with minimal downtime
- **Requires**: `TransparentMultiCluster=true`

### TMCStatusAggregation

- **Status**: Alpha (v1.32+)
- **Default**: `false`
- **Description**: Enables TMC cross-cluster status aggregation
- **Features**: Comprehensive status reporting and health monitoring across all clusters
- **Requires**: `TransparentMultiCluster=true`

## Usage Examples

### Enable All TMC Features

```bash
kcp start --feature-gates=TransparentMultiCluster=true,TMCPlacement=true,TMCSynchronization=true,TMCVirtualWorkspaces=true,TMCMigration=true,TMCStatusAggregation=true
```

### Enable Basic TMC with Selective Features

```bash
# Enable TMC with only placement and synchronization
kcp start --feature-gates=TransparentMultiCluster=true,TMCPlacement=true,TMCSynchronization=true
```

### Development Setup

```bash
# Enable TMC for development/testing
kcp start --feature-gates=TransparentMultiCluster=true
```

## Feature Discovery

### Command Line Discovery

List all available feature flags:
```bash
kcp start options | grep -A 20 feature-gates
```

### Runtime Discovery

Check which TMC features are enabled at runtime by examining KCP server logs:
```
INFO Starting TMC (Transparent Multi-Cluster) manager
INFO TMC features ENABLED: [Placement, Synchronization]
INFO TMC features DISABLED: [VirtualWorkspaces, Migration, StatusAggregation]
```

## Workload Syncer Integration

The workload syncer automatically detects TMC feature flag state and adapts behavior accordingly:

### TMC Enabled Mode
- Full TMC feature integration
- Enhanced error handling, metrics, health monitoring, and tracing
- Advanced resource synchronization capabilities

**Example log output**:
```
INFO Starting TMC-enabled syncer...
INFO TMC (Transparent Multi-Cluster) is ENABLED
INFO TMC features ENABLED: [Placement, Synchronization]
INFO TMC features DISABLED: [VirtualWorkspaces, Migration, StatusAggregation]
```

### TMC Disabled Mode (Legacy)
- Syncer runs in legacy compatibility mode
- Basic functionality without TMC enhancements
- Clear logging about disabled state

**Example log output**:
```
WARNING TMC (Transparent Multi-Cluster) is DISABLED - syncer will run in legacy mode
WARNING To enable TMC features, start KCP with --feature-gates=TransparentMultiCluster=true
INFO Running syncer in legacy mode - TMC features disabled
INFO Syncer started in DISABLED mode - monitoring for TMC feature flag changes
```

## Validation and Error Handling

### Hierarchical Validation

The system enforces hierarchical feature flag dependencies:
```bash
# This will FAIL with validation error
kcp start --feature-gates=TMCPlacement=true
# Error: TMC feature flag validation failed: feature flag TMCPlacement requires TransparentMultiCluster=true

# This will SUCCEED
kcp start --feature-gates=TransparentMultiCluster=true,TMCPlacement=true
```

### Runtime Validation

Feature flag dependencies are validated at startup before any components initialize:
```
INFO Validate TMC feature flag dependencies
ERROR TMC feature flag validation failed: feature flag TMCPlacement requires TransparentMultiCluster=true
```

## Implementation Details

### Code Locations

- **Feature Definitions**: [`pkg/features/kcp_features.go`](../pkg/features/kcp_features.go)
- **KCP Server Integration**: [`cmd/kcp/kcp.go`](../cmd/kcp/kcp.go), [`pkg/server/controllers.go`](../pkg/server/controllers.go)
- **Syncer Integration**: [`cmd/workload-syncer/main.go`](../cmd/workload-syncer/main.go), [`pkg/reconciler/workload/syncer/syncer.go`](../pkg/reconciler/workload/syncer/syncer.go)
- **TMC Manager**: [`pkg/reconciler/workload/tmc/manager.go`](../pkg/reconciler/workload/tmc/manager.go)

### Validation Function

The `ValidateTMCFeatureFlags()` function enforces hierarchical dependencies:
```go
func ValidateTMCFeatureFlags() error {
    gate := DefaultFeatureGate
    
    tmcSubFeatures := []featuregate.Feature{
        TMCPlacement, TMCSynchronization, TMCVirtualWorkspaces, 
        TMCMigration, TMCStatusAggregation,
    }
    
    masterEnabled := gate.Enabled(TransparentMultiCluster)
    
    for _, feature := range tmcSubFeatures {
        if gate.Enabled(feature) && !masterEnabled {
            return fmt.Errorf("feature flag %s requires TransparentMultiCluster=true", feature)
        }
    }
    
    return nil
}
```

## Operational Considerations

### Production Deployment

TMC features are **experimental** (Alpha maturity). For production use:
- Start with `TransparentMultiCluster=true` only
- Gradually enable individual component flags after thorough testing
- Monitor logs for feature flag status and validation messages
- Plan for potential breaking changes in future releases

### Debugging and Troubleshooting

1. **Check Feature Flag Status**: Examine startup logs for TMC feature flag validation and status messages
2. **Component Behavior**: Look for component-specific logging about enabled/disabled TMC features
3. **Graceful Degradation**: Components log clearly when TMC features are disabled but continue operating
4. **Runtime Changes**: Feature flags are read at startup; restart required for changes to take effect

### Monitoring

Both KCP server and workload syncer provide comprehensive logging about TMC feature flag status:
- Startup validation messages
- Feature enablement status per component  
- Graceful degradation notifications
- Runtime feature flag change detection (syncer only)

This follows Kubernetes ecosystem patterns for feature gate management and provides clear operational visibility into TMC functionality status.