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

package tmcexport

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	apisinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha1"
	apisv1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/apis/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "tmc-apiexport"

	// TMCAPIExportName is the name of the TMC APIExport.
	TMCAPIExportName = "tmc.kcp.io"
)

// Controller manages the TMC APIExport, ensuring TMC APIs are available through KCP's APIExport system.
type Controller struct {
	queue workqueue.RateLimitingInterface

	kcpClusterClient kcpclientset.ClusterInterface

	apiExportLister       apisv1alpha1listers.APIExportClusterLister
	apiResourceSchemaLister apisv1alpha1listers.APIResourceSchemaClusterLister

	commit committer.Committer[*apisv1alpha1.APIExport]
}

// NewController creates a new TMC APIExport controller.
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	apiExportInformer apisinformers.APIExportClusterInformer,
	apiResourceSchemaInformer apisinformers.APIResourceSchemaClusterInformer,
) (*Controller, error) {

	c := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),

		kcpClusterClient: kcpClusterClient,

		apiExportLister:       apiExportInformer.Lister(),
		apiResourceSchemaLister: apiResourceSchemaInformer.Lister(),

		commit: committer.NewCommitter[*apisv1alpha1.APIExport](kcpClusterClient.ApisV1alpha1().APIExports()),
	}

	// Watch APIExport changes
	_, err := apiExportInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *apisv1alpha1.APIExport:
				return t.Name == TMCAPIExportName
			case cache.DeletedFinalStateUnknown:
				if export, ok := t.Obj.(*apisv1alpha1.APIExport); ok {
					return export.Name == TMCAPIExportName
				}
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { c.enqueue(obj) },
			UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
			DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch APIResourceSchema changes for TMC schemas
	_, err = apiResourceSchemaInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *apisv1alpha1.APIResourceSchema:
				return isTMCSchema(t)
			case cache.DeletedFinalStateUnknown:
				if schema, ok := t.Obj.(*apisv1alpha1.APIResourceSchema); ok {
					return isTMCSchema(schema)
				}
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { c.enqueueAPIExport() },
			UpdateFunc: func(_, obj interface{}) { c.enqueueAPIExport() },
			DeleteFunc: func(obj interface{}) { c.enqueueAPIExport() },
		},
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start starts the controller.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting TMC APIExport controller")
	defer klog.InfoS("Shutting down TMC APIExport controller")

	for i := 0; i < numThreads; i++ {
		go func() {
			for c.processNextWorkItem(ctx) {
			}
		}()
	}

	<-ctx.Done()
}

// processNextWorkItem processes the next work item from the queue.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.process(ctx, key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("syncing %q failed: %w", key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

// process processes a single work item.
func (c *Controller) process(ctx context.Context, key string) error {
	clusterName, _, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	cluster := logicalcluster.Name(clusterName)

	if name != TMCAPIExportName {
		return nil
	}

	klog.V(2).InfoS("Processing TMC APIExport", "cluster", cluster, "name", name)

	apiExport, err := c.apiExportLister.Cluster(cluster).Get(name)
	if apierrors.IsNotFound(err) {
		// APIExport was deleted or doesn't exist, create it
		return c.ensureTMCAPIExport(ctx, cluster)
	}
	if err != nil {
		return err
	}

	return c.syncAPIExport(ctx, cluster, apiExport)
}

// enqueue adds a work item to the queue.
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// enqueueAPIExport adds the TMC APIExport to the queue for all clusters.
func (c *Controller) enqueueAPIExport() {
	// For simplicity, we'll use a wildcard approach to ensure the TMC APIExport
	// is reconciled across all relevant workspaces
	c.queue.Add("root/" + TMCAPIExportName)
}

// ensureTMCAPIExport ensures the TMC APIExport exists.
func (c *Controller) ensureTMCAPIExport(ctx context.Context, cluster logicalcluster.Name) error {
	klog.V(2).InfoS("Creating TMC APIExport", "cluster", cluster)

	apiExport := &apisv1alpha1.APIExport{
		ObjectMeta: metav1.ObjectMeta{
			Name: TMCAPIExportName,
		},
		Spec: apisv1alpha1.APIExportSpec{
			LatestResourceSchemas: c.getTMCResourceSchemas(cluster),
			PermissionClaims: []apisv1alpha1.PermissionClaim{
				{
					GroupResource: apisv1alpha1.GroupResource{
						Group:    "coordination.k8s.io",
						Resource: "leases",
					},
					All: true,
				},
				{
					GroupResource: apisv1alpha1.GroupResource{
						Group:    "",
						Resource: "events",
					},
					All: true,
				},
			},
		},
	}

	// Set initial conditions
	conditions.MarkTrue(apiExport, apisv1alpha1.APIExportVirtualWorkspaceURLsReady)

	_, err := c.kcpClusterClient.ApisV1alpha1().APIExports().
		Cluster(cluster.Path()).
		Create(ctx, apiExport, metav1.CreateOptions{})
	return err
}

