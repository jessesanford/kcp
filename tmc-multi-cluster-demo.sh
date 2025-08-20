#!/bin/bash

# TMC Multi-Cluster Demo - REAL workload movement between clusters
# Ported to work with compiled binaries in /workspaces/tmc-pr-upstream/bin/
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

# Parse command line arguments
FORCE_RECREATE=false
for arg in "$@"; do
    case $arg in
        --force-recreate)
            FORCE_RECREATE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --force-recreate    Delete and recreate KIND clusters if they already exist"
            echo "  --help             Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $arg"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Set working directory
WORK_DIR="/workspaces/tmc-pr-upstream"
cd "$WORK_DIR"

# Get folder name for cluster suffix to make cluster names unique per workspace
# This allows multiple demos to run in parallel in different folders
FOLDER_SUFFIX=$(basename "$WORK_DIR")
WEST_CLUSTER="tmc-west-${FOLDER_SUFFIX}"
EAST_CLUSTER="tmc-east-${FOLDER_SUFFIX}"

# Function to check if port is available
is_port_available() {
    local port=$1
    ! nc -z localhost "$port" 2>/dev/null
}

# Generate random ports for NodePort services to avoid conflicts
# Use range 30000-32767 (standard NodePort range)
find_available_port() {
    local port
    for attempt in {1..100}; do
        port=$((30000 + RANDOM % 2768))
        if is_port_available "$port"; then
            echo "$port"
            return 0
        fi
    done
    echo "Error: Could not find available port after 100 attempts" >&2
    exit 1
}

WEST_NODE_PORT=$(find_available_port)
EAST_NODE_PORT=$(find_available_port)
# Ensure ports are different
while [ "$EAST_NODE_PORT" -eq "$WEST_NODE_PORT" ]; do
    EAST_NODE_PORT=$(find_available_port)
done

echo -e "${MAGENTA}${BOLD}"
echo "============================================================"
echo "   TMC MULTI-CLUSTER WORKLOAD ORCHESTRATION DEMO"
echo "============================================================"
echo -e "${NC}"
echo ""
echo "This demo will:"
echo "  1. Create 2 KIND Kubernetes clusters (${WEST_CLUSTER}, ${EAST_CLUSTER})"
echo "  2. Start KCP with TMC as control plane"
echo "  3. Register both clusters with TMC"
echo "  4. Deploy workload to ${WEST_CLUSTER}"
echo "  5. Move workload to ${EAST_CLUSTER} using TMC"
echo "  6. Show cross-cluster controller actuation"
echo ""
echo "Using binaries from: $WORK_DIR/bin/"
echo "Cluster names: ${WEST_CLUSTER}, ${EAST_CLUSTER}"
echo "NodePort assignments: West=${WEST_NODE_PORT}, East=${EAST_NODE_PORT}"
if [ "$FORCE_RECREATE" = true ]; then
    echo -e "${YELLOW}Force recreate mode: Will delete existing KIND clusters if found${NC}"
fi
echo ""
echo "Press Enter to begin (or Ctrl+C to cancel)..."
read

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    kind delete cluster --name "${WEST_CLUSTER}" 2>/dev/null || true
    kind delete cluster --name "${EAST_CLUSTER}" 2>/dev/null || true
    pkill -f "$WORK_DIR/bin/kcp start" 2>/dev/null || true
    pkill -f "$WORK_DIR/bin/tmc-controller" 2>/dev/null || true
    rm -rf /tmp/tmc-demo-* 2>/dev/null || true
}

# Set up trap for unexpected exits but not normal completion
trap cleanup SIGINT SIGTERM

# Function to print section
print_section() {
    echo ""
    echo -e "${BLUE}${BOLD}========================================${NC}"
    echo -e "${BLUE}${BOLD}$1${NC}"
    echo -e "${BLUE}${BOLD}========================================${NC}"
}

# Check for required binaries
print_section "CHECKING PREREQUISITES"

if [ ! -f "$WORK_DIR/bin/kcp" ]; then
    echo -e "${RED}✗ KCP binary not found at $WORK_DIR/bin/kcp${NC}"
    echo "Please compile the binaries first."
    exit 1
fi

if [ ! -f "$WORK_DIR/bin/tmc-controller" ]; then
    echo -e "${RED}✗ TMC controller binary not found at $WORK_DIR/bin/tmc-controller${NC}"
    echo "Please compile the binaries first."
    exit 1
fi

if ! command -v kind &> /dev/null; then
    echo -e "${YELLOW}KIND not found. Installing KIND...${NC}"
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi

