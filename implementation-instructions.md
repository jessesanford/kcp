# Implementation Instructions: PR6 - Integration Tests

## PR Overview

**Purpose**: Comprehensive integration and e2e tests for the complete TMC system
**Target Line Count**: 400 lines (excluding generated code)
**Dependencies**: PR4 and PR5 (after merge) - needs full implementation
**Feature Flag**: Tests should verify feature flag behavior

## Files to Create

### 1. test/e2e/tmc/suite_test.go (80 lines)
```
Test suite setup and configuration
Expected content:
- Test suite struct (20 lines)
- BeforeEach/AfterEach setup (30 lines)
- Helper functions for test environment (30 lines)
```

### 2. test/e2e/tmc/cluster_test.go (100 lines)
```
Cluster registration e2e tests
Expected tests:
- Cluster registration lifecycle (30 lines)
- Cluster health monitoring (25 lines)
- Multiple cluster registration (25 lines)
- Cluster deletion and cleanup (20 lines)
```

### 3. test/e2e/tmc/placement_test.go (120 lines)
```
Workload placement e2e tests
Expected tests:
- Basic placement scenarios (30 lines)
- Placement strategy tests (30 lines)
- Placement updates and changes (30 lines)
- Edge cases and error scenarios (30 lines)
```

### 4. test/e2e/tmc/integration_test.go (100 lines)
```
Full workflow integration tests
Expected tests:
- End-to-end workflow test (40 lines)
- Controller interaction tests (30 lines)
- Performance and scale tests (30 lines)
```

## Extraction Instructions

### From Legacy PRs

Since integration tests are mostly new, we'll reference patterns from existing KCP e2e tests:

1. **Reference existing e2e test patterns**:
```bash
# Look at existing KCP e2e test structure
ls -la /workspaces/kcp-worktrees/phase3/test/e2e/

# Reference patterns from:
# - test/e2e/framework/ for test utilities
# - test/e2e/apibinding/ for resource testing patterns
# - test/e2e/workspace/ for workspace-aware testing
```

2. **Extract any existing TMC test patterns**:
```bash
# Check if any TMC tests exist in legacy
find /workspaces/kcp-worktrees/legacy/phase3-original -name "*test*.go" -path "*/tmc/*" -type f

# If found, extract useful patterns but create new comprehensive tests
```

### What to Create New

- ✅ **Comprehensive test scenarios** covering all TMC features
- ✅ **Integration between components** (API + Controllers)
- ✅ **Performance benchmarks** for placement algorithms
- ✅ **Chaos testing** for failure scenarios
- ✅ **Feature flag testing** (enabled/disabled scenarios)

## Implementation Details

### Test Suite Setup

```go
package tmc_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/onsi/ginkgo/v2"
    "github.com/onsi/gomega"
    "github.com/stretchr/testify/require"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/wait"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    "github.com/kcp-dev/kcp/test/e2e/framework"
)

func TestTMC(t *testing.T) {
    gomega.RegisterFailHandler(ginkgo.Fail)
    ginkgo.RunSpecs(t, "TMC Suite")
}

var _ = ginkgo.Describe("TMC E2E Tests", func() {
    var (
        ctx    context.Context
        server framework.RunningServer
        client client.Client
    )
    
    ginkgo.BeforeEach(func() {
        ctx = context.Background()
        
        // Start test server with TMC enabled
        server = framework.SharedKcpServer(t)
        require.Eventually(t, func() bool {
            return server.Ready()
        }, 30*time.Second, time.Second)
        
        // Create client
        config := server.BaseConfig(t)
        client, err := client.New(config, client.Options{})
        require.NoError(t, err)
    })
    
    ginkgo.AfterEach(func() {
        // Cleanup resources
        cleanupTMCResources(ctx, client)
    })
    
    // Test implementations follow...
})
```

### Cluster Registration Tests

