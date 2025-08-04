# TMC Reimplementation Plan 2 - Phase 2: External TMC Controllers

## 🎯 **CRITICAL ARCHITECTURAL FOUNDATION**

**TMC Controllers are EXTERNAL to KCP - they consume KCP APIs via APIBinding**

- **KCP Role**: Provides TMC APIs via APIExport (from Phase 1)
- **TMC Controllers**: External processes that watch KCP APIs and manage physical clusters
- **Integration**: TMC controllers bind to KCP APIs in workspaces, respect multi-tenancy
- **Execution**: Workloads are created on physical clusters, not in KCP

## 📋 **Phase 2 Objectives**

**Build external TMC controller system that properly consumes KCP APIs**

- Create external TMC controller binary
- Implement cluster registration and management
- Build workspace-aware controller patterns
- Establish workload placement foundation
- **Scope**: 900-1200 lines across 2 PRs

## 🏗️ **External Controller Architecture**

### **Understanding KCP External Controller Patterns**

```go
// Study existing KCP external controller patterns:
// - How external controllers connect to KCP via APIBinding
// - Workspace-aware client configuration
// - Multi-tenant controller design
// - Proper LogicalCluster handling
```

**External Controller Principles:**
1. **Separate binary** - TMC controllers run outside KCP
2. **APIBinding consumption** - bind to TMC APIs in workspaces
3. **Physical cluster management** - create/update workloads on real clusters
4. **Status propagation** - report back to KCP through status updates
5. **Multi-workspace support** - handle multiple workspaces with isolation

## 📊 **PR 3: TMC Controller Foundation (~600 lines)**

**Objective**: Create external TMC controller binary with proper KCP integration

### **Files Created:**
```
cmd/tmc-controller/main.go                          (~100 lines)
cmd/tmc-controller/options/options.go               (~150 lines)
pkg/tmc/controller/clusterregistration.go          (~200 lines)
pkg/tmc/controller/clusterregistration_test.go     (~150 lines)
```

### **TMC Controller Main:**
```go
// cmd/tmc-controller/main.go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/kcp/cmd/tmc-controller/options"
    "github.com/kcp-dev/kcp/pkg/tmc/controller"
)

func main() {
    opts := options.NewOptions()
    opts.AddFlags(flag.CommandLine)
    flag.Parse()
    
    if err := opts.Validate(); err != nil {
        klog.Fatalf("Invalid options: %v", err)
    }
    
    ctx, cancel := signal.NotifyContext(context.Background(), 
        syscall.SIGTERM, syscall.SIGINT)
    defer cancel()
    
    // Build KCP client config for workspace
    kcpConfig, err := clientcmd.BuildConfigFromFlags("", opts.KCPKubeconfig)
    if err != nil {
        klog.Fatalf("Error building KCP config: %v", err)
    }
    
    // Build target cluster configs
    clusterConfigs := make(map[string]*rest.Config)
    for name, kubeconfig := range opts.ClusterKubeconfigs {
        config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            klog.Fatalf("Error building cluster config for %s: %v", name, err)
        }
        clusterConfigs[name] = config
    }
    
    // Create TMC controller manager
    mgr, err := controller.NewManager(ctx, &controller.Config{
        KCPConfig:       kcpConfig,
        ClusterConfigs:  clusterConfigs,
        Workspace:       opts.Workspace,
        ResyncPeriod:    opts.ResyncPeriod,
    })
    if err != nil {
        klog.Fatalf("Error creating TMC controller manager: %v", err)
    }
    
    klog.InfoS("Starting TMC controller", "workspace", opts.Workspace)
    
    if err := mgr.Start(ctx); err != nil {
        klog.Fatalf("Error starting TMC controller: %v", err)
    }
    
    <-ctx.Done()
    klog.InfoS("Shutting down TMC controller")
}
```

