# TMC Reimplementation Plan 2: Implementation Instructions

## 🎯 **Mission Statement**

You are tasked with implementing the TMC Reimplementation Plan 2 through a series of carefully crafted, production-ready Pull Requests. Each PR must be atomic, well-tested, thoroughly documented, and ready for KCP maintainer review.

## 📋 **Critical Implementation Requirements**

### ** ONLY BUILD WHAT IS NEEDED FOR TMC**
- DO NOT BOIL THE OCEAN
- Only build the features and functionality necessary to get the TMC feature over the line.
- We don't need every feature under the sun. Start with minimally viable and then add additional features on in later PRs.
- It's OK if a feature needs to be implmeneted over many PRs to get to it's full potential.

### **Generated Code**
- Make sure to run code generation tooling when appropriate. This includes deepcopy and crd generation.
- Commit all generated code. It will not count against your total lines of code rules.

### **FEATURE FLAG USAGE**
- Always implement features behind feature flags.
- There should be one maste feature flag for all TMC functionality that will disable any sub-feature flags as well
- If you need sub feature flags that is fine, just make sure to group and isolate things logically. Don't create too many feature flags that are too fine-grained.
- If you need to use a github username for the feature flags use @jessesanford
- If you need to use a version for the features use 0.1

### **MEASURING UNIQUE LINES OF CODE IN A BRANCH**
- ONLY measure the number of lines of code created between the branch and the branch it is based upon.
- ONLY measure lines of code that are hand created. Generated lines of code (compiled files, compiled templates, crds, deepcopy files) DO NOT COUNT
- Binary Files DO NOT COUNT. You should never commit them anyway.

### **PR Size & Quality Standards**
- **Target PR Size**: 400-700 lines of code per PR  (however generated files do not count, this applies to both deepcopy and crd code)
- **Maximum PR Size**: 800 lines of code per PR (however generated files do not count, this applies to both deepcopy and crd code)
- **Quality Requirements**: Each PR must be:
  - ✅ **Atomic**: Complete, self-contained functionality
  - ✅ **Tested**: Comprehensive unit and integration tests
  - ✅ **Documented**: Code comments, API docs, user guides
  - ✅ **Linted**: Passes all code quality checks
  - ✅ **Reviewed**: Ready for maintainer review

### **If PR Exceeds Size Limit**
When a PR would exceed 800 lines to achieve atomic functionality:
1. **First, try to split functionality** into smaller atomic pieces
2. **If impossible to split**, create a design document and consider alternate approaches. Choose an approach that allows you to decompose the problem into smaller atomic PRs that can meet the requirements above.
3. **Add extra documentation and tests** to compensate for size
4. **Create detailed commit messages** explaining each logical change

## 🌳 **PR and Branch Management Strategy**

## **ALWAYS KEEP PR Plan up to date**
- Use the feature/tmc-planning branch's TMC-REIMPL-PLAN2-PR-PLAN.md file to keep a running list of which branches need to be merged in which order to achieve the totality of all TMC functionality.
- ALWAYS pay close attention to the appropriate ordering of branches. When in doubt check to make sure that the ordering is correct.
- ALWAYS make sure that each branch passes it's tests no matter which branch you choose to base it on. DO NOT MODIFY TESTS TO MAKE THEM PASS. Double check that you are basing on the right branch instead.

### **Branch Naming Convention**
```
feature/tmc2-impl2/XX-description
```
Where:
- `XX` = Two-digit PR order (01, 02, 03, ..., 11)
- `description` = Succinct feature description (kebab-case)

### **Branch Creation Pattern**
- If you are building a feature that does not build upon commits from a different unmerged feature branch then just branch from main.
- If you are building a feature that does require commits from another unmerged feature branch then base your new branch off of that unmerged branch.
- Keep your bran
```bash
# Try to branch from main
git checkout main
git pull origin main
git checkout -b feature/tmc2-impl2/01-api-foundation

# Work on feature, commit logically
git add .
git commit -m "Add TMC API types with proper KCP integration"

# Push when ready for PR
git push origin feature/tmc2-impl2/01-api-foundation
```

