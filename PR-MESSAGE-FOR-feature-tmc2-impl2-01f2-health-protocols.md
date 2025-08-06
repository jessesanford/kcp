<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

Implements the second sub-branch of the 01f-placement-health split plan, adding comprehensive multi-protocol health monitoring capabilities for TMC workloads. This PR introduces the `ProtocolHealthMonitor` API with support for HTTP/HTTPS, TCP, and gRPC health checks.

**Key Features:**
- **Multi-protocol support**: HTTP, HTTPS, TCP, gRPC, and gRPC with TLS
- **Advanced TLS configuration**: Support for mutual TLS authentication and custom certificate validation
- **Endpoint-level monitoring**: Individual endpoint health status tracking with detailed results
- **Protocol-specific configurations**: Tailored health check parameters for each protocol type
- **Comprehensive validation**: Built-in validation for all protocol configurations and health check results
- **KCP integration**: Proper conditions-based status management following KCP patterns

**Implementation Highlights:**
- 563 lines of implementation with 121% test coverage (684 test lines)
- Supports HTTP methods (GET, POST, PUT, HEAD) with custom headers and response validation
- TCP health checks with send/receive data validation
- gRPC health checks with service-specific configurations
- Advanced TLS support including mutual TLS and certificate validation
- Endpoint health status transitions and scoring algorithms

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Implementation Plan 2 - Branch split for 01f-placement-health
Contributes to implementing comprehensive health monitoring for multi-cluster workloads.

## Release Notes

```markdown
Add ProtocolHealthMonitor API for multi-protocol health checking in TMC

This release introduces advanced health monitoring capabilities supporting HTTP/HTTPS, TCP, and gRPC protocols. The new ProtocolHealthMonitor resource provides:

- Multi-protocol health checks with protocol-specific configurations
- Advanced TLS support including mutual authentication
- Endpoint-level monitoring with detailed health status tracking  
- Comprehensive validation and error reporting
- Integration with TMC's multi-cluster workload management

This API enables sophisticated health monitoring strategies across different protocols and transport layers, supporting diverse workload types in multi-cluster environments.
```