## Summary

This PR establishes the core TMC (Topology Management Controller) APIs by implementing comprehensive API types for cluster management and workload placement in KCP. The implementation follows KCP architectural patterns with proper APIExport integration, workspace isolation, and client code generation.

**Core API Types Added:**
- **ClusterRegistration**: Cluster-scoped API for registering physical clusters with location, endpoint, capacity, and status tracking
- **WorkloadPlacement**: Namespaced API for workload placement policies with workload/cluster selectors and placement decisions

**KCP Integration:**
- APIExport manifest (`tmc.kcp.io`) for cross-workspace TMC API access
- APIResourceSchema manifests with proper OpenAPI schemas for KCP integration
- Full client code generation (clientsets, informers, listers) for TMC APIs
- Comprehensive deepcopy code generation for all TMC types

**API Architecture:**
- Workspace-aware design with proper logical cluster support
- Cluster vs Namespaced resource scoping for multi-tenant isolation
- KCP conditions API integration for standardized status management
- Comprehensive validation and defaulting via API machinery

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - establishes foundational APIs for the TMC system.

## Technical Details

**Files Structure:**
```
sdk/apis/tmc/
├── register.go                     (Group constants)
└── v1alpha1/
    ├── doc.go                      (Package documentation + codegen directives)
    ├── register.go                 (Scheme registration)
    ├── types_cluster.go            (ClusterRegistration API)
    ├── types_placement.go          (WorkloadPlacement API)  
    ├── types_shared.go             (Shared types: selectors, conditions)
    ├── types_*_test.go             (Comprehensive test coverage)
    └── zz_generated.*              (Generated deepcopy/defaults)
```

**Generated Artifacts:**
- CRD manifests in `config/crds/tmc.kcp.io_*.yaml`
- APIExport/APIResourceSchema in `config/root-phase0/`
- Client code in `sdk/client/clientset/versioned/*/tmc/`
- Informers in `sdk/client/informers/externalversions/tmc/`
- Listers in `sdk/client/listers/tmc/`

**Code Metrics:**
- Hand-written implementation: 506 lines (under 700 line target)
- Test coverage: 480 lines (comprehensive test scenarios)
- Generated code: ~2000+ lines (not counted toward PR size)

## Testing

**Comprehensive Test Coverage:**
- API validation and defaulting tests
- Edge case handling for all field types
- KCP conditions integration tests  
- JSON marshaling/unmarshaling validation
- Status subresource behavior testing

**Test Results:**
```bash
$ go test github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1
ok      github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1    0.003s
```

## Documentation

- Complete API documentation with examples
- KCP integration patterns documented
- Workspace isolation principles explained
- Code generation directives properly configured

## Breaking Changes

None - this is net new API introduction.

## Release Notes

```yaml
# TMC Core APIs Foundation

This release introduces the foundational APIs for the TMC (Topology Management Controller) system:

## New APIs

- **ClusterRegistration** (cluster-scoped): Register and manage physical Kubernetes clusters
- **WorkloadPlacement** (namespaced): Define workload placement policies across clusters

## Features

- Full KCP integration with APIExport and workspace isolation
- Comprehensive client libraries (clientsets, informers, listers)
- OpenAPI schema definitions for proper validation
- KCP conditions API integration for status management

## Usage

The TMC APIs are exported via APIExport `tmc.kcp.io` and can be consumed by:
- External TMC controllers via APIBinding
- Workspace applications requiring multi-cluster placement
- KCP-aware tooling needing cluster topology information

See the TMC documentation for integration examples and usage patterns.
```

🤖 Generated with [Claude Code](https://claude.ai/code)