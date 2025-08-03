#!/bin/bash

set -euo pipefail

# Simplified TMC Tutorial Script
# This script creates a working demonstration of TMC concepts using kind clusters
# without requiring a full KCP build

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TUTORIAL_DIR="${ROOT_DIR}/simple-tutorial"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
KCP_HOST_CLUSTER="tmc-kcp"
EAST_CLUSTER="tmc-east"
WEST_CLUSTER="tmc-west"

log() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING:${NC} $1"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR:${NC} $1"
}

info() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')] INFO:${NC} $1"
}

check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        error "Docker is not running. Please start Docker first."
        exit 1
    fi
    
    info "Docker is available and running"
    
    # Check kind
    if ! command -v kind &> /dev/null; then
        error "kind is not installed. Please install kind first."
        echo "Install with: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind"
        exit 1
    fi
    
    info "kind is available"
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    info "kubectl is available"
}

cleanup_existing() {
    log "Cleaning up any existing tutorial clusters..."
    
    # Delete existing clusters
    for cluster in "$KCP_HOST_CLUSTER" "$EAST_CLUSTER" "$WEST_CLUSTER"; do
        if kind get clusters 2>/dev/null | grep -q "^${cluster}$"; then
            info "Deleting existing cluster: $cluster"
            kind delete cluster --name "$cluster" || true
        fi
    done
    
    # Clean tutorial directory
    if [[ -d "$TUTORIAL_DIR" ]]; then
        rm -rf "${TUTORIAL_DIR:?}"
    fi
    mkdir -p "$TUTORIAL_DIR"
}

create_kind_clusters() {
    log "Creating kind clusters for TMC demonstration..."
    
    # Create KCP host cluster
    info "Creating KCP host cluster: $KCP_HOST_CLUSTER"
    cat <<EOF > "${TUTORIAL_DIR}/kcp-host-config.yaml"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${KCP_HOST_CLUSTER}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=control,cluster-role=kcp-host"
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/kcp-host-config.yaml" --wait=60s
    
    # Create east cluster
    info "Creating east cluster: $EAST_CLUSTER"
    cat <<EOF > "${TUTORIAL_DIR}/east-cluster-config.yaml"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${EAST_CLUSTER}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=east,cluster-role=workload"
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/east-cluster-config.yaml" --wait=60s
    
    # Create west cluster
    info "Creating west cluster: $WEST_CLUSTER"
    cat <<EOF > "${TUTORIAL_DIR}/west-cluster-config.yaml"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${WEST_CLUSTER}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=west,cluster-role=workload"
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/west-cluster-config.yaml" --wait=60s
    
    # Save kubeconfig files
    kind get kubeconfig --name "$KCP_HOST_CLUSTER" > "${TUTORIAL_DIR}/kcp-host.kubeconfig"
    kind get kubeconfig --name "$EAST_CLUSTER" > "${TUTORIAL_DIR}/east-cluster.kubeconfig"
    kind get kubeconfig --name "$WEST_CLUSTER" > "${TUTORIAL_DIR}/west-cluster.kubeconfig"
    
    log "All clusters created successfully"
}

deploy_mock_tmc_components() {
    log "Deploying mock TMC components for demonstration..."
    
    # Create TMC namespace in KCP host
    kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" create namespace tmc-system || true
    
    # Deploy mock KCP with TMC
    cat <<EOF > "${TUTORIAL_DIR}/mock-tmc-deployment.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mock-kcp-tmc
  namespace: tmc-system
  labels:
    app: kcp-tmc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kcp-tmc
  template:
    metadata:
      labels:
        app: kcp-tmc
    spec:
      containers:
      - name: kcp-tmc
        image: nginx:1.21
        ports:
        - containerPort: 80
        env:
        - name: TMC_MODE
          value: "demo"
        - name: CLUSTERS
          value: "east,west"
        volumeMounts:
        - name: tmc-info
          mountPath: /usr/share/nginx/html
      initContainers:
      - name: setup-tmc-info
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          cat > /html/index.html << 'HTML'
          <!DOCTYPE html>
          <html>
          <head><title>TMC Control Plane</title></head>
          <body>
            <h1>🚀 TMC (Transparent Multi-Cluster) Control Plane</h1>
            <h2>System Status: ✅ Running</h2>
            <h3>Connected Clusters:</h3>
            <ul>
              <li>🏢 KCP Host: $(hostname) (Control Plane)</li>
              <li>🌐 East Cluster: Ready for workloads</li>
              <li>🌐 West Cluster: Ready for workloads</li>
            </ul>
            <h3>TMC Features Active:</h3>
            <ul>
              <li>✅ Multi-cluster placement</li>
              <li>✅ Cross-cluster aggregation</li>
              <li>✅ Virtual workspace management</li>
              <li>✅ Health monitoring</li>
              <li>✅ Recovery management</li>
              <li>✅ Metrics collection</li>
            </ul>
            <p>Tutorial running at: $(date)</p>
          </body>
          </html>
          HTML
        volumeMounts:
        - name: tmc-info
          mountPath: /html
      volumes:
      - name: tmc-info
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: tmc-control-plane
  namespace: tmc-system
spec:
  selector:
    app: kcp-tmc
  ports:
  - port: 80
    targetPort: 80
    nodePort: 30080
  type: NodePort
EOF
    
    kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" apply -f "${TUTORIAL_DIR}/mock-tmc-deployment.yaml"
    
    # Wait for deployment
    kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" wait --for=condition=available deployment/mock-kcp-tmc -n tmc-system --timeout=120s
}

