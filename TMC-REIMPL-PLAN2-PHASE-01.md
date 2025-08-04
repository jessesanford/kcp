# TMC Reimplementation Plan 2 - Phase 1: KCP Integration Foundation

## 🎯 **CRITICAL ARCHITECTURAL UNDERSTANDING**

**TMC is NOT part of KCP - TMC is an external system that consumes KCP APIs**

- **KCP Role**: Control plane for building platforms, provides APIs via APIExport
- **TMC Role**: External controller that watches KCP APIs and manages workloads on physical clusters
- **Integration**: TMC consumes KCP APIs through APIBinding, respects workspace isolation

## 📋 **Phase 1 Objectives**

**Establish proper KCP integration foundation using existing KCP patterns**

- Create TMC-specific APIs following exact KCP conventions
- Integrate with existing APIExport/APIBinding system
- Build workspace-aware API foundation
- Follow established KCP controller patterns exactly
- **Scope**: 800-1000 lines across 2 PRs

## 🏗️ **Correct TMC Architecture**

### **Understanding KCP's Actual Patterns**

```go
// Study these existing KCP patterns before implementing:
// pkg/reconciler/apis/apiexport/apiexport_controller.go - API management
// pkg/reconciler/tenancy/workspace/workspace_controller.go - workspace patterns  
// sdk/apis/apis/v1alpha1/types.go - API design conventions
```

**KCP API Design Principles:**
1. **Focused API groups** - single responsibility per group
2. **Workspace awareness** - all resources respect workspace boundaries
3. **LogicalCluster integration** - proper cluster-aware clients
4. **APIExport/APIBinding integration** - how APIs are shared
5. **Condition management** - standard Kubernetes condition patterns

## 📊 **PR 1: TMC Platform API Foundation (~400 lines)**

**Objective**: Create focused TMC platform API that integrates with KCP's APIExport system

### **Files Created:**
```
pkg/apis/tmc/v1alpha1/doc.go              (~25 lines)
pkg/apis/tmc/v1alpha1/register.go         (~50 lines)
pkg/apis/tmc/v1alpha1/types.go            (~300 lines)
pkg/apis/tmc/install/install.go           (~25 lines)
```

### **TMC Platform API Design:**
```go
// pkg/apis/tmc/v1alpha1/types.go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ClusterRegistration represents a physical cluster that can execute workloads
// This integrates with KCP's APIExport/APIBinding system for API distribution
//
// +crd
// +genclient
// +genclient:nonNamespaced  
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=cr
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterRegistration struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:",inline"`
    
    Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
    Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of a ClusterRegistration
type ClusterRegistrationSpec struct {
    // Location provides logical location information for placement decisions
    Location string `json:"location"`
    
    // Capabilities define what this cluster can execute
    Capabilities []ClusterCapability `json:"capabilities,omitempty"`
    
    // APIBindings specify which KCP APIs this cluster should receive
    // This integrates with KCP's existing APIBinding system
    APIBindings []APIBindingReference `json:"apiBindings,omitempty"`
}

// ClusterCapability defines a capability of the physical cluster
type ClusterCapability struct {
    // Type of capability (e.g., "compute", "storage", "networking")
    Type string `json:"type"`
    
    // Available indicates if the capability is currently available
    Available bool `json:"available"`
    
    // Properties provide additional capability metadata
    Properties map[string]string `json:"properties,omitempty"`
}

// APIBindingReference references an APIBinding for this cluster
type APIBindingReference struct {
    // Name of the APIBinding in the workspace
    Name string `json:"name"`
    
    // Workspace containing the APIBinding (defaults to current workspace)
    Workspace string `json:"workspace,omitempty"`
}

