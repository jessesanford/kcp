# TMC Testing Framework

This directory contains a comprehensive testing framework for TMC (Transit Multi-Cluster) components, designed to work seamlessly with KCP's architecture and ensure proper workspace isolation.

## Overview

The TMC testing framework provides:

- **Unit Testing**: Comprehensive API type validation and component testing
- **Integration Testing**: Controller and cross-component integration testing  
- **End-to-End Testing**: Complete workflow testing with real KCP workspaces
- **Workspace Isolation Testing**: Validation of multi-tenant isolation
- **Performance Testing**: Benchmarking and performance validation
- **Coverage Reporting**: Detailed test coverage analysis

## Directory Structure

```
test/
├── README.md                                   # This file
├── Makefile                                    # Test automation targets
├── integration/                                # Integration tests
│   └── tmc/
│       ├── framework.go                       # Integration testing framework
│       ├── controllers_test.go                # Controller integration tests
│       ├── e2e_test.go                        # End-to-end workflow tests
│       └── workspace_isolation_test.go        # Workspace isolation tests
├── unit/                                      # Unit tests
│   └── tmc/
│       ├── api_testing.go                     # API type testing utilities
│       └── runner_test.go                     # Unit test runner and examples
└── pkg/apis/tmc/testdata/                     # Test data files
    ├── cluster-registration-valid.yaml
    └── workload-placement-valid.yaml
```

## Quick Start

### Running All Tests

```bash
# Run all TMC tests
make test

# Run with coverage reporting
make test-coverage

# Run in CI mode
make test-ci
```

### Running Specific Test Suites

```bash
# Unit tests only
make test-unit

# Integration tests only
make test-integration

# End-to-end tests only
make test-e2e

# Workspace isolation tests
make test-workspace-isolation

# Controller tests
make test-controllers
```

### Development Testing

```bash
# Quick tests for development
make test-dev

# Tests with race detection
make test-race

# Short mode tests
make test-short
```

## Framework Components

### Integration Testing Framework (`integration/tmc/framework.go`)

Provides `TestContext` for KCP-aware testing:

```go
// Create test context with workspace isolation
ctx := NewTestContext(t)
defer ctx.Cleanup()

// Setup isolated workspace
err := ctx.SetupWorkspace("my-test-workspace")
require.NoError(t, err)

// Wait for workspace readiness
err = ctx.WaitForWorkspaceReady()
require.NoError(t, err)

// Validate workspace isolation
err = ctx.ValidateWorkspaceIsolation()
require.NoError(t, err)
```

### Test Suite Pattern

Use the `TMCTestSuite` pattern for organized testing:

```go
suite := TMCTestSuite{
    Name:        "MyTMCFeature",
    Description: "Tests for my TMC feature",
    TestCases: []TMCTestCase{
        {
            Name:        "BasicFunctionality",
            Description: "Test basic functionality",
            TestFunc: func(ctx *TestContext) error {
                // Your test logic here
                return nil
            },
            Timeout: 30 * time.Second,
        },
    },
    SetupFunc:    mySetupFunc,
    TeardownFunc: myTeardownFunc,
}

RunTMCTestSuite(t, suite)
```

### API Testing Utilities (`unit/tmc/api_testing.go`)

Provides utilities for testing TMC API types:

```go
// Test API type validation
suite := &APITypeTestSuite{
    TypeName:     "ClusterRegistration",
    GroupVersion: "tmc.kcp.io/v1alpha1",
    Kind:         "ClusterRegistration",
}

// Validate API type compliance
suite.ValidateAPIType(t, myAPIObject)

// Run API test cases
RunAPITestCases(t, "ClusterRegistration", ClusterRegistrationTestCases())
```

## Test Categories

### Unit Tests

Located in `unit/tmc/`, these tests validate:

- API type structure and validation
- Serialization/deserialization
- Business logic components
- Mock object functionality
- Test data loading

**Example:**

```go
func TestClusterRegistrationValidation(t *testing.T) {
    // Test valid ClusterRegistration
    validCluster := createValidClusterRegistration()
    err := validateClusterRegistration(validCluster)
    require.NoError(t, err)
    
    // Test invalid ClusterRegistration
    invalidCluster := createInvalidClusterRegistration()
    err = validateClusterRegistration(invalidCluster)
    require.Error(t, err)
}
```

### Integration Tests

Located in `integration/tmc/`, these tests validate:

- Controller reconciliation logic
- Cross-component interactions
- KCP integration patterns
- APIBinding functionality
- Status propagation