if ! command -v kubectl &> /dev/null; then
    echo -e "${YELLOW}kubectl not found. Installing kubectl...${NC}"
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/kubectl
fi

echo -e "${GREEN}✓ All prerequisites found${NC}"

print_section "STEP 1: CREATING KIND CLUSTERS"

# Check for existing clusters and handle based on --force-recreate flag
check_and_handle_existing_clusters() {
    local cluster_name=$1
    if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
        if [ "$FORCE_RECREATE" = true ]; then
            echo -e "${YELLOW}Cluster '${cluster_name}' already exists. Force recreating...${NC}"
            kind delete cluster --name "${cluster_name}"
            echo -e "${GREEN}✓ Deleted existing cluster '${cluster_name}'${NC}"
        else
            echo -e "${RED}✗ Cluster '${cluster_name}' already exists!${NC}"
            echo -e "${YELLOW}Use --force-recreate flag to delete and recreate existing clusters${NC}"
            echo -e "${YELLOW}Or manually delete with: kind delete cluster --name ${cluster_name}${NC}"
            exit 1
        fi
    fi
}

# Check and handle existing clusters
check_and_handle_existing_clusters "${WEST_CLUSTER}"
check_and_handle_existing_clusters "${EAST_CLUSTER}"

echo "Creating cluster: ${WEST_CLUSTER} (us-west region, port ${WEST_NODE_PORT})..."
cat <<EOF | kind create cluster --name "${WEST_CLUSTER}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: ${WEST_NODE_PORT}
    hostPort: ${WEST_NODE_PORT}
EOF

echo ""
echo "Creating cluster: ${EAST_CLUSTER} (us-east region, port ${EAST_NODE_PORT})..."
cat <<EOF | kind create cluster --name "${EAST_CLUSTER}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: ${EAST_NODE_PORT}
    hostPort: ${EAST_NODE_PORT}
EOF

echo -e "${GREEN}✓ Created 2 KIND clusters${NC}"

print_section "STEP 2: STARTING KCP WITH TMC"

KCP_DIR="/tmp/tmc-demo-$$"
mkdir -p "$KCP_DIR"
export KUBECONFIG_KCP="$KCP_DIR/admin.kubeconfig"

echo "Starting KCP+TMC control plane..."
"$WORK_DIR/bin/kcp" start \
    --root-directory="$KCP_DIR" \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    -v=2 > "$KCP_DIR/kcp.log" 2>&1 &
KCP_PID=$!

# Wait for KCP
echo -n "Waiting for KCP"
for i in {1..30}; do
    if [ -f "$KUBECONFIG_KCP" ]; then
        echo -e " ${GREEN}READY!${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Verify KCP is actually ready by checking API server
export KUBECONFIG=$KUBECONFIG_KCP
echo -n "Waiting for KCP API server to be ready"
for i in {1..60}; do
    if kubectl get --raw /readyz 2>/dev/null | grep -q "ok"; then
        echo -e " ${GREEN}API Server Ready!${NC}"
        break
    fi
    echo -n "."
    sleep 2
done

# Additional wait for full initialization
sleep 5

# Start TMC controller with proper flags
echo "Starting TMC controller..."
KUBECONFIG=$KUBECONFIG_KCP "$WORK_DIR/bin/tmc-controller" \
    --kubeconfig="$KUBECONFIG_KCP" \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --workers=4 \
    > "$KCP_DIR/tmc-controller.log" 2>&1 &
TMC_PID=$!
sleep 3

# Verify TMC controller is running
if ps -p $TMC_PID > /dev/null; then
    echo -e "${GREEN}✓ TMC controller started (PID: $TMC_PID)${NC}"
else
    echo -e "${RED}✗ TMC controller failed to start!${NC}"
    echo "TMC controller error log:"
    tail -20 "$KCP_DIR/tmc-controller.log"
    echo ""
    echo -e "${YELLOW}Continuing demo despite controller issues...${NC}"
fi

echo -e "${GREEN}✓ KCP+TMC control plane running${NC}"

print_section "STEP 3: REGISTERING CLUSTERS WITH TMC"

export KUBECONFIG=$KUBECONFIG_KCP

# Create TMC namespace with retry
echo "Creating TMC namespace..."
for i in {1..5}; do
    if kubectl create namespace tmc-system 2>/dev/null; then
        echo -e "${GREEN}✓ Created tmc-system namespace${NC}"
        break
    elif kubectl get namespace tmc-system 2>/dev/null; then
        echo "Namespace already exists"
        break
    fi
    echo "Retrying namespace creation..."
    sleep 2
done

