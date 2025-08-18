# Chaos Testing Framework for KCP

This directory contains the chaos testing framework for KCP, designed to validate system resilience and recovery capabilities under various failure conditions.

## Overview

The chaos testing framework provides comprehensive failure injection and recovery validation for the KCP system. It simulates real-world failure scenarios to ensure the system maintains availability, consistency, and performance under adverse conditions.

## Framework Architecture

### Core Components

1. **ChaosTestSuite** (`framework.go`) - Base framework providing:
   - Test environment setup and cleanup
   - Failure tracking and metrics collection
   - System health validation
   - Recovery time measurement

2. **FailureTracker** - Monitors and records:
   - Failure injection events
   - Recovery time objectives (RTO)
   - Success/failure rates
   - Detailed failure analytics

### Failure Scenarios

#### Network Partition Tests (`network_partition_test.go`)
- Simulates network connectivity issues
- Tests workspace isolation during partitions
- Validates client retry behavior
- Measures partition detection and recovery

#### Cluster Failure Tests (`cluster_failure_test.go`)
- Node failure simulation
- Failover behavior validation
- Data consistency during failures
- Graceful degradation testing

#### Controller Crash Tests (`controller_crash_test.go`)
- Controller pod crash simulation
- Leader election recovery
- Workload management continuity
- Automatic restart validation

#### API Server Unavailability (`apiserver_failure_test.go`)
- API server outage simulation
- Client retry mechanism testing
- Partial API availability scenarios
- Workspace isolation maintenance

#### Resource Exhaustion (`resource_exhaustion_test.go`)
- CPU and memory stress testing
- Resource throttling behavior
- System recovery after exhaustion
- Performance degradation analysis

#### Resilience Validation (`resilience_validation_test.go`)
- Comprehensive recovery validation
- RTO compliance testing
- Data consistency verification
- Cascading failure scenarios

## Usage

### Running Individual Test Suites

```bash
# Run network partition tests
go test -v ./test/e2e/chaos -run TestNetworkPartitionRecovery

# Run controller crash tests
go test -v ./test/e2e/chaos -run TestControllerCrashRecovery

# Run API server failure tests
go test -v ./test/e2e/chaos -run TestAPIServerUnavailability

# Run resource exhaustion tests
go test -v ./test/e2e/chaos -run TestResourceExhaustion

# Run cluster failure tests
go test -v ./test/e2e/chaos -run TestClusterFailureScenarios

# Run comprehensive resilience validation
go test -v ./test/e2e/chaos -run TestResilienceValidation
```

### Running All Chaos Tests

```bash
# Run all chaos tests
go test -v ./test/e2e/chaos

# Run with extended timeout for comprehensive testing
go test -v -timeout 30m ./test/e2e/chaos
```

### Test Configuration

The framework uses the following configuration:

- **Namespace**: `chaos-tests` - Isolated namespace for chaos testing
- **Test Data Prefix**: `ct-` - Prefix for all test resources
- **Default Recovery Timeout**: 5 minutes
- **Polling Interval**: 1 second

## Failure Types

The framework defines the following failure types:

| Failure Type | Description | Typical RTO Target |
|-------------|-------------|-------------------|
| NetworkPartitionFailure | Network connectivity issues | 1 minute |
| ClusterFailure | Node or cluster-level failures | 2 minutes |
| ControllerCrash | Controller pod crashes | 30 seconds |
| APIServerFailure | API server unavailability | 1 minute |
| ResourceExhaustion | CPU/Memory exhaustion | 90 seconds |

## Metrics and Reporting

### Failure Tracking

Each failure injection is tracked with:
- Unique failure ID
- Failure type and target
- Start and end timestamps
- Recovery time (RTO)
- Success/failure status
- Error details

### Recovery Metrics

The framework measures:
- **Recovery Rate**: Percentage of successful recoveries
- **Average Recovery Time**: Mean time to recovery
- **RTO Compliance**: Adherence to recovery time objectives
- **Data Consistency**: Data integrity during failures

### Sample Output

```
=== RUN   TestResilienceValidation/ComprehensiveRecoveryValidation
Testing recovery from NetworkPartitionFailure
✓ Recovery from NetworkPartitionFailure: 45.2s
Testing recovery from ControllerCrash
✓ Recovery from ControllerCrash: 12.8s
Overall resilience metrics:
- Recovery rate: 100.0% (5/5)
- Average recovery time: 1m23s
```

## Test Environment Requirements

### Resource Requirements
- Minimum 2 CPU cores
- 4GB RAM
- Kubernetes cluster with KCP
- Network access for external dependencies

### Permissions
Tests require cluster-admin permissions for:
- Namespace creation and deletion
- Pod creation and termination
- Resource quota management
- System resource monitoring

## Integration with CI/CD

### Test Categories

1. **Smoke Tests** - Basic failure/recovery scenarios (5-10 minutes)
2. **Integration Tests** - Comprehensive chaos scenarios (15-30 minutes)
3. **Stress Tests** - Extended resilience validation (30+ minutes)

### Parallel Execution

Tests are designed for parallel execution with proper resource isolation:
- Each test uses unique namespaces
- Resource naming prevents conflicts
- Cleanup ensures no test interference

## Extending the Framework

### Adding New Failure Scenarios

1. Define new failure type in `framework.go`:
```go
const (
    NewFailureType FailureType = "new-failure"
)
```

2. Create test file with scenario implementation:
```go
func TestNewFailureScenario(t *testing.T) {
    // Test implementation
}
```

3. Add failure injector:
```go
type NewFailureInjector struct {
    Suite *ChaosTestSuite
}

func (nfi *NewFailureInjector) SimulateFailure(ctx context.Context) error {
    // Failure simulation logic
}
```

### Custom Metrics

Add custom metrics by extending the FailureRecord struct:

```go
type FailureRecord struct {
    // Existing fields...
    CustomMetric string
    AdditionalData map[string]interface{}
}
```

## Troubleshooting

### Common Issues

1. **Test Timeouts**
   - Increase context timeout for long-running tests
   - Check system resource availability
   - Verify network connectivity

2. **Resource Cleanup Failures**
   - Manually delete test namespaces if cleanup fails
   - Check for resource finalizers blocking deletion
   - Verify RBAC permissions

3. **Inconsistent Results**
   - Run tests in isolation
   - Check system load during testing
   - Verify test resource conflicts

### Debug Logging

Enable debug logging:
```bash
go test -v ./test/e2e/chaos -args -alsologtostderr -v=4
```

## Best Practices

### Test Design
- Keep tests atomic and isolated
- Use realistic failure scenarios
- Validate both failure detection and recovery
- Include negative test cases

### Resource Management
- Always clean up test resources
- Use resource quotas to prevent system impact
- Monitor system resources during testing
- Implement proper timeout handling

### Metrics Collection
- Track all failure events
- Measure recovery times accurately
- Validate data consistency
- Document performance baselines

## Contributing

When contributing new chaos tests:

1. Follow existing test patterns
2. Include comprehensive documentation
3. Add appropriate metrics collection
4. Ensure proper cleanup
5. Validate against multiple failure conditions

## Related Documentation

- [KCP Testing Guide](../../../docs/testing.md)
- [E2E Testing Framework](../framework/README.md)
- [System Architecture](../../../docs/architecture.md)
- [Monitoring and Observability](../../../docs/monitoring.md)