```go
var _ = ginkgo.Describe("Cluster Registration", func() {
    ginkgo.It("should register and monitor cluster health", func() {
        // Create a ClusterRegistration
        cluster := &tmcv1alpha1.ClusterRegistration{
            ObjectMeta: metav1.ObjectMeta{
                Name: "test-cluster-1",
            },
            Spec: tmcv1alpha1.ClusterRegistrationSpec{
                Location:     "us-west-2",
                Capabilities: []string{"gpu", "high-memory"},
                Endpoint:     "https://test-cluster.example.com",
            },
        }
        
        // Create the cluster
        gomega.Expect(client.Create(ctx, cluster)).To(gomega.Succeed())
        
        // Wait for cluster to be ready
        gomega.Eventually(func() bool {
            updated := &tmcv1alpha1.ClusterRegistration{}
            if err := client.Get(ctx, client.ObjectKeyFromObject(cluster), updated); err != nil {
                return false
            }
            return updated.IsReady()
        }, 30*time.Second, time.Second).Should(gomega.BeTrue())
        
        // Verify status conditions
        updated := &tmcv1alpha1.ClusterRegistration{}
        gomega.Expect(client.Get(ctx, client.ObjectKeyFromObject(cluster), updated)).To(gomega.Succeed())
        gomega.Expect(updated.Status.Conditions).To(gomega.HaveLen(2))
        
        readyCondition := meta.FindStatusCondition(updated.Status.Conditions, "Ready")
        gomega.Expect(readyCondition).ToNot(gomega.BeNil())
        gomega.Expect(readyCondition.Status).To(gomega.Equal(metav1.ConditionTrue))
    })
    
    ginkgo.It("should handle multiple clusters", func() {
        // Create multiple clusters
        for i := 0; i < 5; i++ {
            cluster := &tmcv1alpha1.ClusterRegistration{
                ObjectMeta: metav1.ObjectMeta{
                    Name: fmt.Sprintf("cluster-%d", i),
                },
                Spec: tmcv1alpha1.ClusterRegistrationSpec{
                    Location: fmt.Sprintf("region-%d", i%3),
                },
            }
            gomega.Expect(client.Create(ctx, cluster)).To(gomega.Succeed())
        }
        
        // Verify all clusters are registered
        clusterList := &tmcv1alpha1.ClusterRegistrationList{}
        gomega.Expect(client.List(ctx, clusterList)).To(gomega.Succeed())
        gomega.Expect(clusterList.Items).To(gomega.HaveLen(5))
    })
})
```

### Placement Tests

```go
var _ = ginkgo.Describe("Workload Placement", func() {
    ginkgo.BeforeEach(func() {
        // Setup test clusters
        setupTestClusters(ctx, client, 3)
    })
    
    ginkgo.It("should place workload using round-robin strategy", func() {
        placement := &tmcv1alpha1.WorkloadPlacement{
            ObjectMeta: metav1.ObjectMeta{
                Name: "test-placement",
            },
            Spec: tmcv1alpha1.WorkloadPlacementSpec{
                Strategy: "RoundRobin",
                Replicas: 3,
                Selector: metav1.LabelSelector{
                    MatchLabels: map[string]string{
                        "tier": "production",
                    },
                },
            },
        }
        
        gomega.Expect(client.Create(ctx, placement)).To(gomega.Succeed())
        
        // Wait for placement to be scheduled
        gomega.Eventually(func() bool {
            updated := &tmcv1alpha1.WorkloadPlacement{}
            if err := client.Get(ctx, client.ObjectKeyFromObject(placement), updated); err != nil {
                return false
            }
            return updated.IsPlaced()
        }, 30*time.Second, time.Second).Should(gomega.BeTrue())
        
        // Verify placement results
        updated := &tmcv1alpha1.WorkloadPlacement{}
        gomega.Expect(client.Get(ctx, client.ObjectKeyFromObject(placement), updated)).To(gomega.Succeed())
        gomega.Expect(updated.Status.PlacedClusters).To(gomega.HaveLen(3))
    })
    
    ginkgo.It("should update placement when clusters change", func() {
        // Create initial placement
        placement := createTestPlacement(ctx, client, "dynamic-placement")
        
        // Add a new cluster
        newCluster := &tmcv1alpha1.ClusterRegistration{
            ObjectMeta: metav1.ObjectMeta{
                Name: "new-cluster",
            },
            Spec: tmcv1alpha1.ClusterRegistrationSpec{
                Location:     "eu-west-1",
                Capabilities: []string{"high-performance"},
            },
        }
        gomega.Expect(client.Create(ctx, newCluster)).To(gomega.Succeed())
        
        // Verify placement is updated
        gomega.Eventually(func() int {
            updated := &tmcv1alpha1.WorkloadPlacement{}
            if err := client.Get(ctx, client.ObjectKeyFromObject(placement), updated); err != nil {
                return 0
            }
            return len(updated.Status.PlacedClusters)
        }, 30*time.Second, time.Second).Should(gomega.Equal(4))
    })
})
```

### Integration Tests