### **TMC Controller Options:**
```go
// cmd/tmc-controller/options/options.go
package options

import (
    "fmt"
    "time"
    
    "github.com/spf13/pflag"
    "k8s.io/client-go/rest"
)

// Options contains configuration for TMC controller
type Options struct {
    // KCP connection
    KCPKubeconfig string
    Workspace     string
    
    // Target cluster connections
    ClusterKubeconfigs map[string]string
    
    // Controller configuration
    ResyncPeriod    time.Duration
    WorkerCount     int
    MetricsPort     int
    HealthPort      int
    
    // Logging
    LogLevel int
}

// NewOptions creates default options
func NewOptions() *Options {
    return &Options{
        ClusterKubeconfigs: make(map[string]string),
        ResyncPeriod:       30 * time.Second,
        WorkerCount:        5,
        MetricsPort:        8080,
        HealthPort:         8081,
        LogLevel:           2,
    }
}

// AddFlags adds flags to the flagset
func (o *Options) AddFlags(fs *pflag.FlagSet) {
    fs.StringVar(&o.KCPKubeconfig, "kcp-kubeconfig", o.KCPKubeconfig,
        "Path to KCP kubeconfig file")
    fs.StringVar(&o.Workspace, "workspace", o.Workspace,
        "KCP workspace to watch (e.g., root:my-workspace)")
    fs.StringToStringVar(&o.ClusterKubeconfigs, "cluster-kubeconfigs", o.ClusterKubeconfigs,
        "Map of cluster names to kubeconfig paths (e.g., cluster1=/path/to/config)")
    fs.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod,
        "Resync period for controllers")
    fs.IntVar(&o.WorkerCount, "worker-count", o.WorkerCount,
        "Number of worker threads")
    fs.IntVar(&o.MetricsPort, "metrics-port", o.MetricsPort,
        "Port for metrics endpoint")
    fs.IntVar(&o.HealthPort, "health-port", o.HealthPort,
        "Port for health endpoint")
    fs.IntVar(&o.LogLevel, "log-level", o.LogLevel,
        "Log verbosity level")
}

// Validate validates the options
func (o *Options) Validate() error {
    if o.KCPKubeconfig == "" {
        return fmt.Errorf("--kcp-kubeconfig is required")
    }
    if o.Workspace == "" {
        return fmt.Errorf("--workspace is required")
    }
    if len(o.ClusterKubeconfigs) == 0 {
        return fmt.Errorf("at least one --cluster-kubeconfigs entry is required")
    }
    return nil
}
```

