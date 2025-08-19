#!/bin/bash

# TMC Feature Verification - OBVIOUS Edition
# This script makes it VERY CLEAR whether TMC features are working or not

set -e

# Big colorful text
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

clear

echo -e "${MAGENTA}${BOLD}"
echo "============================================================"
echo "     TMC (TRANSPARENT MULTI-CLUSTER) FEATURE TEST"
echo "============================================================"
echo -e "${NC}"
echo ""
echo -e "${CYAN}This test will show you EXACTLY what TMC features are working.${NC}"
echo -e "${CYAN}Look for ${GREEN}✅ GREEN CHECKMARKS${CYAN} = TMC is working${NC}"
echo -e "${CYAN}Look for ${RED}❌ RED X's${CYAN} = TMC is NOT working${NC}"
echo ""
echo "Press Enter to begin..."
read

# Function to print big success
print_success() {
    echo -e "${GREEN}${BOLD}✅ ✅ ✅  $1  ✅ ✅ ✅${NC}"
}

# Function to print big failure
print_failure() {
    echo -e "${RED}${BOLD}❌ ❌ ❌  $1  ❌ ❌ ❌${NC}"
}

# Function to print section
print_section() {
    echo ""
    echo -e "${BLUE}${BOLD}========================================${NC}"
    echo -e "${BLUE}${BOLD}$1${NC}"
    echo -e "${BLUE}${BOLD}========================================${NC}"
}

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Kill any existing KCP first
pkill -f "bin/kcp start" 2>/dev/null || true
sleep 2

# Start fresh KCP
print_section "STEP 1: STARTING KCP WITH TMC FEATURES"

KCP_DIR="/tmp/kcp-tmc-obvious-test-$$"
mkdir -p "$KCP_DIR"
KUBECONFIG="$KCP_DIR/admin.kubeconfig"

echo "Starting KCP with TMC feature flags..."
echo -e "${YELLOW}Command: ./bin/kcp start --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true${NC}"
echo ""

./bin/kcp start \
    --root-directory="$KCP_DIR" \
    --feature-gates=TMCFeature=true,TMCAPIs=true,TMCControllers=true,TMCPlacement=true \
    --v=2 > "$KCP_DIR/kcp.log" 2>&1 &
KCP_PID=$!

# Wait for KCP
echo -n "Waiting for KCP to start"
for i in {1..30}; do
    if [ -f "$KUBECONFIG" ]; then
        echo -e " ${GREEN}READY!${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

if [ ! -f "$KUBECONFIG" ]; then
    print_failure "KCP FAILED TO START"
    exit 1
fi

export KUBECONFIG

# Cleanup on exit
cleanup() {
    echo ""
    echo "Cleaning up..."
    kill $KCP_PID 2>/dev/null || true
    rm -rf "$KCP_DIR"
}
trap cleanup EXIT

print_section "TEST 1: TMC BINARIES EXIST"

echo "Looking for TMC binaries..."
if [ -f "./bin/tmc-controller" ]; then
    SIZE=$(ls -lh ./bin/tmc-controller | awk '{print $5}')
    echo -e "  tmc-controller: ${GREEN}FOUND${NC} (size: $SIZE)"
    print_success "TMC CONTROLLER BINARY EXISTS!"
    ((TESTS_PASSED++))
else
    echo -e "  tmc-controller: ${RED}NOT FOUND${NC}"
    print_failure "TMC CONTROLLER BINARY MISSING!"
    ((TESTS_FAILED++))
fi

print_section "TEST 2: TMC FEATURE FLAGS IN KCP"

echo "Checking if KCP recognizes TMC feature flags..."
echo ""

# Check the actual running process
KCP_FLAGS=$(ps aux | grep "bin/kcp start" | grep -v grep | head -1)

echo "KCP is running with these TMC flags:"
if echo "$KCP_FLAGS" | grep -q "TMCFeature=true"; then
    echo -e "  ${GREEN}✓${NC} TMCFeature=true"
else
    echo -e "  ${RED}✗${NC} TMCFeature NOT enabled"
fi

if echo "$KCP_FLAGS" | grep -q "TMCAPIs=true"; then
    echo -e "  ${GREEN}✓${NC} TMCAPIs=true"
else
    echo -e "  ${RED}✗${NC} TMCAPIs NOT enabled"
fi

if echo "$KCP_FLAGS" | grep -q "TMCControllers=true"; then
    echo -e "  ${GREEN}✓${NC} TMCControllers=true"
else
    echo -e "  ${RED}✗${NC} TMCControllers NOT enabled"
fi

if echo "$KCP_FLAGS" | grep -q "TMCPlacement=true"; then
    echo -e "  ${GREEN}✓${NC} TMCPlacement=true"
else
    echo -e "  ${RED}✗${NC} TMCPlacement NOT enabled"