// syncAPIExport reconciles an existing TMC APIExport.
func (c *Controller) syncAPIExport(ctx context.Context, cluster logicalcluster.Name, apiExport *apisv1alpha1.APIExport) error {
	klog.V(4).InfoS("Syncing TMC APIExport", "cluster", cluster, "name", apiExport.Name)

	// Ensure the APIExport has all required TMC resource schemas
	expectedSchemas := c.getTMCResourceSchemas(cluster)
	updated := false

	// Update resource schemas if needed
	if !c.schemaSetsEqual(apiExport.Spec.LatestResourceSchemas, expectedSchemas) {
		apiExport = apiExport.DeepCopy()
		apiExport.Spec.LatestResourceSchemas = expectedSchemas
		updated = true
	}

	// Ensure permission claims are correct
	expectedPermissions := []apisv1alpha1.PermissionClaim{
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "coordination.k8s.io",
				Resource: "leases",
			},
			All: true,
		},
		{
			GroupResource: apisv1alpha1.GroupResource{
				Group:    "",
				Resource: "events",
			},
			All: true,
		},
	}

	if !c.permissionClaimsEqual(apiExport.Spec.PermissionClaims, expectedPermissions) {
		if apiExport == apiExport {
			apiExport = apiExport.DeepCopy()
		}
		apiExport.Spec.PermissionClaims = expectedPermissions
		updated = true
	}

	if updated {
		klog.V(2).InfoS("Updating TMC APIExport", "cluster", cluster)
		_, err := c.commit.WithContext(ctx).Cluster(cluster.Path()).Update(apiExport, metav1.UpdateOptions{})
		return err
	}

	return nil
}

// getTMCResourceSchemas returns the list of TMC resource schemas that should be included in the APIExport.
func (c *Controller) getTMCResourceSchemas(cluster logicalcluster.Name) []string {
	schemas := []string{}

	// Get all TMC-related APIResourceSchemas
	allSchemas, err := c.apiResourceSchemaLister.Cluster(cluster).List(nil)
	if err != nil {
		klog.ErrorS(err, "Failed to list APIResourceSchemas", "cluster", cluster)
		return schemas
	}

	for _, schema := range allSchemas {
		if isTMCSchema(schema) {
			schemas = append(schemas, schema.Name)
		}
	}

	return schemas
}

// isTMCSchema checks if an APIResourceSchema belongs to the TMC API group.
func isTMCSchema(schema *apisv1alpha1.APIResourceSchema) bool {
	return schema.Spec.Group == tmcv1alpha1.GroupName
}

// schemaSetsEqual compares two slices of schema names for equality.
func (c *Controller) schemaSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aSet := make(map[string]bool)
	for _, item := range a {
		aSet[item] = true
	}

	for _, item := range b {
		if !aSet[item] {
			return false
		}
	}

	return true
}

// permissionClaimsEqual compares two slices of permission claims for equality.
func (c *Controller) permissionClaimsEqual(a, b []apisv1alpha1.PermissionClaim) bool {
	if len(a) != len(b) {
		return false
	}

	// Create a map for efficient comparison
	aMap := make(map[string]apisv1alpha1.PermissionClaim)
	for _, claim := range a {
		key := fmt.Sprintf("%s/%s", claim.GroupResource.Group, claim.GroupResource.Resource)
		aMap[key] = claim
	}

	for _, claim := range b {
		key := fmt.Sprintf("%s/%s", claim.GroupResource.Group, claim.GroupResource.Resource)
		existing, found := aMap[key]
		if !found || !c.permissionClaimEqual(existing, claim) {
			return false
		}
	}

	return true
}

// permissionClaimEqual compares two permission claims for equality.
func (c *Controller) permissionClaimEqual(a, b apisv1alpha1.PermissionClaim) bool {
	if a.GroupResource != b.GroupResource {
		return false
	}
	if a.All != b.All {
		return false
	}
	if len(a.ResourceNames) != len(b.ResourceNames) {
		return false
	}
	
	for i, name := range a.ResourceNames {
		if name != b.ResourceNames[i] {
			return false
		}
	}
	
	return true
}