### **Critical Branch Rules**
- ❌ **NEVER merge to main** - maintainers will do this
- ❌ **NEVER merge feature branches together** - maintain independence  
- ✅ **Always branch from main** - even for dependent PRs
- ✅ **Rebase on main** if conflicts arise
- ✅ **Keep branches focused** - one feature per branch

## **Critical Implementation Way-of-Working**
- Read the Implementation Order & Dependencies section below and create a todo list that represents the work being asked for.
- ALWAYS switch to the feature/tmc-planning branch and read the corresponding TMC-REIMPL-PLAN-PHASE-0X.md (Where X is 0-9) before starting each task within that phase. So if your task falls in Phase 2, then you would need to read the feature/tmc-planning branch's TMC-REIMPL-PLAN-PHASE-02.md file before starting any task in Phase 2.
- ALWAYS make sure to branch off of the correct BASE. If the todo you are working on will be used to create a PR that requires a previous feature branch to have been merged then base your new feature branch off of that other branch.
- ALWAYS REVIEW your TODO list and make sure it is up to date and does not contain any erroneous items or falsely completed items BEFORE moving to the next todo.
- ALWAYS pick the TODOs from the top of the pile. If you need to re-order them or add new TODOS that is fine. Keep the list ordered correctly.
- If you have to add new todos as you go that is ok. Use the guidance on sizing commits, files and PRs to appropriately drive the number of feature branches and PRs you will need.
- After finishing a todo, before moving on to the next TODO, notify the user that you have finished and give a summary of the work. Then fill out a PR message for this work in a file name PR-MESSAGE-FOR-branch-name.md where branch name is the name of the branch you are working on.
- The PR template looks like:
```md
<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

## What Type of PR Is This?

<!--

Add one of the following kinds:
/kind bug
/kind cleanup
/kind documentation
/kind feature

Optionally add one or more of the following kinds if applicable:
/kind api-change
/kind deprecation
/kind failing-test
/kind flake
/kind regression

-->

## Related Issue(s)

Fixes #

## Release Notes

```

## 📊 **Implementation Order & Dependencies**

### **Phase 1: KCP Integration Foundation**

#### **PR 01: TMC API Foundation** (`feature/tmc2-impl2/01-api-foundation`)
**Target Size**: ~400 lines  
**Dependencies**: None

**Implementation Steps**:
1. Create `pkg/apis/tmc/v1alpha1/` directory structure
2. Implement `types.go` with ClusterRegistration and WorkloadPlacement APIs
3. Add `register.go` with proper KCP registration patterns
4. Create `doc.go` with package documentation
5. Add `install/install.go` for API installation
6. Generate deepcopy code and CRDs
7. Write comprehensive API documentation

**Testing Requirements**:
- API validation tests for all types
- Deepcopy functionality tests
- CRD generation verification tests
- API registration tests

**Documentation Requirements**:
- API reference documentation
- Design rationale for API choices
- Integration examples with KCP APIExport

#### **PR 02: APIExport Integration** (`feature/tmc2-impl2/02-apiexport-integration`)
**Target Size**: ~600 lines  
**Dependencies**: PR 01 merged to main

**Implementation Steps**:
1. Create `pkg/reconciler/tmc/tmcexport/` directory
2. Implement TMC APIExport controller following KCP patterns
3. Add proper workspace-aware client handling
4. Create configuration files for TMC APIExport
5. Add integration with existing APIExport system
6. Write controller tests and integration tests

**Testing Requirements**:
- Controller reconciliation tests
- APIExport creation and management tests
- Workspace isolation tests
- Integration tests with KCP APIExport system

**Documentation Requirements**:
- APIExport integration guide
- Workspace setup instructions
- API binding examples for consumers

### **Phase 2: External TMC Controllers**

#### **PR 03: TMC Controller Foundation** (`feature/tmc2-impl2/03-controller-foundation`)
**Target Size**: ~600 lines  
**Dependencies**: PR 02 merged to main

