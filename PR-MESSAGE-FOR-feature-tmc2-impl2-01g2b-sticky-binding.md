<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the second part of the session affinity management system by introducing the **StickyBinding API** and **SessionBindingConstraint API** as part of TMC's session management framework. These APIs provide comprehensive session-to-cluster binding persistence and management functionality to ensure consistent workload placement across multi-cluster environments.

**Key Components Implemented:**
- **StickyBinding CRD**: Session-to-cluster binding persistence with auto-renewal, conflict resolution, and multiple storage backends
- **SessionBindingConstraint CRD**: Operational policies and resource limits for session bindings
- **Comprehensive Storage Backends**: Memory, ConfigMap, Secret, CustomResource, and External (Redis/etcd) with encryption
- **Binding Lifecycle Management**: Auto-renewal, expiration handling, and conflict resolution strategies
- **Performance Metrics**: Request tracking, latency measurement, and error monitoring
- **Security Features**: TLS configuration, encryption algorithms (AES256/ChaCha20Poly1305), and secure secret references

**Architecture Integration:**
- Full KCP workspace isolation with proper `kcp.io/cluster` annotations
- Follows established KCP API patterns with condition management
- Integrates with SessionAffinityPolicy from the foundation branch (01g2a)
- Production-ready with comprehensive validation rules and status tracking

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 1 session affinity functionality (01g2b split plan)

## Release Notes

```release-note
Add StickyBinding and SessionBindingConstraint APIs for TMC session management, providing session-to-cluster binding persistence with auto-renewal, multiple storage backends, and comprehensive constraint enforcement.
```

**Testing Coverage:**
- Comprehensive API validation tests for all binding types
- Storage backend configuration validation (5 backend types)
- Conflict resolution strategy testing
- Binding lifecycle and phase transition validation  
- Performance metrics calculation testing
- Encryption algorithm support verification
- Edge case handling and error condition testing

**CRD Generation:**
- Generated CRDs for both StickyBinding and SessionBindingConstraint
- Generated deepcopy methods for all types
- Updated API registration and schema integration
- Includes proper RBAC markers and kubebuilder annotations

**Implementation Size:**
- Implementation: ~806 lines (types_sticky_binding.go)
- Tests: ~591 lines (types_sticky_binding_test.go) 
- Total functionality including comprehensive constraints system and multiple storage backends

This branch builds upon the core affinity foundation (01g2a) and provides the essential binding management layer for TMC's session affinity system.

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>