#!/bin/bash

# TMC Test Harness - Create and test TMC functionality
# This script starts KCP with TMC, creates TMC objects, and monitors controller behavior

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
KCP_ROOT="/tmp/kcp-tmc-test-$(date +%s)"
KCP_LOG="${KCP_ROOT}/kcp.log"
TMC_LOG="${KCP_ROOT}/tmc.log"
KUBECONFIG="${KCP_ROOT}/admin.kubeconfig"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Test Harness${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Create directories first
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
    if [ -n "$TMC_PID" ]; then
        echo "Stopping TMC controller (PID: $TMC_PID)..."
        kill $TMC_PID 2>/dev/null || true
        wait $TMC_PID 2>/dev/null || true
    fi
    echo "Removing temporary directory: $KCP_ROOT"
    rm -rf $KCP_ROOT
    echo "Cleanup complete."
}
trap cleanup EXIT INT TERM

# Check if binaries exist
if [ ! -f "./bin/kcp" ]; then
    echo -e "${RED}Error: KCP binary not found at ./bin/kcp${NC}"
    echo "Please run this script from the KCP repository root where binaries are built."
    exit 1
fi

if [ ! -f "./bin/tmc-controller" ]; then
    echo -e "${RED}Error: TMC controller binary not found at ./bin/tmc-controller${NC}"
    echo "Please build the TMC controller first."
    exit 1
fi

# Step 1: Start KCP with TMC features
echo -e "${GREEN}Step 1: Starting KCP with TMC features...${NC}"
echo "Command: ./bin/kcp start --root-directory=$KCP_ROOT --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true"
echo "Logs will be written to: $KCP_LOG"
echo ""

# Start KCP in background
./bin/kcp start \
    --root-directory="$KCP_ROOT" \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --external-hostname=localhost \
    --v=2 > "$KCP_LOG" 2>&1 &
KCP_PID=$!

echo "KCP started with PID: $KCP_PID"
echo "Waiting for KCP to initialize..."

# Wait for KCP to be ready
MAX_WAIT=30
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if ! ps -p $KCP_PID > /dev/null 2>&1; then
        echo -e "${RED}KCP process died unexpectedly!${NC}"
        echo "Last 50 lines of log:"
        tail -50 "$KCP_LOG" 2>/dev/null || echo "No log available"
        exit 1
    fi
    
    # Check if kubeconfig exists
    if [ -f "$KUBECONFIG" ]; then
        echo -e "${GREEN}✓ KCP is ready (kubeconfig created)${NC}"
        break
    fi
    
    # Check for common startup issues in log
    if grep -q "panic:" "$KCP_LOG" 2>/dev/null; then
        echo -e "${RED}KCP panicked during startup!${NC}"
        echo "Error details:"
        grep -A5 "panic:" "$KCP_LOG"
        exit 1
    fi
    
    echo "  Waiting... ($WAITED/$MAX_WAIT seconds)"
    sleep 1
    WAITED=$((WAITED + 1))
done

if [ ! -f "$KUBECONFIG" ]; then
    echo -e "${RED}KCP failed to create kubeconfig after $MAX_WAIT seconds${NC}"
    echo "Last 50 lines of KCP log:"
    tail -50 "$KCP_LOG" 2>/dev/null || echo "No log available"
    exit 1
fi

export KUBECONFIG
echo "KUBECONFIG set to: $KUBECONFIG"
echo ""

# Verify we can connect to KCP
echo -e "${GREEN}Verifying KCP connection...${NC}"
if kubectl cluster-info --request-timeout=5s > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Successfully connected to KCP${NC}"
else
    echo -e "${YELLOW}⚠ Could not verify KCP connection, continuing anyway...${NC}"
fi
echo ""

# Step 2: Create a workspace for TMC testing
echo -e "${GREEN}Step 2: Creating TMC test workspace...${NC}"

