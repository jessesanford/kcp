#!/bin/bash

set -euo pipefail

# TMC Hello World Tutorial Setup Script
# This script sets up a local TMC environment using kind clusters

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TUTORIAL_DIR="${ROOT_DIR}/tutorial-env"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
KCP_HOST_CLUSTER="kcp-host"
EAST_CLUSTER="cluster-east"
WEST_CLUSTER="cluster-west"
KCP_VERSION="v0.11.0"
KUBECTL_VERSION="v1.28.0"

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
    
    # Check if running on supported OS
    case "$(uname -s)" in
        Linux*)     MACHINE=Linux;;
        Darwin*)    MACHINE=Mac;;
        *)          error "Unsupported operating system: $(uname -s)" && exit 1;;
    esac
    info "Operating system: $MACHINE"
    
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
    
    # Check available memory
    if [[ "$MACHINE" == "Linux" ]]; then
        TOTAL_MEM=$(free -g | awk '/^Mem:/{print $2}')
        if [[ $TOTAL_MEM -lt 8 ]]; then
            warn "Less than 8GB RAM available. The tutorial may run slowly."
        fi
    fi
}

install_dependencies() {
    log "Installing dependencies..."
    
    mkdir -p "${TUTORIAL_DIR}/bin"
    export PATH="${TUTORIAL_DIR}/bin:$PATH"
    
    # Install kind
    if ! command -v kind &> /dev/null; then
        info "Installing kind..."
        case "$MACHINE" in
            Linux)
                curl -Lo "${TUTORIAL_DIR}/bin/kind" https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
                ;;
            Mac)
                curl -Lo "${TUTORIAL_DIR}/bin/kind" https://kind.sigs.k8s.io/dl/v0.20.0/kind-darwin-amd64
                ;;
        esac
        chmod +x "${TUTORIAL_DIR}/bin/kind"
    else
        info "kind is already installed"
    fi
    
    # Install kubectl
    if ! command -v kubectl &> /dev/null; then
        info "Installing kubectl..."
        case "$MACHINE" in
            Linux)
                curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
                ;;
            Mac)
                curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/darwin/amd64/kubectl"
                ;;
        esac
        chmod +x kubectl
        mv kubectl "${TUTORIAL_DIR}/bin/"
    else
        info "kubectl is already installed"
    fi
    
    # Install jq
    if ! command -v jq &> /dev/null; then
        info "Installing jq..."
        case "$MACHINE" in
            Linux)
                curl -L https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -o "${TUTORIAL_DIR}/bin/jq"
                ;;
            Mac)
                curl -L https://github.com/stedolan/jq/releases/download/jq-1.6/jq-osx-amd64 -o "${TUTORIAL_DIR}/bin/jq"
                ;;
        esac
        chmod +x "${TUTORIAL_DIR}/bin/jq"
    else
        info "jq is already installed"
    fi
    
    # Make sure tools are in PATH
    export PATH="${TUTORIAL_DIR}/bin:$PATH"
}

