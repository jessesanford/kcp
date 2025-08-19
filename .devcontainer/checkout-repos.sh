#!/bin/bash

# Script to checkout additional repositories in /workspaces
# This script excludes the kcp repo since it's automatically checked out by the devcontainer

set -e

WORKSPACES_DIR="/workspaces"
REPOS=(
    "git@github.com:jessesanford/agent-configs.git"
    "git@github.com:jessesanford/kcp-shared-tools.git"
)

echo "🚀 Starting repository checkout process..."

for repo_url in "${REPOS[@]}"; do
    # Extract repository name from URL
    repo_name=$(basename "$repo_url" .git)
    target_dir="$WORKSPACES_DIR/$repo_name"
    
    echo "📁 Processing repository: $repo_name"
    
    if [ -d "$target_dir" ]; then
        echo "✅ Repository $repo_name already exists at $target_dir"
        echo "🔄 Pulling latest changes..."
        cd "$target_dir"
        git pull origin main 2>/dev/null || git pull origin master 2>/dev/null || echo "⚠️  Could not pull latest changes for $repo_name"
    else
        echo "📥 Cloning $repo_name to $target_dir..."
        if git clone "$repo_url" "$target_dir" 2>/dev/null; then
            echo "✅ Successfully cloned $repo_name"
        else
            echo "⚠️  Failed to clone $repo_name (possibly due to authentication or access issues)"
            echo "💡 You may need to:"
            echo "   - Set up SSH keys for GitHub access"
            echo "   - Verify repository access permissions"
            echo "   - Use HTTPS URLs instead of SSH"
        fi
    fi
    echo ""
done

echo "🎉 Repository checkout process completed!"
echo "📋 Available repositories in $WORKSPACES_DIR:"
ls -la "$WORKSPACES_DIR" | grep "^d" | awk '{print "  - " $9}' | grep -v "^\s*-\s*\.$" | grep -v "^\s*-\s*\.\.$"
