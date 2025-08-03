#!/bin/bash

# Helm Chart Validation Script
# This script validates the KCP TMC Helm charts for correctness

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHARTS_DIR="$PROJECT_ROOT/charts"

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Function to validate chart structure
validate_chart_structure() {
    local chart_name=$1
    local chart_path="$CHARTS_DIR/$chart_name"
    
    print_info "Validating $chart_name chart structure..."
    
    # Check required files
    local required_files=(
        "Chart.yaml"
        "values.yaml"
        "templates/_helpers.tpl"
    )
    
    for file in "${required_files[@]}"; do
        if [[ -f "$chart_path/$file" ]]; then
            print_success "$chart_name: $file exists"
        else
            print_error "$chart_name: Missing required file $file"
            return 1
        fi
    done
    
    # Check Chart.yaml content
    if grep -q "apiVersion: v2" "$chart_path/Chart.yaml"; then
        print_success "$chart_name: Chart.yaml has correct API version"
    else
        print_error "$chart_name: Chart.yaml missing or invalid API version"
        return 1
    fi
    
    return 0
}

# Function to lint charts
lint_charts() {
    print_info "Linting Helm charts..."
    
    local charts=("kcp-tmc" "kcp-syncer")
    local lint_failed=false
    
    for chart in "${charts[@]}"; do
        print_info "Linting $chart chart..."
        if helm lint "$CHARTS_DIR/$chart" --quiet; then
            print_success "$chart: Lint passed"
        else
            print_error "$chart: Lint failed"
            lint_failed=true
        fi
    done
    
    if [[ "$lint_failed" == "true" ]]; then
        return 1
    fi
    
    return 0
}

# Function to validate templates
validate_templates() {
    print_info "Validating chart templates..."
    
    local charts=("kcp-tmc" "kcp-syncer")
    
    for chart in "${charts[@]}"; do
        print_info "Validating $chart templates..."
        
        # Test with default values
        if helm template test-release "$CHARTS_DIR/$chart" --debug > /tmp/${chart}-default.yaml 2>/dev/null; then
            print_success "$chart: Default template generation successful"
        else
            print_error "$chart: Default template generation failed"
            return 1
        fi
        
        # Test with custom values
        cat > /tmp/${chart}-test-values.yaml << 'EOF'
global:
  imageRegistry: "test-registry.com"

image:
  tag: "test-tag"
EOF
        
        if helm template test-release "$CHARTS_DIR/$chart" --values /tmp/${chart}-test-values.yaml > /tmp/${chart}-custom.yaml 2>/dev/null; then
            print_success "$chart: Custom values template generation successful"
        else
            print_error "$chart: Custom values template generation failed"
            return 1
        fi
    done
    
    return 0
}

# Function to validate KCP TMC specific features
validate_kcp_tmc_features() {
    print_info "Validating KCP TMC specific features..."
    
    local template_output
    template_output=$(helm template test-release "$CHARTS_DIR/kcp-tmc" --set kcp.tmc.enabled=true)
    
    # Check for TMC-specific command arguments
    if echo "$template_output" | grep -q "enable-tmc"; then
        print_success "KCP TMC: TMC flag present in deployment"
    else
        print_error "KCP TMC: TMC flag missing from deployment"
        return 1
    fi
    
    # Check for TMC metrics port
    if echo "$template_output" | grep -q "containerPort: 8080"; then
        print_success "KCP TMC: Metrics port configured"
    else
        print_warning "KCP TMC: Metrics port not found (may be conditional)"
    fi
    
    # Check for ServiceMonitor
    template_output=$(helm template test-release "$CHARTS_DIR/kcp-tmc" --set monitoring.prometheus.serviceMonitor.enabled=true)
    if echo "$template_output" | grep -q "ServiceMonitor"; then
        print_success "KCP TMC: ServiceMonitor template present"
    else
        print_error "KCP TMC: ServiceMonitor template missing"
        return 1
    fi
    
    return 0
}

