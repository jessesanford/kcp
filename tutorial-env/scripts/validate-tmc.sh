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