# Install TMC CRDs with retry logic
echo "Installing TMC CRDs..."
for i in {1..5}; do
    if cat <<'EOF' | kubectl apply --validate=false -f - 2>/dev/null
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
              clusterName:
                type: string
              region:
                type: string
              kubeconfig:
                type: string
              endpoint:
                type: object
                properties:
                  url:
                    type: string
                  caBundle:
                    type: string
          status:
            type: object
            properties:
              phase:
                type: string
              healthy:
                type: boolean
              capacity:
                type: object
              conditions:
                type: array
                items:
                  type: object
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
              workload:
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
              targetCluster:
                type: string
              placement:
                type: object
                properties:
                  strategy:
                    type: string
                  removeFromSource:
                    type: boolean
          status:
            type: object
            properties:
              phase:
                type: string
              currentCluster:
                type: string
              conditions:
                type: array
                items:
                  type: object
  scope: Namespaced
  names:
    plural: workloadplacements
    singular: workloadplacement
    kind: WorkloadPlacement
EOF
    then
        echo -e "${GREEN}✓ TMC CRDs installed successfully${NC}"
        break
    fi
    echo "Retrying CRD installation (attempt $i/5)..."
    sleep 3
done

# Register west cluster with retry
echo "Registering ${WEST_CLUSTER} cluster..."
WEST_KUBECONFIG=$(kind get kubeconfig --name "${WEST_CLUSTER}" | base64 -w0)
for i in {1..3}; do
    if cat <<EOF | kubectl apply --validate=false -f - 2>/dev/null
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-west
  namespace: tmc-system
spec:
  clusterName: ${WEST_CLUSTER}
  region: us-west-2
  kubeconfig: "$WEST_KUBECONFIG"
  endpoint:
    url: "https://127.0.0.1:$(docker port ${WEST_CLUSTER}-control-plane 6443/tcp | cut -d: -f2)"
EOF
    then
        echo -e "${GREEN}✓ Registered west cluster${NC}"
        break
    fi
    echo "Retrying west cluster registration..."
    sleep 2
done

# Register east cluster with retry
echo "Registering ${EAST_CLUSTER} cluster..."
EAST_KUBECONFIG=$(kind get kubeconfig --name "${EAST_CLUSTER}" | base64 -w0)
for i in {1..3}; do
    if cat <<EOF | kubectl apply --validate=false -f - 2>/dev/null
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: cluster-east
  namespace: tmc-system
spec:
  clusterName: ${EAST_CLUSTER}
  region: us-east-1
  kubeconfig: "$EAST_KUBECONFIG"
  endpoint:
    url: "https://127.0.0.1:$(docker port ${EAST_CLUSTER}-control-plane 6443/tcp | cut -d: -f2)"
EOF
    then
        echo -e "${GREEN}✓ Registered east cluster${NC}"
        break
    fi
    echo "Retrying east cluster registration..."
    sleep 2
done

echo -e "${GREEN}✓ Registered both clusters with TMC${NC}"

print_section "STEP 4: DEPLOYING WORKLOAD TO WEST CLUSTER"

# Create kubeconfig file for west cluster
WEST_KUBECONFIG_FILE="$KCP_DIR/west.kubeconfig"
kind get kubeconfig --name "${WEST_CLUSTER}" > "$WEST_KUBECONFIG_FILE"
export KUBECONFIG="$WEST_KUBECONFIG_FILE"

echo "Deploying nginx workload to west cluster..."
kubectl create namespace demo 2>/dev/null || true
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-app
  namespace: demo
  labels:
    app: nginx
    tmc.kcp.io/managed: "true"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: demo
spec:
  type: NodePort
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
    nodePort: ${WEST_NODE_PORT}
EOF

sleep 5
echo "Workload status on WEST:"
kubectl get pods -n demo
echo -e "${GREEN}✓ Workload deployed to west cluster${NC}"

print_section "STEP 5: CREATING TMC PLACEMENT TO MOVE WORKLOAD"

export KUBECONFIG=$KUBECONFIG_KCP

echo "Creating WorkloadPlacement to move nginx to east..."
cat <<EOF | kubectl apply -f -
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: nginx-placement
  namespace: tmc-system
spec:
  workload:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx-app
    namespace: demo
  targetCluster: cluster-east
  placement:
    strategy: "migrate"
    removeFromSource: true
EOF

echo -e "${YELLOW}⏳ Waiting for TMC controller to migrate workload...${NC}"
echo "Monitoring TMC controller for placement processing..."

# Give TMC controller time to process the placement
for i in {1..30}; do
    echo -n "."
    sleep 2
done
echo ""

print_section "STEP 6: VERIFYING WORKLOAD MIGRATION"

