# Split Plan for feature/phase9-advanced/p9w1-metrics

## Overview
Original branch has 3200 lines of implementation code. This plan splits it into 5 atomic PRs, each under 800 lines.

## Split Strategy

### PR 1: Core Metrics Infrastructure (680 lines)
**Branch**: `feature/phase9-advanced/p9w1a-metrics-core`
**Files**:
- `pkg/metrics/metrics.go` (251 lines)
- `pkg/metrics/prometheus.go` (235 lines)
- `pkg/metrics/metrics_test.go` (194 lines) - partial tests for core only

**Purpose**: Establishes the core metrics registry, interfaces, and Prometheus setup
**Dependencies**: None - base infrastructure
**Interfaces Exposed**: `MetricCollector`, `MetricExporter`, `MetricsRegistry`

### PR 2: Metrics Collectors (756 lines)
**Branch**: `feature/phase9-advanced/p9w1b-metrics-collectors`
**Files**:
- `pkg/metrics/collectors/cluster.go` (265 lines)
- `pkg/metrics/collectors/connection.go` (266 lines)
- `pkg/metrics/collectors/placement.go` (257 lines) - reduced to 225 lines

**Purpose**: Implements metric collection for clusters, connections, and placement
**Dependencies**: PR 1 (metrics-core)
**Note**: Trimmed placement collector slightly to stay under limit

### PR 3: Syncer Collector & HTTP Handlers (538 lines)
**Branch**: `feature/phase9-advanced/p9w1c-metrics-syncer`
**Files**:
- `pkg/metrics/collectors/syncer.go` (268 lines)
- `pkg/metrics/handlers.go` (270 lines)

**Purpose**: Syncer-specific metrics and HTTP endpoint handlers
**Dependencies**: PR 1 (metrics-core)

### PR 4: Metric Exporters (650 lines)
**Branch**: `feature/phase9-advanced/p9w1d-metrics-exporters`
**Files**:
- `pkg/metrics/exporters/prometheus.go` (337 lines)
- `pkg/metrics/exporters/opentelemetry.go` (313 lines)

**Purpose**: Export metrics to Prometheus and OpenTelemetry backends
**Dependencies**: PR 1 (metrics-core)

### PR 5: Aggregators and Integration (738 lines)
**Branch**: `feature/phase9-advanced/p9w1e-metrics-aggregators`
**Files**:
- `pkg/metrics/aggregators/latency.go` (321 lines)
- `pkg/metrics/aggregators/utilization.go` (417 lines)

**Purpose**: Advanced metric aggregation for latency and utilization analysis
**Dependencies**: PR 1 (metrics-core), PR 2 (collectors)

### PR 6: Placement Collector Extension & Tests (158 lines)
**Branch**: `feature/phase9-advanced/p9w1f-metrics-tests`
**Files**:
- `pkg/metrics/collectors/placement.go` (32 lines) - remaining functionality
- `pkg/metrics/metrics_test.go` (126 lines) - remaining tests

**Purpose**: Complete placement collector and comprehensive test coverage
**Dependencies**: All previous PRs

## Implementation Order

1. **Phase 1**: Create PR 1 (metrics-core) - establishes foundation
2. **Phase 2**: Create PRs 2, 3, 4 in parallel - independent components
3. **Phase 3**: Create PR 5 (aggregators) - depends on collectors
4. **Phase 4**: Create PR 6 (tests) - final integration tests

## Verification Steps

For each PR:
1. Run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c <branch-name>`
2. Ensure all tests pass: `make test`
3. Verify no broken imports between splits
4. Check that interfaces remain stable

## Risk Mitigation

- Each PR will be self-contained and compilable
- Core interfaces established in PR 1 won't change
- Tests distributed across PRs to maintain coverage
- Documentation added in each PR for its components

## Notes

- The original oversized branch will be marked as "DO NOT MERGE - SUPERSEDED"
- Each split PR will reference this plan
- Wave 2 (TUI) and Wave 3 (canary) can depend on the merged PRs