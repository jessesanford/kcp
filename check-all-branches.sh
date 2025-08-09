#!/bin/bash

# TMC Branch Line Counter - Check All Branches
echo "=========================================="
echo "TMC Branch Analysis Report"
echo "=========================================="
echo "Date: $(date)"
echo ""

# List of all TMC branches from worktree listing
declare -a branches=(
    "feature/tmc2-impl2/00a1-controller-patterns"
    "feature/tmc2-impl2/00b1-workspace-isolation"
    "feature/tmc2-impl2/00c-feature-flags"
    "feature/tmc2-impl2/01a-cluster-basic"
    "feature/tmc2-impl2/01b-cluster-enhanced"
    "feature/tmc2-impl2/01c-placement-basic"
    "feature/tmc2-impl2/01d-placement-advanced"
    "feature/tmc2-impl2/02a-core-apis"
    "feature/tmc2-impl2/02a1-apiexport-core"
    "feature/tmc2-impl2/02a2-apiexport-schemas"
    "feature/tmc2-impl2/02b-advanced-apis"
    "feature/tmc2-impl2/03a-cluster-api"
    "feature/tmc2-impl2/03a-controller-binary"
    "feature/tmc2-impl2/03b-controller-config"
    "feature/tmc2-impl2/04c-placement-controller"
    "feature/tmc2-impl2/05a2a-api-foundation"
    "feature/tmc2-impl2/05a2b-decision-engine"
    "feature/tmc2-impl2/05a2c-controller-integration"
    "feature/tmc2-impl2/05a2c1a-api-server"
    "feature/tmc2-impl2/05a2c1b-api-helpers"
    "feature/tmc2-impl2/05a2c2-observability"
    "feature/tmc2-impl2/05a2c2a-aggregation"
    "feature/tmc2-impl2/05a2c2b1-dashboard-core"
    "feature/tmc2-impl2/05a2c2b2-dashboard-assets"
    "feature/tmc2-impl2/05a2d-rest-mapper"
    "feature/tmc2-impl2/05b-cluster-controller"
    "feature/tmc2-impl2/05b1-basic-registration"
    "feature/tmc2-impl2/05b2-config-crds"
    "feature/tmc2-impl2/05b3-apply-configs"
    "feature/tmc2-impl2/05b3a-apply-configs-cr"
    "feature/tmc2-impl2/05b3b-apply-configs-wp"
    "feature/tmc2-impl2/05b7-registration-controller"
    "feature/tmc2-impl2/05b7a-controller-base"
    "feature/tmc2-impl2/05b7b-capabilities"
    "feature/tmc2-impl2/05c2-api-types"
    "feature/tmc2-impl2/05d3-factory-core"
    "feature/tmc2-impl2/05e-status-aggregation"
    "feature/tmc2-impl2/05e1-collection-framework"
    "feature/tmc2-impl2/05e2-aggregation-logic"
    "feature/tmc2-impl2/05e3-transformation"
    "feature/tmc2-impl2/05f2-cluster-health"
    "feature/tmc2-impl2/05g1-api-types"
    "feature/tmc2-impl2/05g2-hpa-policy"
    "feature/tmc2-impl2/05g3-observability-base"
    "feature/tmc2-impl2/05g4-metrics-collector"
    "feature/tmc2-impl2/05g5-hpa-controller"
    "feature/tmc2-impl2/05h2b-collectors-clean"
    "feature/tmc2-impl2/05h2c-consolidation"
    "feature/tmc2-impl2/05h2c3-consolidation-integration"
    "feature/tmc2-impl2/05h3-metrics-storage"
    "feature/tmc2-impl2/05h4-metrics-api"
    "feature/tmc2-impl2/05h5-dashboards"
)

# Output file for results
output_file="/tmp/tmc-branch-analysis.txt"
echo "TMC Branch Analysis Report - $(date)" > $output_file
echo "========================================" >> $output_file

for branch in "${branches[@]}"; do
    echo "Analyzing: $branch"
    echo "Branch: $branch" >> $output_file
    echo "----------------------------------------" >> $output_file
    
    # Check if branch exists
    if git show-ref --quiet "refs/heads/$branch" || git show-ref --quiet "refs/remotes/origin/$branch"; then
        # Run line counter and capture results
        result=$(/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh -c "$branch" 2>&1)
        
        # Extract key metrics from result
        impl_lines=$(echo "$result" | grep "Hand-written Lines" | sed 's/.*| \([0-9]*\) .*/\1/')
        test_lines=$(echo "$result" | grep "Test Coverage Lines" | sed 's/.*| \([0-9]*\) .*/\1/')
        status=$(echo "$result" | grep -E "(✅|❌)" | head -1 | sed 's/.*\(✅\|❌\).*/\1/')
        
        echo "  Implementation Lines: ${impl_lines:-"N/A"}"
        echo "  Test Lines: ${test_lines:-"N/A"}"
        echo "  Status: ${status:-"Unknown"}"
        
        echo "Implementation Lines: ${impl_lines:-"N/A"}" >> $output_file
        echo "Test Lines: ${test_lines:-"N/A"}" >> $output_file
        echo "Status: ${status:-"Unknown"}" >> $output_file
    else
        echo "  Branch not found"
        echo "Branch not found" >> $output_file
    fi
    
    echo "" >> $output_file
    echo ""
done

echo "=========================================="
echo "Analysis complete. Results saved to: $output_file"
echo "=========================================="