# Function to validate syncer specific features
validate_syncer_features() {
    print_info "Validating KCP Syncer specific features..."
    
    local template_output
    template_output=$(helm template test-release "$CHARTS_DIR/kcp-syncer" \
        --set syncer.syncTarget.name=test-target \
        --set syncer.kcp.endpoint=https://kcp.test.com:6443)
    
    # Check for required syncer arguments
    if echo "$template_output" | grep -q "sync-target-name=test-target"; then
        print_success "KCP Syncer: Sync target name configured"
    else
        print_error "KCP Syncer: Sync target name missing"
        return 1
    fi
    
    # Check for kubeconfig volumes
    if echo "$template_output" | grep -q "kcp-kubeconfig"; then
        print_success "KCP Syncer: KCP kubeconfig volume present"
    else
        print_error "KCP Syncer: KCP kubeconfig volume missing"
        return 1
    fi
    
    if echo "$template_output" | grep -q "cluster-kubeconfig"; then
        print_success "KCP Syncer: Cluster kubeconfig volume present"
    else
        print_error "KCP Syncer: Cluster kubeconfig volume missing"
        return 1
    fi
    
    return 0
}

# Function to validate Docker files
validate_docker_files() {
    print_info "Validating Docker files..."
    
    local docker_files=(
        "$PROJECT_ROOT/docker/Dockerfile.tmc"
        "$PROJECT_ROOT/docker/health-check.sh"
        "$PROJECT_ROOT/docker/syncer-health-check.sh"
        "$PROJECT_ROOT/docker/tmc-startup.sh"
    )
    
    for file in "${docker_files[@]}"; do
        if [[ -f "$file" ]]; then
            print_success "Docker: $(basename $file) exists"
        else
            print_error "Docker: $(basename $file) missing"
            return 1
        fi
    done
    
    # Validate Dockerfile syntax (basic check)
    if grep -q "FROM.*AS.*" "$PROJECT_ROOT/docker/Dockerfile.tmc"; then
        print_success "Docker: Multi-stage Dockerfile structure correct"
    else
        print_error "Docker: Multi-stage Dockerfile structure invalid"
        return 1
    fi
    
    return 0
}

# Function to validate scripts
validate_scripts() {
    print_info "Validating scripts..."
    
    local scripts=(
        "$PROJECT_ROOT/scripts/helm-demo.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [[ -f "$script" && -x "$script" ]]; then
            print_success "Scripts: $(basename $script) exists and is executable"
        else
            print_error "Scripts: $(basename $script) missing or not executable"
            return 1
        fi
        
        # Basic syntax check
        if bash -n "$script" 2>/dev/null; then
            print_success "Scripts: $(basename $script) syntax valid"
        else
            print_error "Scripts: $(basename $script) syntax error"
            return 1
        fi
    done
    
    return 0
}

# Function to check dependencies
check_dependencies() {
    print_info "Checking dependencies..."
    
    local required_tools=("helm" "docker" "kubectl")
    local missing_tools=()
    
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" >/dev/null 2>&1; then
            print_success "Dependency: $tool available"
        else
            print_warning "Dependency: $tool not available (required for full functionality)"
            missing_tools+=("$tool")
        fi
    done
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        print_info "Missing tools: ${missing_tools[*]}"
        print_info "Some validations may be skipped"
    fi
    
    return 0
}

# Function to validate documentation
validate_documentation() {
    print_info "Validating documentation..."
    
    local docs=(
        "$PROJECT_ROOT/BUILD-TMC.md"
        "$PROJECT_ROOT/helm-deployment-demo.md"
        "$PROJECT_ROOT/charts/README.md"
    )
    
    for doc in "${docs[@]}"; do
        if [[ -f "$doc" ]]; then
            print_success "Documentation: $(basename $doc) exists"
            
            # Check for basic markdown structure
            if grep -q "^#" "$doc"; then
                print_success "Documentation: $(basename $doc) has proper headers"
            else
                print_warning "Documentation: $(basename $doc) may be missing headers"
            fi
        else
            print_error "Documentation: $(basename $doc) missing"
            return 1
        fi
    done
    
    return 0
}

# Main validation function
main() {
    echo -e "${BLUE}🔍 KCP TMC Helm Charts Validation${NC}\n"
    
    local validation_failed=false
    
    # Run all validations
    check_dependencies || validation_failed=true
    validate_chart_structure "kcp-tmc" || validation_failed=true
    validate_chart_structure "kcp-syncer" || validation_failed=true
    
    # Only run Helm-specific validations if Helm is available
    if command -v helm >/dev/null 2>&1; then
        lint_charts || validation_failed=true
        validate_templates || validation_failed=true
        validate_kcp_tmc_features || validation_failed=true
        validate_syncer_features || validation_failed=true
    else
        print_warning "Skipping Helm validations (helm not available)"
    fi
    
    validate_docker_files || validation_failed=true
    validate_scripts || validation_failed=true
    validate_documentation || validation_failed=true
    
    # Cleanup temp files
    rm -f /tmp/kcp-tmc-*.yaml /tmp/kcp-syncer-*.yaml
    
    echo ""
    if [[ "$validation_failed" == "true" ]]; then
        print_error "Validation completed with errors"
        echo -e "${RED}❌ Some validations failed. Please review the errors above.${NC}"
        return 1
    else
        print_success "All validations passed"
        echo -e "${GREEN}🎉 KCP TMC Helm charts are ready for deployment!${NC}"
        echo ""
        echo -e "${BLUE}Next steps:${NC}"
        echo "1. Build and push container images: see BUILD-TMC.md"
        echo "2. Deploy with Helm: see charts/README.md"
        echo "3. Run the complete demo: ./scripts/helm-demo.sh"
        return 0
    fi
}

# Run main function
main "$@"