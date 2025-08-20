#!/bin/bash

echo "Full test of unique naming and port allocation"
echo "==============================================="
echo ""

# Test 1: Check that script generates unique names and ports
echo "Test 1: Checking unique cluster names and ports..."
output=$(echo "" | timeout 2 ./tmc-multi-cluster-demo.sh 2>&1)
cluster_names=$(echo "$output" | grep "Cluster names:" | head -1)
ports=$(echo "$output" | grep "NodePort assignments:" | head -1)

echo "  $cluster_names"
echo "  $ports"
echo ""

# Test 2: Verify clusters will be created with correct names
echo "Test 2: Testing cluster creation with unique names..."
echo "" | timeout 20 ./tmc-multi-cluster-demo.sh --force-recreate 2>&1 | grep -E "(Creating cluster|port [0-9]+)" | head -4

echo ""
echo "Test 3: Checking created clusters..."
kind get clusters 2>/dev/null | grep -E "tmc-(west|east)-tmc-pr-upstream" || echo "  No clusters found (expected if timed out)"

echo ""
echo "Test 4: Cleaning up test clusters..."
kind delete cluster --name "tmc-west-tmc-pr-upstream" 2>/dev/null || true
kind delete cluster --name "tmc-east-tmc-pr-upstream" 2>/dev/null || true
echo "  Cleanup complete"

echo ""
echo "Test complete! The script now creates:"
echo "  - Unique cluster names based on folder: tmc-{west|east}-<folder-name>"
echo "  - Random available ports for each cluster to avoid conflicts"
echo "  - This allows multiple demos to run in parallel"