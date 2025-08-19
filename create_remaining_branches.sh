#!/bin/bash

# Comprehensive batch script to create all remaining branches
set -e

echo "Creating remaining TMC PR branches..."

# Define all remaining waves and branches with their source mappings
declare -A wave_mappings=(
    # Wave 4 - Core APIs
    ["pr-upstream/wave4-021-synctarget-api"]="pr-staging-attempt/wave4-013-synctarget-api"
    ["pr-upstream/wave4-022-apiresource-types"]="pr-staging-attempt/wave4-014-apiresource-types"
    ["pr-upstream/wave4-023-apiresource-core"]="pr-staging-attempt/wave4-015-apiresource-core"
    ["pr-upstream/wave4-024-apiresource-helpers"]="pr-staging-attempt/wave4-016-apiresource-helpers"
    ["pr-upstream/wave4-025-discovery-types"]="pr-staging-attempt/wave4-017-discovery-types"
    ["pr-upstream/wave4-026-discovery-impl"]="pr-staging-attempt/wave4-018-discovery-impl"
    ["pr-upstream/wave4-027-transform-types"]="pr-staging-attempt/wave4-020-transform-types"
    ["pr-upstream/wave4-028-workload-dist"]="pr-staging-attempt/wave4-021-workload-dist"
    
    # Wave 5 - Sync Engine
    ["pr-upstream/wave5-029-sync-engine"]="pr-staging-attempt/wave5-022-sync-engine"
    ["pr-upstream/wave5-030-sync-types"]="pr-staging-attempt/wave5-023-sync-types"
    ["pr-upstream/wave5-031-sync-resource"]="pr-staging-attempt/wave5-024-sync-resource"
    ["pr-upstream/wave5-032-downstream"]="pr-staging-attempt/wave5-025-downstream"
    ["pr-upstream/wave5-033-upstream"]="pr-staging-attempt/wave5-026-upstream"
    ["pr-upstream/wave5-034-transform"]="pr-staging-attempt/wave5-027-transform"
    ["pr-upstream/wave5-035-applier"]="pr-staging-attempt/wave5-028-applier"
    ["pr-upstream/wave5-036-conflict"]="pr-staging-attempt/wave5-029-conflict"
    ["pr-upstream/wave5-037-heartbeat"]="pr-staging-attempt/wave5-030-heartbeat"
    ["pr-upstream/wave5-038-events"]="pr-staging-attempt/wave5-031-events"
)

# Create branches in batches to avoid overwhelming the system
batch_size=5
count=0
batch_num=1

for target_branch in "${!wave_mappings[@]}"; do
    source_branch="${wave_mappings[$target_branch]}"
    
    # Start new batch if needed
    if [ $count -eq 0 ]; then
        echo "Starting batch $batch_num..."
    fi
    
    echo "[$batch_num.$((count+1))] Creating: $target_branch"
    
    # Check if source branch exists
    if ! git show-ref --verify --quiet "refs/heads/$source_branch" && ! git show-ref --verify --quiet "refs/remotes/origin/$source_branch"; then
        echo "  WARNING: Source branch $source_branch not found, skipping"
        continue
    fi
    
    # Create new branch
    if git checkout -b "$target_branch" upstream/main 2>/dev/null; then
        # Get the latest commit from source branch  
        if commit_hash=$(git log --oneline "$source_branch" 2>/dev/null | head -1 | cut -d' ' -f1); then
            # Cherry-pick the commit
            if git cherry-pick "$commit_hash" >/dev/null 2>&1; then
                echo "  Cherry-picked: $commit_hash"
                
                # Quick line count check
                lines=$(git diff --stat upstream/main 2>/dev/null | tail -1 | grep -o '[0-9]\+ insertions' | grep -o '[0-9]\+' 2>/dev/null || echo "0")
                
                if [ "$lines" -lt 800 ] && [ "$lines" -gt 0 ]; then
                    # Push the branch
                    if git push origin "$target_branch" >/dev/null 2>&1; then
                        echo "  SUCCESS: Pushed ($lines lines)"
                    else
                        echo "  ERROR: Failed to push"
                    fi
                else
                    echo "  WARNING: Size issue ($lines lines)"
                fi
            else
                echo "  ERROR: Cherry-pick failed"
                git cherry-pick --abort 2>/dev/null || true
            fi
        else
            echo "  ERROR: Could not get commit hash"
        fi
    else
        echo "  ERROR: Could not create branch"
    fi
    
    count=$((count+1))
    
    # End batch and rest if needed
    if [ $count -eq $batch_size ]; then
        echo "Batch $batch_num completed. Resting..."
        sleep 2
        batch_num=$((batch_num+1))
        count=0
    fi
done

echo "Batch creation phase 1 completed!"

# Check current count
current_count=$(git branch -r | grep -c "origin/pr-upstream" || echo "0")
echo "Current branch count: $current_count"