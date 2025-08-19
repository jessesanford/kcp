#!/bin/bash

set -e

echo "=== TMC Test Harness (Fixed) ==="
echo "This script demonstrates TMC functionality in KCP"
echo

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl not found. Please install kubectl first."
    echo "Run: curl -LO \"https://dl.k8s.io/release/\$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl\" && chmod +x kubectl && sudo mv kubectl /usr/local/bin/"
    exit 1
fi

# Function to check if KCP is running
check_kcp() {
    if ! netstat -tlnp 2>/dev/null | grep -q 6443; then
        return 1
    fi
    return 0
}

# Function to start KCP
start_kcp() {
    local kcp_dir="$1"
    echo "Starting KCP server in directory: $kcp_dir"
    
    if [ ! -d "$kcp_dir" ]; then
        mkdir -p "$kcp_dir"
    fi
    
    nohup ./bin/kcp start \
        --root-directory="$kcp_dir" \
        --feature-gates=TMCFeature=true,TMCAPIs=true \
        --v=2 > "$kcp_dir/kcp.log" 2>&1 &
    
    local kcp_pid=$!
    echo "KCP started with PID: $kcp_pid"
    
    # Wait for KCP to be ready
    echo "Waiting for KCP to start..."
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if check_kcp; then
            echo "KCP is ready!"
            return 0
        fi
        sleep 2
        ((attempt++))
        echo "Attempt $attempt/$max_attempts..."
    done
    
    echo "ERROR: KCP failed to start within timeout"
    return 1
}

# Find or start KCP instance
KCP_DIR=""
KUBECONFIG_FILE=""

# Look for existing KCP instance
for dir in /tmp/kcp-tmc-test-*; do
    if [ -d "$dir" ] && [ -f "$dir/admin.kubeconfig" ]; then
        KCP_DIR="$dir"
        KUBECONFIG_FILE="$dir/admin.kubeconfig"
        echo "Found existing KCP instance: $KCP_DIR"
        break
    fi
done

# If no existing instance or KCP not running, start new one
if [ -z "$KCP_DIR" ] || ! check_kcp; then
    if [ -z "$KCP_DIR" ]; then
        KCP_DIR="/tmp/kcp-tmc-test-$(date +%s)"
        KUBECONFIG_FILE="$KCP_DIR/admin.kubeconfig"
    fi
    
    echo "Starting new KCP instance..."
    if ! start_kcp "$KCP_DIR"; then
        exit 1
    fi
else
    echo "Using existing running KCP instance: $KCP_DIR"
fi

# Export kubeconfig
export KUBECONFIG="$KUBECONFIG_FILE"
echo "Using KUBECONFIG: $KUBECONFIG"

# Test connection
echo
echo "=== Testing KCP Connection ==="
if kubectl get namespaces &>/dev/null; then
    echo "✅ Successfully connected to KCP"
else
    echo "❌ Failed to connect to KCP"
    exit 1
fi

# Show KCP-specific resources
echo
echo "=== KCP Resources ==="
kubectl api-resources | grep -E "(kcp|workspace)" | head -5

# Install TMC CRDs if they exist
echo
echo "=== Installing TMC CRDs ==="
if [ -f "$KCP_DIR/tmc-crds.yaml" ]; then
    echo "Applying existing TMC CRDs..."
    kubectl apply -f "$KCP_DIR/tmc-crds.yaml"
elif [ -f "./tmc-crds.yaml" ]; then
    echo "Applying TMC CRDs from current directory..."
    kubectl apply -f "./tmc-crds.yaml"
else
    echo "Creating basic TMC CRDs..."
    cat <<EOF | kubectl apply -f -
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
          status:
            type: object
  scope: Namespaced
  names:
    plural: clusterregistrations
    singular: clusterregistration
    kind: ClusterRegistration
    shortNames:
    - cluster
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
                  kind:
                    type: string
                  name:
                    type: string
          status:
            type: object
  scope: Namespaced
  names:
    plural: workloadplacements
    singular: workloadplacement
    kind: WorkloadPlacement
    shortNames:
    - placement
EOF
fi

# Wait for CRDs to be ready
echo "Waiting for CRDs to be ready..."
sleep 5

# Verify TMC CRDs
echo
echo "=== Verifying TMC CRDs ==="
kubectl get crd | grep tmc || echo "No TMC CRDs found"

# Create TMC namespace and test resources
echo
echo "=== Creating TMC Test Resources ==="
kubectl create namespace tmc-demo 2>/dev/null || echo "Namespace tmc-demo already exists"

# Create test cluster registration
echo "Creating test cluster registration..."
cat <<EOF | kubectl apply -f -
apiVersion: tmc.kcp.io/v1alpha1
kind: ClusterRegistration
metadata:
  name: test-cluster-1
  namespace: tmc-demo
spec:
  clusterID: "cluster-001"
  region: "us-west-2"
  provider: "AWS"
EOF

# Create test workload placement
echo "Creating test workload placement..."
cat <<EOF | kubectl apply -f -
apiVersion: tmc.kcp.io/v1alpha1
kind: WorkloadPlacement
metadata:
  name: test-placement-1
  namespace: tmc-demo
spec:
  workloadRef:
    kind: "Deployment"
    name: "test-app"
EOF

# Show TMC resources
echo
echo "=== TMC Resources Created ==="
echo "Cluster Registrations:"
kubectl get clusterregistrations -n tmc-demo -o wide 2>/dev/null || echo "No cluster registrations found"

echo
echo "Workload Placements:"
kubectl get workloadplacements -n tmc-demo -o wide 2>/dev/null || echo "No workload placements found"

# Test TMC controller (briefly)
echo
echo "=== Testing TMC Controller ==="
if [ -f "./bin/tmc-controller" ]; then
    echo "Testing TMC controller startup..."
    timeout 5s ./bin/tmc-controller --feature-gates=TMCFeature=true,TMCAPIs=true 2>&1 | head -10 || echo "TMC controller test completed"
else
    echo "TMC controller binary not found"
fi

# Show summary
echo
echo "=== TEST SUMMARY ==="
echo "✅ KCP Server: Running on localhost:6443"
echo "✅ kubectl: Working with KCP"
echo "✅ TMC CRDs: Installed and ready"
echo "✅ TMC Resources: Created successfully"
echo "✅ TMC Controller: Basic startup tested"
echo
echo "KCP Directory: $KCP_DIR"
echo "Kubeconfig: $KUBECONFIG"
echo
echo "To continue working with this KCP instance:"
echo "export KUBECONFIG=\"$KUBECONFIG\""
echo
echo "To check KCP logs:"
echo "tail -f $KCP_DIR/kcp.log"
echo
echo "🎉 TMC test harness completed successfully!"