# Check if kubectl-ws exists
if [ ! -f "./bin/kubectl-ws" ]; then
    echo -e "${YELLOW}kubectl-ws not found, using default workspace${NC}"
else
    ./bin/kubectl-ws create tmc-test --type universal 2>/dev/null || echo "Workspace may already exist"
    ./bin/kubectl-ws use tmc-test 2>/dev/null || true
    WORKSPACE=$(./bin/kubectl-ws current 2>/dev/null || echo "default")
    echo "Current workspace: $WORKSPACE"
fi
echo ""

# Step 3: Check for TMC API resources
echo -e "${GREEN}Step 3: Checking for TMC API resources...${NC}"
echo "Available TMC APIs:"
kubectl api-resources 2>/dev/null | grep -i tmc || echo "No TMC APIs found in api-resources yet"
echo ""

# Step 4: Create TMC CRDs
echo -e "${GREEN}Step 4: Creating TMC CRDs...${NC}"
cat <<'EOF' > ${KCP_ROOT}/tmc-crds.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusterregistrations.tmc.kcp.io
spec:
  group: tmc.kcp.io
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              clusterID:
                type: string
              region:
                type: string
              provider:
                type: string
              capacity:
                type: object
                properties:
                  cpu:
                    type: string
                  memory:
                    type: string
          status:
            type: object
            properties:
              phase:
                type: string
              conditions:
                type: array
                items:
                  type: object
                  properties:
                    type:
                      type: string
                    status:
                      type: string
                    lastTransitionTime:
                      type: string
                    reason:
                      type: string
                    message:
                      type: string
  scope: Namespaced
  names:
    plural: clusterregistrations
    singular: clusterregistration
    kind: ClusterRegistration
    shortNames:
    - cluster
    - clusters
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: workloadplacements.tmc.kcp.io
spec:
  group: tmc.kcp.io
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              workloadRef:
                type: object
                properties:
                  apiVersion:
                    type: string
                  kind:
                    type: string
                  name:
                    type: string
                  namespace:
                    type: string
              placement:
                type: object
                properties:
                  clusters:
                    type: array
                    items:
                      type: string
                  strategy:
                    type: string
                    enum: ["RoundRobin", "Spread", "Pack", "Manual"]
                  constraints:
                    type: array
                    items:
                      type: object
                      properties:
                        key:
                          type: string
                        operator:
                          type: string
                        values:
                          type: array
                          items:
                            type: string
          status:
            type: object
            properties:
              phase:
                type: string
              selectedClusters:
                type: array
                items:
                  type: string
              conditions:
                type: array
                items:
                  type: object
                  properties:
                    type:
                      type: string
                    status:
                      type: string
  scope: Namespaced
  names:
    plural: workloadplacements
    singular: workloadplacement
    kind: WorkloadPlacement
    shortNames:
    - placement
    - placements
EOF

kubectl apply -f ${KCP_ROOT}/tmc-crds.yaml 2>/dev/null || echo "CRDs may already exist or unable to create"
echo ""

# Step 5: Create namespace
echo -e "${GREEN}Step 5: Creating test namespace...${NC}"
kubectl create namespace tmc-demo 2>/dev/null || echo "Namespace tmc-demo already exists"
echo ""

# Step 6: Start TMC Controller (optional, may fail if not fully implemented)
echo -e "${GREEN}Step 6: Attempting to start TMC Controller...${NC}"
if [ -f "./bin/tmc-controller" ]; then
    ./bin/tmc-controller \
        --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
        --kubeconfig="$KUBECONFIG" \
        --v=4 > "$TMC_LOG" 2>&1 &
    TMC_PID=$!
    
    sleep 2
    if ps -p $TMC_PID > /dev/null 2>&1; then
        echo -e "${GREEN}✓ TMC Controller running (PID: $TMC_PID)${NC}"
    else
        echo -e "${YELLOW}⚠ TMC Controller exited (this is expected if not fully implemented)${NC}"
        echo "Controller output:"
        head -20 "$TMC_LOG" 2>/dev/null || echo "No output available"
    fi
