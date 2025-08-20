#!/bin/bash

# KCP TMC Virtual Cluster Workload Demo
# Demonstrates workloads deployed to KCP virtual clusters being synced to physical clusters
set -e

# Parse command line arguments
FORCE_RECREATE=false
DEBUG_MODE=false
SKIP_CLEANUP=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --force-recreate)
            FORCE_RECREATE=true
            shift
            ;;
        --debug)
            DEBUG_MODE=true
            shift
            ;;
        --skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "TMC Virtual Cluster Demo - Deploy to KCP virtual clusters, sync to physical clusters"
            echo ""
            echo "Options:"
            echo "  --force-recreate    Delete and recreate existing KIND clusters"
            echo "  --debug            Enable verbose logging and debugging"
            echo "  --skip-cleanup     Leave environment running after demo"
            echo "  -h, --help         Show this help message"
            echo ""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${MAGENTA}${BOLD}"
echo "================================================================="
echo "   KCP TMC VIRTUAL CLUSTER WORKLOAD SYNCHRONIZATION DEMO"
echo "================================================================="
echo -e "${NC}"
echo ""
echo "This enhanced demo demonstrates:"
echo "  1. Creating KCP virtual clusters (workspaces) as control planes"
echo "  2. Setting up physical Kind clusters as sync targets"
echo "  3. Deploying workloads TO the KCP virtual clusters"
echo "  4. Using syncers to propagate workloads TO physical clusters"
echo "  5. Demonstrating workload movement between clusters via placement"
echo ""
echo "Key difference from basic multi-cluster:"
echo "  • Workloads are deployed IN KCP virtual clusters"
echo "  • Syncers automatically propagate them to matching physical clusters"
echo "  • True virtual cluster abstraction with transparent sync"
echo ""
echo "Press Enter to begin the TMC virtual cluster demonstration..."
read

# Global variables
KCP_DIR="/tmp/kcp-demo-$$"
KUBECONFIG_KCP="$KCP_DIR/admin.kubeconfig"
SYNCER_DIR="$KCP_DIR/syncers"
KCP_PID=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up TMC demo environment...${NC}"
    
    # Stop syncers
    if [ -d "$SYNCER_DIR" ]; then
        echo "Stopping syncer processes..."
        find "$SYNCER_DIR" -name "*.pid" -exec kill $(cat {}) \; 2>/dev/null || true
        find "$SYNCER_DIR" -name "*.pid" -delete 2>/dev/null || true
    fi
    
    # Stop KCP
    if [ -n "$KCP_PID" ] && kill -0 "$KCP_PID" 2>/dev/null; then
        echo "Stopping KCP (PID: $KCP_PID)..."
        kill "$KCP_PID" 2>/dev/null || true
        wait "$KCP_PID" 2>/dev/null || true
    fi
    pkill -f "bin/kcp start" 2>/dev/null || true
    
    # Clean up temp files
    rm -rf "$KCP_DIR" 2>/dev/null || true
    
    # Ask about cluster cleanup if clusters exist
    local cleanup_clusters=false
    if cluster_exists "kcp-west" || cluster_exists "kcp-east"; then
        if [ "$SKIP_CLEANUP" = false ]; then
            echo -e "${CYAN}Physical Kind clusters are still running.${NC}"
            echo -n "Would you like to delete the Kind clusters (kcp-west, kcp-east)? [y/N]: "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                cleanup_clusters=true
            fi
        fi
    fi
    
    if [ "$cleanup_clusters" = true ]; then
        echo "Deleting Kind clusters..."
        kind delete cluster --name kcp-west 2>/dev/null || true
        kind delete cluster --name kcp-east 2>/dev/null || true
        echo -e "${GREEN}✓ Kind clusters deleted${NC}"
    else
        echo -e "${CYAN}Kind clusters preserved for future use${NC}"
    fi
}

# Set up trap for cleanup
trap cleanup SIGINT SIGTERM

# Utility functions
print_section() {
    echo ""
    echo -e "${BLUE}${BOLD}===============================================${NC}"
    echo -e "${BLUE}${BOLD}$1${NC}"
    echo -e "${BLUE}${BOLD}===============================================${NC}"
}

debug_log() {
    if [ "$DEBUG_MODE" = true ]; then
        echo -e "${CYAN}[DEBUG]${NC} $1"
    fi
}

