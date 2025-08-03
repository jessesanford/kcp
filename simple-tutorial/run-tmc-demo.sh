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
