<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the critical **placement engine interface and round-robin algorithm** that **Agent 1 (WorkloadPlacement Controller) depends on** to make cluster placement decisions. This unblocks parallel development by providing the core placement functionality.

### Key Components

1. **PlacementEngine Interface** (`pkg/placement/engine/interface.go`)
   - Clean interface contract for placement algorithms
   - Supports WorkloadPlacement and ClusterRegistration API integration
   - Returns scored placement decisions with rationale

2. **RoundRobinEngine Implementation** (`pkg/placement/engine/round_robin.go`)
   - Thread-safe round-robin distribution across clusters
   - Comprehensive cluster filtering (labels, locations, explicit names)
   - Maintains placement state for consistent distribution
   - Handles edge cases (no eligible clusters, concurrent access)

3. **Comprehensive Test Suite** (`pkg/placement/engine/engine_test.go`)
   - 95.1% test coverage exceeding >80% requirement
   - Table-driven tests for all filtering scenarios
   - Round-robin distribution verification
   - Edge case and error condition testing

### TMC API Integration

The placement engine integrates with the complete TMC API types:
- `WorkloadPlacement` for placement requirements and policies
- `ClusterRegistration` for available cluster information  
- `ClusterSelector` for sophisticated cluster filtering
- `PlacementDecision` for structured placement results

### Agent Coordination

This implementation provides the **critical interface that Agent 1 needs** to complete the WorkloadPlacement Controller. The interface design enables:
- Clean separation between controller logic and placement algorithms
- Future placement engine implementations (resource-aware, affinity-based)
- Testable and maintainable placement decisions

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Fixes: TMC Reimplementation Attempt 2 - Placement Engine Interface (Agent 3 deliverable)

## Release Notes

```yaml
# placement-engine-interface.yaml
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: example-placement
spec:
  workloadSelector:
    labelSelector:
      matchLabels:
        app: my-app
  clusterSelector:
    locationSelector: ["us-west"]
  placementPolicy: RoundRobin
  numberOfClusters: 2
```

The TMC placement engine now supports round-robin workload distribution across filtered clusters with comprehensive test coverage.