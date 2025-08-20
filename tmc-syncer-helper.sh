#!/bin/bash

# KCP TMC Syncer Helper Script
# Provides utilities for managing syncer processes and demonstrating sync functionality

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SYNCER_DIR="/tmp/kcp-demo-*/syncers"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

show_help() {
    cat << EOF
KCP TMC Syncer Helper Script

USAGE:
    $(basename "$0") [COMMAND] [OPTIONS]

COMMANDS:
    status          Show status of all syncer processes
    logs            Show logs from syncer processes  
    start           Start a syncer for a specific cluster
    stop            Stop syncer processes
    sync-workload   Manually sync a workload to demonstrate the process
    verify          Verify sync target connectivity

OPTIONS:
    --cluster NAME  Specify cluster name (kcp-west, kcp-east)
    --help         Show this help message

EXAMPLES:
    # Show syncer status
    $(basename "$0") status
    
    # View syncer logs
    $(basename "$0") logs --cluster kcp-west
    
    # Manually sync a workload
    $(basename "$0") sync-workload --cluster kcp-west

EOF
}

show_syncer_status() {
    echo -e "${CYAN}TMC Syncer Process Status:${NC}"
    echo ""
    
    local syncer_dirs=$(find /tmp -maxdepth 1 -name "kcp-demo-*" -type d 2>/dev/null)
    if [ -z "$syncer_dirs" ]; then
        echo -e "${YELLOW}No active KCP demo environments found.${NC}"
        return 1
    fi
    
    for demo_dir in $syncer_dirs; do
        local syncer_dir="$demo_dir/syncers"
        if [ -d "$syncer_dir" ]; then
            echo "Demo environment: $demo_dir"
            echo "Syncer directory: $syncer_dir"
            echo ""
            
            for pid_file in "$syncer_dir"/*.pid; do
                if [ -f "$pid_file" ]; then
                    local pid=$(cat "$pid_file")
                    local syncer_name=$(basename "$pid_file" .pid)
                    
                    if kill -0 "$pid" 2>/dev/null; then
                        echo -e "  ${GREEN}✓ $syncer_name: Running (PID: $pid)${NC}"
                    else
                        echo -e "  ${RED}✗ $syncer_name: Not running (stale PID: $pid)${NC}"
                    fi
                fi
            done
            echo ""
        fi
    done
}

show_syncer_logs() {
    local cluster_name="$1"
    
    local syncer_dirs=$(find /tmp -maxdepth 1 -name "kcp-demo-*" -type d 2>/dev/null)
    if [ -z "$syncer_dirs" ]; then
        echo -e "${YELLOW}No active KCP demo environments found.${NC}"
        return 1
    fi
    
    for demo_dir in $syncer_dirs; do
        local syncer_dir="$demo_dir/syncers"
        if [ -d "$syncer_dir" ]; then
            if [ -n "$cluster_name" ]; then
                local log_file="$syncer_dir/${cluster_name}-syncer.log"
                if [ -f "$log_file" ]; then
                    echo -e "${CYAN}Logs for $cluster_name syncer:${NC}"
                    echo "----------------------------------------"
                    tail -n 20 "$log_file"
                    echo ""
                    echo -e "${BLUE}To follow logs: tail -f $log_file${NC}"
                else
                    echo -e "${RED}Log file not found: $log_file${NC}"
                fi
            else
                echo -e "${CYAN}Available syncer logs:${NC}"
                for log_file in "$syncer_dir"/*-syncer.log; do
                    if [ -f "$log_file" ]; then
                        local syncer_name=$(basename "$log_file" -syncer.log)
                        echo "  $syncer_name: $log_file"
                    fi
                done
                echo ""
                echo "Use --cluster to view specific logs"
            fi
            break
        fi
    done
}

manually_sync_workload() {
    local cluster_name="$1"
    
    if [ -z "$cluster_name" ]; then
        echo -e "${RED}Error: Cluster name required. Use --cluster kcp-west or --cluster kcp-east${NC}"
        return 1
    fi
    
    echo -e "${CYAN}Manually syncing workload to $cluster_name...${NC}"
    echo ""
    
    # Check if cluster is accessible
    if ! kubectl cluster-info --context "kind-$cluster_name" >/dev/null 2>&1; then
        echo -e "${RED}Error: Cluster kind-$cluster_name is not accessible${NC}"
        return 1
    fi
    
    # Example workload sync
    echo "Creating sample workload in $cluster_name to demonstrate sync:"
    cat <<EOF | kubectl --context "kind-$cluster_name" apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: sync-demo
  labels:
    workload.kcp.io/synced-from: "tmc-virtual-cluster"
    workload.kcp.io/sync-target: "${cluster_name}-target"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sync-demo-app
  namespace: sync-demo
  labels:
    app: sync-demo
    workload.kcp.io/synced-from: "tmc-virtual-cluster"
  annotations:
    workload.kcp.io/sync-timestamp: "$(date -Iseconds)"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sync-demo
  template:
    metadata:
      labels:
        app: sync-demo
        workload.kcp.io/synced-from: "tmc-virtual-cluster"
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        env:
        - name: SYNCED_TO_CLUSTER
          value: "$cluster_name"
        - name: SYNC_DEMO
          value: "true"
EOF
    
    echo ""
    echo -e "${GREEN}✓ Workload synced to $cluster_name${NC}"
    echo ""
    echo "Verifying sync result:"
    kubectl --context "kind-$cluster_name" get deployments,pods -n sync-demo -o wide
}

verify_sync_targets() {
    echo -e "${CYAN}Verifying Sync Target Connectivity:${NC}"
    echo ""
    
    # Find KCP kubeconfig
    local kcp_kubeconfigs=$(find /tmp -path "*/kcp-demo-*/admin.kubeconfig" 2>/dev/null)
    if [ -z "$kcp_kubeconfigs" ]; then
        echo -e "${YELLOW}No KCP environments found.${NC}"
        return 1
    fi
    
    for kubeconfig in $kcp_kubeconfigs; do
        echo "Checking KCP environment: $(dirname "$kubeconfig")"
        
        export KUBECONFIG="$kubeconfig"
        
        # Check if we can access KCP
        if ! kubectl get --raw /readyz >/dev/null 2>&1; then
            echo -e "${RED}✗ KCP not accessible at $kubeconfig${NC}"
            continue
        fi
        
        echo -e "${GREEN}✓ KCP is accessible${NC}"
        
        # Check sync targets
        if kubectl get synctargets >/dev/null 2>&1; then
            echo ""
            echo "Registered Sync Targets:"
            kubectl get synctargets -o custom-columns=\
"NAME:.metadata.name,LOCATION:.spec.location,CLUSTER:.spec.clusterRef.name,READY:.status.conditions[?(@.type=='Ready')].status"
        else
            echo -e "${YELLOW}No sync targets found or not in correct workspace${NC}"
        fi
        
        echo ""
        break
    done
    
    # Verify physical clusters
    echo "Physical Cluster Connectivity:"
    for cluster in kcp-west kcp-east; do
        if kubectl cluster-info --context "kind-$cluster" >/dev/null 2>&1; then
            echo -e "${GREEN}✓ $cluster: Accessible${NC}"
        else
            echo -e "${RED}✗ $cluster: Not accessible${NC}"
        fi
    done
}