**Implementation Steps**:
1. Create `cmd/tmc-controller/main.go` with proper flag handling
2. Implement `cmd/tmc-controller/options/options.go` for configuration
3. Create `pkg/tmc/controller/clusterregistration.go` controller
4. Add proper KCP client integration and workspace awareness
5. Implement cluster health checking and status management
6. Write comprehensive controller tests

**Testing Requirements**:
- Controller startup and configuration tests
- Cluster registration reconciliation tests
- Health checking functionality tests
- Status update mechanism tests
- Error handling and retry logic tests

**Documentation Requirements**:
- TMC controller deployment guide
- Configuration reference
- Cluster registration workflow documentation

#### **PR 04: Workload Placement Controller** (`feature/tmc2-impl2/04-workload-placement`)
**Target Size**: ~500 lines  
**Dependencies**: PR 03 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/controller/workloadplacement.go` controller
2. Implement placement decision logic and cluster selection
3. Add `pkg/tmc/controller/manager.go` for controller coordination
4. Create placement algorithm foundation
5. Add comprehensive placement tests

**Testing Requirements**:
- Placement decision algorithm tests
- Cluster selection logic tests
- Controller manager coordination tests
- Workspace isolation in placement tests

**Documentation Requirements**:
- Placement strategy documentation
- Algorithm explanation and examples
- Configuration guide for placement policies

### **Phase 3: Workload Synchronization**

#### **PR 05: Workload Synchronization Engine** (`feature/tmc2-impl2/05-workload-sync`)
**Target Size**: ~600 lines  
**Dependencies**: PR 04 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/sync/engine.go` synchronization framework
2. Implement `pkg/tmc/sync/deployment_sync.go` for Deployment sync
3. Add resource watching and event handling
4. Create cluster client management
5. Implement resource transformation logic

**Testing Requirements**:
- Synchronization engine functionality tests
- Deployment synchronization tests
- Resource transformation tests
- Error handling and retry mechanism tests
- Multi-cluster synchronization tests

**Documentation Requirements**:
- Synchronization architecture overview
- Resource transformation examples
- Troubleshooting guide for sync issues

#### **PR 06: Status Synchronization & Lifecycle** (`feature/tmc2-impl2/06-status-sync`)
**Target Size**: ~600 lines  
**Dependencies**: PR 05 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/sync/status_sync.go` for bidirectional status
2. Implement `pkg/tmc/sync/lifecycle.go` for resource lifecycle
3. Add `pkg/tmc/sync/transform.go` for advanced transformations
4. Create status aggregation algorithms
5. Add resource cleanup and deletion handling

**Testing Requirements**:
- Status aggregation algorithm tests
- Bidirectional status synchronization tests
- Resource lifecycle management tests
- Cleanup and deletion tests
- Status consistency verification tests

**Documentation Requirements**:
- Status flow architecture documentation
- Aggregation strategy explanations
- Lifecycle management procedures

### **Phase 4: Advanced Placement & Performance**

#### **PR 07: Advanced Placement Engine** (`feature/tmc2-impl2/07-advanced-placement`)
**Target Size**: ~800 lines  
**Dependencies**: PR 06 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/placement/engine.go` with sophisticated algorithms
2. Implement `pkg/tmc/placement/capacity.go` for capacity management
3. Add `pkg/tmc/placement/algorithms.go` with multi-factor scoring
4. Create `pkg/tmc/placement/scheduler.go` for cluster selection
5. Add comprehensive placement testing

**Testing Requirements**:
- Placement algorithm correctness tests
- Capacity management functionality tests
- Multi-factor scoring verification tests
- Performance benchmarks for placement decisions
- Edge case handling tests

**Documentation Requirements**:
- Advanced placement algorithm documentation
- Capacity management explanation
- Performance tuning guide

