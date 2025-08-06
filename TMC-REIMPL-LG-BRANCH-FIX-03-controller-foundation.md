# TMC Branch Split Plan: 03-controller-foundation

## Overview
- **Original Branch**: `feature/tmc2-impl2/03-controller-foundation`
- **Original Size**: 871 lines
- **Target Size**: 700 lines per sub-branch
- **Split Strategy**: 2 sub-branches focusing on controller foundation aspects

## Split Plan

### Sub-branch 1: 03a-controller-base (Pending)
- **Branch**: `feature/tmc2-impl2/03a-controller-base`
- **Estimated Size**: ~435 lines
- **Content**: TMC controller foundation
  - TMC controller main entry point and configuration
  - Controller manager setup and coordination
  - Basic controller lifecycle and error handling
  - Controller metrics and observability foundation

### Sub-branch 2: 03b-cluster-registration (Pending)
- **Branch**: `feature/tmc2-impl2/03b-cluster-registration`
- **Estimated Size**: ~436 lines
- **Content**: Cluster registration controller
  - ClusterRegistration controller implementation
  - Cluster health checking and status management
  - Cluster discovery and registration workflows
  - Registration validation and security checks

## Implementation Order
1. ⏳ 03a-controller-base (Controller foundation)
2. ⏳ 03b-cluster-registration (Cluster registration)

## Dependencies
- 03b depends on 03a for basic controller infrastructure
- Both sub-branches share controller configuration and lifecycle types
- Integration with APIExport system from 02-series branches

## Notes
- Split maintains full TMC controller functionality
- Base provides controller infrastructure and management
- Registration handles cluster onboarding and lifecycle
- Proper separation between controller framework and business logic
- KCP integration patterns followed throughout