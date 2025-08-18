## Summary
This PR implements the Decision Maker component for TMC Phase 8 Wave 2, providing intelligent placement decision coordination that combines scheduler results with CEL policy evaluation. The Decision Maker serves as the final arbitrator for placement decisions, incorporating manual overrides, comprehensive validation, and detailed audit trails.

## What Type of PR Is This?
/kind feature

## Related Issue(s)
Part of TMC Phase 8 Wave 2 implementation - Decision Maker for placement coordination.

## Key Features
- **Intelligent Decision Algorithms**: Four distinct algorithms (WeightedScore, CELPrimary, SchedulerPrimary, Consensus) for flexible placement decision strategies
- **CEL Policy Integration**: Seamless integration with CEL evaluator for custom placement policies and business rules
- **Manual Override System**: Comprehensive override framework supporting Force, Exclude, Prefer, and Avoid directives with priority-based resolution
- **Decision Validation**: Multi-layered validation including resource constraints, policy compliance, and conflict detection
- **Audit Trail & Recording**: Complete decision history with event tracking, metrics, and configurable retention policies
- **Conflict Resolution**: Advanced conflict detection for resource overcommitment, affinity violations, and policy conflicts
- **Override Management**: Full lifecycle management for placement overrides with expiration, conflict checking, and priority handling

## Technical Implementation
The Decision Maker is architected as a modular system with clear separation of concerns:

**Core Components:**
- `DecisionMaker`: Main interface coordinating scheduler results with CEL evaluation
- `DecisionValidator`: Multi-layered validation ensuring resource and policy compliance
- `DecisionRecorder`: Comprehensive audit system with in-memory storage and pruning
- `OverrideManager`: Full override lifecycle management with conflict detection

**Decision Flow:**
1. **CEL Evaluation**: Processes candidates through configurable CEL expressions
2. **Algorithm Application**: Applies selected algorithm to combine scheduler and CEL scores
3. **Validation**: Validates decisions against constraints and policies
4. **Override Processing**: Applies manual overrides with priority-based resolution
5. **Recording**: Captures complete decision history for audit and debugging

**Key Design Patterns:**
- Interface-based design enabling pluggable components and testing
- Comprehensive error handling with structured decision status tracking
- Concurrent-safe in-memory storage with proper mutex protection
- Event-driven audit logging with configurable detail levels

## Component Split Plan
Note: This implementation exceeds size limits and will be split into 5 PRs:

1. **PR1: Types & Interfaces (~450 lines)**
   - Core decision types and interfaces
   - CEL expression and evaluation result types
   - Decision status and conflict types
   - Basic decision request/response structures

2. **PR2: Core Decision Logic (~700 lines)**
   - Main DecisionMaker implementation
   - Four decision algorithms (Weighted, CELPrimary, SchedulerPrimary, Consensus)
   - CEL evaluation integration and context creation
   - Decision rationale generation

3. **PR3: Validation (~450 lines)**
   - DecisionValidator implementation
   - Resource constraint validation
   - Policy compliance checking
   - Multi-level conflict detection (resource, affinity, policy)

4. **PR4: Recording & History (~450 lines)**
   - DecisionRecorder implementation
   - In-memory storage with indexing
   - Decision history and event tracking
   - Configurable retention and pruning

5. **PR5: Override System (~650 lines)**
   - OverrideManager implementation
   - Override validation and conflict detection
   - Priority-based override resolution
   - Override lifecycle management

Each PR will be atomic and independently testable while building toward the complete Decision Maker functionality.

## Testing
**Unit Test Coverage:**
- Comprehensive test suite with 919 lines covering core decision logic
- Table-driven tests for all decision algorithms
- Mock-based testing for component integration
- Error condition and edge case validation
- Performance benchmarks for decision algorithms

**Test Categories:**
- Decision algorithm correctness and score calculation
- CEL integration and expression evaluation
- Override application and conflict resolution
- Validation logic and constraint checking
- Recording and audit trail functionality

**Integration Points:**
- CEL evaluator integration through well-defined interfaces
- Scheduler API compatibility with candidate processing
- Resource capacity tracker integration for validation

## Release Notes
```markdown
### TMC Phase 8 Wave 2: Decision Maker

**New Features:**
- Intelligent placement decision maker with multiple algorithms
- CEL policy integration for custom placement rules
- Manual override system with priority-based conflict resolution
- Comprehensive decision validation and audit trails
- Advanced conflict detection for resources and policies

**Technical Details:**
- Four decision algorithms: WeightedScore, CELPrimary, SchedulerPrimary, Consensus
- Complete override lifecycle with Force, Exclude, Prefer, Avoid operations
- Multi-layered validation: resource constraints, policy compliance, conflict detection
- Full audit system with decision history, events, and configurable retention
- Interface-based design enabling extensibility and comprehensive testing

This component enables sophisticated placement decisions by combining automated scheduling with custom policies and manual interventions, providing the foundation for advanced TMC placement strategies.
```