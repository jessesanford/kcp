#\!/bin/bash
branch=$1
base_branch=${2:-main}

echo "=== Branch: $branch ==="
git checkout $branch 2>/dev/null
if [ $? -ne 0 ]; then
    echo "Branch not found"
    return 1
fi

# Get changed files
changed_files=$(git diff --name-only "$base_branch"...HEAD)

# Count implementation files (excluding tests and generated)
impl_files=$(echo "$changed_files" | grep "\.go$" | grep -v "_test\.go$" | grep -v "zz_generated" | grep -v "/testdata/" | grep -v "/vendor/")
test_files=$(echo "$changed_files" | grep "_test\.go$")

impl_lines=0
test_lines=0

if [ -n "$impl_files" ]; then
    impl_lines=$(echo "$impl_files" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
fi

if [ -n "$test_files" ]; then
    test_lines=$(echo "$test_files" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
fi

total=$((impl_lines + test_lines))

echo "Implementation: $impl_lines lines"
echo "Test: $test_lines lines" 
echo "Total: $total lines"

if [ $total -le 700 ]; then
    echo "Status: ✅ COMPLIANT"
elif [ $total -le 800 ]; then
    echo "Status: ⚠️ NEAR LIMIT"
else
    echo "Status: 🚨 OVER LIMIT"
fi
echo ""
