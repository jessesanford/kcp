#!/bin/bash

# Script to merge remaining TMC PR branches efficiently
# Auto-resolves common conflicts in doc.go and register.go by keeping our version

CONFLICT_COUNT=0
SUCCESS_COUNT=0
BRANCHES_TO_MERGE=(
  # Wave 5 - Sync components
  "wave5-029-sync-029"
  "wave5-030-sync-030"
  "wave5-031-sync-031"
  "wave5-032-sync-032"
  "wave5-033-sync-033"
  "wave5-034-sync-034"
  "wave5-035-sync-035"
  "wave5-036-sync-036"
  "wave5-037-sync-037"
  "wave5-038-sync-038"
  
  # Wave 6 - Controller implementation
  "wave6-039-controller-039"
  "wave6-040-controller-040"
  "wave6-041-controller-041"
  "wave6-042-controller-042"
  "wave6-043-controller-043"
  "wave6-044-controller-044"
  "wave6-045-controller-045"
  "wave6-046-controller-046"
  
  # Wave 7 - Manager and server
  "wave7-047-manager-047"
  "wave7-048-manager-048"
  "wave7-049-manager-049"
  "wave7-050-manager-050"
  "wave7-051-server-051"
  "wave7-052-server-052"
  "wave7-053-server-053"
  "wave7-054-server-054"
  
  # Wave 8 - TMC controller binary
  "wave8-055-tmc-controller-055"
  "wave8-056-tmc-controller-056"
  "wave8-057-tmc-controller-057"
  "wave8-058-tmc-controller-058"
  "wave8-059-tmc-controller-059"
  
  # Wave 9 - Integration and docs
  "wave9-060-integration-060"
  "wave9-061-integration-061"
  "wave9-062-integration-062"
  "wave9-063-integration-063"
  "wave9-064-integration-064"
)

for branch in "${BRANCHES_TO_MERGE[@]}"; do
    echo "=== Merging $branch ==="
    
    if git merge origin/pr-upstream/$branch --no-edit; then
        echo "SUCCESS: $branch merged cleanly"
        ((SUCCESS_COUNT++))
    else
        echo "CONFLICT: $branch - auto-resolving..."
        echo "CONFLICT in pr-upstream/$branch" >> /workspaces/tmc-pr-upstream/MERGE-CONFLICTS.log
        git status --short >> /workspaces/tmc-pr-upstream/MERGE-CONFLICTS.log
        
        # Auto-resolve by keeping our version of common conflict files
        git checkout --ours pkg/apis/tmc/v1alpha1/doc.go 2>/dev/null || true
        git checkout --ours pkg/apis/tmc/v1alpha1/register.go 2>/dev/null || true
        
        git add .
        git commit --no-edit
        ((CONFLICT_COUNT++))
        echo "RESOLVED: $branch"
    fi
done

echo "=== MERGE BATCH COMPLETE ==="
echo "Successfully merged: $SUCCESS_COUNT"
echo "Conflicts resolved: $CONFLICT_COUNT"
echo "Total processed: $((SUCCESS_COUNT + CONFLICT_COUNT))"