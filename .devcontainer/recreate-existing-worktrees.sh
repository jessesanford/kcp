#!/bin/bash

# Script to recreate existing worktrees from the current git worktree list
# This ensures the devcontainer has the same worktree setup as the host

set -e

WORKSPACES_KCP="/workspaces/kcp"

echo "🔄 Recreating existing KCP worktrees..."

# Ensure we're in the main kcp repository
cd "$WORKSPACES_KCP"

# Function to parse worktree list and recreate missing worktrees
recreate_worktrees() {
    local temp_file="/tmp/worktree-list.txt"
    
    # Get current worktree list
    git worktree list > "$temp_file"
    
    echo "📋 Analyzing existing worktree configuration..."
    
    local current_path=""
    local created_count=0
    local skipped_count=0
    local failed_count=0
    
    while IFS= read -r line; do
        # Check if this line starts with / (it's a path line)
        if [[ "$line" =~ ^/ ]]; then
            current_path="$line"
        # Check if this line contains branch info (indented line with brackets)
        elif [[ "$line" =~ ^[[:space:]]+[a-f0-9]+[[:space:]]+\[([^]]+)\] ]]; then
            # Extract branch name from brackets
            local branch_name="${BASH_REMATCH[1]}"
            
            # Skip the main repository entry
            if [[ "$current_path" == "/workspaces/kcp" ]]; then
                ((skipped_count++))
                continue
            fi
            
            echo "🔍 Processing: $(basename "$current_path") -> $branch_name"
            
            # Check if worktree path already exists
            if [[ -d "$current_path" ]]; then
                echo "✅ Already exists: $(basename "$current_path")"
                ((skipped_count++))
            else
                # Create the directory structure
                mkdir -p "$(dirname "$current_path")"
                
                # Try to create the worktree
                if git worktree add "$current_path" "$branch_name" 2>/dev/null; then
                    echo "✅ Recreated: $(basename "$current_path")"
                    ((created_count++))
                else
                    echo "⚠️  Failed to recreate: $(basename "$current_path") (branch: $branch_name)"
                    ((failed_count++))
                fi
            fi
        fi
    done < "$temp_file"
    
    # Cleanup
    rm -f "$temp_file"
    
    echo ""
    echo "🎉 Worktree recreation completed!"
    echo "📊 Summary:"
    echo "  - Created: $created_count worktrees"
    echo "  - Already existed: $skipped_count worktrees"  
    echo "  - Failed: $failed_count worktrees"
    
    if [[ $failed_count -gt 0 ]]; then
        echo ""
        echo "💡 Failed worktrees may need:"
        echo "  - Remote branch fetch: git fetch --all"
        echo "  - Manual branch tracking setup"
    fi
}

# Check if this is a fresh clone that needs remote branches
echo "🔄 Ensuring remote branches are available..."
if ! git branch -r | grep -q "origin/feature"; then
    echo "📡 Fetching remote branches..."
    git fetch --all
else
    echo "✅ Remote branches already available"
fi

# Recreate the worktrees
recreate_worktrees

echo ""
echo "💡 Worktree usage:"
echo "  - List all: git worktree list"  
echo "  - Remove: git worktree remove <path>"
echo "  - Navigate: cd /workspaces/kcp-worktrees/<feature>"
