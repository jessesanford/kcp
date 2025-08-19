#!/bin/bash

# TMC Controller Watcher - Monitor TMC controller activity in real-time

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

# Find the latest KCP root directory
KCP_ROOT=$(ls -dt /tmp/kcp-tmc-test-* 2>/dev/null | head -1)

if [ -z "$KCP_ROOT" ]; then
    echo -e "${RED}No running KCP instance found. Run tmc-test-harness.sh first!${NC}"
    exit 1
fi

KUBECONFIG="${KCP_ROOT}/admin.kubeconfig"
export KUBECONFIG

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}TMC Controller Activity Monitor${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Monitoring KCP root: $KCP_ROOT"
echo ""

# Function to show controller reconciliation
watch_reconciliation() {
    echo -e "${CYAN}=== Controller Reconciliation Activity ===${NC}"
    tail -f ${KCP_ROOT}/kcp.log | grep -E "tmc|TMC|placement|Placement|cluster.*registration|reconcil" --color=always &
    TAIL_PID=$!
    
    # Also watch TMC controller logs if available
    if [ -f "${KCP_ROOT}/tmc.log" ]; then
        tail -f ${KCP_ROOT}/tmc.log | sed 's/^/[TMC] /' &
        TMC_TAIL_PID=$!
    fi
    
    trap "kill $TAIL_PID $TMC_TAIL_PID 2>/dev/null" EXIT
    wait
}

# Function to show resource status
show_status() {
    while true; do
        clear
        echo -e "${BLUE}========================================${NC}"
        echo -e "${BLUE}TMC Resources Status - $(date +%H:%M:%S)${NC}"
        echo -e "${BLUE}========================================${NC}"
        echo ""
        
        echo -e "${YELLOW}ClusterRegistrations:${NC}"
        kubectl get clusterregistrations -A -o custom-columns=\
NAMESPACE:.metadata.namespace,\
NAME:.metadata.name,\
CLUSTER-ID:.spec.clusterID,\
REGION:.spec.region,\
PROVIDER:.spec.provider,\
CPU:.spec.capacity.cpu,\
MEMORY:.spec.capacity.memory,\
PHASE:.status.phase 2>/dev/null || echo "No ClusterRegistrations found"
        
        echo ""
        echo -e "${YELLOW}WorkloadPlacements:${NC}"
        kubectl get workloadplacements -A -o custom-columns=\
NAMESPACE:.metadata.namespace,\
NAME:.metadata.name,\
WORKLOAD:.spec.workloadRef.name,\
STRATEGY:.spec.placement.strategy,\
CLUSTERS:.status.selectedClusters,\
PHASE:.status.phase 2>/dev/null || echo "No WorkloadPlacements found"
        
        echo ""
        echo -e "${YELLOW}Recent Events (last 5):${NC}"
        kubectl get events -n tmc-demo --sort-by='.lastTimestamp' 2>/dev/null | tail -5
        
        echo ""
        echo -e "${CYAN}Press Ctrl+C to exit${NC}"
        sleep 5
    done
}

# Function to trigger reconciliation
trigger_reconciliation() {
    echo -e "${GREEN}Triggering reconciliation by updating resources...${NC}"
    
    # Add/update annotation to trigger reconciliation
    kubectl annotate clusterregistration -n tmc-demo --all \
        tmc.kcp.io/reconcile-trigger="$(date +%s)" --overwrite 2>/dev/null || true
    
    kubectl annotate workloadplacement -n tmc-demo --all \
        tmc.kcp.io/reconcile-trigger="$(date +%s)" --overwrite 2>/dev/null || true
    
    echo -e "${GREEN}✓ Reconciliation triggered${NC}"
}

# Function to simulate cluster events
simulate_events() {
    echo -e "${GREEN}Simulating cluster events...${NC}"
    
    # Update cluster capacity
    kubectl patch clusterregistration cluster-us-west-1 -n tmc-demo --type merge \
        -p '{"spec":{"capacity":{"cpu":"2000","memory":"16000Gi"}}}' 2>/dev/null || true
    
    # Add cluster condition
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    kubectl patch clusterregistration cluster-us-east-1 -n tmc-demo --type merge \
        -p "{\"status\":{\"conditions\":[{\"type\":\"Ready\",\"status\":\"True\",\"lastTransitionTime\":\"$TIMESTAMP\",\"reason\":\"ClusterHealthy\",\"message\":\"Cluster is healthy and accepting workloads\"}]}}" 2>/dev/null || true
    
    echo -e "${GREEN}✓ Events simulated${NC}"
}

# Menu
show_menu() {
    echo ""
    echo -e "${BLUE}Choose monitoring mode:${NC}"
    echo "1) Watch controller logs (live tail)"
    echo "2) Monitor resource status (updates every 5s)"
    echo "3) Trigger reconciliation"
    echo "4) Simulate cluster events"
    echo "5) Show controller metrics"
    echo "6) Test placement decisions"
    echo "7) Exit"
    echo ""
    read -p "Select option: " choice
    
    case $choice in
        1)
            watch_reconciliation
            ;;
        2)
            show_status
            ;;
        3)
            trigger_reconciliation
            show_menu
            ;;
        4)
            simulate_events
            show_menu
            ;;
        5)
            echo -e "${CYAN}Controller Metrics:${NC}"
            curl -s http://localhost:8080/metrics 2>/dev/null | grep -E "tmc|workqueue" | head -20 || echo "Metrics endpoint not available"
            show_menu
            ;;
        6)
            echo -e "${GREEN}Testing placement decisions...${NC}"
            # Create a new placement to trigger controller
            cat <<EOF | kubectl apply -f -
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: test-placement-$(date +%s)
  namespace: tmc-demo
spec:
  workloadRef:
    apiVersion: "apps/v1"
    kind: "Deployment"
    name: "test-app-$(date +%s)"
    namespace: "tmc-demo"
  placement:
    strategy: "RoundRobin"
EOF
            echo -e "${GREEN}✓ Test placement created${NC}"
            show_menu
            ;;
        7)
            echo "Exiting..."
            exit 0
            ;;
        *)
            echo "Invalid option"
            show_menu
            ;;
    esac
}

# Main
show_menu