stop_syncers() {
    echo -e "${CYAN}Stopping TMC Syncer Processes:${NC}"
    
    local syncer_dirs=$(find /tmp -maxdepth 1 -name "kcp-demo-*" -type d 2>/dev/null)
    if [ -z "$syncer_dirs" ]; then
        echo -e "${YELLOW}No active KCP demo environments found.${NC}"
        return 0
    fi
    
    for demo_dir in $syncer_dirs; do
        local syncer_dir="$demo_dir/syncers"
        if [ -d "$syncer_dir" ]; then
            echo "Stopping syncers in: $syncer_dir"
            
            for pid_file in "$syncer_dir"/*.pid; do
                if [ -f "$pid_file" ]; then
                    local pid=$(cat "$pid_file")
                    local syncer_name=$(basename "$pid_file" .pid)
                    
                    if kill -0 "$pid" 2>/dev/null; then
                        echo "  Stopping $syncer_name (PID: $pid)..."
                        kill "$pid" 2>/dev/null || true
                        rm -f "$pid_file"
                        echo -e "    ${GREEN}✓ Stopped${NC}"
                    else
                        echo -e "    ${YELLOW}$syncer_name was already stopped${NC}"
                        rm -f "$pid_file"
                    fi
                fi
            done
        fi
    done
}

# Main command processing
COMMAND="${1:-help}"
CLUSTER_NAME=""

# Parse options
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            if [ -z "$COMMAND" ] || [ "$COMMAND" = "help" ]; then
                COMMAND="$1"
            fi
            shift
            ;;
    esac
done

case "$COMMAND" in
    status)
        show_syncer_status
        ;;
    logs)
        show_syncer_logs "$CLUSTER_NAME"
        ;;
    sync-workload)
        manually_sync_workload "$CLUSTER_NAME"
        ;;
    verify)
        verify_sync_targets
        ;;
    stop)
        stop_syncers
        ;;
    help|*)
        show_help
        ;;
esac