// ClusterRegistrationStatus represents the observed state
type ClusterRegistrationStatus struct {
    // Conditions represent the latest available observations
    // +optional
    Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
    
    // ConnectedAPIs tracks which APIs are successfully bound
    ConnectedAPIs []ConnectedAPI `json:"connectedAPIs,omitempty"`
    
    // LastHeartbeat tracks when the cluster last reported status
    LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`
}

// ConnectedAPI tracks API binding status
type ConnectedAPI struct {
    // APIBinding name
    APIBinding string `json:"apiBinding"`
    
    // Connected indicates successful binding
    Connected bool `json:"connected"`
    
    // Error message if connection failed
    Error string `json:"error,omitempty"`
}

// WorkloadPlacement defines where workloads should be placed
// This works with KCP's workspace system for multi-tenant placement
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=wp
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type WorkloadPlacement struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:",inline"`
    
    Spec   WorkloadPlacementSpec   `json:"spec,omitempty"`
    Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines placement policy
type WorkloadPlacementSpec struct {
    // Strategy defines how workloads are placed
    // +kubebuilder:validation:Enum=RoundRobin;Affinity;Spread
    Strategy PlacementStrategy `json:"strategy"`
    
    // LocationSelector selects clusters by location
    LocationSelector *metav1.LabelSelector `json:"locationSelector,omitempty"`
    
    // CapabilityRequirements specify required cluster capabilities
    CapabilityRequirements []CapabilityRequirement `json:"capabilityRequirements,omitempty"`
}

// PlacementStrategy defines placement strategies
type PlacementStrategy string

const (
    PlacementStrategyRoundRobin PlacementStrategy = "RoundRobin"
    PlacementStrategyAffinity   PlacementStrategy = "Affinity"
    PlacementStrategySpread     PlacementStrategy = "Spread"
)

// CapabilityRequirement specifies a required cluster capability
type CapabilityRequirement struct {
    Type     string `json:"type"`
    Required bool   `json:"required"`
}

// WorkloadPlacementStatus represents the observed placement state
type WorkloadPlacementStatus struct {
    // Conditions represent the latest available observations
    // +optional
    Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
    
    // SelectedClusters are the clusters chosen for placement
    SelectedClusters []string `json:"selectedClusters,omitempty"`
}

// List types
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:",inline"`
    Items           []ClusterRegistration `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:",inline"`
    Items           []WorkloadPlacement `json:"items"`
}

// Condition implementations following KCP patterns
func (cr *ClusterRegistration) GetConditions() conditionsv1alpha1.Conditions {
    return cr.Status.Conditions
}

func (cr *ClusterRegistration) SetConditions(conditions conditionsv1alpha1.Conditions) {
    cr.Status.Conditions = conditions
}

func (wp *WorkloadPlacement) GetConditions() conditionsv1alpha1.Conditions {
    return wp.Status.Conditions
}

func (wp *WorkloadPlacement) SetConditions(conditions conditionsv1alpha1.Conditions) {
    wp.Status.Conditions = conditions
}

// Condition types following KCP conventions
const (
    ClusterRegistrationReady conditionsv1alpha1.ConditionType = "Ready"
    WorkloadPlacementReady   conditionsv1alpha1.ConditionType = "Ready"
)
```

### **Why This API Design Works:**
- **Integrates with APIBinding** - uses existing KCP API distribution system
- **Workspace aware** - all resources respect workspace boundaries
- **Follows KCP patterns** - uses standard condition management
- **Focused responsibility** - cluster registration and placement only
- **Extensible** - can add more TMC concepts in future phases

## 📊 **PR 2: TMC APIExport Integration (~600 lines)**

**Objective**: Create APIExport for TMC APIs and demonstrate proper KCP integration

### **Files Created:**
```
pkg/reconciler/tmc/tmcexport/controller.go         (~300 lines)
pkg/reconciler/tmc/tmcexport/controller_test.go    (~200 lines)
config/exports/tmc.yaml                            (~50 lines)
docs/tmc/integration.md                            (~50 lines)
```

### **TMC APIExport Controller:**
```go
// pkg/reconciler/tmc/tmcexport/controller.go
package tmcexport

import (
    "context"
    "fmt"
    
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"

    kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
    "github.com/kcp-dev/logicalcluster/v3"

    apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    apisv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha1"
)

const ControllerName = "tmc-apiexport"

// Controller manages TMC APIExport following exact KCP patterns
type Controller struct {
    queue workqueue.RateLimitingInterface

    kcpClusterClient kcpclientset.ClusterInterface
    
    apiExportLister  apisv1alpha1informers.APIExportClusterLister
    apiExportIndexer cache.Indexer
}

// NewController creates a new TMC APIExport controller following KCP patterns
func NewController(
    kcpClusterClient kcpclientset.ClusterInterface,
    apiExportInformer apisv1alpha1informers.APIExportClusterInformer,
) (*Controller, error) {
    
    queue := workqueue.NewNamedRateLimitingQueue(
        workqueue.DefaultControllerRateLimiter(), 
        ControllerName,
    )

    c := &Controller{
        queue:            queue,
        kcpClusterClient: kcpClusterClient,
        apiExportLister:  apiExportInformer.Lister(),
        apiExportIndexer: apiExportInformer.Informer().GetIndexer(),
    }

    apiExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { c.enqueue(obj) },
        UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
        DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
    })

    return c, nil
}

// enqueue adds an APIExport to the work queue following KCP patterns
func (c *Controller) enqueue(obj interface{}) {
    key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
    if err != nil {
        utilruntime.HandleError(err)
        return
    }
    c.queue.Add(key)
}

// Start runs the controller following KCP patterns
func (c *Controller) Start(ctx context.Context, numThreads int) {
    defer utilruntime.HandleCrash()
    defer c.queue.ShutDown()

    klog.InfoS("Starting TMC APIExport controller")
    defer klog.InfoS("Shutting down TMC APIExport controller")

    for i := 0; i < numThreads; i++ {
        go c.runWorker(ctx)
    }

    <-ctx.Done()
}

// reconcile handles TMC APIExport resources following KCP patterns
func (c *Controller) reconcile(ctx context.Context, key string) error {
    clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
    if err != nil {
        return err
    }

    apiExport, err := c.apiExportLister.Cluster(clusterName).Get(name)
    if errors.IsNotFound(err) {
        klog.V(2).InfoS("TMC APIExport was deleted", "key", key)
        return nil
    }
    if err != nil {
        return err
    }

    // Only process TMC APIExports
    if !c.isTMCAPIExport(apiExport) {
        return nil
    }

    return c.ensureTMCAPIExport(ctx, clusterName, apiExport)
}

// isTMCAPIExport checks if this is a TMC-related APIExport
func (c *Controller) isTMCAPIExport(apiExport *apisv1alpha1.APIExport) bool {
    return apiExport.Name == "tmc.kcp.io"
}

// ensureTMCAPIExport ensures TMC APIExport is properly configured
func (c *Controller) ensureTMCAPIExport(
    ctx context.Context,
    clusterName logicalcluster.Name,
    apiExport *apisv1alpha1.APIExport,
) error {
    
    // Ensure TMC APIs are properly exported
    // This integrates with KCP's existing APIExport system
    
    klog.V(2).InfoS("Processing TMC APIExport", 
        "cluster", clusterName, 
        "apiExport", apiExport.Name)
    
    // Implementation would ensure proper TMC API configuration
    // Following existing APIExport controller patterns
    
    return nil
}

// Additional helper methods following KCP controller patterns...
```

### **TMC APIExport Configuration:**
```yaml
# config/exports/tmc.yaml
apiVersion: apis.kcp.io/v1alpha1
kind: APIExport
metadata:
  name: tmc.kcp.io
spec:
  latestResourceSchemas:
    - tmc.kcp.io.v1alpha1.ClusterRegistration
    - tmc.kcp.io.v1alpha1.WorkloadPlacement
```

### **Integration Documentation:**
```markdown
# docs/tmc/integration.md

# TMC Integration with KCP

TMC integrates with KCP through the standard APIExport/APIBinding system:

1. **TMC APIs** are exported via `tmc.kcp.io` APIExport
2. **Workspaces** bind to TMC APIs via APIBinding
3. **External TMC controllers** watch bound APIs in workspaces
4. **Physical clusters** register via ClusterRegistration resources

This follows KCP's established patterns for API distribution and multi-tenancy.
```

## ✅ **Phase 1 Success Criteria**

### **KCP Pattern Compliance:**
1. **✅ APIs follow exact KCP conventions** (condition management, annotations, etc.)
2. **✅ APIExport integration** - TMC APIs distributed via standard KCP system
3. **✅ Workspace awareness** - all resources respect workspace boundaries
4. **✅ LogicalCluster integration** - proper cluster-aware clients
5. **✅ Controller patterns** - follows apiexport controller exactly

### **Technical Validation:**
- TMC APIs can be exported and bound in workspaces
- Controller follows KCP reconciliation patterns
- Integration with existing APIExport system works
- Workspace isolation is maintained
- All code follows KCP conventions

### **Testing Requirements:**
```go
// Follow exact patterns from pkg/reconciler/apis/apiexport/apiexport_controller_test.go
func TestTMCAPIExportController(t *testing.T) {
    tests := map[string]struct {
        apiExport *apisv1alpha1.APIExport
        workspace string
        wantError bool
    }{
        "tmc apiexport processing": {
            apiExport: &apisv1alpha1.APIExport{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "tmc.kcp.io",
                    Annotations: map[string]string{
                        logicalcluster.AnnotationKey: "root:tmc",
                    },
                },
            },
            workspace: "root:tmc",
            wantError: false,
        },
    }
    // Implementation following KCP test patterns...
}
```

## 🎯 **Phase 1 Outcome**

This phase establishes:
- **Proper KCP integration** through APIExport/APIBinding
- **Focused TMC APIs** that follow KCP conventions
- **Foundation for external TMC controllers** in Phase 2
- **Workspace-aware resource management**
- **Correct architectural understanding** of KCP's role

**Phase 1 provides the proper foundation for building TMC as an external system that consumes KCP APIs, respecting KCP's design principles and architectural boundaries.**