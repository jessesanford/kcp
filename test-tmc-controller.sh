#!/bin/bash

# Quick test to verify TMC controller starts correctly
set -e

echo "Testing TMC Controller startup..."
echo "================================="
echo ""

# Create a test kubeconfig
cat > /tmp/test-kubeconfig <<EOF
apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://localhost:6443
    insecure-skip-tls-verify: true
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
users:
- name: test-user
  user:
    token: fake-token
EOF

echo "1. Testing TMC controller with --help flag:"
./bin/tmc-controller --help 2>&1 | head -20
echo ""

echo "2. Testing TMC controller version/startup (will fail to connect, but shows it runs):"
timeout 3 ./bin/tmc-controller \
    --kubeconfig=/tmp/test-kubeconfig \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true \
    --workers=4 2>&1 || true
echo ""

echo "3. Checking that controller was compiled with actual implementation:"
if strings ./bin/tmc-controller | grep -q "ClusterRegistrationController"; then
    echo "✓ Found ClusterRegistrationController in binary"
else
    echo "✗ ClusterRegistrationController not found in binary"
fi

if strings ./bin/tmc-controller | grep -q "WorkloadPlacement"; then
    echo "✓ Found WorkloadPlacement references in binary"
else
    echo "✗ WorkloadPlacement not found in binary"
fi

echo ""
echo "Test complete!"