#!/bin/bash

# Simple TMC Demo Runner

echo "Starting TMC Demo..."

# Kill any existing KCP
pkill -f "bin/kcp start" 2>/dev/null

# Setup
KCP_DIR="/tmp/kcp-demo-$$"
mkdir -p "$KCP_DIR"
export KUBECONFIG="$KCP_DIR/admin.kubeconfig"

echo "Starting KCP in background..."
nohup ./bin/kcp start \
    --root-directory="$KCP_DIR" \
    --feature-gates=TMCFeature=true,TMCAPIs=true \
    --v=2 > "$KCP_DIR/kcp.log" 2>&1 &
KCP_PID=$!

echo "KCP PID: $KCP_PID"
echo "Waiting for KCP to be ready..."
sleep 10

if [ ! -f "$KUBECONFIG" ]; then
    echo "KCP failed to start. Check logs at $KCP_DIR/kcp.log"
    tail -50 "$KCP_DIR/kcp.log"
    exit 1
fi

echo "KCP is ready!"
echo ""
echo "Creating TMC resources..."

# Create namespace
kubectl create namespace tmc-demo 2>/dev/null || true

# Create simple TMC CRDs
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.tmc.kcp.io
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
              region:
                type: string
          status:
            type: object
            properties:
              ready:
                type: boolean
  scope: Namespaced
  names:
    plural: clusters
    singular: cluster
    kind: Cluster
EOF

# Create sample cluster
kubectl apply -f - <<EOF
apiVersion: tmc.kcp.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
  namespace: tmc-demo
spec:
  region: us-west-2
EOF

echo ""
echo "Resources created!"
echo ""
echo "Status:"
kubectl get crd | grep tmc
kubectl get clusters.tmc.kcp.io -n tmc-demo
echo ""
echo "KCP is running at PID $KCP_PID"
echo "Logs: tail -f $KCP_DIR/kcp.log"
echo "Stop: kill $KCP_PID"
echo ""
echo "KUBECONFIG=$KUBECONFIG"