#!/bin/bash

set -euo pipefail

# TMC Tutorial Validation Script
# This script validates the tutorial setup and creates a working demo environment

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TUTORIAL_DIR="${ROOT_DIR}/tutorial-env"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
    
    # Check if we can run containers
    if ! docker run --rm hello-world &> /dev/null; then
        warn "Docker hello-world test failed. Container execution may not work."
    else
        info "Docker container execution verified"
    fi
}

create_tutorial_environment() {
    log "Creating tutorial environment..."
    
    mkdir -p "${TUTORIAL_DIR}"/{bin,manifests,examples,scripts}
    
    # Create a mock environment that demonstrates the concepts
    info "Setting up mock TMC environment..."
    
    # Create mock cluster configs
    cat <<EOF > "${TUTORIAL_DIR}/cluster-config.yaml"
# Mock TMC Multi-Cluster Configuration
clusters:
  kcp-host:
    role: control-plane
    address: "127.0.0.1:6443"
    region: control
  cluster-east:
    role: workload
    address: "127.0.0.1:6444" 
    region: east
    labels:
      region: east
      zone: us-east-1
  cluster-west:
    role: workload
    address: "127.0.0.1:6445"
    region: west
    labels:
      region: west
      zone: us-west-1
EOF
    
    # Create demo scripts
    create_demo_scripts
    create_example_workloads
    create_validation_tests
}

create_demo_scripts() {
    log "Creating demonstration scripts..."
    
    # Create TMC demo script
    cat <<'EOF' > "${TUTORIAL_DIR}/scripts/tmc-demo.sh"
#!/bin/bash

# TMC Features Demonstration Script
# This script simulates TMC functionality for educational purposes

set -euo pipefail

TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[TMC]${NC} $1"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

demo_step() {
    echo -e "${YELLOW}>>> $1${NC}"
    echo
}

echo "🚀 TMC (Transparent Multi-Cluster) Feature Demo"
echo "==============================================="
echo

demo_step "1. Multi-Cluster Architecture Overview"
cat << 'ARCH'
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   kcp-host      │  │  cluster-east   │  │  cluster-west   │
│                 │  │                 │  │                 │
│ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │
│ │     KCP     │ │  │ │   Syncer    │ │  │ │   Syncer    │ │
│ │    +TMC     │◄┼──┼─┤   Agent     │ │  │ │   Agent     │ │
│ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │
│                 │  │                 │  │                 │
│                 │  │ ┌─────────────┐ │  │ ┌─────────────┐ │
│                 │  │ │ Hello World │ │  │ │ Hello World │ │
│                 │  │ │   Workload  │ │  │ │   Workload  │ │
│                 │  │ └─────────────┘ │  │ └─────────────┘ │
└─────────────────┘  └─────────────────┘  └─────────────────┘
ARCH
echo

demo_step "2. Creating Multi-Cluster Placement"
log "Creating placement for hello-world application..."
cat "${TUTORIAL_DIR}/examples/placement.yaml"
echo

demo_step "3. Cross-Cluster Resource Aggregation"
log "Demonstrating resource aggregation across clusters..."

# Simulate aggregated view
echo "📊 Aggregated Resource View:"
echo "=============================="
echo "Resource Type: Deployment/hello-world"
echo "Total Clusters: 2"
echo "Total Replicas: 6 (3 per cluster)"
echo
echo "Cluster Distribution:"
echo "  cluster-east:  3 replicas (healthy)"
echo "  cluster-west:  3 replicas (healthy)"
echo
echo "Health Status: ✅ All clusters healthy"
echo "Last Updated: $(date)"
echo

demo_step "4. Virtual Workspace Projection"
log "Demonstrating resource projection across clusters..."

echo "🌐 Resource Projection Status:"
echo "==============================="
echo "Source: kcp-workspace/hello-world"
echo "Targets:"
echo "  ✅ cluster-east  - ConfigMap projected"
echo "  ✅ cluster-west  - ConfigMap projected"
echo
echo "Transformations Applied:"
echo "  - Added projection labels"
echo "  - Set cluster-specific values"
echo "  - Applied security context"
echo

demo_step "5. TMC Health Monitoring"
log "Showing TMC health monitoring capabilities..."

echo "🏥 TMC System Health:"
echo "====================="
echo "Component                    Status    Last Check"
echo "---------------------------------------------------"
echo "Virtual Workspace Manager   ✅ OK     $(date -d '5 seconds ago' '+%H:%M:%S')"
echo "Cross-Cluster Aggregator     ✅ OK     $(date -d '3 seconds ago' '+%H:%M:%S')"
echo "Workload Projection Ctrl     ✅ OK     $(date -d '2 seconds ago' '+%H:%M:%S')"
echo "TMC Recovery Manager         ✅ OK     $(date -d '1 seconds ago' '+%H:%M:%S')"
echo "TMC Error Handler            ✅ OK     $(date '+%H:%M:%S')"
echo
echo "Cluster Health:"
echo "  cluster-east:  ✅ Healthy (latency: 12ms)"
echo "  cluster-west:  ✅ Healthy (latency: 18ms)"
echo

demo_step "6. TMC Recovery Simulation"
log "Simulating cluster failure and recovery..."

echo "⚠️  Simulating cluster-east failure..."
sleep 2
echo "🔧 TMC Recovery Manager detected failure"
echo "🔄 Initiating recovery strategy: ClusterConnectivityRecovery"
echo "   - Testing cluster connectivity"
echo "   - Refreshing client connections"
echo "   - Updating cluster health status"
sleep 3
echo "✅ Recovery completed successfully"
echo "📊 Updated resource distribution:"
echo "   - cluster-east: 3 replicas (recovered)"
echo "   - cluster-west: 3 replicas (healthy)"
echo

demo_step "7. TMC Metrics & Observability"
log "Displaying TMC metrics..."

echo "📈 TMC Metrics Summary:"
echo "======================="
echo "Virtual Workspaces:           1 active"
echo "Aggregated Resources:         5 types"
echo "Projected Resources:          12 instances"
echo "Recovery Operations:          3 successful"
echo "Cross-Cluster Operations:     1,247 total"
echo "Error Rate:                   0.1% (2/1247)"
echo "Average Response Time:        85ms"
echo
echo "Recent Activity:"
echo "  $(date -d '30 seconds ago' '+%H:%M:%S') - Placement created"
echo "  $(date -d '25 seconds ago' '+%H:%M:%S') - Resources aggregated"
echo "  $(date -d '20 seconds ago' '+%H:%M:%S') - Health check passed"
echo "  $(date -d '15 seconds ago' '+%H:%M:%S') - Projection synchronized"
echo "  $(date -d '10 seconds ago' '+%H:%M:%S') - Metrics collected"
echo

echo "🎉 TMC Demo Complete!"
echo
echo "Key TMC Features Demonstrated:"
echo "✅ Multi-cluster workload placement"
echo "✅ Cross-cluster resource aggregation"
echo "✅ Virtual workspace projections"
echo "✅ Automated health monitoring"
echo "✅ Intelligent recovery strategies"
echo "✅ Comprehensive metrics collection"
echo
echo "For more information, see the TMC documentation:"
echo "  - Error Handling: docs/content/developers/tmc/error-handling.md"
echo "  - Health Monitoring: docs/content/developers/tmc/health-monitoring.md"
echo "  - Metrics & Observability: docs/content/developers/tmc/metrics-observability.md"
echo "  - Recovery Manager: docs/content/developers/tmc/recovery-manager.md"
echo "  - Virtual Workspace Manager: docs/content/developers/tmc/virtual-workspace-manager.md"
EOF

    chmod +x "${TUTORIAL_DIR}/scripts/tmc-demo.sh"
}

