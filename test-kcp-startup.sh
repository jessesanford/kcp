#!/bin/bash

# Simple KCP startup test to diagnose issues

echo "Testing KCP startup..."
echo "Current directory: $(pwd)"
echo ""

# Check binary
if [ ! -f "./bin/kcp" ]; then
    echo "ERROR: KCP binary not found at ./bin/kcp"
    exit 1
fi

echo "KCP binary found: $(ls -lh ./bin/kcp | awk '{print $9, $5}')"
echo ""

# Create test directory
TEST_DIR="/tmp/kcp-test-$$"
mkdir -p "$TEST_DIR"
echo "Test directory: $TEST_DIR"
echo ""

# Try to start KCP
echo "Starting KCP with minimal options..."
echo "Command: ./bin/kcp start --root-directory=$TEST_DIR --v=2"
echo ""

timeout 10 ./bin/kcp start \
    --root-directory="$TEST_DIR" \
    --feature-gates=TMCFeature=true \
    --v=2 2>&1 | tee "$TEST_DIR/startup.log" &

PID=$!
echo "KCP started with PID: $PID"
echo ""

# Wait a bit
sleep 5

# Check if still running
if ps -p $PID > /dev/null 2>&1; then
    echo "✓ KCP is running!"
    echo ""
    echo "Checking for kubeconfig..."
    if [ -f "$TEST_DIR/admin.kubeconfig" ]; then
        echo "✓ Kubeconfig created!"
        export KUBECONFIG="$TEST_DIR/admin.kubeconfig"
        kubectl cluster-info 2>&1 | head -5
    else
        echo "✗ No kubeconfig found"
    fi
    
    # Kill it
    kill $PID 2>/dev/null
else
    echo "✗ KCP stopped/crashed"
    echo ""
    echo "Last 20 lines of output:"
    tail -20 "$TEST_DIR/startup.log" 2>/dev/null || echo "No log found"
fi

# Cleanup
rm -rf "$TEST_DIR"

echo ""
echo "Test complete."