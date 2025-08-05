#!/bin/bash

# TMC PR Line Counter Script
# This script calculates lines of code changed in a branch using the same methodology
# as the TMC PR reviews, focusing on hand-written implementation code.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_colored() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to print section headers
print_header() {
    echo
    print_colored "$BLUE" "=========================================="
    print_colored "$BLUE" "$1"
    print_colored "$BLUE" "=========================================="
}

# Function to print results table
print_metrics_table() {
    local impl_lines=$1
    local test_lines=$2
    local target=${3:-700}
    
    echo
    print_colored "$PURPLE" "📊 Implementation Metrics"
    echo
    printf "| %-25s | %-10s | %-15s |\n" "Metric" "Value" "Status"
    printf "|%-27s|%-12s|%-17s|\n" "$(printf '%*s' 27 | tr ' ' '-')" "$(printf '%*s' 12 | tr ' ' '-')" "$(printf '%*s' 17 | tr ' ' '-')"
    
    # Calculate percentage over/under target
    local percentage=$(( (impl_lines * 100) / target ))
    local over_under=$((impl_lines - target))
    
    if [ $impl_lines -le $target ]; then
        local status_color=$GREEN
        local status="✅ EXCELLENT"
        [ $over_under -lt 0 ] && status="✅ $((-over_under * 100 / target))% under target"
    elif [ $impl_lines -le $((target + 100)) ]; then
        local status_color=$YELLOW
        local status="⚠️ OVER TARGET"
    else
        local status_color=$RED
        local status="❌ $(( (impl_lines * 100) / target - 100))% OVER"
    fi
    
    printf "| %-25s | %-10s | " "Hand-written Lines" "$impl_lines"
    print_colored "$status_color" "$status"
    printf "| %-25s | %-10s | %-15s |\n" "Target Threshold" "$target lines" "Baseline"
    printf "| %-25s | %-10s | " "Test Coverage Lines" "$test_lines"
    
    if [ $impl_lines -gt 0 ]; then
        local coverage_percent=$(( (test_lines * 100) / impl_lines ))
        if [ $coverage_percent -ge 100 ]; then
            print_colored "$GREEN" "🏆 ${coverage_percent}% coverage"
        elif [ $coverage_percent -ge 70 ]; then
            print_colored "$GREEN" "✅ ${coverage_percent}% coverage"
        elif [ $coverage_percent -ge 50 ]; then
            print_colored "$YELLOW" "⚠️ ${coverage_percent}% coverage"
        else
            print_colored "$RED" "❌ ${coverage_percent}% coverage"
        fi
    else
        print_colored "$YELLOW" "N/A"
    fi
    echo
}

# Default values
BASE_BRANCH="main"
TARGET_LINES=700
CURRENT_BRANCH=""
SHOW_BREAKDOWN=false
SHOW_HELP=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--base)
            BASE_BRANCH="$2"
            shift 2
            ;;
        -t|--target)
            TARGET_LINES="$2"
            shift 2
            ;;
        -c|--current)
            CURRENT_BRANCH="$2"
            shift 2
            ;;
        -d|--detailed)
            SHOW_BREAKDOWN=true
            shift
            ;;
        -h|--help)
            SHOW_HELP=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            SHOW_HELP=true
            shift
            ;;
    esac
done

# Show help if requested
if [ "$SHOW_HELP" = true ]; then
    echo "TMC PR Line Counter - Calculate lines of code changed in a branch"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -b, --base BRANCH     Base branch to compare against (default: main)"
    echo "  -t, --target LINES    Target line count for assessment (default: 700)"
    echo "  -c, --current BRANCH  Branch to analyze (default: current branch)"
    echo "  -d, --detailed        Show detailed file breakdown"
    echo "  -h, --help           Show this help message"
    echo
    echo "Examples:"
    echo "  $0                                    # Analyze current branch vs main"
    echo "  $0 -b develop -t 500                 # Compare vs develop with 500 line target"
    echo "  $0 -c feature/my-branch -d           # Analyze specific branch with breakdown"
    echo
    exit 0
fi