**Example:**

```go
func TestClusterRegistrationController(t *testing.T) {
    ctx := NewTestContext(t)
    defer ctx.Cleanup()
    
    // Setup test workspace
    require.NoError(t, ctx.SetupWorkspace("controller-test"))
    
    // Create ClusterRegistration
    cluster := createTestClusterRegistration()
    
    // Verify controller processes it
    ctx.Eventually(func() (bool, error) {
        // Check if controller updated status
        return isClusterReady(cluster), nil
    })
}
```

### End-to-End Tests

Located in `integration/tmc/e2e_test.go`, these tests validate:

- Complete workflows from API to execution
- Multi-cluster scenarios
- Complex placement strategies
- Multi-tenant isolation
- Real-world usage patterns

**Example:**

```go
func testRegisterClusterE2E(ctx *TestContext) error {
    // Step 1: Create ClusterRegistration
    cluster := createClusterRegistration("test-cluster", "us-west-2")
    
    // Step 2: Wait for controller processing
    // Step 3: Verify cluster becomes ready
    // Step 4: Verify capabilities are detected
    
    return nil
}
```

### Workspace Isolation Tests

Located in `integration/tmc/workspace_isolation_test.go`, these tests validate:

- Resource isolation between workspaces
- Controller scope isolation
- APIBinding isolation
- Status propagation isolation
- Multi-tenant scenarios

## Test Data

Test data files in `pkg/apis/tmc/testdata/` provide:

- Valid and invalid resource examples
- Edge case scenarios
- Performance test data
- Multi-tenant test scenarios

## Coverage Requirements

The framework enforces comprehensive test coverage:

- **Unit Tests**: 80%+ coverage of API types and business logic
- **Integration Tests**: 70%+ coverage of controller logic
- **End-to-End Tests**: 60%+ coverage of complete workflows
- **Overall**: 60%+ total coverage across all TMC components

## Performance Testing

The framework includes performance testing capabilities:

```bash
# Run benchmarks
make test-benchmark

# Run performance tests
make test-performance
```

Performance tests validate:

- API creation/update latency
- Controller reconciliation speed
- Placement decision performance
- Memory usage patterns
- Concurrent operation handling

## CI/CD Integration

The framework integrates with CI/CD pipelines:

```bash
# CI-suitable test suite
make test-ci
```

CI tests include:

- Linting and formatting checks
- Unit test execution
- Integration test execution  
- Coverage reporting
- Performance regression testing

## Development Workflow

### Adding New Tests

1. **Unit Tests**: Add to `unit/tmc/` with appropriate test cases
2. **Integration Tests**: Add to `integration/tmc/` using `TestContext`
3. **Test Data**: Add YAML files to `pkg/apis/tmc/testdata/`
4. **Documentation**: Update this README with new test patterns

### Test Development Best Practices

1. **Use TestContext**: Always use `TestContext` for integration tests
2. **Workspace Isolation**: Ensure each test uses isolated workspaces
3. **Cleanup**: Always defer cleanup operations
4. **Timeout**: Set appropriate timeouts for async operations
5. **Coverage**: Aim for comprehensive test coverage
6. **Documentation**: Document complex test scenarios

### Debugging Tests

```bash
# Run with verbose output
make test-verbose

# Run specific test pattern
go test -v -run TestSpecificPattern ./integration/tmc/

# Run with race detection
make test-race
```

## Future Enhancements

The testing framework is designed to evolve with TMC development:

1. **API Evolution**: Tests will be updated as TMC APIs evolve
2. **Controller Implementation**: Integration tests will be activated as controllers are implemented
3. **Performance Optimization**: Performance tests will be enhanced based on real usage patterns
4. **Multi-Cluster Testing**: Framework will support testing across multiple physical clusters
5. **Chaos Engineering**: Framework will support fault injection and chaos testing

## Contributing

When adding new TMC components:

1. Add unit tests for new API types
2. Add integration tests for new controllers
3. Add end-to-end tests for new workflows
4. Update test data files as needed
5. Ensure workspace isolation is maintained
6. Update documentation and examples

## Support

For questions about the testing framework:

1. Review existing test patterns in the codebase
2. Check the Makefile for available test targets
3. Review test output for detailed failure information
4. Ensure proper KCP test environment setup

The TMC testing framework provides a solid foundation for ensuring TMC components are robust, well-tested, and maintain proper workspace isolation in the KCP ecosystem.