### **Cluster Registration Controller:**
```go
// pkg/tmc/controller/clusterregistration.go
package controller

import (
    "context"
    "fmt"
    "time"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
    
    kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
    "github.com/kcp-dev/logicalcluster/v3"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    tmcv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/tmc/v1alpha1"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
    "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// ClusterRegistrationController manages physical cluster registration
type ClusterRegistrationController struct {
    queue workqueue.RateLimitingInterface
    
    // KCP clients
    kcpClusterClient         kcpclientset.ClusterInterface
    clusterRegistrationLister tmcv1alpha1informers.ClusterRegistrationClusterLister
    
    // Physical cluster clients
    clusterClients map[string]kubernetes.Interface
    
    // Configuration
    workspace logicalcluster.Name
}

// NewClusterRegistrationController creates a new cluster registration controller
func NewClusterRegistrationController(
    kcpClusterClient kcpclientset.ClusterInterface,
    clusterRegistrationInformer tmcv1alpha1informers.ClusterRegistrationClusterInformer,
    clusterConfigs map[string]*rest.Config,
    workspace logicalcluster.Name,
) (*ClusterRegistrationController, error) {
    
    // Build cluster clients
    clusterClients := make(map[string]kubernetes.Interface)
    for name, config := range clusterConfigs {
        client, err := kubernetes.NewForConfig(config)
        if err != nil {
            return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
        }
        clusterClients[name] = client
    }
    
    c := &ClusterRegistrationController{
        queue: workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(),
            "cluster-registration"),
        kcpClusterClient:          kcpClusterClient,
        clusterRegistrationLister: clusterRegistrationInformer.Lister(),
        clusterClients:           clusterClients,
        workspace:                workspace,
    }
    
    clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { c.enqueue(obj) },
        UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
        DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
    })
    
    return c, nil
}

// enqueue adds a ClusterRegistration to the work queue
func (c *ClusterRegistrationController) enqueue(obj interface{}) {
    key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
    if err != nil {
        utilruntime.HandleError(err)
        return
    }
    c.queue.Add(key)
}

// Start runs the controller
func (c *ClusterRegistrationController) Start(ctx context.Context, numThreads int) {
    defer utilruntime.HandleCrash()
    defer c.queue.ShutDown()
    
    klog.InfoS("Starting ClusterRegistration controller")
    defer klog.InfoS("Shutting down ClusterRegistration controller")
    
    for i := 0; i < numThreads; i++ {
        go c.runWorker(ctx)
    }
    
    <-ctx.Done()
}

// runWorker processes work items from the queue
func (c *ClusterRegistrationController) runWorker(ctx context.Context) {
    for c.processNextWorkItem(ctx) {
    }
}

// processNextWorkItem processes a single work item
func (c *ClusterRegistrationController) processNextWorkItem(ctx context.Context) bool {
    key, quit := c.queue.Get()
    if quit {
        return false
    }
    defer c.queue.Done(key)
    
    err := c.reconcile(ctx, key.(string))
    if err == nil {
        c.queue.Forget(key)
        return true
    }
    
    utilruntime.HandleError(fmt.Errorf("syncing %q failed: %w", key, err))
    c.queue.AddRateLimited(key)
    return true
}

// reconcile handles a single ClusterRegistration resource
func (c *ClusterRegistrationController) reconcile(ctx context.Context, key string) error {
    clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
    if err != nil {
        return err
    }
    
    // Only process resources in our workspace
    if clusterName != c.workspace {
        return nil
    }
    
    clusterReg, err := c.clusterRegistrationLister.Cluster(clusterName).Get(name)
    if errors.IsNotFound(err) {
        klog.V(2).InfoS("ClusterRegistration was deleted", "key", key)
        return nil
    }
    if err != nil {
        return err
    }
    
    return c.syncClusterRegistration(ctx, clusterReg)
}

// syncClusterRegistration processes a ClusterRegistration
func (c *ClusterRegistrationController) syncClusterRegistration(
    ctx context.Context,
    clusterReg *tmcv1alpha1.ClusterRegistration,
) error {
    
    klog.V(2).InfoS("Processing ClusterRegistration", 
        "name", clusterReg.Name,
        "location", clusterReg.Spec.Location)
    
    // Check if we have a client for this cluster
    clusterClient, exists := c.clusterClients[clusterReg.Name]
    if !exists {
        return c.updateClusterRegistrationStatus(ctx, clusterReg, false, 
            "ClusterNotConfigured", "No kubeconfig provided for this cluster")
    }
    
    // Test cluster connectivity
    healthy, err := c.testClusterHealth(ctx, clusterClient)
    if err != nil {
        return c.updateClusterRegistrationStatus(ctx, clusterReg, false,
            "ClusterUnhealthy", fmt.Sprintf("Health check failed: %v", err))
    }
    
    if !healthy {
        return c.updateClusterRegistrationStatus(ctx, clusterReg, false,
            "ClusterUnhealthy", "Cluster failed health checks")
    }
    
    // Update status to ready
    return c.updateClusterRegistrationStatus(ctx, clusterReg, true,
        "ClusterReady", "Cluster is healthy and ready")
}

// testClusterHealth tests if a cluster is healthy
func (c *ClusterRegistrationController) testClusterHealth(
    ctx context.Context, 
    client kubernetes.Interface,
) (bool, error) {
    
    // Simple health check - try to list nodes
    _, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
    if err != nil {
        return false, err
    }
    
    return true, nil
}

// updateClusterRegistrationStatus updates the ClusterRegistration status
func (c *ClusterRegistrationController) updateClusterRegistrationStatus(
    ctx context.Context,
    clusterReg *tmcv1alpha1.ClusterRegistration,
    ready bool,
    reason string,
    message string,
) error {
    
    clusterReg = clusterReg.DeepCopy()
    
    // Update heartbeat
    now := metav1.NewTime(time.Now())
    clusterReg.Status.LastHeartbeat = &now
    
    // Update Ready condition
    status := metav1.ConditionFalse
    if ready {
        status = metav1.ConditionTrue
    }
    
    conditions.MarkTrue(clusterReg, tmcv1alpha1.ClusterRegistrationReady)
    if !ready {
        conditions.MarkFalse(clusterReg, 
            tmcv1alpha1.ClusterRegistrationReady,
            reason, 
            conditionsv1alpha1.ConditionSeverityError,
            message)
    }
    
    _, err := c.kcpClusterClient.Cluster(c.workspace.Path()).
        TmcV1alpha1().
        ClusterRegistrations().
        UpdateStatus(ctx, clusterReg, metav1.UpdateOptions{})
    
    if err != nil {
        klog.ErrorS(err, "Failed to update ClusterRegistration status",
            "name", clusterReg.Name)
    }
    
    return err
}
```