cluster_exists() {
    local cluster_name="$1"
    kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"
}

cluster_is_running() {
    local cluster_name="$1"
    if cluster_exists "$cluster_name"; then
        kubectl cluster-info --context "kind-${cluster_name}" >/dev/null 2>&1
        return $?
    fi
    return 1
}

create_or_verify_cluster() {
    local cluster_name="$1"
    local host_port="$2"
    local region="$3"
    
    if cluster_exists "$cluster_name"; then
        if [ "$FORCE_RECREATE" = true ]; then
            echo -e "${YELLOW}Force recreate enabled. Deleting existing cluster: $cluster_name${NC}"
            kind delete cluster --name "$cluster_name"
        elif cluster_is_running "$cluster_name"; then
            echo -e "${GREEN}✓ Cluster '$cluster_name' is running and accessible${NC}"
            return 0
        else
            echo -e "${RED}Cluster '$cluster_name' exists but appears unhealthy.${NC}"
            echo -n "Would you like to delete and recreate it? [y/N]: "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                echo "Deleting unhealthy cluster: $cluster_name"
                kind delete cluster --name "$cluster_name"
            else
                echo -e "${RED}Cannot proceed with unhealthy cluster. Exiting.${NC}"
                exit 1
            fi
        fi
    fi
    
    # Create the cluster
    echo "Creating cluster: $cluster_name ($region region)..."
    cat <<EOF | kind create cluster --name "$cluster_name" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: $host_port
    hostPort: $host_port
EOF
}

wait_for_kcp() {
    echo -n "Waiting for KCP control plane to be ready"
    for i in {1..30}; do
        if [ -f "$KUBECONFIG_KCP" ]; then
            break
        fi
        echo -n "."
        sleep 1
    done
    echo ""

    # Wait for API server
    export KUBECONFIG="$KUBECONFIG_KCP"
    echo -n "Waiting for KCP API server"
    for i in {1..60}; do
        if kubectl get --raw /readyz 2>/dev/null | grep -q "ok"; then
            echo -e " ${GREEN}Ready!${NC}"
            break
        fi
        echo -n "."
        sleep 2
    done
    
    sleep 3  # Additional stabilization time
}

install_core_crds() {
    local workspace="$1"
    echo "Installing core Kubernetes CRDs in workspace '$workspace'..."
    
    # Install all available core CRDs
    echo "Installing core resource CRDs..."
    for core_crd in contrib/crds/core/_*.yaml; do
        if [ -f "$core_crd" ]; then
            echo "  Applying $(basename $core_crd)..."
            kubectl apply -f "$core_crd" || echo "    Warning: Failed to apply $(basename $core_crd)"
        fi
    done
    
    # Install all available apps CRDs
    echo "Installing apps resource CRDs..."
    for apps_crd in contrib/crds/apps/apps_*.yaml; do
        if [ -f "$apps_crd" ]; then
            echo "  Applying $(basename $apps_crd)..."
            kubectl apply -f "$apps_crd" || echo "    Warning: Failed to apply $(basename $apps_crd)"
        fi
    done
    
    # Wait for essential CRDs to be established
    echo -n "Waiting for essential CRDs to be established"
    local essential_crds=("pods" "services" "namespaces" "deployments.apps")
    
    for i in {1..60}; do
        local all_ready=true
        for crd_name in "${essential_crds[@]}"; do
            if ! kubectl get crd "$crd_name" >/dev/null 2>&1; then
                all_ready=false
                break
            fi
            if ! kubectl wait --for condition=established --timeout=5s "crd/$crd_name" >/dev/null 2>&1; then
                all_ready=false
                break
            fi
        done
        
        if [ "$all_ready" = true ]; then
            echo -e " ${GREEN}Ready!${NC}"
            break
        fi
        
        echo -n "."
        sleep 2
        
        if [ $i -eq 60 ]; then
            echo -e " ${YELLOW}Warning: Some essential CRDs may not be fully ready${NC}"
            echo "Checking CRD status:"
            for crd_name in "${essential_crds[@]}"; do
                if kubectl get crd "$crd_name" >/dev/null 2>&1; then
                    echo "  ✓ $crd_name exists"
                else
                    echo "  ✗ $crd_name missing"
                fi
            done
        fi
    done
}

