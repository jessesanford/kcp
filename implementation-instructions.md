# Implementation Instructions: Advanced Virtual Workspace Features

## Overview
- **Branch**: feature/tmc-phase4-vw-12-advanced-features
- **Purpose**: Add rate limiting, circuit breaking, load balancing, and advanced routing features
- **Target Lines**: 500
- **Dependencies**: Branch vw-11 (caching layer)
- **Estimated Time**: 3 days

## Files to Create

### 1. pkg/virtual/features/ratelimit.go (120 lines)
**Purpose**: Implement per-workspace rate limiting

**Key Components**:
- Token bucket algorithm
- Per-user limits
- Per-workspace limits
- Burst handling

### 2. pkg/virtual/features/circuitbreaker.go (100 lines)
**Purpose**: Implement circuit breaker pattern

**Key Components**:
- Circuit states (closed/open/half-open)
- Failure threshold detection
- Recovery mechanisms
- Fallback strategies

### 3. pkg/virtual/features/loadbalancer.go (80 lines)
**Purpose**: Implement load balancing

**Key Components**:
- Round-robin algorithm
- Weighted distribution
- Health-based routing
- Session affinity

### 4. pkg/virtual/features/retry.go (70 lines)
**Purpose**: Implement retry logic

**Key Components**:
- Exponential backoff
- Jitter addition
- Max retry limits
- Retry conditions

### 5. pkg/virtual/features/metrics_collector.go (70 lines)
**Purpose**: Collect Prometheus metrics

**Key Components**:
- Request counters
- Latency histograms
- Error rates
- Resource usage

### 6. pkg/virtual/features/features_test.go (60 lines)
**Purpose**: Test advanced features

## Implementation Steps

1. **Implement rate limiting**:
   - Token bucket per workspace
   - User-specific limits
   - Burst handling
   - Limit configuration

2. **Add circuit breaker**:
   - Monitor failure rates
   - Open circuit on threshold
   - Implement half-open state
   - Add fallback logic

3. **Create load balancer**:
   - Round-robin baseline
   - Add weighted routing
   - Health-aware routing
   - Session stickiness

4. **Add retry logic**:
   - Exponential backoff
   - Add jitter
   - Respect limits
   - Smart retry conditions

5. **Implement metrics**:
   - Prometheus integration
   - Custom metrics
   - Grafana dashboards
   - Alert rules

## Testing Requirements
- Unit test coverage: >80%
- Test scenarios:
  - Rate limit enforcement
  - Circuit breaker states
  - Load distribution
  - Retry behavior
  - Metrics accuracy

## Integration Points
- Uses: Caching layer from branch vw-11
- Provides: Production-ready features for HA and resilience

## Acceptance Criteria
- [ ] Rate limiting working per workspace
- [ ] Circuit breaker protecting backends
- [ ] Load balancing distributing traffic
- [ ] Retry logic with backoff
- [ ] Prometheus metrics exposed
- [ ] Tests pass with coverage
- [ ] No linting errors

## Common Pitfalls
- **Rate limit fairness**: Don't starve users
- **Circuit breaker tuning**: Avoid flapping
- **Load balancer stickiness**: Handle failures
- **Retry storms**: Prevent cascading failures
- **Metric cardinality**: Avoid explosion

## Code Review Focus
- Rate limiting algorithm
- Circuit breaker thresholds
- Load balancing fairness
- Retry strategy safety
- Metrics performance impact