# Get current branch if not specified
if [ -z "$CURRENT_BRANCH" ]; then
    CURRENT_BRANCH=$(git branch --show-current)
fi

# Verify we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_colored "$RED" "❌ Error: Not in a git repository"
    exit 1
fi

# Verify branches exist
if ! git rev-parse --verify "$BASE_BRANCH" >/dev/null 2>&1; then
    print_colored "$RED" "❌ Error: Base branch '$BASE_BRANCH' does not exist"
    exit 1
fi

if ! git rev-parse --verify "$CURRENT_BRANCH" >/dev/null 2>&1; then
    print_colored "$RED" "❌ Error: Current branch '$CURRENT_BRANCH' does not exist"
    exit 1
fi

print_header "TMC PR Line Counter"
print_colored "$CYAN" "Branch: $CURRENT_BRANCH"
print_colored "$CYAN" "Base: $BASE_BRANCH"
print_colored "$CYAN" "Target: $TARGET_LINES lines"

# Get list of changed files
CHANGED_FILES=$(git diff --name-only "$BASE_BRANCH"..."$CURRENT_BRANCH" 2>/dev/null || true)

if [ -z "$CHANGED_FILES" ]; then
    print_colored "$YELLOW" "⚠️ No changes found between $BASE_BRANCH and $CURRENT_BRANCH"
    exit 0
fi

print_header "File Analysis"

# Count implementation lines (excluding tests and generated files)
IMPL_FILES=$(echo "$CHANGED_FILES" | grep "\.go$" | grep -v "_test\.go$" | grep -v "zz_generated" | grep -v "/testdata/" | grep -v "/vendor/" || true)

