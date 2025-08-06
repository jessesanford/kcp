# TMC Branch Split Plan: 02-apiexport-integration

## Overview
- **Original Branch**: `feature/tmc2-impl2/02-apiexport-integration`
- **Original Size**: 2,101 lines
- **Target Size**: 700 lines per sub-branch
- **Split Strategy**: 3 sub-branches focusing on different APIExport integration aspects

## Split Plan

### Sub-branch 1: 02a-apiexport-foundation (Pending)
- **Branch**: `feature/tmc2-impl2/02a-apiexport-foundation`
- **Estimated Size**: ~700 lines
- **Content**: APIExport integration foundation
  - TMC APIExport controller and reconciliation logic
  - APIExport creation and lifecycle management
  - Workspace-aware client handling and isolation
  - Basic APIExport validation and error handling

### Sub-branch 2: 02b-apibinding-management (Pending)
- **Branch**: `feature/tmc2-impl2/02b-apibinding-management`
- **Estimated Size**: ~700 lines
- **Content**: APIBinding management and coordination
  - APIBinding controller for TMC API consumption
  - Cross-workspace API binding and permission management
  - APIBinding lifecycle and dependency tracking
  - Binding validation and conflict resolution

### Sub-branch 3: 02c-export-policies (Pending)
- **Branch**: `feature/tmc2-impl2/02c-export-policies`
- **Estimated Size**: ~701 lines
- **Content**: APIExport policies and governance
  - APIExport policy enforcement and validation
  - Export access control and permission policies
  - Cross-workspace export sharing and governance
  - Policy compliance monitoring and reporting

## Implementation Order
1. ⏳ 02a-apiexport-foundation (APIExport foundation)
2. ⏳ 02b-apibinding-management (APIBinding management)
3. ⏳ 02c-export-policies (Export policies)

## Dependencies
- 02b depends on 02a for basic APIExport functionality
- 02c depends on both 02a and 02b for policy enforcement
- All sub-branches require workspace isolation and KCP patterns

## Notes
- Split maintains full KCP APIExport integration functionality
- Foundation provides core export/import mechanisms
- Binding management handles API consumption patterns
- Policies provide governance and access control
- Each sub-branch follows KCP architectural patterns