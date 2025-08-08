## Summary

Implement core decision engine for TMC workload placement with advanced constraint evaluation, multi-criteria scoring, and basic selection. This PR provides the foundation for intelligent placement decisions at exactly 700 lines, with advanced selection strategies reserved for a follow-up PR.

## What Type of PR Is This?

/kind feature

## Changes Included

### Core Decision Engine (`decision.go` - 397 lines)
- **DecisionEngine struct**: Main orchestrator for placement decisions
- **Advanced constraint filtering**: Location selector matching with full MatchExpressions support
- **Affinity constraint evaluation**: Required and preferred affinity handling
- **Tolerance evaluation framework**: Placeholder for future taint/toleration support
- **Candidate management**: LocationCandidate with scoring details and constraint validation

### Scoring Infrastructure (`scoring.go` - 227 lines)
- **Multi-criteria scoring**: Affinity, capacity, spread, and latency scoring algorithms
- **Configurable weights**: ScoringWeights structure for customizable placement priorities
- **Individual score calculation**: Modular scoring components for different placement criteria
- **Weighted score combination**: Smart aggregation of multiple scoring factors
- **Human-readable scoring reasons**: Detailed explanations for placement decisions

### Basic Selection Logic (inline in `decision.go`)
- **Score-based selection**: Simple highest-score selection for this PR
- **Cluster count handling**: Respects NumberOfClusters specification
- **Sorted candidate selection**: Efficient O(n log n) selection algorithm
- **Future strategy support**: Architecture ready for advanced strategies in follow-up PR

### Comprehensive Test Coverage (`decision_test.go` - 325 lines)
- **Core functionality testing**: Decision engine with realistic placement scenarios  
- **Constraint validation**: Label selector and affinity constraint testing
- **Edge cases**: Empty location sets and invalid configurations
- **Scoring validation**: Individual scoring component testing with capacity annotations
- **Integration testing**: End-to-end placement decision workflows

## Technical Architecture

### Decision Flow
1. **Constraint Filtering**: Hard constraint evaluation (selectors, affinity)
2. **Candidate Scoring**: Multi-criteria scoring with configurable weights
3. **Score-Based Selection**: Sort by score and select highest-scoring candidates  
4. **Decision Creation**: Convert candidates to placement decisions

### Key Design Decisions
- **Modular scoring**: Separate scoring components for maintainability
- **Simple selection**: Score-based selection in this PR, advanced strategies in follow-up
- **Comprehensive error handling**: Detailed constraint failure reasons
- **KCP integration**: Proper workspace isolation and logical cluster support

## Testing Strategy

- **Unit tests**: All core functions with table-driven tests
- **Core decision testing**: Score-based selection with realistic location sets
- **Constraint validation**: Label selector operators (In, NotIn, Exists, DoesNotExist)
- **Edge case handling**: Empty inputs, invalid configurations, constraint failures

## Performance Considerations

- **Efficient constraint evaluation**: Early filtering to reduce candidate set
- **Lazy scoring**: Only score candidates that pass constraints
- **O(n log n) selection**: Sort-based candidate ranking
- **Memory efficient**: Minimal candidate object allocation

## Integration Points

- **Workload API**: Full integration with `workloadv1alpha1.Placement` specification
- **Location API**: Advanced `workloadv1alpha1.Location` selector support
- **Constraint API**: `workloadv1alpha1.PlacementConstraints` implementation
- **KCP logging**: Structured logging with appropriate verbosity levels

## Future Enhancements (Separate PRs)

- **Advanced selection strategies**: Balanced, packed, and spread selection algorithms
- **Resource-aware scoring**: Actual cluster capacity integration
- **Network topology**: Real latency measurements  
- **Workload spread tracking**: Current placement distribution analysis

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5A2b (Decision Engine Components)

## Release Notes

```
Add core placement decision engine with multi-criteria scoring and score-based selection for TMC workload placement controller. Supports sophisticated constraint evaluation, configurable scoring weights, and foundation for advanced placement strategies.
```

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>