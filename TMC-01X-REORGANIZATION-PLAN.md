# TMC 01x Branch Reorganization Plan

## 📊 Current Branch Analysis

### **Size Analysis Results:**
| Branch | Size | Status | APIs Contained |
|--------|------|--------|----------------|
| **01a-cluster-basic** | 269 lines | ✅ **GOOD** | ClusterRegistration (basic) |
| **01b-cluster-enhanced** | 532 lines | ✅ **GOOD** | ClusterRegistration (enhanced) |
| **01c-placement-basic** | 757 lines | ✅ **GOOD** | ClusterRegistration + WorkloadPlacement |
| **01d-placement-advanced** | 953 lines | ❌ **TOO BIG** | + WorkloadPlacementAdvanced |
| **01e-placement-analysis** | ~1,400 lines | ❌ **TOO BIG** | + analysis features |
| **01f-placement-health** | 1,664 lines | ❌ **TOO BIG** | + WorkloadHealthPolicy |
| **01g-placement-session** | ~1,800 lines | ❌ **TOO BIG** | + WorkloadSessionPolicy |
| **01h-traffic-analysis** | 2,123 lines | ❌ **MASSIVELY TOO BIG** | + TrafficMetrics |
| **01i-scaling-config** | ~2,800 lines | ❌ **MASSIVELY TOO BIG** | + WorkloadScalingPolicy |
| **01j-status-management** | 3,610 lines | ❌ **MASSIVELY TOO BIG** | + WorkloadStatusAggregator |

### **Key Problems:**
1. **Violates Size Rules**: Branches 01d-01j exceed 800-line maximum
2. **Non-Atomic**: Later branches contain multiple unrelated APIs
3. **Dependency Confusion**: Dependencies are not clear between APIs
4. **Scope Creep**: Each branch adds too much functionality

---

## 🎯 Reorganization Strategy

### **New Branch Structure (Size-Compliant & Atomic)**

#### **Phase 1: Foundation APIs (400-600 lines each)**

**PR 01a: Basic Cluster Management API** ✅ **KEEP AS-IS**
```
Branch: feature/tmc2-impl2/01a-cluster-basic
Size: 269 lines (PERFECT)
Dependencies: None
APIs: ClusterRegistration (basic)
```

**PR 01b: Enhanced Cluster Management API** ✅ **KEEP AS-IS**  
```
Branch: feature/tmc2-impl2/01b-cluster-enhanced  
Size: 532 lines (GOOD)
Dependencies: Builds on 01a
APIs: ClusterRegistration (enhanced with capabilities, credentials, quotas)
```

**PR 01c: Basic Placement API** ✅ **KEEP AS-IS**
```
Branch: feature/tmc2-impl2/01c-placement-basic
Size: 757 lines (WITHIN LIMITS)
Dependencies: Requires 01a (uses ClusterRegistration)
APIs: WorkloadPlacement (basic placement policies)
```

#### **Phase 2: Advanced APIs (Split Oversized Branches)**

**NEW PR 01d: Advanced Placement Core** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01d-placement-core (NEW - extract from current 01d)
Target Size: ~400 lines
Dependencies: Builds on 01c
APIs: WorkloadPlacementAdvanced (core affinity rules only)
Content: Extract just the core WorkloadPlacementAdvanced API without complex features
```

**NEW PR 01e: Placement Rollout Strategies** 🔄 **NEEDS CREATION**
```
Branch: feature/tmc2-impl2/01e-placement-rollouts (NEW - extract from current 01d)
Target Size: ~300 lines  
Dependencies: Requires 01d (builds on WorkloadPlacementAdvanced)
APIs: Add rollout strategy support to WorkloadPlacementAdvanced
Content: Rollout phases, strategy definitions, canary deployments
```

**NEW PR 01f: Health Policy API** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01f-health-policy (NEW - extract from current 01f)
Target Size: ~500 lines
Dependencies: Requires 01c (uses WorkloadPlacement)
APIs: WorkloadHealthPolicy (basic health monitoring)
Content: Extract just WorkloadHealthPolicy without complex integration
```

**NEW PR 01g: Session Policy API** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01g-session-policy (NEW - extract from current 01g)
Target Size: ~400 lines
Dependencies: Requires 01c (uses WorkloadPlacement)
APIs: WorkloadSessionPolicy (session stickiness)  
Content: Extract just WorkloadSessionPolicy without complex features
```

**NEW PR 01h: Traffic Metrics API** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01h-traffic-metrics (NEW - extract from current 01h)
Target Size: ~500 lines
Dependencies: Requires 01c (uses WorkloadPlacement)
APIs: TrafficMetrics (basic traffic analysis)
Content: Extract core TrafficMetrics API without advanced analytics
```

