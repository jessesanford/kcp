## Summary

This PR implements the foundational API types for TMC (Transparent Multi-Cluster) SyncTarget resources. The SyncTarget API represents physical clusters that can receive workloads through the TMC syncer infrastructure.

Key features:
- **SyncTarget API**: Comprehensive API definition for cluster registration and syncer management
- **KCP Integration**: Full integration with KCP's workspace isolation and logical cluster patterns  
- **Workload Placement**: Support for workload selection criteria and resource quotas
- **Status Management**: Rich status reporting including syncer health and cluster capacity
- **TMC Foundation**: Core API types that enable the TMC syncer controller functionality

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Wave 2A: SyncTarget Controller foundation

## Release Notes

```
Add SyncTarget API types for TMC syncer infrastructure.

The SyncTarget resource represents physical clusters that can receive 
workloads through the TMC syncer system. This provides the foundational
API for cluster registration, syncer deployment, and workload placement
within KCP's multi-tenant workspace model.
```

## Notes

This PR establishes the API foundation for the TMC syncer system. The actual controller implementation will follow in a separate PR to maintain appropriate PR sizing.

The API includes:
- Core SyncTarget resource definition with comprehensive spec/status
- Integration with ClusterRegistration for cluster access
- Support for workload selection and resource quotas
- Rich status conditions for syncer and cluster health
- Full KCP workspace isolation support

Generated code (deepcopy) is included and will be maintained through normal KCP code generation processes.