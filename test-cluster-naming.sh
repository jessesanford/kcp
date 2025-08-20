#!/bin/bash

# Test that cluster names include folder suffix

echo "Testing cluster naming with folder suffix"
echo "=========================================="
echo ""

# Get the expected names
FOLDER_SUFFIX=$(basename "/workspaces/tmc-pr-upstream")
EXPECTED_WEST="tmc-west-${FOLDER_SUFFIX}"
EXPECTED_EAST="tmc-east-${FOLDER_SUFFIX}"

echo "Expected cluster names:"
echo "  West: ${EXPECTED_WEST}"
echo "  East: ${EXPECTED_EAST}"
echo ""

# Test that the script will create clusters with these names
echo "Testing cluster creation (will timeout, just checking names)..."
echo "" | timeout 15 ./tmc-multi-cluster-demo.sh --force-recreate 2>&1 | grep -E "Creating cluster: tmc-(west|east)-" | head -2

echo ""
echo "Checking if clusters were created..."
kind get clusters 2>/dev/null | grep -E "tmc-(west|east)-${FOLDER_SUFFIX}" || echo "No clusters found (expected if timed out early)"

# Clean up any partial clusters
echo ""
echo "Cleaning up test clusters..."
kind delete cluster --name "${EXPECTED_WEST}" 2>/dev/null || true
kind delete cluster --name "${EXPECTED_EAST}" 2>/dev/null || true

echo "Test complete!"