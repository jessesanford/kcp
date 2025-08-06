<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR adds the ConstraintEvaluationEngine API and properly integrates it with KCP's APIExport system for workspace isolation. This provides the foundation for rule-based constraint evaluation in the TMC system.

**Key Features Added:**
- ConstraintEvaluationEngine CRD with comprehensive rule evaluation capabilities
- Proper KCP APIExport integration for multi-tenant workspace isolation  
- Rule-based evaluation with Threshold and Capacity rule types
- Basic violation handling and performance metrics tracking
- Comprehensive test coverage for all API components

**KCP Integration:**
- Added ConstraintEvaluationEngine to tmc.kcp.io APIExport configuration
- Created corresponding APIResourceSchema with proper schema hash (v250806-019d0a59)
- Ensures workspace-aware operation with logical cluster isolation
- Follows established KCP patterns for multi-tenancy

**Core Functionality:**
- Simple rule-based constraint evaluation engine (MVP implementation)
- Support for basic rule types: Threshold and Capacity constraints
- CEL expression support for flexible condition evaluation
- Basic remediation strategies: AutoRemediate and LogOnly
- Performance metrics and violation tracking
- Comprehensive API validation and kubebuilder annotations

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

This implements the core constraint evaluation engine for TMC Reimplementation Plan 2, addressing the need for rule-based constraint evaluation and proper KCP workspace isolation.

## Release Notes

```markdown
Add ConstraintEvaluationEngine API for TMC constraint evaluation with proper KCP workspace isolation support. This provides a foundational rule-based evaluation engine for session binding constraints with basic violation handling and performance tracking.
```