**NEW PR 01i: Scaling Policy API** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01i-scaling-policy (NEW - extract from current 01i)
Target Size: ~400 lines
Dependencies: Requires 01c (uses WorkloadPlacement)
APIs: WorkloadScalingPolicy (basic auto-scaling)
Content: Extract core WorkloadScalingPolicy without complex algorithms
```

**NEW PR 01j: Status Aggregation API** 🔄 **NEEDS SPLIT**
```
Branch: feature/tmc2-impl2/01j-status-aggregator (NEW - extract from current 01j)
Target Size: ~400 lines
Dependencies: Requires 01c (uses WorkloadPlacement)
APIs: WorkloadStatusAggregator (status collection)
Content: Extract core WorkloadStatusAggregator without complex aggregation
```

---

## 🔧 Implementation Plan

### **Step 1: Preserve Good Branches** ✅
- Keep 01a, 01b, 01c as-is (they're already compliant)
- These form the solid foundation for all future work

### **Step 2: Create New Size-Compliant Branches** 🔄

**Method: Surgical Extraction**
For each oversized branch (01d-01j):

1. **Create new branch from main**
2. **Cherry-pick only the core API** from the oversized branch
3. **Remove complex features** to meet size limits
4. **Ensure atomic functionality** (one API per PR)
5. **Add minimal tests** to verify the API works
6. **Update register.go** to include only the new API

### **Step 3: Dependency Chain** 📈

**Linear Dependencies:**
```
01a (basic cluster)
  ↓
01b (enhanced cluster) 
  ↓  
01c (basic placement)
  ↓
01d (advanced placement core)
  ↓
01e (placement rollouts)
```

**Parallel Extensions** (all depend on 01c):
```
01c (basic placement)
  ├→ 01f (health policy)
  ├→ 01g (session policy)  
  ├→ 01h (traffic metrics)
  ├→ 01i (scaling policy)
  └→ 01j (status aggregator)
```

### **Step 4: Update Documentation**
- Update PR plan with new branches
- Create dependency diagrams
- Document API progression

---

## 📋 Detailed Implementation Steps

### **For Each Oversized Branch:**

#### **Extract Core API Pattern:**
```bash
# 1. Create new branch from main
git checkout main
git pull origin main
git checkout -b feature/tmc2-impl2/01d-placement-core

# 2. Cherry-pick minimal API definition
# Extract just the core struct definitions
# Remove complex features, advanced algorithms
# Keep only basic functionality

# 3. Ensure size compliance
find /workspaces/kcp/pkg/apis/tmc/v1alpha1 -name "*.go" -not -name "zz_generated*" | xargs wc -l
# Target: <800 lines, ideally 400-600

# 4. Test atomic functionality
make test
go run ./hack/codegen.sh

# 5. Commit with proper size
git add .
git commit -s -S -m "feat(api): add core WorkloadPlacementAdvanced API

- Extract core affinity and placement rules
- Basic node/pod affinity support  
- Foundation for rollout strategies
- Size: ~400 lines (WITHIN LIMITS)
"
```

### **Size Targets for New Branches:**
- **01d-placement-core**: 400 lines (core WorkloadPlacementAdvanced)
- **01e-placement-rollouts**: 300 lines (rollout strategies)
- **01f-health-policy**: 500 lines (WorkloadHealthPolicy)
- **01g-session-policy**: 400 lines (WorkloadSessionPolicy)
- **01h-traffic-metrics**: 500 lines (TrafficMetrics)
- **01i-scaling-policy**: 400 lines (WorkloadScalingPolicy)
- **01j-status-aggregator**: 400 lines (WorkloadStatusAggregator)

**Total: 2,800 lines across 7 focused, atomic PRs**

---

## ✅ Success Criteria

### **Each New Branch Must:**
- ✅ **Size Compliant**: ≤800 lines (target 400-600)
- ✅ **Atomic**: Contains exactly one API or feature
- ✅ **Testable**: Includes comprehensive tests for the API
- ✅ **Buildable**: Compiles and generates code correctly
- ✅ **Self-Contained**: Can be reviewed and merged independently
- ✅ **Clear Dependencies**: Obvious what it builds on

### **Overall Result:**
- **10 size-compliant branches** instead of 4 oversized ones
- **Clear dependency chain** that's easy to follow
- **Atomic functionality** that's easy to review
- **Linear progression** from basic to advanced features
- **Maintainer-friendly** PRs that follow KCP conventions

---

## 🚀 Next Actions

1. **Start with 01d**: Split current 01d into 01d-core + 01e-rollouts
2. **Validate Pattern**: Ensure the extraction works and meets size limits  
3. **Continue Systematically**: Apply same pattern to 01f-01j
4. **Update Dependencies**: Ensure new branches build on correct foundations
5. **Test Integration**: Verify all APIs work together properly
6. **Update PR Plan**: Reflect new branch structure in submission plan

This reorganization will result in **properly sized, atomic PRs** that follow TMC Reimplementation Plan 2 guidelines and are ready for maintainer review.