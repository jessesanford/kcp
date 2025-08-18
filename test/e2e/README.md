# E2E Test Framework

## Overview
This framework provides comprehensive E2E testing infrastructure for TMC functionality within KCP. It offers isolated test environments, workspace management, and resource creation utilities.

## Framework Components

### TestClient (`framework/client.go`)
- Unified client interface for KCP, Kubernetes, and Dynamic clients
- Logical cluster scoping support
- Context management

### TestEnvironment (`framework/environment.go`)
- Isolated test environments with automatic cleanup
- Integration with KCP testing framework
- Resource cleanup management

### Test Helpers (`framework/helpers.go`)
- Resource assertion utilities (`AssertResourceExists`, `AssertResourceDeleted`)
- Timeout-based condition waiting with `WaitForCondition`
- Generic resource management patterns

## Usage

### Creating a Test
```go
func TestMyScenario(t *testing.T) {
    env := framework.NewTestEnvironment(t)
    // Cleanup is automatic via testing.T integration
    
    // Use env.Clients() to access KCP, Kubernetes, and Dynamic clients
}
```

### Available Helpers
- `CreateTestWorkspace`: Create isolated workspaces
- `CreateTestNamespace`: Create test namespaces
- `WaitForCondition`: Generic condition polling
- `AssertResourceExists`: Verify resource presence
- `AssertResourceDeleted`: Verify resource deletion

### Test Patterns
1. Always use TestEnvironment for isolation
2. Leverage automatic cleanup via testing.T integration
3. Use provided wait utilities for robust testing
4. Follow AAA pattern (Arrange, Act, Assert)

## Example
See `scenarios/cluster_lifecycle_test.go` for a complete example demonstrating workspace lifecycle and resource management.