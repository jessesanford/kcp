<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR completes the 3-way split of session affinity functionality by adding advanced constraint management capabilities to the TMC API. It implements a sophisticated `ConstraintEvaluationEngine` API that provides enterprise-grade policy enforcement, exemption management, and observability for session binding constraints.

This is the final branch (01g2c) in the session affinity split, building upon the foundation established in 01g2b-sticky-binding. The advanced constraint engine provides rule-based evaluation, sophisticated exemption workflows, violation tracking, and comprehensive metrics collection.

**Key Features Implemented:**
- **Advanced Rule-Based Constraint Engine**: Supports threshold, capacity, utilization, network, and custom rule types with CEL expression evaluation
- **Sophisticated Exemption Management**: Time-based exemptions, approval workflows with escalation, and audit logging
- **Violation Tracking & Remediation**: Comprehensive violation detection, automatic remediation strategies, and custom webhook integration
- **Performance Optimization**: Caching, batch processing, concurrent evaluation, and configurable optimization levels
- **Advanced Observability**: Detailed metrics collection, alerting, reporting, and trend analysis
- **Integration Hooks**: Pre/post placement hooks for advanced placement scenarios with filtering and authentication

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

This completes the session affinity constraint management functionality that was split from the original 01g2 branch for better reviewability.

## Key Implementation Details

### ConstraintEvaluationEngine API
- Implements multiple engine types: RuleBasedEngine, PolicyEngine, MLEngine, HybridEngine
- Supports advanced rule conditions with CEL expressions and multiple trigger types
- Provides configurable performance optimization with caching and batch processing
- Includes comprehensive status tracking with detailed metrics and statistics

### Advanced Exemption Management
- Time-based exemption windows with timezone support and recurring patterns
- Approval workflows with multi-level escalation and timeout handling
- Audit logging with configurable retention and destination routing
- Emergency, maintenance, upgrade, and load-based exemption conditions

### Violation Handling & Remediation
- Automated violation tracking with configurable history limits
- Multi-channel alerting (Email, Slack, Teams, Webhook, SMS) with severity thresholds
- Automated reporting with multiple formats (JSON, YAML, PDF, CSV, HTML)
- Custom remediation hooks with authentication and timeout controls

### Integration & Performance Features
- Pre/post placement integration hooks with advanced filtering
- Configurable evaluation caching with multiple eviction policies (LRU, LFU, FIFO)
- Concurrent evaluation with configurable thread pools and batch processing
- Comprehensive performance metrics and throughput monitoring

## Testing Coverage

The implementation includes extensive test coverage (751 lines of tests):
- API validation and condition management tests
- Advanced exemption policy validation with approval workflows
- Violation handling and alerting configuration tests
- Performance optimization and caching behavior tests
- Integration hook configuration and filtering tests
- Engine metrics and status reporting validation
- Rule evaluation types and action type coverage
- Remediation strategy testing

## Generated Code

- Updated register.go to include new ConstraintEvaluationEngine types
- Generated comprehensive deepcopy code for all new types
- Created CRD definition (tmc.kcp.io_constraintevaluationengines.yaml)
- Updated APIExport configuration to include the new resource

## Branch Dependencies

This branch builds on `feature/tmc2-impl2/01g2b-sticky-binding` and completes the 3-way split:
1. **01g2a-core-affinity**: Basic session affinity policies and placement logic
2. **01g2b-sticky-binding**: StickyBinding and SessionBindingConstraint foundation APIs  
3. **01g2c-constraints-advanced**: Advanced constraint evaluation engine (this PR)

## Release Notes

```
Add advanced constraint evaluation engine for TMC session affinity management.

This introduces a comprehensive ConstraintEvaluationEngine API that provides:
- Rule-based constraint evaluation with multiple engine types
- Advanced exemption management with approval workflows and time windows
- Violation tracking, remediation, and comprehensive reporting
- Performance optimization with caching and batch processing
- Integration hooks for advanced placement scenarios
- Detailed metrics collection and observability features

The constraint engine enables enterprise-grade policy enforcement for
session binding constraints with sophisticated exemption handling and
automated remediation capabilities.
```

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>