```go
var _ = ginkgo.Describe("TMC Integration", func() {
    ginkgo.It("should handle complete workflow", func() {
        // Step 1: Register clusters
        clusters := registerTestClusters(ctx, client, []string{
            "production-1", "production-2", "staging-1",
        })
        
        // Step 2: Create placement policy
        placement := &tmcv1alpha1.WorkloadPlacement{
            ObjectMeta: metav1.ObjectMeta{
                Name: "production-workload",
            },
            Spec: tmcv1alpha1.WorkloadPlacementSpec{
                Strategy: "LeastLoaded",
                Selector: metav1.LabelSelector{
                    MatchLabels: map[string]string{
                        "env": "production",
                    },
                },
            },
        }
        gomega.Expect(client.Create(ctx, placement)).To(gomega.Succeed())
        
        // Step 3: Verify placement decisions
        gomega.Eventually(func() bool {
            updated := &tmcv1alpha1.WorkloadPlacement{}
            client.Get(ctx, client.ObjectKeyFromObject(placement), updated)
            return len(updated.Status.PlacedClusters) == 2
        }, 30*time.Second).Should(gomega.BeTrue())
        
        // Step 4: Simulate cluster failure
        failedCluster := clusters[0]
        failedCluster.Status.Conditions = []metav1.Condition{
            {
                Type:   "Ready",
                Status: metav1.ConditionFalse,
                Reason: "Unreachable",
            },
        }
        gomega.Expect(client.Status().Update(ctx, failedCluster)).To(gomega.Succeed())
        
        // Step 5: Verify placement is updated
        gomega.Eventually(func() bool {
            updated := &tmcv1alpha1.WorkloadPlacement{}
            client.Get(ctx, client.ObjectKeyFromObject(placement), updated)
            // Should not include failed cluster
            for _, cluster := range updated.Status.PlacedClusters {
                if cluster == failedCluster.Name {
                    return false
                }
            }
            return true
        }, 30*time.Second).Should(gomega.BeTrue())
    })
    
    ginkgo.It("should test feature flag behavior", func() {
        // Test with feature flag disabled
        // This would require restarting the server with different flags
        // or using a mock feature gate
        
        ginkgo.Skip("Feature flag testing requires server restart capability")
    })
})
```

### Performance Tests

```go
var _ = ginkgo.Describe("TMC Performance", func() {
    ginkgo.It("should handle large number of clusters", func() {
        start := time.Now()
        
        // Create 100 clusters
        for i := 0; i < 100; i++ {
            cluster := &tmcv1alpha1.ClusterRegistration{
                ObjectMeta: metav1.ObjectMeta{
                    Name: fmt.Sprintf("perf-cluster-%d", i),
                },
                Spec: tmcv1alpha1.ClusterRegistrationSpec{
                    Location: fmt.Sprintf("region-%d", i%10),
                },
            }
            gomega.Expect(client.Create(ctx, cluster)).To(gomega.Succeed())
        }
        
        // Create placement for all clusters
        placement := &tmcv1alpha1.WorkloadPlacement{
            ObjectMeta: metav1.ObjectMeta{
                Name: "perf-placement",
            },
            Spec: tmcv1alpha1.WorkloadPlacementSpec{
                Strategy: "RoundRobin",
                Replicas: 50,
            },
        }
        gomega.Expect(client.Create(ctx, placement)).To(gomega.Succeed())
        
        // Measure time to complete placement
        gomega.Eventually(func() bool {
            updated := &tmcv1alpha1.WorkloadPlacement{}
            client.Get(ctx, client.ObjectKeyFromObject(placement), updated)
            return updated.IsPlaced()
        }, 60*time.Second).Should(gomega.BeTrue())
        
        duration := time.Since(start)
        ginkgo.By(fmt.Sprintf("Placement completed in %v", duration))
        
        // Assert reasonable performance
        gomega.Expect(duration).To(gomega.BeNumerically("<", 30*time.Second))
    })
})
```

## Testing Requirements

### Test Coverage Areas

1. **API Validation**:
   - Valid and invalid resource creation
   - Update scenarios
   - Deletion and cleanup

2. **Controller Behavior**:
   - Reconciliation correctness
   - Error handling
   - Retry logic

3. **Integration Scenarios**:
   - Multi-cluster management
   - Placement decision making
   - Status updates and events

4. **Edge Cases**:
   - Cluster failures
   - Network partitions
   - Resource conflicts

5. **Performance**:
   - Scale testing
   - Latency measurements
   - Resource consumption

## Verification Checklist

- [ ] Test suite properly configured
- [ ] All TMC features have test coverage
- [ ] Integration between components tested
- [ ] Performance benchmarks included
- [ ] Edge cases and error scenarios covered
- [ ] Feature flag behavior tested
- [ ] Tests are idempotent and repeatable
- [ ] Cleanup properly handled
- [ ] Line count under 400:
  ```bash
  /workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c feature/tmc2-impl2/pr6-integration-tests
  ```
- [ ] Tests pass:
  ```bash
  go test ./test/e2e/tmc/... -v
  ```

## Test Execution

### Running Tests Locally

```bash
# Run all TMC e2e tests
go test ./test/e2e/tmc/... -v

# Run specific test
go test ./test/e2e/tmc/... -v -run "TestClusterRegistration"

# Run with coverage
go test ./test/e2e/tmc/... -v -cover

# Run with race detection
go test ./test/e2e/tmc/... -v -race
```

### CI Integration

```yaml
# Example CI configuration
- name: TMC E2E Tests
  run: |
    # Start KCP server with TMC enabled
    ./bin/kcp start --feature-gates=TMCAlpha=true &
    
    # Wait for server
    sleep 10
    
    # Run tests
    go test ./test/e2e/tmc/... -v -timeout 10m
```

## Notes

- These tests verify the complete TMC system integration
- Should catch issues that unit tests might miss
- Performance tests ensure scalability
- Feature flag tests verify proper gating
- Tests should be maintainable and clear in intent