#### **PR 08: Performance Optimization** (`feature/tmc2-impl2/08-performance-optimization`)
**Target Size**: ~700 lines  
**Dependencies**: PR 07 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/placement/performance.go` for optimization
2. Add caching mechanisms and batch processing
3. Implement performance monitoring and metrics
4. Create load balancing for placement decisions
5. Add performance testing and benchmarks

**Testing Requirements**:
- Caching mechanism functionality tests
- Batch processing efficiency tests
- Performance benchmark tests
- Load testing for high-throughput scenarios
- Memory usage and optimization tests

**Documentation Requirements**:
- Performance optimization guide
- Caching strategy documentation
- Scaling recommendations

### **Phase 5: Production Features & Enterprise**

#### **PR 09: Security & RBAC Integration** (`feature/tmc2-impl2/09-security-rbac`)
**Target Size**: ~600 lines  
**Dependencies**: PR 08 merged to main

**Implementation Steps**:
1. Create `pkg/tmc/security/rbac.go` with TMC RBAC roles
2. Implement `pkg/tmc/security/auth.go` for authentication
3. Add `pkg/tmc/security/secrets.go` for secret management
4. Create `pkg/tmc/security/tls.go` for mTLS configuration
5. Add comprehensive security testing

**Testing Requirements**:
- RBAC role and binding tests
- Authentication mechanism tests
- Security validation tests
- Access control verification tests
- TLS/mTLS functionality tests

**Documentation Requirements**:
- Security architecture overview
- RBAC setup and configuration guide
- Authentication integration examples

#### **PR 10: Monitoring & Observability** (`feature/tmc2-impl2/10-monitoring-observability`)
**Target Size**: ~500 lines  
**Dependencies**: PR 08 merged to main (parallel with PR 09)

**Implementation Steps**:
1. Create `pkg/tmc/observability/metrics.go` with Prometheus metrics
2. Implement `pkg/tmc/observability/tracing.go` for distributed tracing
3. Add `pkg/tmc/observability/logging.go` for structured logging
4. Create monitoring dashboards and alerts
5. Add observability testing

**Testing Requirements**:
- Metrics collection and exposure tests
- Tracing functionality tests
- Logging format and structure tests
- Dashboard rendering tests
- Alert rule validation tests

**Documentation Requirements**:
- Monitoring setup guide
- Metrics reference documentation
- Troubleshooting with observability tools

#### **PR 11: CLI Tools & Operations** (`feature/tmc2-impl2/11-cli-tools`)
**Target Size**: ~600 lines  
**Dependencies**: PR 08 merged to main (parallel with PRs 09-10)

**Implementation Steps**:
1. Create `cmd/tmcctl/main.go` CLI framework
2. Implement `cmd/tmcctl/cmd/cluster.go` for cluster management
3. Add `cmd/tmcctl/cmd/placement.go` for placement management
4. Create `cmd/tmcctl/cmd/workload.go` for workload operations
5. Add comprehensive CLI testing and documentation

**Testing Requirements**:
- CLI command functionality tests
- Flag parsing and validation tests
- Integration tests with KCP APIs
- User experience and error message tests
- CLI help and documentation tests

**Documentation Requirements**:
- CLI reference documentation
- User guide with examples
- Operator workflow documentation

## 🔧 **Implementation Guidelines**

### **Code Quality Standards**
```go
// Every function must have comprehensive documentation
// NewController creates a new TMC controller following KCP patterns.
// It integrates with the APIExport system and maintains workspace isolation.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client
//   - informerFactory: Shared informer factory for the workspace
//   - workspace: Logical cluster name for workspace isolation
//
// Returns:
//   - *Controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    informerFactory kcpinformers.SharedInformerFactory,
    workspace logicalcluster.Name,
) (*Controller, error) {
    // Implementation...
}
```

### **Testing Standards**
```go
// Every feature requires comprehensive test coverage
func TestClusterRegistrationController(t *testing.T) {
    tests := map[string]struct {
        cluster   *tmcv1alpha1.ClusterRegistration
        workspace string
        wantError bool
        wantConditions []metav1.Condition
    }{
        "healthy cluster registration": {
            cluster: &tmcv1alpha1.ClusterRegistration{
                ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
                Spec: tmcv1alpha1.ClusterRegistrationSpec{
                    Location: "us-west-2",
                },
            },
            workspace: "root:test",
            wantError: false,
            wantConditions: []metav1.Condition{
                {Type: "Ready", Status: "True"},
            },
        },
        // More test cases...
    }
    
    for name, tc := range tests {
        t.Run(name, func(t *testing.T) {
            // Test implementation...
        })
    }
}
```

### **Commit Standards**
- NEVER commit binary files. Add them to .gitignore
- NEVER commit changes that are not logically grouped together with some specific change intent. Break up the commits so that they encapsulate a specific intent.
- NEVER use `git commit -a`
- ALWAYS stage each file individually into the staging area and then commit once the files in the staging area completely encapsulate the change intent for the commit.
- ALWAYS sign commits for both the DCO and GPG signing: Use `git commit -s -S`
- If you have to rebase or rewrite the git commit history make sure that it tells a logical linear story from commit to commit.
- NEVER allow the git commit history to interleave or "mix" bodies of changes that correspond to different intents.
- ALWAYS ALWAYS ALWAYS! When in doubt commit files to a temporary branch. Do not allow them to be lost.

### **Commit Message Standards**
```
feat(api): add TMC ClusterRegistration and WorkloadPlacement APIs

