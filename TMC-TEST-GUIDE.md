# TMC Test Harness Guide

## Overview
This test harness allows you to create TMC (Transparent Multi-Cluster) objects and observe the controllers performing TMC functionality in real-time.

## Components

### 1. Main Test Harness (`tmc-test-harness.sh`)
The primary script that:
- Starts KCP with TMC features enabled
- Creates TMC CRDs (ClusterRegistration, WorkloadPlacement)
- Starts the TMC controller
- Creates sample TMC resources
- Monitors controller activity

### 2. Demo Resources (`tmc-demo-resources.yaml`)
Additional test resources including:
- Multi-region cluster configurations
- Complex placement strategies
- Edge computing scenarios
- GPU workload placements
- High-availability configurations

### 3. Controller Watcher (`tmc-watch-controllers.sh`)
Interactive monitoring tool that provides:
- Real-time controller log streaming
- Resource status monitoring
- Reconciliation triggering
- Event simulation
- Metrics viewing

## Quick Start

### Step 1: Run the Test Harness
```bash
./tmc-test-harness.sh
```

This will:
1. Start KCP with TMC features enabled
2. Create a test workspace
3. Install TMC CRDs
4. Start the TMC controller
5. Create sample clusters and placements
6. Begin monitoring

### Step 2: In Another Terminal - Watch Controllers
```bash
./tmc-watch-controllers.sh
```

Choose from the menu:
- Option 1: Watch live controller logs
- Option 2: Monitor resource status
- Option 3: Trigger reconciliation
- Option 4: Simulate cluster events

### Step 3: Create Additional Resources
```bash
# Apply the demo resources
kubectl apply -f tmc-demo-resources.yaml

# Or create custom resources
kubectl apply -f - <<EOF
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: my-cluster
  namespace: tmc-demo
spec:
  clusterID: "cluster-999"
  region: "us-central-1"
  provider: "GCP"
  capacity:
    cpu: "500"
    memory: "4000Gi"
EOF
```

## What to Observe

### Controller Reconciliation
Look for log entries showing:
- `Reconciling ClusterRegistration`
- `Reconciling WorkloadPlacement`
- `Placement decision made`
- `Cluster selected for workload`

### Resource Status Changes
Watch for:
- ClusterRegistration phase transitions
- WorkloadPlacement cluster selections
- Condition updates
- Event generation

### Placement Decisions
The controller should:
1. Read WorkloadPlacement specs
2. Evaluate available clusters
3. Apply placement strategy (Spread, Pack, RoundRobin, Manual)
4. Update status with selected clusters

## Testing Scenarios

### Scenario 1: Basic Placement
```bash
# Create a simple placement
kubectl apply -f - <<EOF
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: simple-placement
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "Deployment"
    name: "my-app"
    namespace: "tmc-demo"
  placement:
    strategy: "RoundRobin"
EOF

# Watch the controller select clusters
kubectl get workloadplacement simple-placement -n tmc-demo -w
```

### Scenario 2: Constraint-Based Placement
```bash
# Create placement with constraints
kubectl apply -f - <<EOF
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: constrained-placement
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "Deployment"
    name: "regional-app"
  placement:
    strategy: "Spread"
    constraints:
    - key: "region"
      operator: "In"
      values: ["us-west-2", "us-east-1"]
    - key: "provider"
      operator: "NotIn"
      values: ["OnPrem"]
EOF
```

### Scenario 3: Cluster Health Changes
```bash
# Mark a cluster as unhealthy
kubectl patch clusterregistration cluster-us-west-1 -n tmc-demo --type merge \
  -p '{"status":{"conditions":[{"type":"Ready","status":"False","reason":"Maintenance"}]}}'

# Observe placement controller reactions
./tmc-watch-controllers.sh
# Choose option 1 to watch logs
```

### Scenario 4: Capacity-Based Placement
```bash
# Update cluster capacity
kubectl patch clusterregistration cluster-us-east-1 -n tmc-demo --type merge \
  -p '{"spec":{"capacity":{"cpu":"10","memory":"100Gi"}}}'

# Create a resource-intensive placement
kubectl apply -f - <<EOF
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: high-resource-placement
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "Deployment"
    name: "resource-intensive-app"
  placement:
    strategy: "Pack"
    resourceRequirements:
      cpu: "500"
      memory: "2000Gi"
EOF
```

## Monitoring Commands

### Check TMC Resources
```bash
# List all TMC resources
kubectl get clusterregistrations,workloadplacements -A

# Describe specific resources
kubectl describe clusterregistration cluster-us-west-1 -n tmc-demo
kubectl describe workloadplacement web-app-placement -n tmc-demo

# Watch resources for changes
kubectl get workloadplacements -n tmc-demo -w
```

### View Controller Logs
```bash
# KCP logs with TMC entries
tail -f /tmp/kcp-tmc-test-*/kcp.log | grep -i tmc

# TMC controller logs
tail -f /tmp/kcp-tmc-test-*/tmc.log

# All placement-related logs
tail -f /tmp/kcp-tmc-test-*/kcp.log | grep -i placement
```

### Check Events
```bash
# Recent events in TMC namespace
kubectl get events -n tmc-demo --sort-by='.lastTimestamp'

# Watch for new events
kubectl get events -n tmc-demo -w
```

## Expected Controller Behaviors

### When Creating ClusterRegistration:
1. Controller detects new ClusterRegistration
2. Validates cluster specification
3. Updates status with initial conditions
4. Adds cluster to available pool for placements

### When Creating WorkloadPlacement:
1. Controller detects new WorkloadPlacement
2. Reads placement strategy and constraints
3. Evaluates available clusters
4. Applies placement algorithm
5. Updates status with selected clusters
6. Creates events for placement decisions

### When Updating Resources:
1. Controller detects change via watch
2. Re-evaluates placement if needed
3. Updates status accordingly
4. Generates events for significant changes

## Troubleshooting

### Controllers Not Processing Resources
1. Check feature gates are enabled:
   ```bash
   ps aux | grep kcp | grep feature-gates
   ```

2. Verify CRDs are installed:
   ```bash
   kubectl get crd | grep tmc
   ```

3. Check controller is running:
   ```bash
   ps aux | grep tmc-controller
   ```

### No Placement Decisions
1. Check WorkloadPlacement has valid spec
2. Verify ClusterRegistrations exist
3. Look for errors in controller logs
4. Check if constraints are too restrictive

### Resources Stuck in Pending
1. Check controller logs for errors
2. Verify RBAC permissions
3. Check for validation webhook issues
4. Look for resource conflicts

## Cleanup

To stop the test harness and clean up:
```bash
# Press Ctrl+C in the terminal running tmc-test-harness.sh
# This will automatically:
# - Stop KCP and TMC controller
# - Clean up temporary directories
# - Remove test resources
```

## Advanced Testing

For more complex testing scenarios, you can:
1. Modify controller verbosity: Add `--v=6` for detailed logs
2. Enable specific features: Use granular feature gates
3. Test failure scenarios: Kill controllers and observe recovery
4. Load test: Create hundreds of resources simultaneously
5. Integration test: Connect actual clusters via syncer

## Next Steps

1. Implement actual controller logic for placement decisions
2. Add webhook validation for resources
3. Implement status aggregation from clusters
4. Add metrics and observability
5. Create integration tests
6. Document API specifications