# TMC Manual Verification Tests

## Quick Verification Commands

### 1. Basic Binary Tests
```bash
# Check binaries exist and are executable
ls -lh ./bin/kcp ./bin/tmc-controller

# Check binary sizes (should be >5MB for tmc-controller, >50MB for kcp)
du -h ./bin/kcp ./bin/tmc-controller

# Get version info
./bin/kcp version
./bin/tmc-controller --version
```

### 2. Feature Flag Verification
```bash
# List all TMC feature flags in KCP
./bin/kcp start options 2>&1 | grep -i tmc

# Count TMC feature flags (should be 7)
./bin/kcp start options 2>&1 | grep -c "TMC"

# Test feature flag acceptance
./bin/kcp start --feature-gates=TMCFeature=true --dry-run
```

### 3. TMC Controller Tests
```bash
# Test TMC controller help
./bin/tmc-controller --help | head -20

# Start TMC controller with all features
./bin/tmc-controller \
  --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true \
  --v=2

# Start with specific features
./bin/tmc-controller \
  --feature-gates=TMCPlacement=true \
  --kubeconfig=/path/to/kubeconfig
```

### 4. KCP Server with TMC Tests
```bash
# Start KCP with TMC features enabled
./bin/kcp start \
  --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true \
  --root-directory=/tmp/kcp-tmc-test \
  --v=2

# In another terminal, check APIs
export KUBECONFIG=/tmp/kcp-tmc-test/admin.kubeconfig
kubectl api-resources | grep tmc

# Check for TMC CRDs
kubectl get crd | grep tmc
```

### 5. API Type Verification
```bash
# Check TMC API types are present
find ./pkg/apis/tmc -name "*.go" | wc -l  # Should be > 5

# Check for generated code
ls -la ./pkg/apis/tmc/v1alpha1/zz_generated*

# Verify ClusterRegistration type
grep -l "ClusterRegistration" ./pkg/apis/tmc/v1alpha1/*.go

# Verify WorkloadPlacement type  
grep -l "WorkloadPlacement" ./pkg/apis/tmc/v1alpha1/*.go
```

### 6. Controller Implementation Tests
```bash
# Check controller packages
find ./pkg/tmc -type f -name "*.go" | wc -l  # Should be > 10

# Look for controller implementations
ls -la ./pkg/tmc/*/controller.go

# Check for placement logic
grep -r "PlacementController" ./pkg/tmc/
```

### 7. Build System Tests
```bash
# Test code generation
make codegen

# Test full build
make clean && make build

# Test specific TMC targets (if any)
make help | grep -i tmc
```

### 8. Integration Tests
```bash
# Check TMC imports in main KCP
grep -r "pkg/apis/tmc" ./cmd/

# Check feature gate registration
grep -r "TMCFeature" ./pkg/features/

# Check for TMC in server configuration
grep -r "tmc" ./pkg/server/
```

### 9. End-to-End Test
```bash
# Start KCP with TMC in background
./bin/kcp start \
  --root-directory=/tmp/kcp-tmc-e2e \
  --feature-gates=TMCFeature=true,TMCAPIs=true \
  > /tmp/kcp-tmc.log 2>&1 &
KCP_PID=$!

# Wait for server to start
sleep 10

# Set kubeconfig
export KUBECONFIG=/tmp/kcp-tmc-e2e/admin.kubeconfig

# Create a workspace
./bin/kubectl-ws create tmc-test --type universal

# Use the workspace
./bin/kubectl-ws use tmc-test

# Try to create TMC resources (if CRDs are available)
cat <<EOF | kubectl apply -f -
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: test-cluster
spec:
  clusterID: test-123
  region: us-west-2
EOF

# Check the resource
kubectl get clusterregistrations

# Clean up
kill $KCP_PID
rm -rf /tmp/kcp-tmc-e2e
```

### 10. Performance Tests
```bash
# Check binary startup time
time ./bin/tmc-controller --help

# Check memory usage at startup
/usr/bin/time -v ./bin/tmc-controller --dry-run 2>&1 | grep "Maximum resident"

# Check KCP startup with TMC
time ./bin/kcp start \
  --feature-gates=TMCFeature=true \
  --root-directory=/tmp/perf-test \
  --dry-run
```

## Expected Results

### ✅ Successful Integration Indicators:
1. Both `kcp` and `tmc-controller` binaries exist and run
2. All 7 TMC feature flags appear in KCP options
3. TMC controller starts without errors
4. KCP accepts TMC feature gates
5. TMC API types are compiled into the binary
6. Generated deepcopy code exists
7. Controller packages are present

### ⚠️ Potential Issues to Check:
1. If binaries don't exist → run `make build`
2. If feature flags missing → check `pkg/features/kcp_features.go`
3. If controller fails → check for kubeconfig/auth issues
4. If APIs not found → verify CRD generation with `make crds`

## Automated Test Runner

Run all tests automatically:
```bash
./TMC-VERIFICATION-TESTS.sh
```

This will run through all basic verification tests and provide a summary.