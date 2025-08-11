/*
Copyright 2024 The KCP Authors.

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

package apibinding

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/indexers"
	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	apisv1alpha2client "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/typed/apis/v1alpha2"
	apisv1alpha2informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha2"
	tmcv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/tmc/v1alpha1"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

const (
	ControllerName = "kcp-tmc-apibinding"
	
	// TMCAPIExportPrefix is the prefix for TMC-related APIExports
	TMCAPIExportPrefix = "tmc-"
	
	// ClusterRegistrationAPIExport is the name of the ClusterRegistration APIExport
	ClusterRegistrationAPIExport = "tmc-cluster-registration"
	
	// WorkloadPlacementAPIExport is the name of the WorkloadPlacement APIExport  
	WorkloadPlacementAPIExport = "tmc-workload-placement"
)

// NewController returns a new TMC APIBinding controller for managing TMC-specific APIBindings.
// This controller ensures that TMC workload APIs (ClusterRegistration, WorkloadPlacement) are properly
// bound to workspaces that need TMC functionality, following KCP APIBinding patterns.
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	apiBindingInformer apisv1alpha2informers.APIBindingClusterInformer,
	apiExportInformer apisv1alpha2informers.APIExportClusterInformer,
	clusterRegistrationInformer tmcv1alpha1informers.ClusterRegistrationClusterInformer,
	workloadPlacementInformer tmcv1alpha1informers.WorkloadPlacementClusterInformer,
) (*controller, error) {
	c := &controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),
		kcpClusterClient: kcpClusterClient,

		listAPIBindings: func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIBinding, error) {
			return apiBindingInformer.Lister().Cluster(clusterName).List(labels.Everything())
		},
		getTMCAPIBindings: func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIBinding, error) {
			bindings, err := apiBindingInformer.Lister().Cluster(clusterName).List(labels.Everything())
			if err != nil {
				return nil, err
			}
			
			var tmcBindings []*apisv1alpha2.APIBinding
			for _, binding := range bindings {
				if isTMCAPIBinding(binding) {
					tmcBindings = append(tmcBindings, binding)
				}
			}
			return tmcBindings, nil
		},
		getAPIBinding: func(clusterName logicalcluster.Name, name string) (*apisv1alpha2.APIBinding, error) {
			return apiBindingInformer.Lister().Cluster(clusterName).Get(name)
		},
		getAPIExport: func(clusterName logicalcluster.Name, name string) (*apisv1alpha2.APIExport, error) {
			return apiExportInformer.Lister().Cluster(clusterName).Get(name)
		},
		listAPIExports: func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIExport, error) {
			return apiExportInformer.Lister().Cluster(clusterName).List(labels.Everything())
		},
		listClusterRegistrations: func(clusterName logicalcluster.Name) ([]*tmcv1alpha1.ClusterRegistration, error) {
			return clusterRegistrationInformer.Lister().Cluster(clusterName).List(labels.Everything())
		},
		listWorkloadPlacements: func(clusterName logicalcluster.Name) ([]*tmcv1alpha1.WorkloadPlacement, error) {
			return workloadPlacementInformer.Lister().Cluster(clusterName).List(labels.Everything())
		},
		createAPIBinding: func(ctx context.Context, clusterPath logicalcluster.Path, binding *apisv1alpha2.APIBinding) (*apisv1alpha2.APIBinding, error) {
			return kcpClusterClient.ApisV1alpha2().APIBindings().Cluster(clusterPath).Create(ctx, binding, metav1.CreateOptions{})
		},
		updateAPIBinding: func(ctx context.Context, clusterPath logicalcluster.Path, binding *apisv1alpha2.APIBinding) (*apisv1alpha2.APIBinding, error) {
			return kcpClusterClient.ApisV1alpha2().APIBindings().Cluster(clusterPath).Update(ctx, binding, metav1.UpdateOptions{})
		},

		commit: committer.NewCommitter[*APIBinding, Patcher, *APIBindingSpec, *APIBindingStatus](kcpClusterClient.ApisV1alpha2().APIBindings()),
	}

	logger := logging.WithReconciler(klog.Background(), ControllerName)

	// APIBinding handlers - watch for TMC-related APIBindings
	_, _ = apiBindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			binding := obj.(*apisv1alpha2.APIBinding)
			return isTMCAPIBinding(binding)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueueAPIBinding(objOrTombstone[*apisv1alpha2.APIBinding](obj), logger, "")
			},
			UpdateFunc: func(_, obj interface{}) {
				c.enqueueAPIBinding(objOrTombstone[*apisv1alpha2.APIBinding](obj), logger, "")
			},
			DeleteFunc: func(obj interface{}) {
				c.enqueueAPIBinding(objOrTombstone[*apisv1alpha2.APIBinding](obj), logger, "")
			},
		},
	})

	// APIExport handlers - watch for TMC-related APIExports
	_, _ = apiExportInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			export := obj.(*apisv1alpha2.APIExport)
			return isTMCAPIExport(export)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueueAPIExport(objOrTombstone[*apisv1alpha2.APIExport](obj), logger, "")
			},
			UpdateFunc: func(_, obj interface{}) {
				c.enqueueAPIExport(objOrTombstone[*apisv1alpha2.APIExport](obj), logger, "")
			},
			DeleteFunc: func(obj interface{}) {
				c.enqueueAPIExport(objOrTombstone[*apisv1alpha2.APIExport](obj), logger, "")
			},
		},
	})

	// ClusterRegistration handlers - watch for TMC resource changes
	_, _ = clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueClusterRegistration(objOrTombstone[*tmcv1alpha1.ClusterRegistration](obj), logger, "")
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueClusterRegistration(objOrTombstone[*tmcv1alpha1.ClusterRegistration](obj), logger, "")
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueClusterRegistration(objOrTombstone[*tmcv1alpha1.ClusterRegistration](obj), logger, "")
		},
	})

	// WorkloadPlacement handlers - watch for TMC resource changes
	_, _ = workloadPlacementInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueWorkloadPlacement(objOrTombstone[*tmcv1alpha1.WorkloadPlacement](obj), logger, "")
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueWorkloadPlacement(objOrTombstone[*tmcv1alpha1.WorkloadPlacement](obj), logger, "")
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueWorkloadPlacement(objOrTombstone[*tmcv1alpha1.WorkloadPlacement](obj), logger, "")
		},
	})

	return c, nil
}

func objOrTombstone[T runtime.Object](obj any) T {
	if t, ok := obj.(T); ok {
		return t
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if t, ok := tombstone.Obj.(T); ok {
			return t
		}
		panic(fmt.Errorf("tombstone %T is not a %T", tombstone, new(T)))
	}
	panic(fmt.Errorf("%T is not a %T", obj, new(T)))
}

type APIBinding = apisv1alpha2.APIBinding
type APIBindingSpec = apisv1alpha2.APIBindingSpec
type APIBindingStatus = apisv1alpha2.APIBindingStatus
type Patcher = apisv1alpha2client.APIBindingInterface
type Resource = committer.Resource[*APIBindingSpec, *APIBindingStatus]
type CommitFunc = func(context.Context, *Resource, *Resource) error

// controller manages TMC-specific APIBindings, ensuring that workspaces requiring TMC functionality
// have the appropriate APIBindings for ClusterRegistration and WorkloadPlacement APIs.
type controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	kcpClusterClient kcpclientset.ClusterInterface

	listAPIBindings   func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIBinding, error)
	getTMCAPIBindings func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIBinding, error)
	getAPIBinding     func(clusterName logicalcluster.Name, name string) (*apisv1alpha2.APIBinding, error)

	getAPIExport  func(clusterName logicalcluster.Name, name string) (*apisv1alpha2.APIExport, error)
	listAPIExports func(clusterName logicalcluster.Name) ([]*apisv1alpha2.APIExport, error)

	listClusterRegistrations func(clusterName logicalcluster.Name) ([]*tmcv1alpha1.ClusterRegistration, error)
	listWorkloadPlacements   func(clusterName logicalcluster.Name) ([]*tmcv1alpha1.WorkloadPlacement, error)

	createAPIBinding func(ctx context.Context, clusterPath logicalcluster.Path, binding *apisv1alpha2.APIBinding) (*apisv1alpha2.APIBinding, error)
	updateAPIBinding func(ctx context.Context, clusterPath logicalcluster.Path, binding *apisv1alpha2.APIBinding) (*apisv1alpha2.APIBinding, error)

	commit CommitFunc
}

// isTMCAPIBinding determines if an APIBinding is TMC-related based on its reference
func isTMCAPIBinding(binding *apisv1alpha2.APIBinding) bool {
	if binding.Spec.Reference.Export == nil {
		return false
	}
	
	exportName := binding.Spec.Reference.Export.Name
	return exportName == ClusterRegistrationAPIExport || 
		   exportName == WorkloadPlacementAPIExport ||
		   (len(exportName) > len(TMCAPIExportPrefix) && exportName[:len(TMCAPIExportPrefix)] == TMCAPIExportPrefix)
}

// isTMCAPIExport determines if an APIExport is TMC-related
func isTMCAPIExport(export *apisv1alpha2.APIExport) bool {
	return export.Name == ClusterRegistrationAPIExport ||
		   export.Name == WorkloadPlacementAPIExport ||
		   (len(export.Name) > len(TMCAPIExportPrefix) && export.Name[:len(TMCAPIExportPrefix)] == TMCAPIExportPrefix)
}

// enqueueAPIBinding enqueues a TMC APIBinding for reconciliation
func (c *controller) enqueueAPIBinding(binding *apisv1alpha2.APIBinding, logger logr.Logger, logSuffix string) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(binding)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	logging.WithQueueKey(logger, key).V(4).Info(fmt.Sprintf("queueing TMC APIBinding%s", logSuffix))
	c.queue.Add(key)
}

// enqueueAPIExport enqueues workspaces that might need TMC APIBindings when TMC APIExports change
func (c *controller) enqueueAPIExport(export *apisv1alpha2.APIExport, logger logr.Logger, logSuffix string) {
	clusterName := logicalcluster.From(export)
	logger = logging.WithObject(logger, export)

	// Find existing APIBindings that reference this APIExport
	bindings, err := c.listAPIBindings(clusterName)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, binding := range bindings {
		if binding.Spec.Reference.Export != nil && binding.Spec.Reference.Export.Name == export.Name {
			c.enqueueAPIBinding(binding, logger, fmt.Sprintf(" because of APIExport%s", logSuffix))
		}
	}
}

// enqueueClusterRegistration triggers workspace reconciliation when ClusterRegistration changes
func (c *controller) enqueueClusterRegistration(cluster *tmcv1alpha1.ClusterRegistration, logger logr.Logger, logSuffix string) {
	clusterName := logicalcluster.From(cluster)
	logger = logging.WithObject(logger, cluster)

	// Enqueue any existing TMC APIBindings in this workspace
	bindings, err := c.getTMCAPIBindings(clusterName)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, binding := range bindings {
		c.enqueueAPIBinding(binding, logger, fmt.Sprintf(" because of ClusterRegistration%s", logSuffix))
	}
}

// enqueueWorkloadPlacement triggers workspace reconciliation when WorkloadPlacement changes
func (c *controller) enqueueWorkloadPlacement(placement *tmcv1alpha1.WorkloadPlacement, logger logr.Logger, logSuffix string) {
	clusterName := logicalcluster.From(placement)
	logger = logging.WithObject(logger, placement)

	// Enqueue any existing TMC APIBindings in this workspace
	bindings, err := c.getTMCAPIBindings(clusterName)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, binding := range bindings {
		c.enqueueAPIBinding(binding, logger, fmt.Sprintf(" because of WorkloadPlacement%s", logSuffix))
	}
}

// Start starts the controller with the specified number of worker threads
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting TMC APIBinding controller")
	defer logger.Info("Shutting down TMC APIBinding controller")

	for range numThreads {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *controller) processNextWorkItem(ctx context.Context) bool {
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(4).Info("processing TMC APIBinding key")

	defer c.queue.Done(key)

	if requeue, err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", ControllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	} else if requeue {
		c.queue.Add(key)
		return true
	}
	
	c.queue.Forget(key)
	return true
}

func (c *controller) process(ctx context.Context, key string) (bool, error) {
	logger := klog.FromContext(ctx)
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return false, nil
	}

	binding, err := c.getAPIBinding(clusterName, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("TMC APIBinding not found, may have been deleted", "cluster", clusterName, "name", name)
			return false, nil
		}
		return false, err
	}

	old := binding
	binding = binding.DeepCopy()

	logger = logging.WithObject(logger, binding)
	ctx = klog.NewContext(ctx, logger)

	var errs []error
	requeue, err := c.reconcile(ctx, binding)
	if err != nil {
		errs = append(errs, err)
	}

	// Commit any changes to the APIBinding status
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &Resource{ObjectMeta: binding.ObjectMeta, Spec: &binding.Spec, Status: &binding.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return requeue, utilerrors.NewAggregate(errs)
}

// reconcile handles the main reconciliation logic for TMC APIBindings
func (c *controller) reconcile(ctx context.Context, binding *apisv1alpha2.APIBinding) (bool, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("reconciling TMC APIBinding")

	if binding.DeletionTimestamp != nil {
		logger.V(4).Info("TMC APIBinding is being deleted")
		return false, nil
	}

	if !isTMCAPIBinding(binding) {
		logger.V(4).Info("APIBinding is not TMC-related, skipping")
		return false, nil
	}

	clusterName := logicalcluster.From(binding)
	
	// Validate that the referenced APIExport exists and is healthy
	if binding.Spec.Reference.Export == nil {
		logger.Error(nil, "TMC APIBinding has no export reference")
		return false, nil
	}

	exportName := binding.Spec.Reference.Export.Name
	exportPath := binding.Spec.Reference.Export.Path
	
	var targetCluster logicalcluster.Name
	if exportPath != "" {
		targetCluster = logicalcluster.NewPath(exportPath).Last()
	} else {
		targetCluster = clusterName
	}

	export, err := c.getAPIExport(targetCluster, exportName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "TMC APIExport not found", "exportName", exportName, "exportCluster", targetCluster)
			return true, nil // Requeue to wait for APIExport
		}
		return false, err
	}

	// Update APIBinding status based on the export state
	return c.updateAPIBindingStatus(ctx, binding, export)
}

// updateAPIBindingStatus updates the APIBinding status based on the referenced APIExport
func (c *controller) updateAPIBindingStatus(ctx context.Context, binding *apisv1alpha2.APIBinding, export *apisv1alpha2.APIExport) (bool, error) {
	logger := klog.FromContext(ctx)

	// Check if the APIExport is ready
	exportReady := false
	for _, condition := range export.Status.Conditions {
		if condition.Type == apisv1alpha2.APIExportValid && condition.Status == metav1.ConditionTrue {
			exportReady = true
			break
		}
	}

	// Update conditions based on export state
	conditions := binding.Status.Conditions
	now := metav1.NewTime(time.Now())

	if exportReady {
		// Set or update the Ready condition to True
		setAPIBindingCondition(&conditions, apisv1alpha2.APIBindingInitialBindingCompleted, metav1.ConditionTrue, "TMCAPIExportReady", 
			fmt.Sprintf("TMC APIExport %s is ready for binding", export.Name), now)
	} else {
		// Set or update the Ready condition to False
		setAPIBindingCondition(&conditions, apisv1alpha2.APIBindingInitialBindingCompleted, metav1.ConditionFalse, "TMCAPIExportNotReady",
			fmt.Sprintf("TMC APIExport %s is not ready", export.Name), now)
	}

	binding.Status.Conditions = conditions
	
	logger.V(4).Info("updated TMC APIBinding status", "ready", exportReady, "exportName", export.Name)
	return false, nil
}

// setAPIBindingCondition sets or updates a condition in the conditions slice
func setAPIBindingCondition(conditions *[]metav1.Condition, conditionType string, status metav1.ConditionStatus, reason, message string, transitionTime metav1.Time) {
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: transitionTime,
		Reason:             reason,
		Message:            message,
	}

	for i, condition := range *conditions {
		if condition.Type == conditionType {
			if condition.Status != status {
				condition.Status = status
				condition.LastTransitionTime = transitionTime
				condition.Reason = reason
				condition.Message = message
				(*conditions)[i] = condition
			}
			return
		}
	}

	*conditions = append(*conditions, newCondition)
}

// InstallIndexers installs the indexers required by this controller
func InstallIndexers(
	apiBindingInformer apisv1alpha2informers.APIBindingClusterInformer,
) {
	indexers.AddIfNotPresentOrDie(apiBindingInformer.Informer().GetIndexer(), cache.Indexers{
		IndexTMCAPIBindingsByExport: IndexTMCAPIBindingsByExportFunc,
	})
}