else
    echo -e "${YELLOW}TMC controller binary not found, skipping${NC}"
fi
echo ""

# Step 7: Create test TMC resources
echo -e "${GREEN}Step 7: Creating test TMC resources...${NC}"

# Create ClusterRegistrations
echo "Creating ClusterRegistration resources..."
cat <<EOF | kubectl apply -f - || echo "Failed to create ClusterRegistrations"
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-us-west-1
  namespace: tmc-demo
spec:
  clusterID: "cluster-001"
  region: "us-west-2"
  provider: "AWS"
  capacity:
    cpu: "1000"
    memory: "8000Gi"
---
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-us-east-1
  namespace: tmc-demo
spec:
  clusterID: "cluster-002"
  region: "us-east-1"
  provider: "AWS"
  capacity:
    cpu: "500"
    memory: "4000Gi"
---
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-eu-west-1
  namespace: tmc-demo
spec:
  clusterID: "cluster-003"
  region: "eu-west-1"
  provider: "Azure"
  capacity:
    cpu: "800"
    memory: "6000Gi"
EOF

echo ""
echo "Creating WorkloadPlacement resources..."
cat <<EOF | kubectl apply -f - || echo "Failed to create WorkloadPlacements"
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: web-app-placement
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "Deployment"
    name: "web-app"
    namespace: "tmc-demo"
  placement:
    strategy: "Spread"
    constraints:
    - key: "region"
      operator: "In"
      values: ["us-west-2", "us-east-1"]
---
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: database-placement
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "StatefulSet"
    name: "postgres-db"
    namespace: "tmc-demo"
  placement:
    strategy: "Manual"
    clusters:
    - "cluster-001"
EOF

echo ""
echo "Creating sample Deployment..."
cat <<EOF | kubectl apply -f - || echo "Failed to create Deployment"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: tmc-demo
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
EOF
echo ""

# Step 8: Display status
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Resources Status${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}ClusterRegistrations:${NC}"
kubectl get clusterregistrations -n tmc-demo 2>/dev/null || echo "No ClusterRegistrations found"

echo ""
echo -e "${YELLOW}WorkloadPlacements:${NC}"
kubectl get workloadplacements -n tmc-demo 2>/dev/null || echo "No WorkloadPlacements found"

echo ""
echo -e "${YELLOW}Deployments:${NC}"
kubectl get deployments -n tmc-demo 2>/dev/null || echo "No Deployments found"

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Useful Commands${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "# Watch TMC resources:"
echo "kubectl get clusterregistrations,workloadplacements -n tmc-demo -w"
echo ""
echo "# View KCP logs:"
echo "tail -f $KCP_LOG | grep -i tmc"
echo ""
echo "# View TMC controller logs:"
echo "tail -f $TMC_LOG"
echo ""
echo "# Check events:"
echo "kubectl get events -n tmc-demo --sort-by='.lastTimestamp'"
echo ""
echo "# Apply more resources:"
echo "kubectl apply -f tmc-demo-resources.yaml"
echo ""
echo -e "${GREEN}✓ Test harness setup complete!${NC}"
echo -e "${GREEN}KCP is running with PID: $KCP_PID${NC}"
echo -e "${GREEN}Root directory: $KCP_ROOT${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop and cleanup.${NC}"
echo ""

# Keep running and show periodic status
while true; do
    sleep 30
    echo -e "\n${BLUE}=== Status Update $(date +%H:%M:%S) ===${NC}"
    echo "Clusters: $(kubectl get clusterregistrations -n tmc-demo --no-headers 2>/dev/null | wc -l)"
    echo "Placements: $(kubectl get workloadplacements -n tmc-demo --no-headers 2>/dev/null | wc -l)"
    echo "KCP running: $(ps -p $KCP_PID -o comm= 2>/dev/null || echo 'stopped')"
done