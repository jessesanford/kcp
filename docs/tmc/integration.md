# TMC Integration with KCP

TMC (Tenant Multi-Cluster) integrates with KCP through the standard APIExport/APIBinding system as an external system that consumes KCP APIs while respecting workspace isolation.

## Architecture Overview

TMC is **NOT** part of KCP - it is an external controller system that:

- **Consumes KCP APIs** through APIBinding for multi-tenant API access
- **Manages workload placement** across multiple physical clusters
- **Respects workspace boundaries** for tenant isolation
- **Follows KCP patterns** for API design and controller implementation

## API Integration Flow

1. **TMC APIs** are exported via `tmc.kcp.io` APIExport in a TMC workspace
2. **Tenant workspaces** bind to TMC APIs via APIBinding
3. **External TMC controllers** watch bound APIs in tenant workspaces
4. **Physical clusters** register via ClusterRegistration resources
5. **Workload placement** is managed via WorkloadPlacement resources

## TMC API Types

### ClusterRegistration

Represents a physical cluster that can execute workloads:

- **Location-based placement** - logical location for placement decisions
- **Capability management** - describes what the cluster can execute
- **API binding integration** - connects to KCP APIs via APIBinding
- **Health tracking** - monitors cluster connectivity and status

### WorkloadPlacement

Defines placement policies for workloads across clusters:

- **Placement strategies** - RoundRobin, Affinity, Spread
- **Location selection** - cluster selection by location labels
- **Capability requirements** - required cluster capabilities
- **Multi-tenant aware** - operates within workspace boundaries

## Controller Architecture

The TMC APIExport controller follows exact KCP patterns:

- **Workspace isolation** - operates on TMC APIExports only
- **Standard reconciliation** - follows KCP controller patterns
- **Resource validation** - ensures proper APIResourceSchema configuration
- **Integration verification** - validates TMC API availability

## Usage Examples

### 1. Create TMC APIExport

```yaml
apiVersion: apis.kcp.io/v1alpha1
kind: APIExport
metadata:
  name: tmc.kcp.io
spec:
  latestResourceSchemas:
    - tmc.kcp.io.v1alpha1.ClusterRegistration
    - tmc.kcp.io.v1alpha1.WorkloadPlacement
```

### 2. Bind to TMC APIs

```yaml
apiVersion: apis.kcp.io/v1alpha1
kind: APIBinding
metadata:
  name: tmc-binding
spec:
  reference:
    export:
      path: root:tmc
      name: tmc.kcp.io
```

### 3. Register a Cluster

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-us-west
spec:
  location: us-west-2
  capabilities:
  - type: compute
    available: true
  - type: storage
    available: true
```

### 4. Define Placement Policy

```yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: web-app-placement
spec:
  strategy: Spread
  locationSelector:
    matchLabels:
      region: us-west
  capabilityRequirements:
  - type: compute
    required: true
```

## Implementation Notes

This implementation provides the **foundation** for TMC APIExport integration with KCP. The controller:

- **Validates TMC APIExport configuration** for proper KCP integration
- **Ensures APIResourceSchemas exist** for TMC resource types  
- **Follows KCP architectural patterns** exactly
- **Maintains workspace isolation** throughout

Future phases will build upon this foundation to add:

- External TMC controllers for cluster and workload management
- Workload synchronization engines
- Advanced placement algorithms
- Monitoring and observability integration

This follows KCP's established patterns for API distribution and multi-tenant system integration.