IMPL_LINE_COUNT=0
if [ -n "$IMPL_FILES" ]; then
    # Create temporary file list for xargs
    TEMP_IMPL_FILE=$(mktemp)
    echo "$IMPL_FILES" > "$TEMP_IMPL_FILE"
    
    if [ -s "$TEMP_IMPL_FILE" ]; then
        IMPL_LINE_COUNT=$(cat "$TEMP_IMPL_FILE" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
        
        if [ "$SHOW_BREAKDOWN" = true ]; then
            print_colored "$GREEN" "📄 Implementation Files:"
            cat "$TEMP_IMPL_FILE" | xargs wc -l 2>/dev/null | head -n -1 | sort -nr | while read line file; do
                printf "  %4d lines: %s\n" "$line" "$file"
            done
        else
            IMPL_FILE_COUNT=$(cat "$TEMP_IMPL_FILE" | wc -l)
            print_colored "$GREEN" "📄 Implementation Files: $IMPL_FILE_COUNT files, $IMPL_LINE_COUNT lines"
        fi
    fi
    rm -f "$TEMP_IMPL_FILE"
else
    print_colored "$YELLOW" "📄 Implementation Files: 0 files"
fi

# Count test lines
TEST_FILES=$(echo "$CHANGED_FILES" | grep "_test\.go$" | grep -v "/testdata/" | grep -v "/vendor/" || true)

TEST_LINE_COUNT=0
if [ -n "$TEST_FILES" ]; then
    # Create temporary file list for xargs
    TEMP_TEST_FILE=$(mktemp)
    echo "$TEST_FILES" > "$TEMP_TEST_FILE"
    
    if [ -s "$TEMP_TEST_FILE" ]; then
        TEST_LINE_COUNT=$(cat "$TEMP_TEST_FILE" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
        
        if [ "$SHOW_BREAKDOWN" = true ]; then
            print_colored "$CYAN" "🧪 Test Files:"
            cat "$TEMP_TEST_FILE" | xargs wc -l 2>/dev/null | head -n -1 | sort -nr | while read line file; do
                printf "  %4d lines: %s\n" "$line" "$file"
            done
        else
            TEST_FILE_COUNT=$(cat "$TEMP_TEST_FILE" | wc -l)
            print_colored "$CYAN" "🧪 Test Files: $TEST_FILE_COUNT files, $TEST_LINE_COUNT lines"
        fi
    fi
    rm -f "$TEMP_TEST_FILE"
else
    print_colored "$YELLOW" "🧪 Test Files: 0 files"
fi

# Count excluded files for transparency
EXCLUDED_FILES=$(echo "$CHANGED_FILES" | grep -E "(zz_generated|/testdata/|/vendor/|\.yaml$|\.yml$|\.md$)" || true)
EXCLUDED_COUNT=0
if [ -n "$EXCLUDED_FILES" ]; then
    EXCLUDED_COUNT=$(echo "$EXCLUDED_FILES" | wc -l)
    if [ "$SHOW_BREAKDOWN" = true ]; then
        print_colored "$YELLOW" "🚫 Excluded Files (generated/config/docs):"
        echo "$EXCLUDED_FILES" | while read file; do
            printf "  %s\n" "$file"
        done
    else
        print_colored "$YELLOW" "🚫 Excluded Files: $EXCLUDED_COUNT files (generated/config/docs)"
    fi
fi

# Show TMC-specific analysis if TMC files are present
TMC_FILES=$(echo "$IMPL_FILES" | grep "/tmc/" || true)
if [ -n "$TMC_FILES" ]; then
    print_header "TMC-Specific Analysis"
    
    TEMP_TMC_FILE=$(mktemp)
    echo "$TMC_FILES" > "$TEMP_TMC_FILE"
    
    TMC_LINE_COUNT=$(cat "$TEMP_TMC_FILE" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
    TMC_FILE_COUNT=$(cat "$TEMP_TMC_FILE" | wc -l)
    
    print_colored "$PURPLE" "🏗️ TMC Implementation: $TMC_FILE_COUNT files, $TMC_LINE_COUNT lines"
    
    if [ "$SHOW_BREAKDOWN" = true ]; then
        cat "$TEMP_TMC_FILE" | xargs wc -l 2>/dev/null | head -n -1 | sort -nr | while read line file; do
            printf "  %4d lines: %s\n" "$line" "$file"
        done
    fi
    
    rm -f "$TEMP_TMC_FILE"
fi

print_header "Assessment Results"

# Print metrics table
print_metrics_table "$IMPL_LINE_COUNT" "$TEST_LINE_COUNT" "$TARGET_LINES"

# Provide recommendations based on results
if [ $IMPL_LINE_COUNT -le $TARGET_LINES ]; then
    print_colored "$GREEN" "✅ APPROVED FOR SUBMISSION"
    echo "   Size is within guidelines and appropriate for focused review."
elif [ $IMPL_LINE_COUNT -le $((TARGET_LINES + 100)) ]; then
    print_colored "$YELLOW" "⚠️ REVIEW REQUIRED" 
    echo "   Slightly over target. Consider if scope can be reduced."
else
    print_colored "$RED" "❌ TOO LARGE - REQUIRES SPLITTING"
    echo "   Significantly over target. Should be split into smaller PRs."
fi

# Test coverage recommendations
if [ $IMPL_LINE_COUNT -gt 0 ]; then
    COVERAGE_PERCENT=$(( (TEST_LINE_COUNT * 100) / IMPL_LINE_COUNT ))
    echo
    if [ $COVERAGE_PERCENT -ge 70 ]; then
        print_colored "$GREEN" "🧪 Test coverage is excellent ($COVERAGE_PERCENT%)"
    elif [ $COVERAGE_PERCENT -ge 50 ]; then
        print_colored "$YELLOW" "🧪 Test coverage is adequate ($COVERAGE_PERCENT%) but could be improved"
    else
        print_colored "$RED" "🧪 Test coverage is insufficient ($COVERAGE_PERCENT%) - needs improvement"
    fi
fi

print_header "Summary"
echo "This script uses the same methodology as TMC PR reviews:"
echo "• Counts only hand-written Go implementation files"
echo "• Excludes generated files (zz_generated.*)"
echo "• Excludes test files from size calculation (counted separately)"
echo "• Excludes config files, documentation, and vendor code"
echo "• Focuses on reviewable code that requires maintenance"
echo
print_colored "$BLUE" "Target: Keep implementation under $TARGET_LINES lines for optimal review"