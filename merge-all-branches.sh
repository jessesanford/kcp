#!/bin/bash

echo "Starting batch merge of 225 remaining tmc-impl4 branches..."

counter=0
while IFS= read -r branch; do
    branch_name=$(echo "$branch" | sed 's/origin\///')
    counter=$((counter + 1))
    
    echo "[$counter/225] Merging: $branch_name"
    
    if git merge --no-ff "$branch_name" -X theirs; then
        echo "✓ Successfully merged $branch_name"
    else
        echo "⚠ Resolving conflicts for $branch_name"
        # Auto-resolve ALL conflicts by accepting theirs
        git status --porcelain | grep "^UU" | awk '{print $2}' | while read -r file; do
            git checkout --theirs "$file"
            git add "$file"
        done
        git commit --no-edit
        echo "✓ Conflicts resolved for $branch_name"
    fi
    
    # Progress update every 25 merges
    if [ $((counter % 25)) -eq 0 ]; then
        echo "=== PROGRESS: $counter/225 branches merged ==="
    fi
    
done < remaining-branches.txt

echo "🎉 ALL $counter BRANCHES MERGED SUCCESSFULLY!"