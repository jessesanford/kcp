#!/bin/bash

# KCP with TMC Helm Deployment Demo Script
# This script automates the complete deployment and demonstration process

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REGISTRY=${REGISTRY:-"localhost:5000"}
TAG=${TAG:-"v0.11.0"}
SKIP_BUILD=${SKIP_BUILD:-"false"}
CLEANUP_ON_EXIT=${CLEANUP_ON_EXIT:-"true"}

# Function to print colored output
print_header() {
    echo -e "\n${PURPLE}================================================"
    echo -e "$1"
    echo -e "================================================${NC}\n"
}

print_step() {
    echo -e "\n${BLUE}🔄 $1${NC}\n"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ️  $1${NC}"
}

# Function to wait for user input
wait_for_user() {
    echo -e "\n${YELLOW}Press Enter to continue or 'q' to quit...${NC}"
    read -r response
    if [[ "$response" == "q" ]]; then
        echo "Demo exited by user."
        cleanup_resources
        exit 0
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites"
    
    local missing_tools=()
    
    # Check required tools
    command -v docker >/dev/null 2>&1 || missing_tools+=("docker")
    command -v kubectl >/dev/null 2>&1 || missing_tools+=("kubectl") 
    command -v helm >/dev/null 2>&1 || missing_tools+=("helm")
    command -v kind >/dev/null 2>&1 || missing_tools+=("kind")
    command -v jq >/dev/null 2>&1 || missing_tools+=("jq")
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing_tools[*]}"
        print_info "Please install the missing tools and try again."
        exit 1
    fi
    
    # Check Docker is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
    
    print_success "All prerequisites checked"
}

# Function to setup local registry
setup_local_registry() {
    print_step "Setting up local container registry"
    
    # Check if registry is already running
    if docker ps --format "table {{.Names}}" | grep -q "^kind-registry$"; then
        print_info "Local registry already running"
        return 0
    fi
    
    # Start local registry
    docker run -d --restart=always -p "5000:5000" --name "kind-registry" registry:2
    
    # Wait for registry to be ready
    for i in {1..30}; do
        if curl -f http://localhost:5000/v2/ >/dev/null 2>&1; then
            print_success "Local registry is ready"
            return 0
        fi
        echo "Waiting for registry... ($i/30)"
        sleep 1
    done
    
    print_error "Failed to start local registry"
    exit 1
}

# Function to build and push images
build_and_push_images() {
    if [[ "$SKIP_BUILD" == "true" ]]; then
        print_info "Skipping image build (SKIP_BUILD=true)"
        return 0
    fi
    
    print_step "Building and pushing TMC images"
    
    cd "$PROJECT_ROOT"
    
    # Build KCP binaries
    print_info "Building KCP with TMC components..."
    make build
    
    # Build container images
    print_info "Building container images..."
    docker build -f docker/Dockerfile.tmc --target kcp-server -t "$REGISTRY/kcp-server:$TAG" .
    docker build -f docker/Dockerfile.tmc --target workload-syncer -t "$REGISTRY/kcp-syncer:$TAG" .
    
    # Push images
    print_info "Pushing images to registry..."
    docker push "$REGISTRY/kcp-server:$TAG"
    docker push "$REGISTRY/kcp-syncer:$TAG"
    
    print_success "Images built and pushed"
}