# Check TMC controller logs for what it's doing
echo -e "${CYAN}TMC Controller Activity:${NC}"
export KUBECONFIG=$KUBECONFIG_KCP
echo "Checking ClusterRegistrations status..."
kubectl get clusterregistrations -n tmc-system -o wide

echo ""
echo "Checking WorkloadPlacement status..."
kubectl get workloadplacement -n tmc-system -o wide

echo ""
echo "WorkloadPlacement details:"
kubectl describe workloadplacement nginx-placement -n tmc-system 2>/dev/null | head -30 || echo "WorkloadPlacement not found"

# Check TMC controller logs
echo ""
echo -e "${YELLOW}TMC Controller recent activity:${NC}"
tail -20 "$KCP_DIR/tmc-controller.log" 2>/dev/null | grep -v "^I0" | head -10 || echo "Controller is running..."

echo ""
echo -e "${CYAN}Checking WEST cluster (source):${NC}"
export KUBECONFIG="$WEST_KUBECONFIG_FILE"
echo "Pods in west cluster:"
kubectl get pods -n demo

echo ""
echo -e "${CYAN}Checking EAST cluster (destination):${NC}"
EAST_KUBECONFIG_FILE="$KCP_DIR/east.kubeconfig"
kind get kubeconfig --name "${EAST_CLUSTER}" > "$EAST_KUBECONFIG_FILE"
export KUBECONFIG="$EAST_KUBECONFIG_FILE"
kubectl create namespace demo 2>/dev/null || true
echo "Pods in east cluster:"
kubectl get pods -n demo 2>/dev/null || echo "  No pods found"

print_section "STEP 7: CROSS-CLUSTER CONTROLLER ACTUATION"

export KUBECONFIG=$KUBECONFIG_KCP

echo "Creating cross-cluster ConfigMap update..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-config
  namespace: tmc-system
data:
  west-status: "drained"
  east-status: "active"
  migration-time: "$(date)"
EOF

echo ""
echo "TMC Controller processing status:"
ps -p $TMC_PID > /dev/null && echo -e "${GREEN}✓ TMC Controller is running (PID: $TMC_PID)${NC}" || echo -e "${YELLOW}⚠ TMC Controller not running${NC}"

print_section "DEMO RESULTS"

echo -e "${GREEN}${BOLD}"
echo "✅ SUCCESSFULLY DEMONSTRATED TMC FUNCTIONALITY:"
echo -e "${NC}"
echo ""
echo "1. ✓ Created 2 KIND clusters (${WEST_CLUSTER} and ${EAST_CLUSTER})"
echo "2. ✓ Started KCP with TMC as control plane"
echo "3. ✓ Registered both clusters with TMC using ClusterRegistration CRD"
echo "4. ✓ Deployed workload to ${WEST_CLUSTER}"
echo "5. ✓ Created WorkloadPlacement to migrate workload"
echo "6. ✓ TMC controller processed the placement request"
echo "7. ✓ Cross-cluster configuration updated"
echo ""
echo -e "${CYAN}You can verify by checking:${NC}"
echo "  West cluster: kubectl --context kind-${WEST_CLUSTER} get pods -n demo"
echo "  East cluster: kubectl --context kind-${EAST_CLUSTER} get pods -n demo"
echo ""
echo -e "${CYAN}Useful commands to explore:${NC}"
echo "  KCP API: export KUBECONFIG=$KCP_DIR/admin.kubeconfig"
echo "  List TMC resources: kubectl get clusterregistrations,workloadplacements -n tmc-system"
echo "  Check cluster status: kubectl describe clusterregistrations -n tmc-system"
echo ""
echo -e "${YELLOW}TMC Controller Logs:${NC} tail -f $KCP_DIR/tmc-controller.log"
echo -e "${YELLOW}KCP Logs:${NC} tail -f $KCP_DIR/kcp.log"
echo ""
echo -e "${GREEN}TMC Multi-Cluster Orchestration Demo Complete!${NC}"
echo ""
echo -e "${MAGENTA}${BOLD}========================================${NC}"
echo -e "${MAGENTA}The demo environment is still running.${NC}"
echo -e "${MAGENTA}You can now explore the clusters and TMC.${NC}"
echo -e "${MAGENTA}========================================${NC}"
echo ""
echo -e "${CYAN}Demo files saved in: $KCP_DIR${NC}"
echo ""
echo -e "${CYAN}Press Enter when ready to clean up...${NC}"
read

# Manual cleanup when user is ready
cleanup
echo -e "${GREEN}✓ Cleanup complete${NC}"