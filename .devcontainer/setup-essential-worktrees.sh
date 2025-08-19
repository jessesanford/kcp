#!/bin/bash

# Lightweight script to setup essential worktrees for devcontainer
# This runs as part of postCreateCommand and only creates commonly used worktrees

set -e

WORKSPACES_KCP="/workspaces/kcp"
WORKTREES_BASE="/workspaces/kcp-worktrees"

echo "🌱 Setting up essential KCP worktrees for development..."

# Ensure we're in the main kcp repository  
cd "$WORKSPACES_KCP"

# Ensure worktrees directory exists
mkdir -p "$WORKTREES_BASE"

# Function to create worktree if it doesn't exist
create_worktree() {
    local branch="$1"
    local worktree_path="$2"
    local display_name="$3"
    
    # Check if worktree already exists
    if git worktree list | grep -q "$worktree_path"; then
        echo "✅ $display_name (already exists)"
        return 0
    fi
    
    # Check if branch exists
    if ! git show-ref --verify --quiet "refs/remotes/$branch"; then
        echo "⚠️  $display_name (branch $branch not found)"
        return 1
    fi
    
    # Create directory structure
    mkdir -p "$(dirname "$worktree_path")"
    
    # Create the worktree
    if git worktree add "$worktree_path" "$branch" 2>/dev/null; then
        echo "✅ $display_name (created)"
    else
        echo "❌ $display_name (failed)"
        return 1
    fi
}

echo "📁 Creating essential worktrees..."

# Essential TMC Implementation worktrees
create_worktree "origin/feature/tmc-impl4/00-feature-flags" "$WORKTREES_BASE/tmc-impl4/00-feature-flags" "TMC Feature Flags"
create_worktree "origin/feature/tmc-impl4/01-base-controller" "$WORKTREES_BASE/tmc-impl4/01-base-controller" "TMC Base Controller"
create_worktree "origin/feature/tmc-impl4/02-workqueue" "$WORKTREES_BASE/tmc-impl4/02-workqueue" "TMC Workqueue"

# Key Phase 5 API Foundation worktrees
create_worktree "origin/feature/phase5-api-foundation/p5w1-apiresource-core" "$WORKTREES_BASE/phase5/api-foundation/p5w1-apiresource-core" "API Resource Core"
create_worktree "origin/feature/phase5-api-foundation/p5w1-apiresource-types" "$WORKTREES_BASE/phase5/api-foundation/p5w1-apiresource-types" "API Resource Types"

# Active TMC Completion worktrees  
create_worktree "origin/feature/tmc-completion/p8w2-scheduler" "$WORKTREES_BASE/tmc-completion/p8w2-scheduler" "TMC Scheduler"
create_worktree "origin/feature/tmc-completion/p8w3-binding-core" "$WORKTREES_BASE/tmc-completion/p8w3-binding-core" "TMC Binding Core"

# Phase 10 Integration worktrees
create_worktree "origin/feature/phase10-integration-hardening/p10w1-e2e-framework-final" "$WORKTREES_BASE/phase10/integration/p10w1-e2e-framework" "E2E Framework"
create_worktree "origin/feature/phase10-integration-hardening/p10w2a-perf-framework" "$WORKTREES_BASE/phase10/integration/p10w2a-perf-framework" "Performance Framework"

echo ""
echo "🎉 Essential worktrees setup completed!"
echo "💡 To setup ALL worktrees (700+ branches), run:"
echo "   cd /workspaces/kcp && ./.devcontainer/setup-worktrees.sh"
echo ""
echo "📋 Quick access paths:"
echo "  - TMC Implementation: /workspaces/kcp-worktrees/tmc-impl4/"
echo "  - Phase 5 API: /workspaces/kcp-worktrees/phase5/api-foundation/"
echo "  - TMC Completion: /workspaces/kcp-worktrees/tmc-completion/"
echo "  - Phase 10 Integration: /workspaces/kcp-worktrees/phase10/integration/"
