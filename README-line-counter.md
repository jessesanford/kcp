# TMC PR Line Counter

This bash script calculates lines of code changed in a branch using the same methodology as the TMC PR reviews. It focuses on hand-written implementation code while excluding generated files, tests, and configuration.

## Usage

```bash
# Make the script executable (first time only)
chmod +x tmc-pr-line-counter.sh

# Basic usage - analyze current branch vs main
./tmc-pr-line-counter.sh

# Analyze specific branch with detailed breakdown
./tmc-pr-line-counter.sh -c feature/my-branch -d

# Custom base branch and target line count
./tmc-pr-line-counter.sh -b develop -t 500 -d

# Show help
./tmc-pr-line-counter.sh --help
```

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-b, --base BRANCH` | Base branch to compare against | `main` |
| `-t, --target LINES` | Target line count for assessment | `700` |
| `-c, --current BRANCH` | Branch to analyze | Current branch |
| `-d, --detailed` | Show detailed file breakdown | Off |
| `-h, --help` | Show help message | - |

## What Gets Counted

### ✅ **Included in Size Calculation**
- Hand-written Go implementation files (`*.go`)
- TMC-specific code (`/tmc/` paths)
- New source code requiring review and maintenance

### ❌ **Excluded from Size Calculation**
- Generated files (`zz_generated.*`, CRDs, client code)
- Test files (`*_test.go`) - counted separately for coverage
- Configuration files (`*.yaml`, `*.yml`)
- Documentation files (`*.md`)
- Vendor dependencies

## Assessment Criteria

### Size Targets
- **✅ Under 700 lines**: Excellent, approved for submission
- **⚠️ 700-800 lines**: Review required, consider scope reduction
- **❌ Over 800 lines**: Too large, requires splitting into smaller PRs

### Test Coverage Targets
- **🏆 100%+ coverage**: Outstanding
- **✅ 70-99% coverage**: Good
- **⚠️ 50-69% coverage**: Needs improvement
- **❌ <50% coverage**: Insufficient

## Example Output

```
==========================================
TMC PR Line Counter
==========================================
Branch: feature/tmc2-impl2/01b-cluster-enhanced
Base: main
Target: 700 lines

==========================================
File Analysis
==========================================
📄 Implementation Files: 4 files, 286 lines
🧪 Test Files: 2 files, 324 lines
🚫 Excluded Files: 18 files (generated/config/docs)

==========================================
TMC-Specific Analysis
==========================================
🏗️ TMC Implementation: 4 files, 286 lines

==========================================
Assessment Results
==========================================

📊 Implementation Metrics

| Metric                    | Value      | Status          |
|---------------------------|------------|-----------------|
| Hand-written Lines        | 286        | ✅ 59% under target
| Target Threshold          | 700 lines  | Baseline        |
| Test Coverage Lines       | 324        | 🏆 113% coverage

✅ APPROVED FOR SUBMISSION
   Size is within guidelines and appropriate for focused review.
```

## Why This Methodology?

1. **Focus on Intent**: Measures actual developer effort, not generated code
2. **Maintainability**: Hand-written code requires ongoing maintenance
3. **Review Scope**: Reviewers focus on implementation logic, not generated files
4. **Quality Gates**: Prevents scope creep in individual PRs
5. **TMC Standards**: Follows established KCP project review practices

## Integration with CI/CD

You can integrate this script into your CI/CD pipeline:

```yaml
# Example GitHub Actions step
- name: Check PR Size
  run: |
    chmod +x tmc-pr-line-counter.sh
    ./tmc-pr-line-counter.sh -t 700
    if [ $? -ne 0 ]; then
      echo "PR size check failed"
      exit 1
    fi
```

The script exits with code 0 for approved PRs and non-zero for PRs that are too large.