## 📊 **PR 4: Workload Placement Controller (~500 lines)**

**Objective**: Add workload placement logic that consumes WorkloadPlacement resources

### **Files Created:**
```
pkg/tmc/controller/workloadplacement.go             (~250 lines)
pkg/tmc/controller/workloadplacement_test.go       (~150 lines)
pkg/tmc/controller/manager.go                      (~100 lines)
```

### **Workload Placement Controller:**
```go
// pkg/tmc/controller/workloadplacement.go
package controller

import (
    "context"
    "fmt"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/util/workqueue"
    "k8s.io/klog/v2"
    
    kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
    "github.com/kcp-dev/logicalcluster/v3"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    tmcv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/tmc/v1alpha1"
    conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
    "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// WorkloadPlacementController manages workload placement decisions
type WorkloadPlacementController struct {
    queue workqueue.RateLimitingInterface
    
    kcpClusterClient             kcpclientset.ClusterInterface
    workloadPlacementLister      tmcv1alpha1informers.WorkloadPlacementClusterLister
    clusterRegistrationLister    tmcv1alpha1informers.ClusterRegistrationClusterLister
    
    workspace logicalcluster.Name
}

// NewWorkloadPlacementController creates a new workload placement controller
func NewWorkloadPlacementController(
    kcpClusterClient kcpclientset.ClusterInterface,
    workloadPlacementInformer tmcv1alpha1informers.WorkloadPlacementClusterInformer,
    clusterRegistrationInformer tmcv1alpha1informers.ClusterRegistrationClusterInformer,
    workspace logicalcluster.Name,
) (*WorkloadPlacementController, error) {
    
    c := &WorkloadPlacementController{
        queue: workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(),
            "workload-placement"),
        kcpClusterClient:          kcpClusterClient,
        workloadPlacementLister:   workloadPlacementInformer.Lister(),
        clusterRegistrationLister: clusterRegistrationInformer.Lister(),
        workspace:                 workspace,
    }
    
    workloadPlacementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { c.enqueue(obj) },
        UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
        DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
    })
    
    // Watch cluster registrations for placement decisions
    clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    func(obj interface{}) { c.enqueuePlacements() },
        UpdateFunc: func(_, obj interface{}) { c.enqueuePlacements() },
        DeleteFunc: func(obj interface{}) { c.enqueuePlacements() },
    })
    
    return c, nil
}

// reconcile handles WorkloadPlacement resources
func (c *WorkloadPlacementController) reconcile(ctx context.Context, key string) error {
    clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
    if err != nil {
        return err
    }
    
    if clusterName != c.workspace {
        return nil
    }
    
    placement, err := c.workloadPlacementLister.Cluster(clusterName).Get(name)
    if errors.IsNotFound(err) {
        klog.V(2).InfoS("WorkloadPlacement was deleted", "key", key)
        return nil
    }
    if err != nil {
        return err
    }
    
    return c.syncWorkloadPlacement(ctx, placement)
}

// syncWorkloadPlacement processes a WorkloadPlacement
func (c *WorkloadPlacementController) syncWorkloadPlacement(
    ctx context.Context,
    placement *tmcv1alpha1.WorkloadPlacement,
) error {
    
    klog.V(2).InfoS("Processing WorkloadPlacement",
        "name", placement.Name,
        "strategy", placement.Spec.Strategy)
    
    // Get available clusters
    availableClusters, err := c.getAvailableClusters(ctx, placement)
    if err != nil {
        return err
    }
    
    // Apply placement strategy
    selectedClusters := c.applyPlacementStrategy(placement, availableClusters)
    
    // Update placement status
    return c.updateWorkloadPlacementStatus(ctx, placement, selectedClusters)
}

// getAvailableClusters finds clusters that match placement requirements
func (c *WorkloadPlacementController) getAvailableClusters(
    ctx context.Context,
    placement *tmcv1alpha1.WorkloadPlacement,
) ([]*tmcv1alpha1.ClusterRegistration, error) {
    
    allClusters, err := c.clusterRegistrationLister.Cluster(c.workspace).List(labels.Everything())
    if err != nil {
        return nil, err
    }
    
    var availableClusters []*tmcv1alpha1.ClusterRegistration
    
    for _, cluster := range allClusters {
        // Check if cluster is ready
        if !conditions.IsTrue(cluster, tmcv1alpha1.ClusterRegistrationReady) {
            continue
        }
        
        // Apply location selector if specified
        if placement.Spec.LocationSelector != nil {
            selector, err := metav1.LabelSelectorAsSelector(placement.Spec.LocationSelector)
            if err != nil {
                continue
            }
            
            clusterLabels := labels.Set{"location": cluster.Spec.Location}
            if !selector.Matches(clusterLabels) {
                continue
            }
        }
        
        // Check capability requirements
        if c.meetsCapabilityRequirements(cluster, placement.Spec.CapabilityRequirements) {
            availableClusters = append(availableClusters, cluster)
        }
    }
    
    return availableClusters, nil
}

// applyPlacementStrategy selects clusters based on strategy
func (c *WorkloadPlacementController) applyPlacementStrategy(
    placement *tmcv1alpha1.WorkloadPlacement,
    availableClusters []*tmcv1alpha1.ClusterRegistration,
) []string {
    
    if len(availableClusters) == 0 {
        return nil
    }
    
    switch placement.Spec.Strategy {
    case tmcv1alpha1.PlacementStrategyRoundRobin:
        // Simple round-robin: select first available
        return []string{availableClusters[0].Name}
        
    case tmcv1alpha1.PlacementStrategySpread:
        // Spread across all available clusters
        var selected []string
        for _, cluster := range availableClusters {
            selected = append(selected, cluster.Name)
        }
        return selected
        
    case tmcv1alpha1.PlacementStrategyAffinity:
        // For now, same as round-robin
        return []string{availableClusters[0].Name}
        
    default:
        return []string{availableClusters[0].Name}
    }
}

// Helper methods for capability checking, status updates, etc...
```

