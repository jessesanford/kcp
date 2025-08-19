#!/bin/bash

echo "Cleaning up KCP processes and ports..."

# Kill all KCP processes
echo "Stopping KCP processes..."
pkill -f "bin/kcp start" || echo "No KCP processes found"

# Wait for ports to be released
sleep 2

# Check port 6443
if lsof -i :6443 >/dev/null 2>&1; then
    echo "WARNING: Port 6443 still in use by:"
    lsof -i :6443
else
    echo "✓ Port 6443 is free"
fi

# Clean up old test directories (keep last 2)
echo "Cleaning old test directories..."
ls -dt /tmp/kcp-tmc-test-* 2>/dev/null | tail -n +3 | xargs rm -rf 2>/dev/null || true
ls -dt /tmp/kcp-test-* 2>/dev/null | tail -n +3 | xargs rm -rf 2>/dev/null || true
ls -dt /tmp/kcp-demo-* 2>/dev/null | tail -n +3 | xargs rm -rf 2>/dev/null || true

echo "✓ Cleanup complete"

# Show remaining directories
echo ""
echo "Remaining test directories:"
ls -la /tmp/kcp-* 2>/dev/null | head -5 || echo "None"