- Implement ClusterRegistration API for cluster management
- Add WorkloadPlacement API for placement policies
- Follow KCP API design patterns with proper conditions
- Include workspace awareness and logical cluster support
- Add comprehensive API validation and defaults

Fixes: #XXX
```

## 🎯 **Success Criteria for Each PR**

### **Before Creating PR**
1. ✅ **No uncommited files** - Never leave uncommited files in the working copy. If files do not need to be tracked then add them to .gitignore.
1. ✅ **All tests pass** - no failing tests
2. ✅ **Linting passes** - code meets quality standards
3. ✅ **Documentation complete** - API docs, user guides, examples
4. ✅ **Commit messages clear** - descriptive and following standards
5. ✅ **Branch up to date** - rebased on latest main if needed

### **PR Description Requirements**
Each PR must include:
- **Linear easy to understand commit history** Make sure that the story of the intent of the PR is told in a linear and understandable way from commit to commit through the entirety of the PR history.
- **Clear title** following conventional commits
- **Detailed description** of functionality added
- **Testing section** describing test coverage
- **Documentation section** listing docs added/updated
- **Dependencies** clearly stated if applicable
- **Breaking changes** section if relevant

### **Review Readiness Checklist**
- [ ] Follows exact KCP architectural patterns
- [ ] Maintains workspace isolation throughout
- [ ] Includes comprehensive error handling
- [ ] Has performance considerations documented
- [ ] Security implications assessed
- [ ] Backward compatibility maintained
- [ ] Integration points clearly documented

## 🚀 **Getting Started**

1. **Read all plan documents** thoroughly before starting
2. **Start with PR 01** (API Foundation) - branch from main
3. **Implement incrementally** - commit logical chunks frequently
4. **Test continuously** - run tests after each logical change
5. **Document as you go** - don't leave docs until the end
6. **Review your own work** before pushing - be your own first reviewer

## ⚠️ **Critical Warnings**

- **NEVER leave uncommited files** - Add all files into a logical commit that encapsulates the intent of a change. If the files do not need to be tracked then add them to .gitignore
- **NEVER shortcuts tests** - every feature must be tested
- **NEVER REWRITE HISTORY** - git commits must be maintained. If you need to reorder them then create a new branch.
- **NEVER merge branches** - maintainers control main
- **NEVER exceed 800 lines per file (generated files do not count, this applies to both deepcopy and crd code)** - Re-architect/refactor to allow for smaller file sizes with better encapsulation/isolation between objects.
- **NEVER exceed 800 lines per commit (generated files do not count, this applies to both deepcopy and crd code)** - Break up commits into smaller atomic commits that still tell the appropriate story of the intent of the changes. Make sure tests continue to pass.
- **NEVER exceed 800 lines per PR (generated files do not count, this applies to both deepcopy and crd code)** - Break up PRs into smaller atomic PRs at logical breaking points. Make sure tests continue to pass in the smaller PRs before continuing.
- **NEVER break workspace isolation** - security is paramount
- **NEVER violate KCP patterns** - follow established conventions exactly

**Remember: Each PR represents your craftsmanship as a developer. Make every PR something you'd be proud to have your name on in the KCP codebase.**