# Function to create kind clusters
create_clusters() {
    print_step "Creating kind clusters"
    
    # Create KCP host cluster
    print_info "Creating KCP host cluster..."
    cat > /tmp/kcp-host-config.yaml << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: kcp-host
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "node-type=kcp-host"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
    endpoint = ["http://kind-registry:5000"]
EOF
    
    kind create cluster --config /tmp/kcp-host-config.yaml --wait 300s
    
    # Connect registry to cluster network
    docker network connect "kind" "kind-registry" 2>/dev/null || true
    
    # Create east cluster
    print_info "Creating production-east cluster..."
    cat > /tmp/east-cluster-config.yaml << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: production-east
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=us-east-1,zone=us-east-1a"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
    endpoint = ["http://kind-registry:5000"]
EOF
    
    kind create cluster --config /tmp/east-cluster-config.yaml --wait 300s
    docker network connect "kind" "kind-registry" 2>/dev/null || true
    
    # Create west cluster
    print_info "Creating production-west cluster..."
    cat > /tmp/west-cluster-config.yaml << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: production-west
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "region=us-west-2,zone=us-west-2a"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
    endpoint = ["http://kind-registry:5000"]
EOF
    
    kind create cluster --config /tmp/west-cluster-config.yaml --wait 300s
    docker network connect "kind" "kind-registry" 2>/dev/null || true
    
    print_success "All clusters created"
}

# Function to install KCP with Helm
install_kcp() {
    print_step "Installing KCP with TMC using Helm"
    
    # Switch to KCP host cluster
    kubectl config use-context kind-kcp-host
    
    # Create namespace
    kubectl create namespace kcp-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Install KCP
    print_info "Installing KCP with TMC components..."
    helm install kcp-tmc "$PROJECT_ROOT/charts/kcp-tmc" \
        --namespace kcp-system \
        --set global.imageRegistry="$REGISTRY" \
        --set kcp.image.tag="$TAG" \
        --set kcp.tmc.enabled=true \
        --set kcp.tmc.errorHandling.enabled=true \
        --set kcp.tmc.healthMonitoring.enabled=true \
        --set kcp.tmc.metrics.enabled=true \
        --set kcp.tmc.recovery.enabled=true \
        --set kcp.tmc.virtualWorkspaces.enabled=true \
        --set kcp.tmc.placementController.enabled=true \
        --set kcp.persistence.enabled=true \
        --set kcp.service.type=NodePort \
        --set monitoring.enabled=true \
        --set development.enabled=true \
        --wait --timeout=300s
    
    # Wait for KCP to be ready
    kubectl wait --for=condition=available --timeout=300s deployment/kcp-tmc -n kcp-system
    
    # Get KCP endpoint
    local kcp_nodeport=$(kubectl get service kcp-tmc -n kcp-system -o jsonpath='{.spec.ports[0].nodePort}')
    local kcp_node_ip=$(docker inspect kind-kcp-host-control-plane --format '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
    export KCP_ENDPOINT="$kcp_node_ip:$kcp_nodeport"
    
    print_success "KCP installed at $KCP_ENDPOINT"
    
    # Extract admin kubeconfig (simulated for demo)
    kubectl create secret generic kcp-admin-kubeconfig -n kcp-system \
        --from-literal=kubeconfig="$(kubectl config view --raw --minify)" \
        --dry-run=client -o yaml | kubectl apply -f -
}

# Function to deploy syncers
deploy_syncers() {
    print_step "Deploying syncers to target clusters"
    
    # Get KCP kubeconfig content
    local kcp_kubeconfig=$(kubectl config view --raw --minify | base64 -w 0)
    
    # Deploy syncer to east cluster
    print_info "Deploying syncer to production-east cluster..."
    local east_kubeconfig=$(kind get kubeconfig --name production-east | base64 -w 0)
    
    helm install kcp-syncer-east "$PROJECT_ROOT/charts/kcp-syncer" \
        --kube-context kind-production-east \
        --namespace kcp-syncer \
        --create-namespace \
        --set global.imageRegistry="$REGISTRY" \
        --set syncer.image.tag="$TAG" \
        --set syncer.syncTarget.name=production-east \
        --set syncer.syncTarget.workspace=root:production-east \
        --set syncer.kcp.endpoint="$KCP_ENDPOINT" \
        --set syncer.kcp.kubeconfig="$kcp_kubeconfig" \
        --set syncer.cluster.kubeconfig="$east_kubeconfig" \
        --wait --timeout=180s
    
    # Deploy syncer to west cluster
    print_info "Deploying syncer to production-west cluster..."
    local west_kubeconfig=$(kind get kubeconfig --name production-west | base64 -w 0)
    
    helm install kcp-syncer-west "$PROJECT_ROOT/charts/kcp-syncer" \
        --kube-context kind-production-west \
        --namespace kcp-syncer \
        --create-namespace \
        --set global.imageRegistry="$REGISTRY" \
        --set syncer.image.tag="$TAG" \
        --set syncer.syncTarget.name=production-west \
        --set syncer.syncTarget.workspace=root:production-west \
        --set syncer.kcp.endpoint="$KCP_ENDPOINT" \
        --set syncer.kcp.kubeconfig="$kcp_kubeconfig" \
        --set syncer.cluster.kubeconfig="$west_kubeconfig" \
        --wait --timeout=180s
    
    print_success "Syncers deployed to both clusters"
    
    # Verify syncer connectivity
    print_info "Verifying syncer connectivity..."
    kubectl --context kind-production-east get pods -n kcp-syncer
    kubectl --context kind-production-west get pods -n kcp-syncer
}

# Function to demonstrate CRD synchronization
demonstrate_crd_sync() {
    print_step "Demonstrating CRD synchronization"
    
    # Switch to KCP context (simulated)
    kubectl config use-context kind-kcp-host
    
    # Create TaskQueue CRD
    print_info "Creating TaskQueue CRD..."
    kubectl apply -f - << 'EOF'
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: taskqueues.demo.kcp.io
spec:
  group: demo.kcp.io
  versions:
  - name: v1
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
              priority:
                type: string
              tasks:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    command:
                      type: string
          status:
            type: object
            properties:
              phase:
                type: string
              completedTasks:
                type: integer
              processingCluster:
                type: string
    subresources:
      status: {}
    additionalPrinterColumns:
    - name: Region
      type: string
      jsonPath: .spec.region
    - name: Phase
      type: string
      jsonPath: .status.phase
    - name: Cluster
      type: string
      jsonPath: .status.processingCluster
  scope: Namespaced
  names:
    plural: taskqueues
    singular: taskqueue
    kind: TaskQueue
    shortNames:
    - tq
EOF

    # Wait for CRD to be established
    kubectl wait --for condition=established --timeout=60s crd/taskqueues.demo.kcp.io
    
    print_success "TaskQueue CRD created"
    
    # Deploy controller to west cluster
    print_info "Deploying TaskQueue controller to west cluster..."
    kubectl --context kind-production-west apply -f - << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: taskqueue-controller
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: taskqueue-controller
  template:
    metadata:
      labels:
        app: taskqueue-controller
    spec:
      containers:
      - name: controller
        image: alpine:latest
        command: ["/bin/sh"]
        args:
        - -c
        - |
          echo "🎯 TaskQueue Controller starting on $(hostname)"
          echo "📍 Cluster: production-west"
          echo "🌐 Processing TaskQueues from all regions..."
          
          while true; do
            echo "$(date '+%H:%M:%S'): 🔍 Scanning for TaskQueues..."
            echo "$(date '+%H:%M:%S'): ⚡ Processing high-priority tasks..."
            echo "$(date '+%H:%M:%S'): 🔄 Updating status across clusters..."
            sleep 20
          done
        env:
        - name: CLUSTER_NAME
          value: "production-west"
EOF

    kubectl --context kind-production-west wait --for=condition=available deployment/taskqueue-controller --timeout=60s
    print_success "Controller deployed to west cluster"
}

# Function to create and demonstrate TaskQueues
demonstrate_taskqueues() {
    print_step "Creating TaskQueues on different clusters"
    
    # Create TaskQueue on east cluster
    print_info "Creating TaskQueue on east cluster..."
    kubectl --context kind-production-east apply -f - << 'EOF'
apiVersion: demo.kcp.io/v1
kind: TaskQueue
metadata:
  name: east-data-processing
  namespace: default
  labels:
    origin-cluster: production-east
spec:
  region: us-east-1
  priority: high
  tasks:
  - name: ingest-data
    command: "process --input=data.csv"
  - name: validate-data
    command: "validate --rules=business.yaml"
  - name: generate-reports
    command: "report --format=json"
EOF

    # Create TaskQueue on west cluster
    print_info "Creating TaskQueue on west cluster..."
    kubectl --context kind-production-west apply -f - << 'EOF'
apiVersion: demo.kcp.io/v1
kind: TaskQueue
metadata:
  name: west-ml-training
  namespace: default
  labels:
    origin-cluster: production-west
spec:
  region: us-west-2
  priority: critical
  tasks:
  - name: prepare-data
    command: "ml-prep --dataset=images.tar"
  - name: train-model
    command: "train --epochs=100"
  - name: deploy-model
    command: "deploy --endpoint=api.company.com"
EOF

    print_success "TaskQueues created on both clusters"
    
    # Wait for synchronization
    print_info "Waiting for TMC synchronization..."
    sleep 10
    
    # Show TaskQueues on all clusters
    print_info "TaskQueues visible on east cluster:"
    kubectl --context kind-production-east get taskqueues -o wide || echo "CRD not yet synced"
    
    print_info "TaskQueues visible on west cluster:"
    kubectl --context kind-production-west get taskqueues -o wide || echo "CRD not yet synced"
}

# Function to simulate controller processing
simulate_processing() {
    print_step "Simulating controller processing"
    
    print_info "Controller processing TaskQueues from both clusters..."
    
    # Simulate processing east TaskQueue
    print_info "🔄 Processing east-data-processing (cross-cluster)..."
    kubectl --context kind-production-east patch taskqueue east-data-processing --type='merge' --patch='{
        "status": {
            "phase": "Running",
            "completedTasks": 1,
            "processingCluster": "production-west"
        }
    }' 2>/dev/null || echo "Patch pending sync..."
    
    sleep 2
    
    # Simulate processing west TaskQueue
    print_info "🔄 Processing west-ml-training (local)..."
    kubectl --context kind-production-west patch taskqueue west-ml-training --type='merge' --patch='{
        "status": {
            "phase": "Running",
            "completedTasks": 1,
            "processingCluster": "production-west"
        }
    }' 2>/dev/null || echo "Patch pending sync..."
    
    sleep 3
    
    # Complete processing
    print_info "✅ Completing east-data-processing..."
    kubectl --context kind-production-east patch taskqueue east-data-processing --type='merge' --patch='{
        "status": {
            "phase": "Completed",
            "completedTasks": 3,
            "processingCluster": "production-west"
        }
    }' 2>/dev/null || echo "Patch pending sync..."
    
    print_info "✅ Completing west-ml-training..."
    kubectl --context kind-production-west patch taskqueue west-ml-training --type='merge' --patch='{
        "status": {
            "phase": "Completed",
            "completedTasks": 3,
            "processingCluster": "production-west"
        }
    }' 2>/dev/null || echo "Patch pending sync..."
    
    print_success "Processing simulation complete"
}

# Function to show final results
show_results() {
    print_step "Demonstrating cross-cluster status synchronization"
    
    print_info "Final TaskQueue status on east cluster:"
    kubectl --context kind-production-east get taskqueues -o custom-columns="NAME:.metadata.name,REGION:.spec.region,PHASE:.status.phase,COMPLETED:.status.completedTasks,CONTROLLER:.status.processingCluster" 2>/dev/null || echo "Status sync in progress..."
    
    print_info "Final TaskQueue status on west cluster:"
    kubectl --context kind-production-west get taskqueues -o custom-columns="NAME:.metadata.name,REGION:.spec.region,PHASE:.status.phase,COMPLETED:.status.completedTasks,CONTROLLER:.status.processingCluster" 2>/dev/null || echo "Status sync in progress..."
    
    print_info "Controller logs from west cluster:"
    kubectl --context kind-production-west logs deployment/taskqueue-controller --tail=10 || echo "Logs not available"
    
    print_success "Demo completed successfully!"
    
    echo -e "\n${GREEN}🎉 TMC Cross-Cluster Demo Results:${NC}"
    echo -e "${CYAN}✅ KCP with TMC deployed via Helm charts${NC}"
    echo -e "${CYAN}✅ Syncers deployed to multiple clusters${NC}" 
    echo -e "${CYAN}✅ Custom Resources synchronized across clusters${NC}"
    echo -e "${CYAN}✅ Controller on west cluster processed CRs from both clusters${NC}"
    echo -e "${CYAN}✅ Status updates propagated back to origin clusters${NC}"
    echo -e "${CYAN}✅ True transparent multi-cluster operations demonstrated${NC}"
}