cleanup_existing() {
    log "Cleaning up any existing tutorial environment..."
    
    # Delete existing clusters
    for cluster in "$KCP_HOST_CLUSTER" "$EAST_CLUSTER" "$WEST_CLUSTER"; do
        if kind get clusters | grep -q "^${cluster}$"; then
            info "Deleting existing cluster: $cluster"
            kind delete cluster --name "$cluster" || true
        fi
    done
    
    # Clean tutorial directory
    if [[ -d "$TUTORIAL_DIR" ]]; then
        rm -rf "${TUTORIAL_DIR:?}"/*
    fi
    mkdir -p "$TUTORIAL_DIR"
}

create_kind_clusters() {
    log "Creating kind clusters..."
    
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
        node-labels: "region=control"
  extraPortMappings:
  - containerPort: 6443
    hostPort: 16443
    protocol: TCP
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 16443
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/kcp-host-config.yaml"
    
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
        node-labels: "region=east"
  extraPortMappings:
  - containerPort: 6443
    hostPort: 16444
    protocol: TCP
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 16444
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/east-cluster-config.yaml"
    
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
        node-labels: "region=west"
  extraPortMappings:
  - containerPort: 6443
    hostPort: 16445
    protocol: TCP
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 16445
EOF
    
    kind create cluster --config="${TUTORIAL_DIR}/west-cluster-config.yaml"
    
    # Save kubeconfig files
    kind get kubeconfig --name "$KCP_HOST_CLUSTER" > "${TUTORIAL_DIR}/kcp-host.kubeconfig"
    kind get kubeconfig --name "$EAST_CLUSTER" > "${TUTORIAL_DIR}/cluster-east.kubeconfig"
    kind get kubeconfig --name "$WEST_CLUSTER" > "${TUTORIAL_DIR}/cluster-west.kubeconfig"
    
    log "All clusters created successfully"
}

build_and_deploy_kcp() {
    log "Building and deploying KCP with TMC..."
    
    cd "$ROOT_DIR"
    
    # Build KCP
    info "Building KCP..."
    make build
    
    # Create KCP deployment manifests
    mkdir -p "${TUTORIAL_DIR}/manifests"
    
    # Create KCP namespace
    cat <<EOF > "${TUTORIAL_DIR}/manifests/kcp-namespace.yaml"
apiVersion: v1
kind: Namespace
metadata:
  name: kcp-system
EOF
    
    # Create KCP deployment
    cat <<EOF > "${TUTORIAL_DIR}/manifests/kcp-deployment.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kcp
  namespace: kcp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kcp
  template:
    metadata:
      labels:
        app: kcp
    spec:
      containers:
      - name: kcp
        image: registry.k8s.io/kube-apiserver:v1.28.0
        command:
        - /usr/local/bin/kube-apiserver
        args:
        - --secure-port=6443
        - --bind-address=0.0.0.0
        - --advertise-address=127.0.0.1
        - --etcd-servers=http://etcd:2379
        - --allow-privileged=true
        - --service-cluster-ip-range=10.96.0.0/16
        - --enable-admission-plugins=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,DefaultTolerationSeconds,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota
        - --disable-admission-plugins=StorageObjectInUseProtection
        - --runtime-config=api/all=true
        - --enable-bootstrap-token-auth=true
        ports:
        - containerPort: 6443
          name: api
        volumeMounts:
        - name: kcp-config
          mountPath: /etc/kcp
      volumes:
      - name: kcp-config
        configMap:
          name: kcp-config
---
apiVersion: v1
kind: Service
metadata:
  name: kcp
  namespace: kcp-system
spec:
  selector:
    app: kcp
  ports:
  - port: 6443
    targetPort: 6443
    nodePort: 30443
  type: NodePort
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: etcd
  namespace: kcp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: etcd
  template:
    metadata:
      labels:
        app: etcd
    spec:
      containers:
      - name: etcd
        image: gcr.io/etcd-development/etcd:v3.5.9
        command:
        - etcd
        args:
        - --data-dir=/etcd-data
        - --listen-client-urls=http://0.0.0.0:2379
        - --advertise-client-urls=http://etcd:2379
        - --initial-cluster-state=new
        ports:
        - containerPort: 2379
        volumeMounts:
        - name: etcd-data
          mountPath: /etcd-data
      volumes:
      - name: etcd-data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: etcd
  namespace: kcp-system
spec:
  selector:
    app: etcd
  ports:
  - port: 2379
    targetPort: 2379
EOF
    
    # Create a simple KCP config
    cat <<EOF > "${TUTORIAL_DIR}/manifests/kcp-config.yaml"
apiVersion: v1
kind: ConfigMap
metadata:
  name: kcp-config
  namespace: kcp-system
data:
  config.yaml: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        server: https://kcp:6443
      name: kcp
    contexts:
    - context:
        cluster: kcp
        user: admin
      name: kcp
    current-context: kcp
    users:
    - name: admin
      user:
        client-certificate-data: ""
        client-key-data: ""
EOF
    
    # Deploy to host cluster
    info "Deploying KCP to host cluster..."
    kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" apply -f "${TUTORIAL_DIR}/manifests/"
    
    # Wait for KCP to be ready
    info "Waiting for KCP to be ready..."
    kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" wait --for=condition=available deployment/kcp -n kcp-system --timeout=300s
    
    # Create admin kubeconfig for KCP
    info "Creating KCP admin kubeconfig..."
    cat <<EOF > "${TUTORIAL_DIR}/admin.kubeconfig"
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:30080
    insecure-skip-tls-verify: true
  name: kcp
contexts:
- context:
    cluster: kcp
    user: admin
  name: admin
current-context: admin
users:
- name: admin
  user:
    client-certificate-data: ""
    client-key-data: ""
EOF
}

setup_syncers() {
    log "Setting up syncer agents..."
    
    # Create syncer manifests for each cluster
    for cluster in "$EAST_CLUSTER" "$WEST_CLUSTER"; do
        info "Setting up syncer for $cluster"
        
        # Create syncer namespace
        kubectl --kubeconfig="${TUTORIAL_DIR}/${cluster}.kubeconfig" create namespace kcp-syncer || true
        
        # Create a simple syncer deployment
        cat <<EOF > "${TUTORIAL_DIR}/manifests/syncer-${cluster}.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: syncer
  namespace: kcp-syncer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: syncer
  template:
    metadata:
      labels:
        app: syncer
    spec:
      containers:
      - name: syncer
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          echo "Syncer agent running for cluster ${cluster}"
          echo "This is a simplified syncer for tutorial purposes"
          while true; do
            echo "Syncer heartbeat: \$(date)"
            sleep 30
          done
        env:
        - name: CLUSTER_NAME
          value: "${cluster}"
        - name: KCP_URL
          value: "https://host.docker.internal:30443"
EOF
        
        kubectl --kubeconfig="${TUTORIAL_DIR}/${cluster}.kubeconfig" apply -f "${TUTORIAL_DIR}/manifests/syncer-${cluster}.yaml"
    done
    
    # Create SyncTarget resources in KCP
    info "Creating SyncTarget resources..."
    
    cat <<EOF > "${TUTORIAL_DIR}/manifests/synctargets.yaml"
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: cluster-east
  labels:
    region: east
spec:
  cluster: cluster-east
  unschedulable: false
---
apiVersion: workload.kcp.io/v1alpha1
kind: SyncTarget
metadata:
  name: cluster-west
  labels:
    region: west
spec:
  cluster: cluster-west
  unschedulable: false
EOF
    
    # Note: This would normally be created by the actual syncer
    # For tutorial purposes, we'll create mock SyncTargets
}

create_tutorial_scripts() {
    log "Creating tutorial helper scripts..."
    
    # Create environment setup script
    cat <<'EOF' > "${TUTORIAL_DIR}/setup-env.sh"
#!/bin/bash
# Source this file to set up your environment for the TMC tutorial

export TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export PATH="${TUTORIAL_DIR}/bin:$PATH"
export KUBECONFIG="${TUTORIAL_DIR}/admin.kubeconfig"

echo "TMC Tutorial environment configured!"
echo "Available clusters:"
kind get clusters
echo
echo "Use 'kubectl get nodes' to verify KCP access"
echo "Use 'kubectl config view' to see cluster configuration"
EOF
    
    # Create cluster status check script
    cat <<'EOF' > "${TUTORIAL_DIR}/check-status.sh"
#!/bin/bash

TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🏥 TMC Tutorial Status Check"
echo "============================"
echo

echo "📦 Kind Clusters:"
kind get clusters

echo
echo "🔧 KCP Pods:"
kubectl --kubeconfig="${TUTORIAL_DIR}/kcp-host.kubeconfig" get pods -n kcp-system

echo
echo "🎯 Syncer Agents:"
for cluster in cluster-east cluster-west; do
    echo "  $cluster:"
    kubectl --kubeconfig="${TUTORIAL_DIR}/${cluster}.kubeconfig" get pods -n kcp-syncer 2>/dev/null || echo "    Not found"
done

echo
echo "📊 Tutorial Environment Ready!"
echo "Next steps:"
echo "  1. cd $(pwd)"
echo "  2. source setup-env.sh"
echo "  3. Follow the tutorial steps in the documentation"
EOF
    
    chmod +x "${TUTORIAL_DIR}/setup-env.sh"
    chmod +x "${TUTORIAL_DIR}/check-status.sh"
}

create_sample_workloads() {
    log "Creating sample workload manifests..."
    
    mkdir -p "${TUTORIAL_DIR}/examples"
    
    # Hello world deployment
    cat <<'EOF' > "${TUTORIAL_DIR}/examples/hello-world.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    app: hello-world
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
      - name: hello-world
        image: nginx:1.21
        ports:
        - containerPort: 80
        env:
        - name: MESSAGE
          value: "Hello from TMC!"
        - name: CLUSTER_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
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
          echo "<h1>Hello from TMC!</h1>" > /html/index.html
          echo "<p>Running on cluster: ${CLUSTER_NAME:-unknown}</p>" >> /html/index.html
          echo "<p>Pod: $(hostname)</p>" >> /html/index.html
          echo "<p>Time: $(date)</p>" >> /html/index.html
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
  name: hello-world-service
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
    
    # Sample placement
    cat <<'EOF' > "${TUTORIAL_DIR}/examples/placement.yaml"
apiVersion: scheduling.kcp.io/v1alpha1
kind: Placement
metadata:
  name: hello-world-placement
spec:
  locationSelectors:
  - matchLabels:
      region: east
  - matchLabels:
      region: west
  numberOfClusters: 2
  namespaceSelector:
    matchNames:
    - default
EOF
}

print_success_message() {
    log "TMC Tutorial setup completed successfully! 🎉"
    echo
    echo -e "${GREEN}=========================${NC}"
    echo -e "${GREEN}  Setup Complete!        ${NC}"
    echo -e "${GREEN}=========================${NC}"
    echo
    echo "Your TMC tutorial environment is ready!"
    echo
    echo "Tutorial directory: ${TUTORIAL_DIR}"
    echo
    echo "Next steps:"
    echo "  1. cd ${TUTORIAL_DIR}"
    echo "  2. source setup-env.sh"
    echo "  3. ./check-status.sh"
    echo "  4. Follow the tutorial documentation"
    echo
    echo "Available clusters:"
    kind get clusters
    echo
    echo "Documentation: ${ROOT_DIR}/docs/content/tutorials/tmc-hello-world.md"
    echo
    warn "Note: This is a simplified tutorial setup. Some TMC features are simulated."
}

main() {
    log "Starting TMC Hello World Tutorial setup..."
    
    check_prerequisites
    install_dependencies
    cleanup_existing
    create_kind_clusters
    build_and_deploy_kcp
    setup_syncers
    create_tutorial_scripts
    create_sample_workloads
    print_success_message
}

# Allow script to be sourced for testing
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi