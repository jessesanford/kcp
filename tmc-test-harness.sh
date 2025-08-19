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

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [ -n "$KCP_PID" ]; then
        kill $KCP_PID 2>/dev/null || true
    fi
    if [ -n "$TMC_PID" ]; then
        kill $TMC_PID 2>/dev/null || true
    fi
    rm -rf $KCP_ROOT
}
trap cleanup EXIT

# Step 1: Start KCP with TMC features
echo -e "${GREEN}Step 1: Starting KCP with TMC features...${NC}"
echo "Root directory: $KCP_ROOT"
echo "Logs: $KCP_LOG"
echo ""

./bin/kcp start \
    --root-directory=$KCP_ROOT \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --external-hostname=localhost \
    --v=2 > $KCP_LOG 2>&1 &
KCP_PID=$!

echo "Waiting for KCP to start (PID: $KCP_PID)..."
sleep 5

# Check if KCP is running
if ! ps -p $KCP_PID > /dev/null; then
    echo -e "${RED}KCP failed to start! Check logs:${NC}"
    tail -20 $KCP_LOG
    exit 1
fi

export KUBECONFIG=$KUBECONFIG

# Wait for kubeconfig to be created
for i in {1..10}; do
    if [ -f "$KUBECONFIG" ]; then
        break
    fi
    sleep 1
done

if [ ! -f "$KUBECONFIG" ]; then
    echo -e "${RED}Kubeconfig not created!${NC}"
    exit 1
fi

echo -e "${GREEN}✓ KCP started successfully${NC}"
echo ""

# Step 2: Create a workspace for TMC testing
echo -e "${GREEN}Step 2: Creating TMC test workspace...${NC}"
./bin/kubectl-ws create tmc-test --type universal || true
./bin/kubectl-ws use tmc-test
WORKSPACE=$(./bin/kubectl-ws current)
echo "Current workspace: $WORKSPACE"
echo ""

# Step 3: Check for TMC CRDs
echo -e "${GREEN}Step 3: Checking for TMC CRDs...${NC}"
kubectl api-resources | grep -i tmc || echo "No TMC CRDs found yet"
echo ""

# Step 4: Create TMC CRDs manually if needed
echo -e "${GREEN}Step 4: Creating TMC CRDs...${NC}"
cat <<EOF | kubectl apply -f - || true
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
EOF
echo ""

# Step 5: Start TMC Controller separately
echo -e "${GREEN}Step 5: Starting TMC Controller...${NC}"
./bin/tmc-controller \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --kubeconfig=$KUBECONFIG \
    --v=4 > $TMC_LOG 2>&1 &
TMC_PID=$!

echo "TMC Controller started (PID: $TMC_PID)"
sleep 3

# Check if TMC controller is running
if ps -p $TMC_PID > /dev/null; then
    echo -e "${GREEN}✓ TMC Controller running${NC}"
else
    echo -e "${YELLOW}⚠ TMC Controller may have exited (expected for basic implementation)${NC}"
fi
echo ""

# Step 6: Create test TMC resources
echo -e "${GREEN}Step 6: Creating test TMC resources...${NC}"

# Create namespace
kubectl create namespace tmc-demo 2>/dev/null || true

# Create sample ClusterRegistration
echo "Creating ClusterRegistration..."
cat <<EOF | kubectl apply -f -
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
EOF

cat <<EOF | kubectl apply -f -
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
EOF

cat <<EOF | kubectl apply -f -
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

echo -e "${GREEN}✓ Created 3 ClusterRegistrations${NC}"
echo ""

# Create sample WorkloadPlacement
echo "Creating WorkloadPlacement..."
cat <<EOF | kubectl apply -f -
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
EOF

cat <<EOF | kubectl apply -f -
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

echo -e "${GREEN}✓ Created 2 WorkloadPlacements${NC}"
echo ""

# Step 7: Create sample workloads
echo -e "${GREEN}Step 7: Creating sample workloads...${NC}"
cat <<EOF | kubectl apply -f -
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

echo -e "${GREEN}✓ Created sample Deployment${NC}"
echo ""

# Step 8: Monitor TMC resources
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Resources Status${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}ClusterRegistrations:${NC}"
kubectl get clusterregistrations -n tmc-demo -o wide

echo ""
echo -e "${YELLOW}WorkloadPlacements:${NC}"
kubectl get workloadplacements -n tmc-demo -o wide

echo ""
echo -e "${YELLOW}Deployments:${NC}"
kubectl get deployments -n tmc-demo

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Controller Logs (last 20 lines)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${YELLOW}KCP Server logs (TMC-related):${NC}"
grep -i tmc $KCP_LOG | tail -10 || echo "No TMC logs found"

echo ""
echo -e "${YELLOW}TMC Controller logs:${NC}"
if [ -f "$TMC_LOG" ]; then
    tail -20 $TMC_LOG
else
    echo "No TMC controller logs available"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Interactive Commands${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "You can now interact with TMC resources:"
echo ""
echo "  # Watch TMC resources"
echo "  watch kubectl get clusterregistrations,workloadplacements -n tmc-demo"
echo ""
echo "  # Describe a ClusterRegistration"
echo "  kubectl describe clusterregistration cluster-us-west-1 -n tmc-demo"
echo ""
echo "  # Check controller logs"
echo "  tail -f $KCP_LOG | grep -i tmc"
echo ""
echo "  # Create more resources"
echo "  kubectl apply -f your-tmc-resources.yaml"
echo ""
echo "  # Check API resources"
echo "  kubectl api-resources | grep tmc"
echo ""
echo "  # Get events"
echo "  kubectl get events -n tmc-demo --sort-by='.lastTimestamp'"
echo ""
echo -e "${GREEN}Test harness is running. Press Ctrl+C to stop and cleanup.${NC}"
echo ""

# Keep running and show logs
while true; do
    sleep 30
    echo -e "\n${YELLOW}=== Status Update $(date) ===${NC}"
    kubectl get clusterregistrations,workloadplacements -n tmc-demo --no-headers 2>/dev/null | head -5
    echo "KCP PID $KCP_PID: $(ps -p $KCP_PID -o comm= 2>/dev/null || echo 'stopped')"
    echo "TMC PID $TMC_PID: $(ps -p $TMC_PID -o comm= 2>/dev/null || echo 'stopped')"
done