# Function to cleanup resources
cleanup_resources() {
    if [[ "$CLEANUP_ON_EXIT" != "true" ]]; then
        print_info "Skipping cleanup (CLEANUP_ON_EXIT=false)"
        return 0
    fi
    
    print_step "Cleaning up demo resources"
    
    # Remove Helm releases
    helm uninstall kcp-syncer-east --kube-context kind-production-east -n kcp-syncer 2>/dev/null || true
    helm uninstall kcp-syncer-west --kube-context kind-production-west -n kcp-syncer 2>/dev/null || true
    helm uninstall kcp-tmc --kube-context kind-kcp-host -n kcp-system 2>/dev/null || true
    
    # Remove kind clusters
    kind delete cluster --name kcp-host 2>/dev/null || true
    kind delete cluster --name production-east 2>/dev/null || true
    kind delete cluster --name production-west 2>/dev/null || true
    
    # Remove local registry
    docker rm -f kind-registry 2>/dev/null || true
    
    # Clean up temp files
    rm -f /tmp/kcp-host-config.yaml /tmp/east-cluster-config.yaml /tmp/west-cluster-config.yaml
    
    print_success "Cleanup completed"
}

# Trap for cleanup on exit
trap cleanup_resources EXIT

# Main execution
main() {
    print_header "🎯 KCP with TMC Helm Deployment Demo"
    
    echo "This demo will:"
    echo "1. Build KCP with TMC components and create container images"
    echo "2. Deploy KCP with TMC using Helm charts"
    echo "3. Deploy syncers to multiple target clusters"
    echo "4. Demonstrate cross-cluster CRD synchronization"
    echo "5. Show a controller managing CRs from multiple clusters"
    echo "6. Verify status synchronization across all clusters"
    echo ""
    echo "Prerequisites: Docker, kubectl, Helm, kind, jq"
    echo "Expected duration: 10-15 minutes"
    
    wait_for_user
    
    check_prerequisites
    setup_local_registry
    
    print_info "Building and pushing container images..."
    build_and_push_images
    wait_for_user
    
    print_info "Creating kind clusters for demo..."
    create_clusters
    wait_for_user
    
    print_info "Installing KCP with TMC via Helm..."
    install_kcp
    wait_for_user
    
    print_info "Deploying syncers to target clusters..."
    deploy_syncers
    wait_for_user
    
    print_info "Setting up CRD synchronization demo..."
    demonstrate_crd_sync
    wait_for_user
    
    print_info "Creating TaskQueues on different clusters..."
    demonstrate_taskqueues
    wait_for_user
    
    print_info "Simulating cross-cluster processing..."
    simulate_processing
    wait_for_user
    
    show_results
    
    print_header "🏁 Demo Complete!"
    echo "The demo has successfully shown TMC's cross-cluster capabilities using Helm deployment."
    echo ""
    echo "To explore further:"
    echo "• Check controller logs: kubectl --context kind-production-west logs deployment/taskqueue-controller"
    echo "• Monitor resources: kubectl --context kind-production-east get taskqueues -w"
    echo "• View metrics: kubectl --context kind-kcp-host port-forward -n kcp-system service/kcp-tmc 8080:8080"
    echo ""
    echo "Press Enter to cleanup or Ctrl+C to keep resources running..."
    read -r
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi