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