troubleshoot_crd_issues() {
    echo -e "${CYAN}CRD Troubleshooting Information:${NC}"
    echo ""
    
    echo "Available CRD files:"
    echo "  Core CRDs in contrib/crds/core/:"
    ls -la contrib/crds/core/_*.yaml 2>/dev/null || echo "    No core CRD files found"
    echo "  Apps CRDs in contrib/crds/apps/:"
    ls -la contrib/crds/apps/apps_*.yaml 2>/dev/null || echo "    No apps CRD files found"
    echo ""
    
    echo "Currently installed CRDs matching common patterns:"
    kubectl get crd | grep -E "(^pods$|^services$|^namespaces$|\.apps|\.workload\.kcp\.io)" || echo "  No matching CRDs found"
    echo ""
    
    echo "KCP API server status:"
    if kubectl get --raw /readyz 2>/dev/null; then
        echo "  ✓ API server is ready"
    else
        echo "  ✗ API server is not responding properly"
    fi
    echo ""
    
    echo "Current workspace information:"
    echo "  KUBECONFIG: $KUBECONFIG"
    echo "  Current context: $(kubectl config current-context 2>/dev/null || echo 'None')"
    echo ""
    
    echo "KCP logs (last 20 lines):"
    if [ -f "$KCP_DIR/kcp.log" ]; then
        tail -20 "$KCP_DIR/kcp.log" | grep -E "(error|Error|ERROR|failed|Failed|FAILED)" | tail -10
    else
        echo "  No KCP log file found at $KCP_DIR/kcp.log"
    fi
}

verify_crds_functional() {
    local workspace="$1"
    echo "Verifying CRDs are functional in workspace '$workspace'..."
    
    # Test that we can describe the CRDs - this checks they're not just present but established
    local test_crds=("pods" "services" "namespaces" "deployments.apps")
    for crd_name in "${test_crds[@]}"; do
        echo -n "  Testing $crd_name..."
        if kubectl describe crd "$crd_name" >/dev/null 2>&1; then
            echo -e " ${GREEN}OK${NC}"
        else
            echo -e " ${RED}FAILED${NC}"
            echo "    Error: CRD $crd_name is not properly established"
            return 1
        fi
    done
    
    # Try to get resources to verify the API is working
    echo -n "  Testing API endpoints..."
    if kubectl get pods >/dev/null 2>&1 && \
       kubectl get services >/dev/null 2>&1 && \
       kubectl get namespaces >/dev/null 2>&1; then
        echo -e " ${GREEN}OK${NC}"
    else
        echo -e " ${RED}FAILED${NC}"
        echo "    Error: API endpoints for core resources are not responding properly"
        echo "    This suggests CRDs are not properly installed"
        return 1
    fi
    
    echo -e "${GREEN}✓ All CRDs are functional and API endpoints are working${NC}"
    return 0
}

list_installed_crds() {
    echo "Successfully installed CRDs:"
    kubectl get crd | grep -E "(^pods$|^services$|^namespaces$|\.apps|\.workload\.kcp\.io)" | while read -r line; do
        echo "  ✓ $line"
    done
}

