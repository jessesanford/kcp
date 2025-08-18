# Performance Benchmarks Split Tracking

## Original Branch Status: DO NOT MERGE - BEING SPLIT

This branch (`feature/phase10-integration-hardening/p10w2-performance-bench`) exceeded the 800-line limit with 1053 lines and has been split into two compliant branches.

### Split Details

**Original Branch:** `feature/phase10-integration-hardening/p10w2-performance-bench` (1053 lines - OVERSIZED)

**Split Branches:**

1. **`feature/phase10-integration-hardening/p10w2a-perf-framework`** (~695 lines)
   - `test/e2e/performance/framework.go` (262 lines)
   - `test/e2e/performance/metrics.go` (433 lines)
   - Performance framework and metrics collection infrastructure

2. **`feature/phase10-integration-hardening/p10w2b-perf-benchmarks`** (~355 lines)
   - `test/e2e/performance/profiling.go` (355 lines)
   - All benchmark test files (test files don't count toward limit)
   - Performance profiling and benchmark execution

### Merge Order

1. Merge `p10w2a-perf-framework` first (performance framework foundation)
2. Merge `p10w2b-perf-benchmarks` second (benchmarks that use the framework)

### Original Branch Disposition

❌ **DO NOT MERGE** - This original branch should be archived/closed once both split branches are merged.

---

*Split completed on: $(date)*
*Split by: kcp-go-lang-sr-sw-eng agent*
*Line counter used: /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh*