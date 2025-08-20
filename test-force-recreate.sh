#!/bin/bash

# Test the --force-recreate functionality

echo "Testing --force-recreate functionality"
echo "======================================="
echo ""

# Check current clusters
echo "Current KIND clusters:"
kind get clusters 2>/dev/null || echo "No clusters found"
echo ""

# Create test clusters if they don't exist
if ! kind get clusters 2>/dev/null | grep -q "tmc-west"; then
    echo "Creating test cluster tmc-west..."
    kind create cluster --name tmc-west --wait 30s
fi

if ! kind get clusters 2>/dev/null | grep -q "tmc-east"; then
    echo "Creating test cluster tmc-east..."
    kind create cluster --name tmc-east --wait 30s
fi

echo ""
echo "Clusters after setup:"
kind get clusters 2>/dev/null
echo ""

# Test 1: Try to run without --force-recreate (should fail)
echo "Test 1: Running without --force-recreate (should fail)..."
echo "n" | timeout 2 ./tmc-multi-cluster-demo.sh 2>&1 | grep -E "(already exists|force-recreate)" || true
echo ""

# Test 2: Run with --force-recreate (should work)
echo "Test 2: Running with --force-recreate (should delete and proceed)..."
echo "" | timeout 5 ./tmc-multi-cluster-demo.sh --force-recreate 2>&1 | grep -E "(Force recreating|Deleted existing cluster|Creating cluster)" | head -10 || true
echo ""

echo "Test complete!"