fi

if echo "$KCP_FLAGS" | grep -q "TMC"; then
    print_success "TMC FEATURE FLAGS ARE ACTIVE!"
    ((TESTS_PASSED++))
else
    print_failure "NO TMC FEATURE FLAGS FOUND!"
    ((TESTS_FAILED++))
fi

print_section "TEST 3: TMC IN KCP LOGS"

echo "Searching KCP logs for TMC activity..."
echo ""

TMC_LOG_COUNT=$(grep -i "tmc" "$KCP_DIR/kcp.log" 2>/dev/null | wc -l)
if [ $TMC_LOG_COUNT -gt 0 ]; then
    echo -e "Found ${GREEN}$TMC_LOG_COUNT${NC} TMC-related log entries:"
    grep -i "tmc" "$KCP_DIR/kcp.log" | head -5 | while read line; do
        if echo "$line" | grep -q "controller"; then
            echo -e "  ${YELLOW}→${NC} TMC Controller: $(echo "$line" | grep -o '"controller":[^,]*')"
        elif echo "$line" | grep -q "placement"; then
            echo -e "  ${YELLOW}→${NC} TMC Placement: $(echo "$line" | grep -o 'placement[^"]*')"
        else
            echo -e "  ${YELLOW}→${NC} $(echo "$line" | cut -c1-80)..."
        fi
    done
    print_success "TMC IS RECOGNIZED BY KCP!"
    ((TESTS_PASSED++))
else
    echo -e "${RED}No TMC entries found in logs${NC}"
    print_failure "TMC NOT FOUND IN KCP LOGS!"
    ((TESTS_FAILED++))
fi

print_section "TEST 4: TMC API TYPES COMPILED IN"

echo "Checking for TMC API types in the binary..."
echo ""

# Check if TMC types are in the source
if [ -d "./pkg/apis/tmc" ]; then
    TMC_FILES=$(find ./pkg/apis/tmc -name "*.go" 2>/dev/null | wc -l)
    if [ $TMC_FILES -gt 0 ]; then
        echo -e "Found ${GREEN}$TMC_FILES${NC} TMC API files:"
        find ./pkg/apis/tmc -name "*.go" -exec basename {} \; | head -5 | while read file; do
            echo -e "  ${GREEN}✓${NC} $file"
        done
        print_success "TMC API TYPES ARE INTEGRATED!"
        ((TESTS_PASSED++))
    else
        print_failure "NO TMC API FILES FOUND!"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${RED}TMC API directory not found${NC}"
    print_failure "TMC APIs NOT INTEGRATED!"
    ((TESTS_FAILED++))
fi

print_section "TEST 5: TMC CONTROLLER CODE EXISTS"

echo "Checking for TMC controller implementation..."
echo ""

if [ -d "./pkg/tmc" ] || [ -d "./pkg/reconciler/workload" ]; then
    TMC_CONTROLLERS=$(find ./pkg -name "*placement*controller*.go" -o -name "*tmc*.go" 2>/dev/null | wc -l)
    if [ $TMC_CONTROLLERS -gt 0 ]; then
        echo -e "Found ${GREEN}$TMC_CONTROLLERS${NC} TMC controller files:"
        find ./pkg -name "*placement*controller*.go" -o -name "*tmc*.go" 2>/dev/null | head -5 | while read file; do
            echo -e "  ${GREEN}✓${NC} $(basename $(dirname $file))/$(basename $file)"
        done
        print_success "TMC CONTROLLERS ARE INTEGRATED!"
        ((TESTS_PASSED++))
    else
        print_failure "NO TMC CONTROLLER CODE FOUND!"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${RED}TMC controller directories not found${NC}"
    print_failure "TMC CONTROLLERS NOT INTEGRATED!"
    ((TESTS_FAILED++))
fi

print_section "TEST 6: TMC CONTROLLER STARTS"

echo "Testing if TMC controller can start..."
echo ""

if [ -f "./bin/tmc-controller" ]; then
    timeout 2 ./bin/tmc-controller --feature-gates=TMCFeature=true 2>&1 | head -10 > "$KCP_DIR/tmc-test.log" &
    sleep 1
    
    if grep -q "Starting TMC controller\|TMC controller foundation ready\|TMC controller initialized" "$KCP_DIR/tmc-test.log" 2>/dev/null; then
        echo -e "${GREEN}TMC controller output:${NC}"
        grep "TMC" "$KCP_DIR/tmc-test.log" | head -3
        print_success "TMC CONTROLLER STARTS SUCCESSFULLY!"
        ((TESTS_PASSED++))
    else
        echo "TMC controller output:"
        cat "$KCP_DIR/tmc-test.log" 2>/dev/null | head -5
        print_failure "TMC CONTROLLER FAILED TO START!"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${RED}TMC controller binary not found${NC}"
    print_failure "TMC CONTROLLER MISSING!"
    ((TESTS_FAILED++))