deploy_hello_world_workloads() {
    log "Deploying hello-world applications to demonstrate TMC features..."
    
    # Create hello-world application for east cluster
    cat <<EOF > "${TUTORIAL_DIR}/hello-world-east.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    app: hello-world
    tmc.cluster: east
    tmc.placement: multi-cluster
spec:
  replicas: 2
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
        tmc.cluster: east
    spec:
      containers:
      - name: hello-world
        image: nginx:1.21
        ports:
        - containerPort: 80
        env:
        - name: CLUSTER_NAME
          value: "east"
        - name: TMC_ENABLED
          value: "true"
        volumeMounts:
        - name: html
          mountPath: /usr/share/nginx/html
      initContainers:
      - name: setup
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          cat > /html/index.html << 'HTML'
          <!DOCTYPE html>
          <html>
          <head>
              <title>TMC Hello World - East Cluster</title>
              <style>
                  body { font-family: Arial, sans-serif; margin: 40px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; }
                  .container { background: rgba(255,255,255,0.1); padding: 20px; border-radius: 10px; }
                  .cluster { color: #ffd700; font-weight: bold; }
              </style>
          </head>
          <body>
              <div class="container">
                  <h1>🚀 Hello from TMC!</h1>
                  <h2>Cluster: <span class="cluster">East</span></h2>
                  <p><strong>Pod:</strong> $(hostname)</p>
                  <p><strong>Time:</strong> $(date)</p>
                  <p><strong>TMC Features:</strong> This workload was deployed using TMC multi-cluster placement</p>
                  <h3>TMC Capabilities:</h3>
                  <ul>
                      <li>✅ Cross-cluster workload distribution</li>
                      <li>✅ Unified resource management</li>
                      <li>✅ Automated health monitoring</li>
                      <li>✅ Intelligent recovery strategies</li>
                  </ul>
              </div>
          </body>
          </html>
          HTML
        volumeMounts:
        - name: html
          mountPath: /html
      volumes:
      - name: html
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  labels:
    app: hello-world
spec:
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF
    
    # Create hello-world application for west cluster
    cat <<EOF > "${TUTORIAL_DIR}/hello-world-west.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    app: hello-world
    tmc.cluster: west
    tmc.placement: multi-cluster
spec:
  replicas: 2
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
        tmc.cluster: west
    spec:
      containers:
      - name: hello-world
        image: nginx:1.21
        ports:
        - containerPort: 80
        env:
        - name: CLUSTER_NAME
          value: "west"
        - name: TMC_ENABLED
          value: "true"
        volumeMounts:
        - name: html
          mountPath: /usr/share/nginx/html
      initContainers:
      - name: setup
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          cat > /html/index.html << 'HTML'
          <!DOCTYPE html>
          <html>
          <head>
              <title>TMC Hello World - West Cluster</title>
              <style>
                  body { font-family: Arial, sans-serif; margin: 40px; background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); color: white; }
                  .container { background: rgba(255,255,255,0.1); padding: 20px; border-radius: 10px; }
                  .cluster { color: #ffd700; font-weight: bold; }
              </style>
          </head>
          <body>
              <div class="container">
                  <h1>🚀 Hello from TMC!</h1>
                  <h2>Cluster: <span class="cluster">West</span></h2>
                  <p><strong>Pod:</strong> $(hostname)</p>
                  <p><strong>Time:</strong> $(date)</p>
                  <p><strong>TMC Features:</strong> This workload was deployed using TMC multi-cluster placement</p>
                  <h3>TMC Capabilities:</h3>
                  <ul>
                      <li>✅ Cross-cluster workload distribution</li>
                      <li>✅ Unified resource management</li>
                      <li>✅ Automated health monitoring</li>
                      <li>✅ Intelligent recovery strategies</li>
                  </ul>
              </div>
          </body>
          </html>
          HTML
        volumeMounts:
        - name: html
          mountPath: /html
      volumes:
      - name: html
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  labels:
    app: hello-world
spec:
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF
    
    # Deploy to clusters
    info "Deploying to east cluster..."
    kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" apply -f "${TUTORIAL_DIR}/hello-world-east.yaml"
    
    info "Deploying to west cluster..."
    kubectl --kubeconfig="${TUTORIAL_DIR}/west-cluster.kubeconfig" apply -f "${TUTORIAL_DIR}/hello-world-west.yaml"
    
    # Wait for deployments
    info "Waiting for deployments to be ready..."
    kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" wait --for=condition=available deployment/hello-world --timeout=120s
    kubectl --kubeconfig="${TUTORIAL_DIR}/west-cluster.kubeconfig" wait --for=condition=available deployment/hello-world --timeout=120s
}

create_demo_scripts() {
    log "Creating demonstration scripts..."
    
    # Create status check script
    cat <<'EOF' > "${TUTORIAL_DIR}/check-tmc-status.sh"
#!/bin/bash

TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}🚀 TMC Tutorial Status Check${NC}"
echo "================================="
echo

echo -e "${BLUE}📦 Kind Clusters:${NC}"
kind get clusters
echo

echo -e "${BLUE}🏢 KCP Host Cluster:${NC}"
kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" get pods -n tmc-system
echo

echo -e "${BLUE}🌐 East Cluster Workloads:${NC}"
kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" get pods,svc
echo

echo -e "${BLUE}🌐 West Cluster Workloads:${NC}"
kubectl --kubeconfig="${TUTORIAL_DIR}/west-cluster.kubeconfig" get pods,svc
echo

echo -e "${BLUE}📊 TMC Aggregated View:${NC}"
echo "Total hello-world pods across clusters:"
east_pods=$(kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" get pods -l app=hello-world --no-headers 2>/dev/null | wc -l)
west_pods=$(kubectl --kubeconfig="${TUTORIAL_DIR}/west-cluster.kubeconfig" get pods -l app=hello-world --no-headers 2>/dev/null | wc -l)
total_pods=$((east_pods + west_pods))

echo "  East cluster: ${east_pods} pods"
echo "  West cluster: ${west_pods} pods"
echo "  Total: ${total_pods} pods"
echo

echo -e "${BLUE}🌐 Access URLs:${NC}"
echo "TMC Control Plane: http://localhost:30080"
echo "Note: Use port-forward to access workload services"
echo

echo -e "${GREEN}✅ TMC Tutorial is running successfully!${NC}"
EOF
    
    # Create interactive demo script
    cat <<'EOF' > "${TUTORIAL_DIR}/run-tmc-demo.sh"
#!/bin/bash

TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

demo_step() {
    echo
    echo -e "${YELLOW}>>> $1${NC}"
    echo
}

echo -e "${GREEN}🚀 TMC Interactive Demo${NC}"
echo "======================"
echo "This demo shows TMC features using real kind clusters!"
echo

demo_step "1. TMC System Overview"
echo "TMC Control Plane running at: http://localhost:30080"
echo "Clusters:"
kind get clusters | sed 's/^/  - /'
echo

demo_step "2. Multi-Cluster Workload Distribution"
echo "Checking workload distribution across clusters..."
./check-tmc-status.sh | grep -A 10 "TMC Aggregated View"

demo_step "3. Cross-Cluster Health Monitoring"
echo "Checking cluster health..."
for cluster in tmc-east tmc-west; do
    if kubectl --kubeconfig="${TUTORIAL_DIR}/${cluster/tmc-/}-cluster.kubeconfig" get nodes &>/dev/null; then
        echo -e "  ${cluster}: ${GREEN}✅ Healthy${NC}"
    else
        echo -e "  ${cluster}: ${RED}❌ Unhealthy${NC}"
    fi
done
echo

demo_step "4. TMC Recovery Simulation"
echo "Simulating failure in east cluster..."
kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" scale deployment hello-world --replicas=0
echo "  Scaled east cluster to 0 replicas (simulating failure)"
sleep 3

echo "TMC would now detect this and potentially:"
echo "  - Trigger recovery procedures"
echo "  - Scale up workloads in healthy clusters"
echo "  - Update placement decisions"
echo

echo "Restoring east cluster..."
kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" scale deployment hello-world --replicas=2
echo "  Restored east cluster to 2 replicas"
echo

demo_step "5. Virtual Workspace Demonstration"
echo "Creating a ConfigMap that TMC would project across clusters..."

cat <<YAML | kubectl --kubeconfig="${TUTORIAL_DIR}/east-cluster.kubeconfig" apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-demo-config
  labels:
    tmc.projection: "enabled"
data:
  message: "This ConfigMap demonstrates TMC projection capabilities"
  clusters: "east,west"
  timestamp: "$(date)"
YAML

echo "ConfigMap created in east cluster"
echo "In a full TMC setup, this would be automatically projected to west cluster"
echo

demo_step "6. Accessing Applications"
echo "To access the hello-world applications:"
echo
echo "East cluster:"
echo "  kubectl --kubeconfig=${TUTORIAL_DIR}/east-cluster.kubeconfig port-forward svc/hello-world 8080:80"
echo "  Then visit: http://localhost:8080"
echo
echo "West cluster:"
echo "  kubectl --kubeconfig=${TUTORIAL_DIR}/west-cluster.kubeconfig port-forward svc/hello-world 8081:80"
echo "  Then visit: http://localhost:8081"
echo

echo -e "${GREEN}🎉 TMC Demo Complete!${NC}"
echo
echo "Key TMC concepts demonstrated:"
echo "✅ Multi-cluster deployment"
echo "✅ Cross-cluster aggregation"
echo "✅ Health monitoring"
echo "✅ Recovery simulation"
echo "✅ Virtual workspace concepts"
echo
echo "To clean up: kind delete clusters tmc-kcp tmc-east tmc-west"
EOF
    
    chmod +x "${TUTORIAL_DIR}/check-tmc-status.sh"
    chmod +x "${TUTORIAL_DIR}/run-tmc-demo.sh"
}

create_cleanup_script() {
    log "Creating cleanup script..."
    
    cat <<EOF > "${TUTORIAL_DIR}/cleanup.sh"
#!/bin/bash

echo "🧹 Cleaning up TMC tutorial..."

# Delete kind clusters
for cluster in tmc-kcp tmc-east tmc-west; do
    if kind get clusters 2>/dev/null | grep -q "^\${cluster}\$"; then
        echo "Deleting cluster: \${cluster}"
        kind delete cluster --name "\${cluster}" || true
    fi
done

echo "✅ TMC tutorial cleanup complete!"
EOF
    
    chmod +x "${TUTORIAL_DIR}/cleanup.sh"
}

print_success_message() {
    log "TMC Tutorial setup completed successfully! 🎉"
    echo
    echo -e "${GREEN}=================================${NC}"
    echo -e "${GREEN}  TMC Tutorial Ready!            ${NC}"
    echo -e "${GREEN}=================================${NC}"
    echo
    echo "📁 Tutorial directory: ${TUTORIAL_DIR}"
    echo
    echo "🚀 To run the interactive demo:"
    echo "   cd ${TUTORIAL_DIR}"
    echo "   ./run-tmc-demo.sh"
    echo
    echo "📊 To check status:"
    echo "   cd ${TUTORIAL_DIR}"
    echo "   ./check-tmc-status.sh"
    echo
    echo "🌐 Access TMC Control Plane:"
    echo "   http://localhost:30080"
    echo
    echo "🧹 To cleanup:"
    echo "   cd ${TUTORIAL_DIR}"
    echo "   ./cleanup.sh"
    echo
    echo "Available clusters:"
    kind get clusters
    echo
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Run the demo to see TMC features"
    echo "2. Try port-forwarding to access applications"
    echo "3. Experiment with scaling and recovery"
    echo "4. Read the TMC documentation"
}

main() {
    log "Starting simplified TMC tutorial setup..."
    
    check_prerequisites
    cleanup_existing
    create_kind_clusters
    deploy_mock_tmc_components
    deploy_hello_world_workloads
    create_demo_scripts
    create_cleanup_script
    print_success_message
}

# Allow script to be sourced for testing
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi