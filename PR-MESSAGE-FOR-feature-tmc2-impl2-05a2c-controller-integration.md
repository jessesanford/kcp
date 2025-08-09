<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR implements the TMC placement controller integration (Part 3/4) for KCP's Transparent Multi-Cluster system. It adds the core placement decision engine with comprehensive reconciliation logic, enabling intelligent workload placement across available clusters based on placement specifications.

### Key Components

**Placement Controller (`controller.go`)**
- Implements KCP controller patterns with proper workspace isolation
- Manages informers for Placement and Location resources
- Provides event handling for placement decisions and location changes
- Integrates with KCP's shared informer factory and client systems
- Includes comprehensive error handling and logging

**Placement Reconciler (`reconciler.go`)**  
- Core reconciliation logic for WorkloadPlacement resources
- Placement specification validation with constraint checking
- Location filtering based on selectors and requirements
- Placement scoring algorithm for optimal cluster selection
- Status condition management with proper error propagation
- Support for affinity/anti-affinity rules (foundation for future PRs)

**Integration Wiring**
- Registers controller with KCP server controller manager
- TMCAPIs feature gate protection for controlled rollout
- Proper integration with KCP's informer factory system
- Controller lifecycle management with graceful shutdown

### Features Implemented

- **Placement Decision Engine**: Intelligent selection of target clusters based on placement specifications
- **Location-Based Filtering**: Filter clusters using location selectors and label requirements  
- **Placement Scoring**: Prioritize clusters using scoring algorithm for optimal placement
- **Constraint Validation**: Comprehensive validation of placement specifications
- **Status Management**: Proper condition tracking with Ready, Valid, and Scheduled states
- **Event-Driven Processing**: React to placement and location resource changes
- **Workspace Isolation**: Maintain logical cluster boundaries for multi-tenancy

### Dependencies

This PR builds on:
- `05a2a-api-foundation`: Workload API types (Placement, Location)
- `05a2b-decision-engine`: Core placement decision logic

### Testing Coverage

The implementation includes:
- Comprehensive input validation with error handling
- Proper status condition management
- Resource filtering and scoring logic
- Integration point validation
- Informer event handling

### Future Enhancements

Ready for extension in subsequent PRs:
- Advanced constraint types (resource requirements, taints/tolerations)
- Sophisticated scoring algorithms with multiple criteria
- Placement session management and affinity rules
- Health-based placement decisions
- Metrics and observability integration

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC implementation plan - placement controller integration phase.

## Release Notes

```yaml
feature/tmc2-impl2/05a2c-controller-integration:
  - Implements TMC placement controller with decision engine for intelligent workload placement
  - Adds comprehensive reconciliation logic with location filtering and cluster scoring
  - Integrates placement controller with KCP server infrastructure and informer factory
  - Provides foundation for advanced placement policies and constraint satisfaction
```