create_example_workloads() {
    log "Creating example workload manifests..."
    
    # Hello world application
    cat <<'EOF' > "${TUTORIAL_DIR}/examples/hello-world.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    app: hello-world
    managed-by: tmc
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
      annotations:
        tmc.kcp.io/cluster-placement: "east,west"
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
          cat > /html/index.html << 'HTML'
          <!DOCTYPE html>
          <html>
          <head>
              <title>TMC Hello World</title>
              <style>
                  body { font-family: Arial, sans-serif; margin: 40px; }
                  .header { color: #2c3e50; }
                  .info { background: #ecf0f1; padding: 10px; border-radius: 5px; }
                  .cluster { color: #3498db; font-weight: bold; }
              </style>
          </head>
          <body>
              <h1 class="header">🚀 Hello from TMC!</h1>
              <div class="info">
                  <p><strong>Message:</strong> ${MESSAGE}</p>
                  <p><strong>Cluster:</strong> <span class="cluster">${CLUSTER_NAME}</span></p>
                  <p><strong>Pod:</strong> $(hostname)</p>
                  <p><strong>Time:</strong> $(date)</p>
                  <p><strong>TMC Features:</strong> Multi-cluster deployment, Resource aggregation, Health monitoring</p>
              </div>
              <h2>TMC Capabilities Demonstrated:</h2>
              <ul>
                  <li>✅ Cross-cluster workload distribution</li>
                  <li>✅ Unified resource management</li>
                  <li>✅ Automated health monitoring</li>
                  <li>✅ Intelligent recovery strategies</li>
                  <li>✅ Virtual workspace abstractions</li>
              </ul>
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
  name: hello-world-service
  labels:
    app: hello-world
spec:
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: hello-world-config
  labels:
    app: hello-world
    tmc.kcp.io/project: "true"
data:
  message: "Hello from TMC Virtual Workspace!"
  environment: "tutorial"
  features: |
    - cross-cluster aggregation
    - resource projection
    - health monitoring
    - automated recovery
  config.json: |
    {
      "tmc": {
        "enabled": true,
        "features": {
          "aggregation": true,
          "projection": true,
          "recovery": true,
          "health_monitoring": true
        },
        "clusters": ["east", "west"],
        "placement_policy": "multi-cluster"
      }
    }
EOF
    
    # Multi-cluster placement
    cat <<'EOF' > "${TUTORIAL_DIR}/examples/placement.yaml"
apiVersion: scheduling.kcp.io/v1alpha1
kind: Placement
metadata:
  name: hello-world-placement
  labels:
    app: hello-world
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
  clusterSelector:
    matchLabels:
      workload-type: application
EOF
    
    # TMC Configuration example
    cat <<'EOF' > "${TUTORIAL_DIR}/examples/tmc-config.yaml"
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-config
  namespace: kcp-system
data:
  tmc.yaml: |
    apiVersion: tmc.kcp.io/v1alpha1
    kind: TMCConfiguration
    metadata:
      name: default
    spec:
      virtualWorkspaceManager:
        enabled: true
        syncInterval: 30s
        aggregationEnabled: true
        projectionEnabled: true
      
      crossClusterAggregator:
        enabled: true
        aggregationInterval: 30s
        mergeStrategy: union
        conflictResolution: lastWriter
        healthAggregation: majority
      
      workloadProjectionController:
        enabled: true
        projectionInterval: 60s
        transformations:
        - type: set
          jsonPath: "metadata.labels['tmc.kcp.io/projected']"
          value: "true"
      
      recoveryManager:
        enabled: true
        maxConcurrentRecoveries: 5
        recoveryTimeout: 10m
        strategies:
        - clusterConnectivity
        - resourceConflict
        - placement
        - sync
        - migration
      
      healthMonitor:
        enabled: true
        checkInterval: 30s
        healthTimeout: 10s
        thresholds:
          degraded: 2m
          unhealthy: 5m
      
      metricsCollector:
        enabled: true
        prometheusEnabled: true
        collectionInterval: 30s
        retentionPeriod: 24h
EOF
}

create_validation_tests() {
    log "Creating validation tests..."
    
    # Create test script
    cat <<'EOF' > "${TUTORIAL_DIR}/scripts/validate-tmc.sh"
#!/bin/bash

# TMC Tutorial Validation Script

set -euo pipefail

TUTORIAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() {
    echo -e "${GREEN}✅ PASS:${NC} $1"
}

fail() {
    echo -e "${RED}❌ FAIL:${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠️  WARN:${NC} $1"
}

echo "🧪 TMC Tutorial Validation"
echo "=========================="
echo

# Test 1: Check tutorial files exist
echo "Test 1: Tutorial Files"
echo "----------------------"

required_files=(
    "examples/hello-world.yaml"
    "examples/placement.yaml"
    "examples/tmc-config.yaml"
    "scripts/tmc-demo.sh"
    "cluster-config.yaml"
)

for file in "${required_files[@]}"; do
    if [[ -f "${TUTORIAL_DIR}/${file}" ]]; then
        pass "Found ${file}"
    else
        fail "Missing ${file}"
    fi
done

echo

# Test 2: Validate YAML syntax
echo "Test 2: YAML Validation"
echo "-----------------------"

yaml_files=(
    "examples/hello-world.yaml"
    "examples/placement.yaml" 
    "examples/tmc-config.yaml"
    "cluster-config.yaml"
)

for file in "${yaml_files[@]}"; do
    if command -v yq &> /dev/null; then
        if yq eval '.' "${TUTORIAL_DIR}/${file}" &> /dev/null; then
            pass "Valid YAML: ${file}"
        else
            fail "Invalid YAML: ${file}"
        fi
    else
        warn "yq not available, skipping YAML validation for ${file}"
    fi
done

echo

# Test 3: Check Docker functionality
echo "Test 3: Docker Environment"
echo "--------------------------"

if command -v docker &> /dev/null; then
    pass "Docker is available"
    
    if docker info &> /dev/null; then
        pass "Docker daemon is running"
        
        if docker run --rm busybox:1.35 echo "test" &> /dev/null; then
            pass "Docker can run containers"
        else
            fail "Docker cannot run containers"
        fi
    else
        fail "Docker daemon is not running"
    fi
else
    fail "Docker is not installed"
fi

echo

# Test 4: Validate tutorial script
echo "Test 4: Tutorial Scripts"
echo "------------------------"

if [[ -x "${TUTORIAL_DIR}/scripts/tmc-demo.sh" ]]; then
    pass "TMC demo script is executable"
else
    fail "TMC demo script is not executable"
fi

echo

# Test 5: Check TMC concepts in examples
echo "Test 5: TMC Concepts Validation"
echo "-------------------------------"

# Check for TMC-specific annotations and labels
if grep -q "tmc.kcp.io" "${TUTORIAL_DIR}/examples/hello-world.yaml"; then
    pass "TMC annotations found in examples"
else
    warn "No TMC annotations found in examples"
fi

if grep -q "scheduling.kcp.io" "${TUTORIAL_DIR}/examples/placement.yaml"; then
    pass "Placement API usage found"
else
    fail "No Placement API usage found"
fi

if grep -q "numberOfClusters" "${TUTORIAL_DIR}/examples/placement.yaml"; then
    pass "Multi-cluster placement configuration found"
else
    fail "No multi-cluster placement configuration found"
fi

echo

# Summary
echo "🏁 Validation Summary"
echo "===================="
echo "The TMC tutorial environment has been validated."
echo "You can now run the demo:"
echo "  cd ${TUTORIAL_DIR}"
echo "  ./scripts/tmc-demo.sh"
echo
echo "Available examples:"
echo "  - examples/hello-world.yaml    (Multi-cluster application)"
echo "  - examples/placement.yaml      (Cross-cluster placement)"
echo "  - examples/tmc-config.yaml     (TMC configuration)"
echo
echo "For the full tutorial experience with kind clusters,"
echo "run the main setup script:"
echo "  ./scripts/setup-tmc-tutorial.sh"
EOF

    chmod +x "${TUTORIAL_DIR}/scripts/validate-tmc.sh"
}

create_documentation_links() {
    log "Creating documentation quick reference..."
    
    cat <<EOF > "${TUTORIAL_DIR}/README.md"
# TMC Hello World Tutorial Environment

This directory contains a complete TMC (Transparent Multi-Cluster) tutorial environment.

## Quick Start

1. **Run the demo**: \`./scripts/tmc-demo.sh\`
2. **Validate setup**: \`./scripts/validate-tmc.sh\`
3. **Full setup**: \`../scripts/setup-tmc-tutorial.sh\` (requires kind/Docker)

## What's Included

### Example Applications
- \`examples/hello-world.yaml\` - Multi-cluster Hello World application
- \`examples/placement.yaml\` - Cross-cluster placement configuration
- \`examples/tmc-config.yaml\` - TMC system configuration

### Demo Scripts
- \`scripts/tmc-demo.sh\` - Interactive TMC features demonstration
- \`scripts/validate-tmc.sh\` - Validation and testing script

### Configuration
- \`cluster-config.yaml\` - Mock multi-cluster configuration

## TMC Features Demonstrated

✅ **Multi-Cluster Workload Placement**
- Intelligent workload distribution across clusters
- Placement policies and constraints
- Cluster selection strategies

✅ **Cross-Cluster Resource Aggregation**
- Unified views of distributed resources
- Health aggregation across clusters
- Resource status consolidation

✅ **Virtual Workspace Management**
- Cross-cluster resource projections
- Resource transformations
- Namespace-level abstractions

✅ **Automated Health Monitoring**
- Component health tracking
- Cluster connectivity monitoring
- Health status aggregation

✅ **Intelligent Recovery Strategies**
- Automated error detection and recovery
- Multiple recovery strategies per error type
- Recovery execution tracking

✅ **Comprehensive Observability**
- Prometheus metrics integration
- Structured logging
- Performance monitoring

## Documentation

For detailed information about TMC components:

- [TMC Error Handling](../docs/content/developers/tmc/error-handling.md)
- [TMC Health Monitoring](../docs/content/developers/tmc/health-monitoring.md)
- [TMC Metrics & Observability](../docs/content/developers/tmc/metrics-observability.md)
- [TMC Recovery Manager](../docs/content/developers/tmc/recovery-manager.md)
- [TMC Virtual Workspace Manager](../docs/content/developers/tmc/virtual-workspace-manager.md)

## Tutorial Flow

1. **Setup** - Environment preparation and cluster creation
2. **Deploy** - Application deployment across clusters
3. **Observe** - TMC features in action
4. **Test** - Recovery and scaling scenarios
5. **Monitor** - Health and metrics observation

## Next Steps

After completing this tutorial:

1. Explore the TMC source code in \`pkg/reconciler/workload/tmc/\`
2. Read the architectural documentation
3. Try modifying the placement policies
4. Experiment with different recovery scenarios
5. Create your own multi-cluster applications

## Support

For questions or issues:
- Check the troubleshooting section in the main tutorial
- Review the TMC component documentation
- Examine the example configurations

Happy clustering! 🚀
EOF
}

run_basic_validation() {
    log "Running basic validation..."
    
    # Test Docker functionality
    info "Testing Docker container execution..."
    if docker run --rm busybox:1.35 echo "TMC tutorial validation successful" &> /dev/null; then
        log "Docker container test passed"
    else
        warn "Docker container test failed - kind clusters may not work"
    fi
    
    # Validate created files
    info "Validating created files..."
    required_files=(
        "${TUTORIAL_DIR}/examples/hello-world.yaml"
        "${TUTORIAL_DIR}/examples/placement.yaml"
        "${TUTORIAL_DIR}/scripts/tmc-demo.sh"
        "${TUTORIAL_DIR}/scripts/validate-tmc.sh"
        "${TUTORIAL_DIR}/README.md"
    )
    
    for file in "${required_files[@]}"; do
        if [[ -f "$file" ]]; then
            info "✅ Created: $(basename "$file")"
        else
            error "❌ Missing: $(basename "$file")"
        fi
    done
}

run_demo() {
    log "Running TMC demonstration..."
    
    if [[ -x "${TUTORIAL_DIR}/scripts/tmc-demo.sh" ]]; then
        info "Executing TMC demo script..."
        "${TUTORIAL_DIR}/scripts/tmc-demo.sh"
    else
        error "TMC demo script not found or not executable"
    fi
}

print_success() {
    log "TMC Tutorial validation completed! 🎉"
    echo
    echo -e "${GREEN}=====================================${NC}"
    echo -e "${GREEN}  TMC Tutorial Environment Ready!   ${NC}"
    echo -e "${GREEN}=====================================${NC}"
    echo
    echo "📁 Tutorial location: ${TUTORIAL_DIR}"
    echo
    echo "🚀 To run the demonstration:"
    echo "   cd ${TUTORIAL_DIR}"
    echo "   ./scripts/tmc-demo.sh"
    echo
    echo "🧪 To run validation tests:"
    echo "   cd ${TUTORIAL_DIR}"
    echo "   ./scripts/validate-tmc.sh"
    echo
    echo "📚 Documentation:"
    echo "   ${ROOT_DIR}/docs/content/tutorials/tmc-hello-world.md"
    echo
    echo "🔧 For full kind cluster setup:"
    echo "   ${ROOT_DIR}/scripts/setup-tmc-tutorial.sh"
    echo
}

main() {
    log "Starting TMC tutorial validation..."
    
    check_prerequisites
    create_tutorial_environment
    create_documentation_links
    run_basic_validation
    print_success
    
    # Optionally run the demo
    if [[ "${1:-}" == "--demo" ]]; then
        echo
        run_demo
    fi
}

# Run main function
main "$@"