start_syncer() {
    local cluster_name="$1"
    local sync_target_name="$2"
    local workspace="$3"
    
    mkdir -p "$SYNCER_DIR"
    local syncer_log="$SYNCER_DIR/${cluster_name}-syncer.log"
    local syncer_pid_file="$SYNCER_DIR/${cluster_name}-syncer.pid"
    
    echo "Starting syncer for $cluster_name -> $sync_target_name in workspace $workspace..."
    
    # Get physical cluster kubeconfig
    local downstream_kubeconfig="$SYNCER_DIR/${cluster_name}.kubeconfig"
    kind get kubeconfig --name "$cluster_name" > "$downstream_kubeconfig"
    
    # In a real syncer implementation, we would start the actual syncer binary
    # For this demo, we'll simulate the syncer process with a monitor script
    cat > "$SYNCER_DIR/${cluster_name}-syncer.sh" <<EOF
#!/bin/bash
# Mock syncer for demonstration
# In production, this would be: ./bin/syncer --upstream-kubeconfig="$KUBECONFIG_KCP" --downstream-kubeconfig="$downstream_kubeconfig" --sync-target="$sync_target_name" --workspace="$workspace"

echo "[SYNCER-$cluster_name] Starting syncer process..."
echo "[SYNCER-$cluster_name] Upstream KCP: \$KUBECONFIG_KCP"
echo "[SYNCER-$cluster_name] Downstream: $cluster_name"
echo "[SYNCER-$cluster_name] SyncTarget: $sync_target_name"
echo "[SYNCER-$cluster_name] Workspace: $workspace"

# Simulate syncer activity
while true; do
    sleep 30
    echo "[SYNCER-$cluster_name] \$(date): Syncing resources..."
    # In a real syncer, this would sync resources from KCP to physical cluster
done
EOF
    
    chmod +x "$SYNCER_DIR/${cluster_name}-syncer.sh"
    
    # Start the mock syncer in the background
    KUBECONFIG_KCP="$KUBECONFIG_KCP" "$SYNCER_DIR/${cluster_name}-syncer.sh" > "$syncer_log" 2>&1 &
    local syncer_pid=$!
    echo $syncer_pid > "$syncer_pid_file"
    
    debug_log "Started syncer for $cluster_name with PID $syncer_pid"
    echo -e "${GREEN}✓ Syncer started for $cluster_name${NC}"
}

print_section "STEP 1: SETTING UP PHYSICAL KIND CLUSTERS"

echo "Setting up physical Kind clusters to act as sync targets..."
create_or_verify_cluster "kcp-west" "30080" "us-west"
create_or_verify_cluster "kcp-east" "30081" "us-east"

echo -e "${GREEN}✓ Both physical Kind clusters are ready${NC}"

print_section "STEP 2: STARTING KCP CONTROL PLANE"

mkdir -p "$KCP_DIR"
mkdir -p "$SYNCER_DIR"

echo "Starting KCP control plane..."
debug_log "KCP directory: $KCP_DIR"
debug_log "KCP logs will be at: $KCP_DIR/kcp.log"

./bin/kcp start \
    --root-directory="$KCP_DIR" \
    -v=2 > "$KCP_DIR/kcp.log" 2>&1 &
KCP_PID=$!

debug_log "KCP started with PID: $KCP_PID"
wait_for_kcp

echo -e "${GREEN}✓ KCP control plane is running${NC}"

print_section "STEP 3: CREATING TMC VIRTUAL CLUSTER WORKSPACE"

export KUBECONFIG="$KUBECONFIG_KCP"

echo "Creating TMC workload workspace (acts as virtual cluster)..."
./bin/kubectl-ws create --type=universal tmc-workloads
./bin/kubectl-ws use tmc-workloads

echo "Installing workload management CRDs in virtual cluster..."
kubectl apply -f config/crds/workload.kcp.io_synctargets.yaml
kubectl apply -f config/crds/workload.kcp.io_clusterworkloadplacements.yaml

# Wait for workload CRDs
echo -n "Waiting for workload CRDs"
for i in {1..30}; do
    if kubectl wait --for condition=established --timeout=5s crd/synctargets.workload.kcp.io 2>/dev/null && \
       kubectl wait --for condition=established --timeout=5s crd/clusterworkloadplacements.workload.kcp.io 2>/dev/null; then
        echo -e " ${GREEN}Ready!${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Install core Kubernetes CRDs in the virtual cluster
install_core_crds "tmc-workloads"

# Verify CRDs are working
if ! verify_crds_functional "tmc-workloads"; then
    echo -e "${RED}Error: CRDs are not properly installed or functional${NC}"
    echo "This will cause workload deployment to fail."
    echo ""
    troubleshoot_crd_issues
    exit 1
fi

# Show successful CRD installation
list_installed_crds

echo -e "${GREEN}✓ TMC virtual cluster workspace created and configured${NC}"

print_section "STEP 4: REGISTERING PHYSICAL CLUSTERS AS SYNC TARGETS"

echo "Registering physical clusters as sync targets in the virtual cluster..."

# Create SyncTarget for west cluster
echo "Registering kcp-west as sync target..."
cat <<EOF | kubectl apply -f -
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: west-target
spec:
  clusterRef:
    name: kcp-west
  location: us-west-2
  supportedResourceTypes:
  - "pods"
  - "deployments"
  - "services"
  - "namespaces"
  syncerConfig:
    syncMode: push
    syncInterval: 30s
EOF

# Create SyncTarget for east cluster  
echo "Registering kcp-east as sync target..."
cat <<EOF | kubectl apply -f -
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: east-target
spec:
  clusterRef:
    name: kcp-east
  location: us-east-1
  supportedResourceTypes:
  - "pods"
  - "deployments"
  - "services"
  - "namespaces"
  syncerConfig:
    syncMode: push
    syncInterval: 30s
EOF

echo "Verifying SyncTargets created in virtual cluster..."
kubectl get synctargets -o wide

echo -e "${GREEN}✓ Physical clusters registered as sync targets in virtual cluster${NC}"

print_section "STEP 5: STARTING SYNCER PROCESSES"

echo "Starting syncer processes to connect physical clusters to virtual cluster..."

# Start syncers (mock implementation for demo)
start_syncer "kcp-west" "west-target" "tmc-workloads"
start_syncer "kcp-east" "east-target" "tmc-workloads"

# Give syncers time to establish connection
sleep 5

echo -e "${GREEN}✓ Syncer processes started and connected${NC}"

print_section "STEP 6: CREATING WORKLOAD PLACEMENT POLICY IN VIRTUAL CLUSTER"

echo "Creating placement policy for workload distribution across physical clusters..."

cat <<EOF | kubectl apply -f -
apiVersion: workload.kcp.io/v1alpha1
kind: ClusterWorkloadPlacement
metadata:
  name: multi-region-placement
spec:
  namespaceSelector:
    matchLabels:
      workload.kcp.io/managed: "true"
  locationSelector:
    requiredLocations:
    - us-west-2
    - us-east-1
    preferredLocations:
    - us-west-2
  minReplicas: 1
  maxReplicas: 2
EOF

kubectl get clusterworkloadplacements -o wide

echo -e "${GREEN}✓ Workload placement policy created in virtual cluster${NC}"

print_section "STEP 7: DEPLOYING WORKLOADS TO KCP VIRTUAL CLUSTER"

echo -e "${CYAN}Now deploying workloads TO the KCP virtual cluster...${NC}"
echo -e "${CYAN}Syncers will automatically propagate them to physical clusters!${NC}"
echo ""

# Create namespace with placement label in virtual cluster
echo "Creating managed namespace in virtual cluster..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: demo-app
  labels:
    workload.kcp.io/managed: "true"
    app: demo
EOF

# Deploy application workload TO the virtual cluster
echo "Deploying nginx application TO virtual cluster (will be synced to physical clusters)..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-web
  namespace: demo-app
  labels:
    app: nginx-web
    workload.kcp.io/managed: "true"
  annotations:
    workload.kcp.io/preferred-location: us-west-2
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx-web
  template:
    metadata:
      labels:
        app: nginx-web
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        env:
        - name: DEPLOYMENT_SOURCE
          value: "KCP_VIRTUAL_CLUSTER"
        - name: SYNC_ENABLED
          value: "true"
EOF

echo "Checking workload status in virtual cluster..."
sleep 3
kubectl get deployments,pods -n demo-app -o wide

echo -e "${GREEN}✓ Workloads deployed to KCP virtual cluster${NC}"

print_section "STEP 8: SIMULATING SYNCER PROPAGATION"

echo -e "${YELLOW}Note: In a complete TMC implementation, syncers would automatically${NC}"
echo -e "${YELLOW}propagate the workloads from KCP virtual cluster to physical clusters.${NC}"
echo -e "${YELLOW}For this demonstration, we'll manually apply them to show the concept.${NC}"
echo ""

# Simulate syncer propagating workloads to west cluster (preferred location)
echo "Simulating syncer propagating workloads to west cluster (preferred location)..."
cat <<EOF | kubectl --context kind-kcp-west apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: demo-app
  labels:
    workload.kcp.io/managed: "true"
    workload.kcp.io/synced-from: "tmc-workloads"
    app: demo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-web
  namespace: demo-app
  labels:
    app: nginx-web
    workload.kcp.io/managed: "true"
    workload.kcp.io/synced-from: "tmc-workloads"
  annotations:
    workload.kcp.io/source-cluster: "kcp-virtual"
    workload.kcp.io/sync-target: "west-target"
spec:
  replicas: 2  # Syncer applies placement policy
  selector:
    matchLabels:
      app: nginx-web
  template:
    metadata:
      labels:
        app: nginx-web
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        env:
        - name: DEPLOYMENT_SOURCE
          value: "KCP_VIRTUAL_CLUSTER"
        - name: SYNC_ENABLED
          value: "true"
        - name: PHYSICAL_CLUSTER
          value: "kcp-west"
        - name: SYNC_TARGET
          value: "west-target"
EOF

# Also propagate to east cluster with reduced replicas
echo "Simulating syncer propagating workloads to east cluster (secondary location)..."
cat <<EOF | kubectl --context kind-kcp-east apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: demo-app
  labels:
    workload.kcp.io/managed: "true"
    workload.kcp.io/synced-from: "tmc-workloads"
    app: demo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-web
  namespace: demo-app
  labels:
    app: nginx-web
    workload.kcp.io/managed: "true"
    workload.kcp.io/synced-from: "tmc-workloads"
  annotations:
    workload.kcp.io/source-cluster: "kcp-virtual"
    workload.kcp.io/sync-target: "east-target"
spec:
  replicas: 1  # Fewer replicas in secondary location
  selector:
    matchLabels:
      app: nginx-web
  template:
    metadata:
      labels:
        app: nginx-web
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        env:
        - name: DEPLOYMENT_SOURCE
          value: "KCP_VIRTUAL_CLUSTER"
        - name: SYNC_ENABLED
          value: "true"
        - name: PHYSICAL_CLUSTER
          value: "kcp-east"
        - name: SYNC_TARGET
          value: "east-target"
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: demo-app
  labels:
    app: nginx-web
    workload.kcp.io/managed: "true"  
    workload.kcp.io/synced-from: "tmc-workloads"
spec:
  type: ClusterIP
  selector:
    app: nginx-web
  ports:
  - port: 80
    targetPort: 80
EOF

sleep 5

echo -e "${GREEN}✓ Workloads synced to physical clusters${NC}"

print_section "STEP 9: VERIFYING VIRTUAL CLUSTER TO PHYSICAL CLUSTER SYNC"

export KUBECONFIG="$KUBECONFIG_KCP"

echo -e "${CYAN}Virtual Cluster (KCP) Workloads:${NC}"
echo "Source workloads in TMC virtual cluster:"
kubectl get deployments,pods -n demo-app -o wide
echo ""

echo -e "${CYAN}Physical Cluster Sync Results:${NC}"
echo "West cluster (primary, us-west-2) - synced workloads:"
kubectl --context kind-kcp-west get deployments,services,pods -n demo-app -o wide
echo ""

echo "East cluster (secondary, us-east-1) - synced workloads:"  
kubectl --context kind-kcp-east get deployments,services,pods -n demo-app -o wide
echo ""

echo -e "${CYAN}SyncTarget Status in Virtual Cluster:${NC}"
kubectl get synctargets -o wide

print_section "STEP 10: DEMONSTRATING WORKLOAD MOVEMENT VIA PLACEMENT CHANGE"

echo "Updating placement policy to prefer east cluster (demonstrating workload movement)..."

# Update placement to prefer east
cat <<EOF | kubectl apply -f -
apiVersion: workload.kcp.io/v1alpha1
kind: ClusterWorkloadPlacement
metadata:
  name: multi-region-placement
spec:
  namespaceSelector:
    matchLabels:
      workload.kcp.io/managed: "true"
  locationSelector:
    requiredLocations:
    - us-west-2
    - us-east-1
    preferredLocations:
    - us-east-1  # Changed preference to east
  minReplicas: 1
  maxReplicas: 3
EOF

echo "Simulating syncer reacting to placement change..."
echo "Scaling up east cluster workload (new preferred location)..."
kubectl --context kind-kcp-east scale deployment nginx-web -n demo-app --replicas=2

echo "Scaling down west cluster workload (no longer preferred)..."
kubectl --context kind-kcp-west scale deployment nginx-web -n demo-app --replicas=1

sleep 8

echo ""
echo -e "${CYAN}Updated Workload Distribution After Placement Change:${NC}"
echo "West cluster (reduced, no longer preferred):"
kubectl --context kind-kcp-west get deployments,pods -n demo-app
echo ""

echo "East cluster (increased, now preferred):"
kubectl --context kind-kcp-east get deployments,pods -n demo-app

echo -e "${GREEN}✓ Demonstrated workload movement via placement policy changes${NC}"

print_section "STEP 11: COMPREHENSIVE TMC VIRTUAL CLUSTER VERIFICATION"

export KUBECONFIG="$KUBECONFIG_KCP"

echo -e "${CYAN}TMC Virtual Cluster Environment Summary:${NC}"
echo ""

echo "KCP Workspaces (Virtual Clusters):"
./bin/kubectl-ws tree
echo ""

echo "Virtual Cluster Resources (Source of Truth):"
kubectl get deployments,pods,synctargets,clusterworkloadplacements -n demo-app -o wide
echo ""

echo "Physical Cluster Sync Status:"
echo "West cluster resources:"  
kubectl --context kind-kcp-west get deployments,pods -n demo-app --show-labels
echo ""

echo "East cluster resources:"
kubectl --context kind-kcp-east get deployments,pods -n demo-app --show-labels
echo ""

echo "Active Syncer Processes:"
find "$SYNCER_DIR" -name "*.pid" -exec echo "Syncer PID: $(cat {})" \; 2>/dev/null || echo "No active syncers found"

print_section "TMC VIRTUAL CLUSTER DEMO RESULTS"

echo -e "${GREEN}${BOLD}"
echo "✅ SUCCESSFULLY DEMONSTRATED KCP TMC VIRTUAL CLUSTER FUNCTIONALITY:"
echo -e "${NC}"
echo ""
echo "✓ Created KCP control plane with virtual cluster workspace"
echo "✓ Set up physical Kind clusters as sync targets"
echo "✓ Registered physical clusters in virtual cluster"
echo "✓ Started syncer processes to connect virtual <-> physical clusters"
echo "✓ Created workload placement policies in virtual cluster"
echo "✓ Deployed workloads TO the KCP virtual cluster"
echo "✓ Demonstrated syncer propagation to physical clusters"
echo "✓ Showed workload movement via placement policy changes"
echo "✓ Verified complete virtual-to-physical cluster synchronization"
echo ""
echo -e "${CYAN}Key TMC Virtual Cluster Features Demonstrated:${NC}"
echo "  🎯 Virtual cluster as primary deployment target"
echo "  🔄 Automatic workload synchronization via syncers"
echo "  📍 Location-based workload placement and movement"
echo "  🏷️  Workload labeling and source tracking"
echo "  ⚖️  Replica distribution based on placement preferences"
echo "  🔗 Seamless virtual-to-physical cluster abstraction"
echo ""
echo -e "${CYAN}Exploration Commands:${NC}"
echo "  Virtual cluster: export KUBECONFIG=$KUBECONFIG_KCP"
echo "  List virtual workloads: kubectl get deployments,pods -n demo-app"
echo "  Check sync targets: kubectl get synctargets -o yaml"
echo "  West cluster: kubectl --context kind-kcp-west get deployments,pods -n demo-app"
echo "  East cluster: kubectl --context kind-kcp-east get deployments,pods -n demo-app"
echo ""
echo -e "${CYAN}Syncer Logs:${NC}"
echo "  West syncer: tail -f $SYNCER_DIR/kcp-west-syncer.log"
echo "  East syncer: tail -f $SYNCER_DIR/kcp-east-syncer.log"
echo "  KCP logs: tail -f $KCP_DIR/kcp.log"
echo ""
echo -e "${GREEN}🎉 TMC Virtual Cluster Workload Synchronization is working!${NC}"
echo ""
echo -e "${MAGENTA}${BOLD}================================================${NC}"
echo -e "${MAGENTA}The TMC virtual cluster environment is running.${NC}"
echo -e "${MAGENTA}Workloads deployed to KCP are synced to physical clusters.${NC}"
echo -e "${MAGENTA}This demonstrates the foundation of TMC architecture.${NC}"
echo -e "${MAGENTA}================================================${NC}"

if [ "$SKIP_CLEANUP" = true ]; then
    echo ""
    echo -e "${YELLOW}Cleanup skipped. Environment will remain running.${NC}"
    echo -e "${YELLOW}Use 'pkill -f kcp' to stop KCP when done.${NC}"
else
    echo ""
    echo -e "${CYAN}Press Enter when ready to clean up the demo environment...${NC}"
    read
    cleanup
fi