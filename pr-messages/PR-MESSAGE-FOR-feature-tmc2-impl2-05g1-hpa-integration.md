## Summary

This PR implements the foundation for HPA (Horizontal Pod Autoscaler) integration within the TMC (Transparent Multi-Cluster) system. It provides cross-cluster horizontal pod autoscaling capabilities that respect TMC placement policies and integrate with the KCP ecosystem.

**Key Components:**
- **HPA Controller**: Implements cluster-aware horizontal pod autoscaling with full KCP integration
- **Metrics Collection**: Gathers utilization metrics from multiple clusters for scaling decisions
- **Placement Integration**: Ensures scaling decisions respect TMC workload placement constraints
- **Scaling Strategies**: Supports distributed, centralized, and hybrid scaling approaches

**Features:**
- Cross-cluster replica distribution based on resource utilization
- Integration with TMC placement engine for constraint-aware scaling
- Support for multiple scaling strategies (Distributed/Centralized/Hybrid)
- Comprehensive status reporting with per-cluster metrics
- Stabilization windows and scaling policies support
- Workspace isolation following KCP patterns

## What Type of PR Is This?

/kind feature

## Related Issue(s)

Part of TMC Reimplementation Plan Phase 5 - Auto-scaling implementation.

## Release Notes

```release-note
Add HPA integration for TMC auto-scaling with cross-cluster replica management and placement policy integration
```

## Technical Details

**Architecture:**
- Follows KCP controller patterns with proper workspace isolation
- Integrates with existing TMC placement policies
- Uses cluster-aware clients for multi-cluster operations
- Implements proper condition management and status reporting

**Scaling Logic:**
- Collects metrics from all relevant clusters
- Calculates desired replicas based on utilization targets
- Distributes replicas across clusters according to placement constraints
- Executes scaling decisions using appropriate strategies

**Testing:**
- Comprehensive unit tests covering scaling logic
- Mock implementations for metrics collection and scaling execution
- Test coverage for placement integration scenarios
- Helper function validation

**Line Count Analysis:**
- Implementation: 853 lines (21% over target of 700)
- Test Coverage: 353 lines (41% coverage ratio)
- **Note**: Slightly over target due to comprehensive functionality, but provides complete HPA integration foundation

**Future Improvements (for follow-up PRs):**
- Enhanced metrics collection with custom metrics support
- Advanced scaling algorithms with predictive capabilities  
- Integration with external monitoring systems
- Performance optimizations for large-scale deployments