### **Controller Manager:**
```go
// pkg/tmc/controller/manager.go
package controller

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "k8s.io/client-go/rest"
    "k8s.io/klog/v2"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
    kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

// Manager manages all TMC controllers
type Manager struct {
    kcpClusterClient   kcpclientset.ClusterInterface
    informerFactory    kcpinformers.SharedInformerFactory
    
    clusterRegistrationController *ClusterRegistrationController
    workloadPlacementController   *WorkloadPlacementController
    
    workspace    logicalcluster.Name
    resyncPeriod time.Duration
}

// Config contains manager configuration
type Config struct {
    KCPConfig      *rest.Config
    ClusterConfigs map[string]*rest.Config
    Workspace      string
    ResyncPeriod   time.Duration
}

// NewManager creates a new TMC controller manager
func NewManager(ctx context.Context, config *Config) (*Manager, error) {
    workspace := logicalcluster.Name(config.Workspace)
    
    // Create KCP client
    kcpClusterClient, err := kcpclientset.NewForConfig(config.KCPConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create KCP client: %w", err)
    }
    
    // Create informer factory for our workspace
    informerFactory := kcpinformers.NewSharedInformerFactoryWithOptions(
        kcpClusterClient,
        config.ResyncPeriod,
        kcpinformers.WithCluster(workspace),
    )
    
    // Create controllers
    clusterRegController, err := NewClusterRegistrationController(
        kcpClusterClient,
        informerFactory.Tmc().V1alpha1().ClusterRegistrations(),
        config.ClusterConfigs,
        workspace,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create cluster registration controller: %w", err)
    }
    
    placementController, err := NewWorkloadPlacementController(
        kcpClusterClient,
        informerFactory.Tmc().V1alpha1().WorkloadPlacements(),
        informerFactory.Tmc().V1alpha1().ClusterRegistrations(),
        workspace,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create workload placement controller: %w", err)
    }
    
    return &Manager{
        kcpClusterClient:              kcpClusterClient,
        informerFactory:               informerFactory,
        clusterRegistrationController: clusterRegController,
        workloadPlacementController:   placementController,
        workspace:                     workspace,
        resyncPeriod:                  config.ResyncPeriod,
    }, nil
}

// Start starts all controllers
func (m *Manager) Start(ctx context.Context) error {
    klog.InfoS("Starting TMC controller manager", "workspace", m.workspace)
    
    // Start informers
    m.informerFactory.Start(ctx.Done())
    
    // Wait for cache sync
    if !cache.WaitForCacheSync(ctx.Done(), m.informerFactory.WaitForCacheSync(ctx.Done())...) {
        return fmt.Errorf("failed to wait for informer caches to sync")
    }
    
    var wg sync.WaitGroup
    
    // Start controllers
    wg.Add(1)
    go func() {
        defer wg.Done()
        m.clusterRegistrationController.Start(ctx, 2)
    }()
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        m.workloadPlacementController.Start(ctx, 2)
    }()
    
    wg.Wait()
    return nil
}
```

