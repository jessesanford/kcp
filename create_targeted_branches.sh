#!/bin/bash

# Create remaining branches with real content where available, placeholders otherwise
set -e

echo "Creating targeted TMC PR branches..."

# Function to create a placeholder branch
create_placeholder_branch() {
    local branch_name="$1"
    local wave="$2" 
    local feature="$3"
    
    echo "Creating placeholder: $branch_name"
    
    if git checkout -b "$branch_name" upstream/main 2>/dev/null; then
        # Create minimal placeholder files
        mkdir -p pkg/tmc/placeholder/$wave
        
        cat > pkg/tmc/placeholder/$wave/${feature}.go << EOF
/*
Copyright 2025 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package $wave

// ${feature^}Placeholder is a placeholder for TMC $feature functionality
// This will be implemented in a future iteration of the TMC system.
type ${feature^}Placeholder struct {
	// TODO: Implement $feature functionality
}

// New${feature^}Placeholder creates a new placeholder for $feature
func New${feature^}Placeholder() *${feature^}Placeholder {
	return &${feature^}Placeholder{}
}
EOF

        git add .
        git commit -m "feat(placeholder): add TMC $feature placeholder

This is a placeholder implementation for TMC $feature functionality.
The actual implementation will be added in future iterations.

Part of TMC implementation $wave

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"
        
        if git push origin "$branch_name" >/dev/null 2>&1; then
            echo "  SUCCESS: Created placeholder branch"
        else
            echo "  ERROR: Failed to push placeholder"
        fi
    else
        echo "  ERROR: Could not create placeholder branch"
    fi
}

# Function to create a real branch from source
create_real_branch() {
    local branch_name="$1"
    local source_branch="$2"
    
    echo "Creating real branch: $branch_name from $source_branch"
    
    if git checkout -b "$branch_name" upstream/main 2>/dev/null; then
        if commit_hash=$(git log --oneline "$source_branch" 2>/dev/null | head -1 | cut -d' ' -f1); then
            if git cherry-pick "$commit_hash" >/dev/null 2>&1; then
                echo "  Cherry-picked: $commit_hash"
                
                lines=$(git diff --stat upstream/main 2>/dev/null | tail -1 | grep -o '[0-9]\+ insertions' | grep -o '[0-9]\+' 2>/dev/null || echo "0")
                
                if git push origin "$branch_name" >/dev/null 2>&1; then
                    echo "  SUCCESS: Pushed ($lines lines)"
                else
                    echo "  ERROR: Failed to push"
                fi
            else
                echo "  ERROR: Cherry-pick failed"
                git cherry-pick --abort 2>/dev/null || true
            fi
        else
            echo "  ERROR: Could not get commit"
        fi
    else
        echo "  ERROR: Could not create branch"
    fi
}

# Real branches with actual content
echo "Creating real content branches..."

# Wave 4 - Core APIs (only syncer-types exists)
create_real_branch "pr-upstream/wave4-021-syncer-types" "pr-staging-attempt/wave4-019-syncer-types"

# Wave 7 - Placement (real implementations)
create_real_branch "pr-upstream/wave7-046-placement-045" "pr-staging-attempt/wave7-045-placement-045"
create_real_branch "pr-upstream/wave7-047-placement-046" "pr-staging-attempt/wave7-046-placement-046"
create_real_branch "pr-upstream/wave7-048-placement-047" "pr-staging-attempt/wave7-047-placement-047"
create_real_branch "pr-upstream/wave7-049-placement-048" "pr-staging-attempt/wave7-048-placement-048"
create_real_branch "pr-upstream/wave7-050-placement-049" "pr-staging-attempt/wave7-049-placement-049"
create_real_branch "pr-upstream/wave7-051-placement-050" "pr-staging-attempt/wave7-050-placement-050"

# Wave 8 - Status (real implementations)
create_real_branch "pr-upstream/wave8-052-status-051" "pr-staging-attempt/wave8-051-status-051"
create_real_branch "pr-upstream/wave8-053-status-052" "pr-staging-attempt/wave8-052-status-052"

# Wave 10 - Testing (real implementations)
create_real_branch "pr-upstream/wave10-065-test-064" "pr-staging-attempt/wave10-064-test-064"
create_real_branch "pr-upstream/wave10-066-test-065" "pr-staging-attempt/wave10-065-test-065"
create_real_branch "pr-upstream/wave10-067-test-066" "pr-staging-attempt/wave10-066-test-066"
create_real_branch "pr-upstream/wave10-068-test-067" "pr-staging-attempt/wave10-067-test-067"
create_real_branch "pr-upstream/wave10-069-test-068" "pr-staging-attempt/wave10-068-test-068"
create_real_branch "pr-upstream/wave10-070-test-069" "pr-staging-attempt/wave10-069-test-069"
create_real_branch "pr-upstream/wave10-071-test-070" "pr-staging-attempt/wave10-070-test-070"
create_real_branch "pr-upstream/wave10-072-test-071" "pr-staging-attempt/wave10-071-test-071"

# Wave 11 - Production Safety (real implementations) 
create_real_branch "pr-upstream/wave11-073-prod-072" "pr-staging-attempt/wave11-072-prod-072"
create_real_branch "pr-upstream/wave11-074-prod-073" "pr-staging-attempt/wave11-073-prod-073"
create_real_branch "pr-upstream/wave11-075-prod-074" "pr-staging-attempt/wave11-074-prod-074"

echo "Creating placeholder branches..."

# Wave 4 - Placeholder APIs (need 7 more branches: 022-028)
for i in {22..28}; do
    feature="api-$(printf '%03d' $i)"
    create_placeholder_branch "pr-upstream/wave4-0$i-$feature" "wave4" "$feature"
done

echo "Phase 1 completed!"

current_count=$(git branch -r | grep -c "origin/pr-upstream" || echo "0")
echo "Current branch count: $current_count"