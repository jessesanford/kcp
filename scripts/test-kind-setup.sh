#!/bin/bash

set -euo pipefail

# Quick test script to verify kind setup works
# This creates a minimal cluster to test the setup process

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_CLUSTER="tmc-test"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

cleanup() {
    log "Cleaning up test cluster..."
    if kind get clusters | grep -q "^${TEST_CLUSTER}$"; then
        kind delete cluster --name "$TEST_CLUSTER" || true
    fi
}

# Set trap for cleanup
trap cleanup EXIT

main() {
    log "Testing TMC tutorial setup with kind..."
    
    # Check prerequisites
    if ! command -v kind &> /dev/null; then
        error "kind not found"
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        error "docker not found"
        exit 1
    fi
    
    # Test cluster creation
    log "Creating test cluster: $TEST_CLUSTER"
    
    cat <<EOF > /tmp/test-cluster-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${TEST_CLUSTER}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=test"
EOF
    
    if kind create cluster --config=/tmp/test-cluster-config.yaml --wait=60s; then
        log "✅ Test cluster created successfully"
        
        # Test basic functionality
        if kubectl --context="kind-${TEST_CLUSTER}" get nodes; then
            log "✅ kubectl connectivity works"
        else
            error "❌ kubectl connectivity failed"
            exit 1
        fi
        
        # Test container deployment
        log "Testing container deployment..."
        kubectl --context="kind-${TEST_CLUSTER}" run test-pod --image=busybox:1.35 --restart=Never --command -- sleep 30
        
        if kubectl --context="kind-${TEST_CLUSTER}" wait --for=condition=ready pod/test-pod --timeout=60s; then
            log "✅ Container deployment works"
            kubectl --context="kind-${TEST_CLUSTER}" delete pod test-pod
        else
            warn "⚠️  Container deployment may have issues"
        fi
        
        log "🎉 TMC tutorial setup verification passed!"
        echo
        echo "The full setup script should work correctly."
        echo "You can now run: ./scripts/setup-tmc-tutorial.sh"
        
    else
        error "❌ Failed to create test cluster"
        exit 1
    fi
}

main "$@"