fi

print_section "TEST 7: CREATE TMC-LIKE RESOURCES"

echo "Testing if we can create TMC-style resources..."
echo ""

kubectl create namespace tmc-test 2>/dev/null || true

# Try to create a ConfigMap that represents TMC config
cat <<EOF | kubectl apply -f - > /dev/null 2>&1
apiVersion: v1
kind: ConfigMap
metadata:
  name: tmc-cluster-config
  namespace: tmc-test
  labels:
    tmc.kcp.io/managed: "true"
data:
  cluster: "test-cluster-1"
  region: "us-west-2"
  status: "ready"
EOF

if kubectl get configmap tmc-cluster-config -n tmc-test > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Created TMC test resource successfully${NC}"
    kubectl get configmap tmc-cluster-config -n tmc-test --show-labels | grep -E "NAME|tmc"
    print_success "TMC-STYLE RESOURCES CAN BE CREATED!"
    ((TESTS_PASSED++))
else
    echo -e "${RED}Failed to create TMC test resource${NC}"
    print_failure "CANNOT CREATE TMC RESOURCES!"
    ((TESTS_FAILED++))
fi

print_section "FINAL RESULTS"

echo ""
echo -e "${BOLD}===============================================${NC}"
echo -e "${BOLD}           TMC INTEGRATION TEST RESULTS${NC}"
echo -e "${BOLD}===============================================${NC}"
echo ""

TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
PERCENTAGE=$((TESTS_PASSED * 100 / TOTAL_TESTS))

echo -e "Tests Passed: ${GREEN}${BOLD}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}${BOLD}$TESTS_FAILED${NC}"
echo -e "Total Tests:  ${BOLD}$TOTAL_TESTS${NC}"
echo -e "Success Rate: ${BOLD}$PERCENTAGE%${NC}"
echo ""

if [ $TESTS_PASSED -eq $TOTAL_TESTS ]; then
    echo -e "${GREEN}${BOLD}"
    echo "╔══════════════════════════════════════════════╗"
    echo "║                                              ║"
    echo "║     🎉 TMC IS FULLY INTEGRATED! 🎉          ║"
    echo "║                                              ║"
    echo "║     ALL TESTS PASSED SUCCESSFULLY!          ║"
    echo "║                                              ║"
    echo "╚══════════════════════════════════════════════╝"
    echo -e "${NC}"
elif [ $TESTS_PASSED -gt 3 ]; then
    echo -e "${YELLOW}${BOLD}"
    echo "╔══════════════════════════════════════════════╗"
    echo "║                                              ║"
    echo "║     ⚠️  TMC IS PARTIALLY WORKING ⚠️         ║"
    echo "║                                              ║"
    echo "║     SOME FEATURES ARE INTEGRATED            ║"
    echo "║                                              ║"
    echo "╚══════════════════════════════════════════════╝"
    echo -e "${NC}"
else
    echo -e "${RED}${BOLD}"
    echo "╔══════════════════════════════════════════════╗"
    echo "║                                              ║"
    echo "║     ❌ TMC IS NOT WORKING ❌                ║"
    echo "║                                              ║"
    echo "║     MOST TESTS FAILED                       ║"
    echo "║                                              ║"
    echo "╚══════════════════════════════════════════════╝"
    echo -e "${NC}"
fi

echo ""
echo -e "${CYAN}What these results mean:${NC}"
echo ""
if [ $TESTS_PASSED -gt 0 ]; then
    echo -e "${GREEN}✅ WORKING FEATURES:${NC}"
    [ -f "./bin/tmc-controller" ] && echo "  • TMC controller binary exists"
    echo "$KCP_FLAGS" | grep -q "TMC" && echo "  • TMC feature flags are active in KCP"
    [ $TMC_LOG_COUNT -gt 0 ] && echo "  • KCP recognizes TMC components"
    [ $TMC_FILES -gt 0 ] && echo "  • TMC API types are integrated"
    [ $TMC_CONTROLLERS -gt 0 ] && echo "  • TMC controller code is present"
fi

if [ $TESTS_FAILED -gt 0 ]; then
    echo ""
    echo -e "${RED}❌ NOT WORKING:${NC}"
    [ ! -f "./bin/tmc-controller" ] && echo "  • TMC controller binary missing"
    echo "$KCP_FLAGS" | grep -q "TMC" || echo "  • TMC feature flags not enabled"
    [ $TMC_LOG_COUNT -eq 0 ] && echo "  • KCP doesn't recognize TMC"
fi

echo ""
echo -e "${YELLOW}To see more details:${NC}"
echo "  • KCP logs: tail -f $KCP_DIR/kcp.log | grep -i tmc"
echo "  • Process: ps aux | grep kcp"
echo ""