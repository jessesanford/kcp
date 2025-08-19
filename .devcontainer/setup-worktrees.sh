#!/bin/bash

# Comprehensive script to setup git worktrees for kcp development
# Organizes worktrees by phase and feature categories

set -e

WORKSPACES_KCP="/workspaces/kcp"
WORKTREES_BASE="/workspaces/kcp-worktrees"

echo "🌳 Setting up KCP development worktrees..."
echo "📍 Main repo: $WORKSPACES_KCP"
echo "📁 Worktrees base: $WORKTREES_BASE"

# Ensure we're in the main kcp repository
cd "$WORKSPACES_KCP"

# Function to determine worktree path based on branch name
get_worktree_path() {
    local branch_name="$1"
    local clean_name="${branch_name#origin/}"
    
    # Skip certain branches
    case "$clean_name" in
        "main"|"HEAD"|"master")
            return 1
            ;;
    esac
    
    # Handle different branch patterns
    case "$clean_name" in
        # TMC Implementation branches
        "feature/tmc-impl4/"*)
            local impl_name="${clean_name#feature/tmc-impl4/}"
            echo "$WORKTREES_BASE/tmc-impl4/${impl_name}"
            ;;
        # Phase-specific branches
        "feature/phase"*"/p"*"w"*)
            if [[ $clean_name =~ feature/phase([0-9]+)-([^/]+)/p([0-9]+)w([^-]+)-(.+) ]]; then
                local phase="${BASH_REMATCH[1]}"
                local category="${BASH_REMATCH[2]}"
                local phase_num="${BASH_REMATCH[3]}"
                local wave="${BASH_REMATCH[4]}"
                local feature="${BASH_REMATCH[5]}"
                echo "$WORKTREES_BASE/phase${phase}/${category}/worktrees/p${phase_num}w${wave}-${feature}"
            else
                # Fallback for phase branches
                local phase_part="${clean_name#feature/}"
                echo "$WORKTREES_BASE/phases/${phase_part}"
            fi
            ;;
        # TMC Completion branches
        "feature/tmc-completion/"*)
            local comp_name="${clean_name#feature/tmc-completion/}"
            if [[ $comp_name =~ p([0-9]+)w([^-]+)-(.+) ]]; then
                local phase="${BASH_REMATCH[1]}"
                local wave="${BASH_REMATCH[2]}"
                local feature="${BASH_REMATCH[3]}"
                echo "$WORKTREES_BASE/tmc-completion/phase${phase}/p${phase}w${wave}-${feature}"
            else
                echo "$WORKTREES_BASE/tmc-completion/${comp_name}"
            fi
            ;;
        # TMC Syncer branches
        "feature/tmc-syncer-"*)
            local syncer_name="${clean_name#feature/tmc-syncer-}"
            echo "$WORKTREES_BASE/tmc-syncer/${syncer_name}"
            ;;
        # TMC Phase4 branches
        "feature/tmc-phase4-"*)
            local phase4_name="${clean_name#feature/tmc-phase4-}"
            echo "$WORKTREES_BASE/phase4/tmc-specific/${phase4_name}"
            ;;
        # TMC Impl2 branches
        "feature/tmc2-impl2/"*)
            local impl2_name="${clean_name#feature/tmc2-impl2/}"
            echo "$WORKTREES_BASE/tmc-impl2/${impl2_name}"
            ;;
        # Defunct branches
        "feature/defunct-"*)
            local defunct_name="${clean_name#feature/defunct-}"
            echo "$WORKTREES_BASE/defunct/${defunct_name}"
            ;;
        # Generic feature branches
        "feature/"*)
            local feature_name="${clean_name#feature/}"
            echo "$WORKTREES_BASE/features/${feature_name}"
            ;;
        # Other branches
        *)
            echo "$WORKTREES_BASE/other/${clean_name}"
            ;;
    esac
}

# Function to create a worktree if it doesn't exist
create_worktree_if_needed() {
    local branch="$1"
    local worktree_path="$2"
    
    # Check if worktree already exists
    if git worktree list | grep -q "$worktree_path"; then
        echo "✅ Worktree already exists: $(basename "$worktree_path")"
        return 0
    fi
    
    # Create directory structure
    mkdir -p "$(dirname "$worktree_path")"
    
    # Create the worktree
    echo "📁 Creating worktree: $(basename "$worktree_path") -> $branch"
    if git worktree add "$worktree_path" "$branch" 2>/dev/null; then
        echo "✅ Created: $worktree_path"
    else
        echo "⚠️  Failed to create worktree for $branch (may not exist locally)"
        return 1
    fi
}

# Get all remote branches and process them
echo "🔍 Analyzing remote branches..."
created_count=0
skipped_count=0
failed_count=0

# Process branches in smaller batches to avoid overwhelming the system
git branch -r | grep -v "HEAD" | while read -r branch; do
    branch=$(echo "$branch" | tr -d ' ')
    
    # Get the worktree path for this branch
    if worktree_path=$(get_worktree_path "$branch"); then
        if create_worktree_if_needed "$branch" "$worktree_path"; then
            ((created_count++))
        else
            ((failed_count++))
        fi
    else
        ((skipped_count++))
    fi
    
    # Progress indicator every 50 branches
    if (( (created_count + skipped_count + failed_count) % 50 == 0 )); then
        echo "📊 Progress: Created $created_count, Skipped $skipped_count, Failed $failed_count"
    fi
done

echo ""
echo "🎉 Worktree setup completed!"
echo "📊 Final statistics:"
echo "  - Created: $created_count new worktrees"
echo "  - Skipped: $skipped_count branches (main/HEAD/existing)"
echo "  - Failed: $failed_count branches"
echo ""
echo "📋 Worktree structure in $WORKTREES_BASE:"
find "$WORKTREES_BASE" -maxdepth 3 -type d | head -20
echo "... (and more)"
echo ""
echo "💡 Use 'git worktree list' to see all worktrees"
echo "💡 Use 'cd /workspaces/kcp-worktrees/<path>' to switch between features"
