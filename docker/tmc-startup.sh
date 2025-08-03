#!/bin/bash

# TMC All-in-One startup script
# This script can start KCP server, syncer, or both based on environment variables

set -e

# Configuration
MODE=${TMC_MODE:-"kcp"}  # Options: kcp, syncer, all
KCP_ROOT_DIR=${KCP_ROOT_DIR:-"/var/lib/tmc"}
SYNC_TARGET_NAME=${SYNC_TARGET_NAME:-"local-target"}
WORKSPACE_CLUSTER=${WORKSPACE_CLUSTER:-"root:default"}
KCP_KUBECONFIG=${KCP_KUBECONFIG:-""}
CLUSTER_KUBECONFIG=${CLUSTER_KUBECONFIG:-""}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}$(date '+%Y-%m-%d %H:%M:%S') [INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}$(date '+%Y-%m-%d %H:%M:%S') [WARN]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}$(date '+%Y-%m-%d %H:%M:%S') [ERROR]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}$(date '+%Y-%m-%d %H:%M:%S') [SUCCESS]${NC} $1"
}

# Function to start KCP server
start_kcp() {
    log_info "Starting KCP server with TMC components..."
    
    # Ensure data directory exists
    mkdir -p "$KCP_ROOT_DIR"
    
    # Start KCP with TMC enabled
    exec /usr/local/bin/kcp start \
        --root-directory="$KCP_ROOT_DIR" \
        --enable-tmc \
        --tmc-error-handling=true \
        --tmc-health-monitoring=true \
        --tmc-metrics=true \
        --tmc-recovery=true \
        --tmc-virtual-workspaces=true \
        --tmc-placement-controller=true \
        --bind-address=0.0.0.0 \
        --secure-port=6443 \
        --v=2 \
        "$@"
}

# Function to start syncer
start_syncer() {
    log_info "Starting workload syncer..."
    
    # Validate required configuration
    if [ -z "$KCP_KUBECONFIG" ]; then
        log_error "KCP_KUBECONFIG is required for syncer mode"
        exit 1
    fi
    
    if [ -z "$CLUSTER_KUBECONFIG" ]; then
        log_error "CLUSTER_KUBECONFIG is required for syncer mode"
        exit 1
    fi
    
    # Generate sync target UID if not provided
    SYNC_TARGET_UID=${SYNC_TARGET_UID:-$(cat /proc/sys/kernel/random/uuid)}
    
    log_info "Syncer configuration:"
    log_info "  Sync Target: $SYNC_TARGET_NAME"
    log_info "  Workspace: $WORKSPACE_CLUSTER"
    log_info "  UID: $SYNC_TARGET_UID"
    
    # Start syncer
    exec /usr/local/bin/workload-syncer \
        --sync-target-name="$SYNC_TARGET_NAME" \
        --sync-target-uid="$SYNC_TARGET_UID" \
        --workspace-cluster="$WORKSPACE_CLUSTER" \
        --kcp-kubeconfig="$KCP_KUBECONFIG" \
        --cluster-kubeconfig="$CLUSTER_KUBECONFIG" \
        --metrics-bind-address=0.0.0.0:8080 \
        --v=2 \
        "$@"
}

# Function to start both KCP and syncer (for development)
start_all() {
    log_info "Starting KCP and syncer in all-in-one mode..."
    log_warn "This mode is for development/testing only!"
    
    # Start KCP in background
    log_info "Starting KCP server..."
    /usr/local/bin/kcp start \
        --root-directory="$KCP_ROOT_DIR" \
        --enable-tmc \
        --tmc-error-handling=true \
        --tmc-health-monitoring=true \
        --tmc-metrics=true \
        --tmc-recovery=true \
        --bind-address=0.0.0.0 \
        --secure-port=6443 \
        --v=2 &
    
    KCP_PID=$!
    log_info "KCP started with PID: $KCP_PID"
    
    # Wait for KCP to be ready
    log_info "Waiting for KCP to be ready..."
    for i in {1..30}; do
        if /usr/local/bin/health-check.sh >/dev/null 2>&1; then
            log_success "KCP is ready!"
            break
        fi
        log_info "Waiting for KCP... ($i/30)"
        sleep 2
    done
    
    # Set up kubeconfig for syncer
    KCP_KUBECONFIG="$KCP_ROOT_DIR/admin.kubeconfig"
    CLUSTER_KUBECONFIG="$KCP_KUBECONFIG"  # Self-syncer for demo
    
    # Start syncer if kubeconfig exists
    if [ -f "$KCP_KUBECONFIG" ]; then
        log_info "Starting syncer..."
        start_syncer &
        SYNCER_PID=$!
        log_info "Syncer started with PID: $SYNCER_PID"
    else
        log_error "KCP kubeconfig not found at $KCP_KUBECONFIG"
    fi
    
    # Wait for both processes
    wait $KCP_PID $SYNCER_PID
}

# Function to display usage
usage() {
    echo "TMC All-in-One Container"
    echo ""
    echo "Environment Variables:"
    echo "  TMC_MODE                 - Startup mode: kcp, syncer, or all (default: kcp)"
    echo "  KCP_ROOT_DIR            - KCP data directory (default: /var/lib/tmc)"
    echo "  SYNC_TARGET_NAME        - Syncer target name (default: local-target)"
    echo "  SYNC_TARGET_UID         - Syncer target UID (auto-generated if not set)"
    echo "  WORKSPACE_CLUSTER       - KCP workspace (default: root:default)"
    echo "  KCP_KUBECONFIG          - Path to KCP kubeconfig (required for syncer)"
    echo "  CLUSTER_KUBECONFIG      - Path to target cluster kubeconfig (required for syncer)"
    echo ""
    echo "Examples:"
    echo "  # Start KCP server only"
    echo "  docker run -e TMC_MODE=kcp kcp-tmc:latest"
    echo ""
    echo "  # Start syncer only"
    echo "  docker run -e TMC_MODE=syncer \\"
    echo "             -e KCP_KUBECONFIG=/etc/kcp/kubeconfig \\"
    echo "             -e CLUSTER_KUBECONFIG=/etc/cluster/kubeconfig \\"
    echo "             kcp-tmc:latest"
    echo ""
    echo "  # Start both (development only)"
    echo "  docker run -e TMC_MODE=all kcp-tmc:latest"
}

# Trap signals for graceful shutdown
cleanup() {
    log_info "Received shutdown signal, cleaning up..."
    if [ ! -z "$KCP_PID" ]; then
        log_info "Stopping KCP (PID: $KCP_PID)"
        kill -TERM "$KCP_PID" 2>/dev/null || true
    fi
    if [ ! -z "$SYNCER_PID" ]; then
        log_info "Stopping syncer (PID: $SYNCER_PID)"
        kill -TERM "$SYNCER_PID" 2>/dev/null || true
    fi
    wait
    log_info "Cleanup complete"
    exit 0
}

trap cleanup SIGTERM SIGINT

# Main execution
main() {
    log_info "TMC All-in-One Container Starting..."
    log_info "Mode: $MODE"
    
    case "$MODE" in
        "kcp")
            start_kcp "$@"
            ;;
        "syncer")
            start_syncer "$@"
            ;;
        "all")
            start_all "$@"
            ;;
        "help"|"--help"|"-h")
            usage
            exit 0
            ;;
        *)
            log_error "Invalid mode: $MODE"
            log_error "Valid modes: kcp, syncer, all"
            usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"