## ✅ **Phase 2 Success Criteria**

### **External Controller Compliance:**
1. **✅ External binary** - TMC controllers run outside KCP
2. **✅ APIBinding consumption** - controllers bind to TMC APIs in workspaces
3. **✅ Physical cluster management** - controllers manage real clusters
4. **✅ Workspace isolation** - controllers respect workspace boundaries
5. **✅ Proper status propagation** - status flows back to KCP

### **Technical Validation:**
- TMC controller can connect to KCP workspace via APIBinding
- Cluster registration works with physical clusters
- Workload placement decisions are made correctly
- Status updates flow back to KCP properly
- Multi-workspace isolation is maintained

### **Usage Example:**
```bash
# Start TMC controller for a workspace
tmc-controller \
  --kcp-kubeconfig=/path/to/kcp/config \
  --workspace=root:production \
  --cluster-kubeconfigs=east=/path/to/east/config,west=/path/to/west/config \
  --worker-count=5
```

## 🎯 **Phase 2 Outcome**

This phase establishes:
- **External TMC controller system** that properly consumes KCP APIs
- **Physical cluster management** outside of KCP
- **Workspace-aware controller patterns** for multi-tenancy
- **Foundation for workload synchronization** in Phase 3
- **Proper separation of concerns** between KCP and TMC

**Phase 2 provides the correct external controller architecture for TMC, respecting KCP's role as API provider while handling workload execution externally.**