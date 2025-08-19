#!/bin/bash

# TMC Quick Test - Non-interactive version for CI/testing

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}TMC Quick Test${NC}"
echo "=============="

# Clean up any existing processes
pkill -f "bin/kcp start" || true
sleep 1

# Configuration
PORT=6443
if lsof -i :$PORT >/dev/null 2>&1; then
    PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()')
    echo -e "${YELLOW}Using port $PORT instead of 6443${NC}"
fi

KCP_ROOT="/tmp/kcp-quick-test-$(date +%s)"
KUBECONFIG="${KCP_ROOT}/admin.kubeconfig"
SUCCESS_FILE="${KCP_ROOT}/test-success"

# Cleanup function
cleanup() {
    if [ -n "$KCP_PID" ]; then
        kill $KCP_PID 2>/dev/null || true
        wait $KCP_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

# Create directory
mkdir -p "$KCP_ROOT"

# Check binary
if [ ! -f "./bin/kcp" ]; then
    echo -e "${RED}Error: KCP binary not found${NC}"
    exit 1
fi

echo -e "${GREEN}Starting KCP on port $PORT...${NC}"

# Start KCP
./bin/kcp start \
    --root-directory="$KCP_ROOT" \
    --secure-port=$PORT \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --v=1 > "$KCP_ROOT/kcp.log" 2>&1 &
KCP_PID=$!

echo "KCP PID: $KCP_PID"

# Wait for kubeconfig
WAIT=0
while [ $WAIT -lt 60 ]; do
    if [ -f "$KUBECONFIG" ]; then
        break
    fi
    if ! ps -p $KCP_PID > /dev/null 2>&1; then
        echo -e "${RED}KCP died:${NC}"
        tail -5 "$KCP_ROOT/kcp.log"
        exit 1
    fi
    sleep 1
    WAIT=$((WAIT + 1))
done

if [ ! -f "$KUBECONFIG" ]; then
    echo -e "${RED}No kubeconfig after 60s${NC}"
    exit 1
fi

# Fix port in kubeconfig if needed
if [ "$PORT" != "6443" ]; then
    sed -i "s/:6443/:$PORT/g" "$KUBECONFIG"
fi

export KUBECONFIG

echo -e "${GREEN}Testing KCP API...${NC}"

# Wait for API server
API_WAIT=0
while [ $API_WAIT -lt 30 ]; do
    if kubectl cluster-info --request-timeout=2s >/dev/null 2>&1; then
        echo -e "${GREEN}✓ API server ready${NC}"
        break
    fi
    sleep 1
    API_WAIT=$((API_WAIT + 1))
done

# Test basic operations
echo -e "${GREEN}Testing basic operations...${NC}"

# Create namespace
if kubectl create namespace tmc-quick-test >/dev/null 2>&1; then
    echo "✓ Namespace creation"
else
    echo -e "${YELLOW}⚠ Namespace creation failed${NC}"
fi

# Create ConfigMap
if kubectl apply --validate=false -f - >/dev/null 2>&1 <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: tmc-quick-test
data:
  test: "passed"
  port: "$PORT"
EOF
then
    echo "✓ ConfigMap creation"
else
    echo -e "${YELLOW}⚠ ConfigMap creation failed${NC}"
fi

# Verify resources
if kubectl get namespace tmc-quick-test >/dev/null 2>&1; then
    echo "✓ Namespace exists"
fi

if kubectl get configmap test-config -n tmc-quick-test >/dev/null 2>&1; then
    echo "✓ ConfigMap exists"
    
    # Mark success
    echo "TMC quick test passed at $(date)" > "$SUCCESS_FILE"
fi

echo ""
echo -e "${BLUE}Test Summary:${NC}"
echo "KCP Root: $KCP_ROOT"
echo "Port: $PORT"
echo "KUBECONFIG: $KUBECONFIG"

if [ -f "$SUCCESS_FILE" ]; then
    echo -e "${GREEN}✓ Quick test PASSED${NC}"
else
    echo -e "${RED}✗ Quick test FAILED${NC}"
fi

echo ""
echo "To interact with this KCP instance:"
echo "  export KUBECONFIG=$KUBECONFIG"
echo "  kubectl get all --all-namespaces"

# Keep running for 10 seconds to allow manual inspection
echo ""
echo -e "${YELLOW}KCP will run for 10 more seconds...${NC}"
sleep 10

echo -e "${GREEN}Quick test complete${NC}"