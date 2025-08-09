#!/bin/bash

# TMC Branch Dependency Analysis
echo "=========================================="
echo "TMC Branch Dependency Analysis"
echo "=========================================="
echo "Date: $(date)"
echo ""

# Function to analyze branch base
analyze_branch_base() {
    local branch=$1
    if git show-ref --quiet "refs/heads/$branch" || git show-ref --quiet "refs/remotes/origin/$branch"; then
        # Get merge base with main
        merge_base=$(git merge-base main "$branch" 2>/dev/null || echo "main")
        
        # Check if branch is directly based on main
        if git merge-base --is-ancestor main "$branch" 2>/dev/null; then
            echo "  Base: main (direct)"
        else
            # Check for common TMC branch patterns
            commits=$(git log --oneline --max-count=5 main.."$branch" 2>/dev/null | head -3)
            echo "  Base: main (with $branch commits)"
            echo "  Recent commits:"
            echo "$commits" | sed 's/^/    /'
        fi
        
        # Get branch creation info
        first_commit=$(git log --reverse --oneline main.."$branch" 2>/dev/null | head -1 | cut -d' ' -f1)
        if [ ! -z "$first_commit" ]; then
            echo "  First commit: $first_commit"
        fi
    else
        echo "  Base: Branch not found"
    fi
}

# Get all TMC branches
echo "Analyzing TMC branch dependencies..."
echo ""

# Foundation branches
echo "FOUNDATION BRANCHES:"
echo "--------------------"

branches_foundation=(
    "feature/tmc2-impl2/00a1-controller-patterns"
    "feature/tmc2-impl2/00b1-workspace-isolation"
    "feature/tmc2-impl2/00c-feature-flags"
)

for branch in "${branches_foundation[@]}"; do
    echo "Branch: $branch"
    analyze_branch_base "$branch"
    echo ""
done

# API branches
echo "API BRANCHES:"
echo "-------------"

branches_api=(
    "feature/tmc2-impl2/01a-cluster-basic"
    "feature/tmc2-impl2/01b-cluster-enhanced"
    "feature/tmc2-impl2/01c-placement-basic"
    "feature/tmc2-impl2/01d-placement-advanced"
    "feature/tmc2-impl2/02a-core-apis"
    "feature/tmc2-impl2/02a1-apiexport-core"
    "feature/tmc2-impl2/02a2-apiexport-schemas"
    "feature/tmc2-impl2/02b-advanced-apis"
)

for branch in "${branches_api[@]}"; do
    echo "Branch: $branch"
    analyze_branch_base "$branch"
    echo ""
done

# Controller branches
echo "CONTROLLER BRANCHES:"
echo "-------------------"

branches_controller=(
    "feature/tmc2-impl2/03a-cluster-api"
    "feature/tmc2-impl2/03a-controller-binary"
    "feature/tmc2-impl2/03b-controller-config"
    "feature/tmc2-impl2/04c-placement-controller"
    "feature/tmc2-impl2/05a2a-api-foundation"
    "feature/tmc2-impl2/05a2b-decision-engine"
    "feature/tmc2-impl2/05a2c-controller-integration"
)

for branch in "${branches_controller[@]}"; do
    echo "Branch: $branch"
    analyze_branch_base "$branch"
    echo ""
done

echo "Analysis complete."