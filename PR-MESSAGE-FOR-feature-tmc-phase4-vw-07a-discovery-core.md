<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the core infrastructure for KCP-aware resource discovery in virtual workspaces. This is the **first PR in a 3-PR split** to deliver complete discovery functionality while maintaining manageable review sizes.

### What's Included in This PR (710 lines)

- **ResourceDiscoveryInterface**: Core interface defining discovery capabilities for virtual workspaces
- **DiscoveryCache Interface**: Workspace-isolated caching interface with proper security boundaries
- **KCPDiscoveryProvider**: Main discovery provider integrating with KCP's APIExport system
- **Discovery Contracts**: Shared constants and configuration values
- **Prometheus Metrics**: Comprehensive monitoring for discovery operations
- **StubDiscoveryCache**: Temporary cache implementation to enable compilation during split
- **Security Improvements**: Replaced string workspace parameters with `logicalcluster.Name` throughout
- **Comprehensive Tests**: Unit tests covering provider initialization and core functionality

### PR Split Strategy

This implementation follows a careful 3-PR split to maintain code quality and reviewability:

1. **PR 1 (This PR)**: Core interfaces, provider, and metrics (~710 lines)
2. **PR 2 (Next)**: Cache implementation and APIExport converter (~650 lines)  
3. **PR 3 (Final)**: Watcher integration and end-to-end functionality (~700 lines)

### Key Features

- **Workspace Isolation**: Proper use of `logicalcluster.Name` ensures workspace security
- **KCP Integration**: Seamless integration with APIExport and informer infrastructure
- **Thread Safety**: All operations are thread-safe with proper synchronization
- **Monitoring**: Rich Prometheus metrics for discovery operations, cache hits, and active watchers
- **Extensible Design**: Clean interfaces enable future enhancements

### Security & Quality

- ✅ **Workspace Security**: Fixed security vulnerabilities identified in code review
- ✅ **Type Safety**: Use of `logicalcluster.Name` prevents workspace confusion
- ✅ **Error Handling**: Comprehensive error handling throughout
- ✅ **Documentation**: Extensive godoc comments for all public APIs
- ✅ **Testing**: Unit tests with 145 lines covering core functionality
- ✅ **Size Compliance**: 710 lines (under 800 line limit)

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of virtual workspace discovery implementation epic.

## Release Notes

```
Add KCP-aware resource discovery infrastructure for virtual workspaces with workspace isolation and comprehensive monitoring capabilities.
```

---

**Note**: This PR implements stubbed cache and watcher dependencies to maintain compilation. The actual implementations will be added in subsequent PRs following the planned split strategy.