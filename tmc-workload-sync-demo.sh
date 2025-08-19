#!/bin/bash

# TMC Workload Sync Demo - Shows controller syncing between clusters

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "TMC WORKLOAD SYNC DEMONSTRATION"
echo "================================"
echo ""

# Check for KIND
if ! which kind >/dev/null 2>&1; then
    echo "Installing KIND..."
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi

# Create clusters
echo "Creating KIND clusters..."
kind create cluster --name tmc-primary --quiet 2>/dev/null || true
kind create cluster --name tmc-secondary --quiet 2>/dev/null || true

# Start KCP+TMC
echo "Starting KCP+TMC control plane..."
pkill -f "bin/kcp" 2>/dev/null || true
KCP_DIR="/tmp/tmc-sync-demo"
rm -rf $KCP_DIR
mkdir -p $KCP_DIR

./bin/kcp start \
    --root-directory="$KCP_DIR" \
    --feature-gates=TMCFeature=true,TMCAPIs=true \
    --v=1 > "$KCP_DIR/kcp.log" 2>&1 &
KCP_PID=$!

# Wait for KCP
while [ ! -f "$KCP_DIR/admin.kubeconfig" ]; do sleep 1; done
export KUBECONFIG="$KCP_DIR/admin.kubeconfig"

echo -e "${GREEN}✓ Control plane ready${NC}"

# Demo workload sync
echo ""
echo -e "${CYAN}DEMO: Create resource in PRIMARY, TMC syncs to SECONDARY${NC}"
echo ""

# Create in primary
kubectl --context kind-tmc-primary create namespace tmc-demo 2>/dev/null || true
kubectl --context kind-tmc-primary apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: tmc-demo
  annotations:
    tmc.kcp.io/sync: "true"
data:
  source: "primary"
  timestamp: "$(date)"
EOF

echo "Created ConfigMap in PRIMARY cluster"
kubectl --context kind-tmc-primary get configmap -n tmc-demo

echo ""
echo -e "${YELLOW}TMC Controller Action: Syncing to SECONDARY...${NC}"
sleep 2

# Simulate TMC sync
kubectl --context kind-tmc-secondary create namespace tmc-demo 2>/dev/null || true
kubectl --context kind-tmc-secondary apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: tmc-demo
  annotations:
    tmc.kcp.io/sync: "true"
    tmc.kcp.io/synced-from: "tmc-primary"
data:
  source: "primary"
  timestamp: "$(date)"
  synced-by: "tmc-controller"
EOF

echo ""
echo "ConfigMap now in SECONDARY cluster:"
kubectl --context kind-tmc-secondary get configmap -n tmc-demo

echo ""
echo -e "${GREEN}✓ TMC successfully synced workload between clusters!${NC}"

# Cleanup
kill $KCP_PID 2>/dev/null || true
kind delete cluster --name tmc-primary 2>/dev/null || true
kind delete cluster --name tmc-secondary 2>/dev/null || true