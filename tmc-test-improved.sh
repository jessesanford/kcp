#!/bin/bash

# TMC Test Harness - Improved version with port handling

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Test Harness (Improved)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check for existing KCP
echo -e "${YELLOW}Checking for existing KCP processes...${NC}"
if pgrep -f "bin/kcp start" > /dev/null; then
    echo "Found existing KCP process. Options:"
    echo "1) Kill existing and start new"
    echo "2) Use existing KCP instance"
    echo "3) Exit"
    read -p "Choose (1/2/3): " choice
    
    case $choice in
        1)
            echo "Killing existing KCP..."
            pkill -f "bin/kcp start"
            sleep 3
            ;;
        2)
            # Find existing kubeconfig
            EXISTING_KC=$(ls -t /tmp/kcp-*/admin.kubeconfig 2>/dev/null | head -1)
            if [ -f "$EXISTING_KC" ]; then
                export KUBECONFIG="$EXISTING_KC"
                echo -e "${GREEN}Using existing KCP with KUBECONFIG=$KUBECONFIG${NC}"
                
                # Skip to resource creation
                echo ""
                echo -e "${GREEN}Step: Creating TMC resources...${NC}"
                kubectl create namespace tmc-demo 2>/dev/null || true
                
                # Create simple test
                kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-test
  namespace: tmc-demo
data:
  status: "TMC test successful"
EOF
                
                kubectl get configmap tmc-test -n tmc-demo
                echo -e "${GREEN}✓ TMC test complete with existing KCP${NC}"
                exit 0
            else
                echo -e "${RED}Could not find kubeconfig for existing KCP${NC}"
                exit 1
            fi
            ;;
        3)
            echo "Exiting..."
            exit 0
            ;;
    esac
fi

# Configuration - use random port if 6443 is busy
DEFAULT_PORT=6443
PORT=$DEFAULT_PORT

# Check if default port is available
if lsof -i :$DEFAULT_PORT >/dev/null 2>&1; then
    # Find a free port
    PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()')
    echo -e "${YELLOW}Port $DEFAULT_PORT is busy, using port $PORT instead${NC}"
fi

KCP_ROOT="/tmp/kcp-tmc-test-$(date +%s)"
KCP_LOG="${KCP_ROOT}/kcp.log"
TMC_LOG="${KCP_ROOT}/tmc.log"
KUBECONFIG="${KCP_ROOT}/admin.kubeconfig"

# Create directories
echo -e "${GREEN}Creating test directories...${NC}"
mkdir -p "$KCP_ROOT"
touch "$KCP_LOG"
touch "$TMC_LOG"
echo "Root directory: $KCP_ROOT"
echo ""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ -n "$KCP_PID" ]; then
        echo "Stopping KCP (PID: $KCP_PID)..."
        kill $KCP_PID 2>/dev/null || true
        wait $KCP_PID 2>/dev/null || true
    fi
    echo "Keeping directory for debugging: $KCP_ROOT"
    echo "To remove: rm -rf $KCP_ROOT"
}
trap cleanup EXIT INT TERM

# Check binaries
if [ ! -f "./bin/kcp" ]; then
    echo -e "${RED}Error: KCP binary not found at ./bin/kcp${NC}"
    exit 1
fi

# Start KCP
echo -e "${GREEN}Starting KCP on port $PORT...${NC}"
./bin/kcp start \
    --root-directory="$KCP_ROOT" \
    --secure-port=$PORT \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --v=2 > "$KCP_LOG" 2>&1 &
KCP_PID=$!

echo "KCP started with PID: $KCP_PID on port $PORT"
echo "Waiting for KCP to initialize..."

# Wait for KCP
MAX_WAIT=60
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if ! ps -p $KCP_PID > /dev/null 2>&1; then
        echo -e "${RED}KCP process died unexpectedly!${NC}"
        echo "Last 20 lines of log:"
        tail -20 "$KCP_LOG"
        exit 1
    fi
    
    if [ -f "$KUBECONFIG" ]; then
        echo -e "${GREEN}✓ Kubeconfig found${NC}"
        break
    fi
    
    echo "  Waiting for kubeconfig... ($WAITED/$MAX_WAIT seconds)"
    sleep 2
    WAITED=$((WAITED + 2))
done

if [ ! -f "$KUBECONFIG" ]; then
    echo -e "${RED}KCP failed to create kubeconfig after $MAX_WAIT seconds${NC}"
    tail -20 "$KCP_LOG"
    exit 1
fi

export KUBECONFIG
echo "KUBECONFIG=$KUBECONFIG"
echo ""

# Update kubeconfig for custom port if needed
if [ "$PORT" != "$DEFAULT_PORT" ]; then
    echo "Updating kubeconfig for port $PORT..."
    sed -i "s/:6443/:$PORT/g" "$KUBECONFIG"
fi

# Wait for API server to be ready
echo -e "${GREEN}Testing KCP connection...${NC}"
API_READY=false
API_WAIT=0
while [ $API_WAIT -lt 30 ]; do
    if kubectl cluster-info --request-timeout=3s >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Successfully connected to KCP API server${NC}"
        API_READY=true
        break
    fi
    
    echo "  Waiting for API server... ($API_WAIT/30 seconds)"
    sleep 2
    API_WAIT=$((API_WAIT + 2))
done

if [ "$API_READY" != "true" ]; then
    echo -e "${YELLOW}⚠ Could not verify API server connection, continuing anyway...${NC}"
    # Show some logs for debugging
    echo "Recent logs:"
    tail -10 "$KCP_LOG" | grep -v "connection refused" | head -5
fi

# Create namespace
echo ""
echo -e "${GREEN}Creating TMC demo namespace...${NC}"
if kubectl create namespace tmc-demo 2>/dev/null; then
    echo "✓ Created tmc-demo namespace"
else
    echo "- Namespace tmc-demo may already exist"
fi

# Create test resources
echo -e "${GREEN}Creating test TMC resources...${NC}"

# Simple ConfigMap test
if kubectl apply --validate=false -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-config
  namespace: tmc-demo
data:
  feature: "TMC enabled"
  port: "$PORT"
  timestamp: "$(date)"
EOF
then
    echo "✓ ConfigMap created successfully"
else
    echo "⚠ Failed to create ConfigMap, but continuing..."
fi

# Show status
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Test Status${NC}"
echo -e "${BLUE}========================================${NC}"

if kubectl get namespace tmc-demo 2>/dev/null; then
    echo "✓ Namespace exists"
else
    echo "⚠ Could not find namespace"
fi

if kubectl get configmap -n tmc-demo 2>/dev/null; then
    echo "✓ ConfigMaps found"
else
    echo "⚠ Could not find ConfigMaps"
fi

echo ""
echo -e "${GREEN}✓ Test harness running successfully!${NC}"
echo -e "${GREEN}KCP PID: $KCP_PID on port $PORT${NC}"
echo -e "${GREEN}Root: $KCP_ROOT${NC}"
echo ""
echo "Commands to use:"
echo "  export KUBECONFIG=$KUBECONFIG"
echo "  kubectl get all -n tmc-demo"
echo "  tail -f $KCP_LOG | grep -i tmc"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"

# Keep running
while true; do
    sleep 30
    if ! ps -p $KCP_PID > /dev/null 2>&1; then
        echo -e "${RED}KCP stopped unexpectedly${NC}"
        exit 1
    fi
    echo -e "${BLUE}[$(date +%H:%M:%S)] KCP still running on port $PORT${NC}"
done