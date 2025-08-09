## Summary

This PR adds the **ResourceAwareEngine** with advanced cluster placement algorithms. This is the **third split** of an oversized placement engine PR (702 lines) to meet KCP size requirements.

**Key Components:**
- `ResourceAwareEngine` implementing sophisticated placement scoring
- Multi-factor cluster evaluation: CPU, memory, storage resources
- Location affinity with distance calculations  
- Cluster health status and constraint validation
- Configurable scoring weights and comprehensive error handling

**Dependencies:** Requires feature/tmc2-impl2/05c2-api-types (full API types)

**Agent Integration:** Provides the advanced placement engine Agent 1 needs for sophisticated cluster selection beyond simple round-robin.

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Emergency PR Split (3/4)

## Release Notes

```yaml
apiVersion: v1
kind: ConfigMap  
metadata:
  name: release-notes
data:
  note: |
    Adds ResourceAwareEngine with advanced cluster placement algorithms
    including resource utilization analysis, location affinity, health
    validation, and configurable scoring for optimal workload placement.
```