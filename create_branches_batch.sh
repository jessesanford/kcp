#!/bin/bash

# Batch script to create remaining branches quickly
set -e

# Define remaining waves and branches
declare -A branches=(
    # Wave 1 - Controller interfaces and API scheme
    ["pr-upstream/wave1-008-controller-interfaces"]="pr-staging-attempt/wave1-007-controller-interfaces"
    ["pr-upstream/wave1-009-api-scheme"]="pr-staging-attempt/wave1-008-api-scheme"
    
    # Wave 2 - Plugins & Libraries
    ["pr-upstream/wave2-010-workqueue"]="pr-staging-attempt/wave2-009-workqueue"
    ["pr-upstream/wave2-011-metrics-base"]="pr-staging-attempt/wave2-010-metrics-base"  
    ["pr-upstream/wave2-012-validation-helpers"]="pr-staging-attempt/wave2-011-validation-helpers"
    ["pr-upstream/wave2-013-shared-helpers"]="pr-staging-attempt/wave2-012-shared-helpers"
)

for target_branch in "${!branches[@]}"; do
    source_branch="${branches[$target_branch]}"
    
    echo "Creating branch: $target_branch from $source_branch"
    
    # Create new branch
    git checkout -b "$target_branch" upstream/main
    
    # Get the latest commit from source branch
    commit_hash=$(git log --oneline "$source_branch" | head -1 | cut -d' ' -f1)
    
    # Cherry-pick the commit
    if git cherry-pick "$commit_hash"; then
        echo "Successfully cherry-picked $commit_hash"
        
        # Check if under 800 lines
        lines=$(git diff --stat upstream/main | tail -1 | grep -o '[0-9]\+ insertions' | grep -o '[0-9]\+' || echo "0")
        
        if [ "$lines" -lt 800 ]; then
            # Push the branch
            git push origin "$target_branch"
            echo "Branch $target_branch created and pushed ($lines lines)"
        else
            echo "Branch $target_branch is too large ($lines lines) - needs splitting"
        fi
    else
        echo "Failed to cherry-pick for $target_branch"
        git cherry-pick --abort || true
    fi
    
    echo "---"
done

echo "Batch creation completed!"