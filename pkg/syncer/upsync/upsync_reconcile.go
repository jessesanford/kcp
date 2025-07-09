/*
Copyright 2023 The KCP Authors.

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

package upsync

import (
	"context"

	"github.com/kcp-dev/logicalcluster/v3"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/syncer/shared"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	. "github.com/kcp-dev/kcp/tmc/pkg/logging"
)

const (
	// ResourceVersionAnnotation is an annotation set on a resource upsynced upstream
	// that contains the resourceVersion of the corresponding downstream resource
	// when it was last upsynced.
	// It is used to check easily, without having to compare the resource contents,
	// whether an upsynced upstream resource is up-to-date with the downstream resource.
	ResourceVersionAnnotation = "workload.kcp.io/rv"
)

type reconciler struct {
	cleanupReconciler

	getUpstreamUpsyncerLister func(clusterName logicalcluster.Name, gvr schema.GroupVersionResource) (cache.GenericLister, error)
	getUpsyncedGVRs           func(clusterName logicalcluster.Name) ([]schema.GroupVersionResource, error)

	syncTargetKey string
}

func (c *reconciler) reconcile(ctx context.Context, upstreamObject *unstructured.Unstructured, gvr schema.GroupVersionResource, upstreamClusterName logicalcluster.Name, upstreamNamespace, upstreamName string, dirtyStatus bool) (bool, error) {
	logger := klog.FromContext(ctx)

	downstreamResource, err := c.cleanupReconciler.getDownstreamResource(ctx, gvr, upstreamClusterName, upstreamNamespace, upstreamName)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if downstreamResource == nil {
		// Downstream resource not present => force delete resource upstream (also remove finalizers)
		err = c.deleteOrphanUpstreamResource(ctx, gvr, upstreamClusterName, upstreamNamespace, upstreamName)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	upstreamClient, err := c.getUpstreamClient(upstreamClusterName)
	if err != nil {
		return false, err
	}

	logger = logger.WithValues(DownstreamNamespace, downstreamResource.GetNamespace())
	ctx = klog.NewContext(ctx, logger)

	downstreamRV := downstreamResource.GetResourceVersion()
	markedForDeletionDownstream := downstreamResource.GetDeletionTimestamp() != nil

	// (potentially) create object upstream
	if upstreamObject == nil {
		if markedForDeletionDownstream {
			return false, nil
		}
		logger.V(1).Info("Creating resource upstream")
		preparedResource := c.prepareResourceForUpstream(ctx, gvr, upstreamNamespace, upstreamClusterName, downstreamResource)

		if !dirtyStatus {
			// if no status needs to be upsynced upstream, then we can set the resource version annotation at the same time as we create the
			// resource
			preparedResource.SetAnnotations(addResourceVersionAnnotation(downstreamRV, preparedResource.GetAnnotations()))
			// Create the resource
			_, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Create(ctx, preparedResource, metav1.CreateOptions{})
			return false, err
		}

		// Status also needs to be upsynced so let's do it in 3 steps:
		// - create the resource
		createdResource, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Create(ctx, preparedResource, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		// - update the status as a distinct action,
		preparedResource.SetResourceVersion(createdResource.GetResourceVersion())
		updatedResource, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).UpdateStatus(ctx, preparedResource, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		// - finally update the main content again to set the resource version annotation to the value of the downstream resource version
		preparedResource.SetAnnotations(addResourceVersionAnnotation(downstreamRV, preparedResource.GetAnnotations()))
		preparedResource.SetResourceVersion(updatedResource.GetResourceVersion())
		_, err = upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Update(ctx, preparedResource, metav1.UpdateOptions{})
		return false, err
	}

	// update upstream when annotation RV differs
	resourceVersionUpstream := upstreamObject.GetAnnotations()[ResourceVersionAnnotation]
	if downstreamRV != resourceVersionUpstream {
		logger.V(1).Info("Updating upstream resource")
		preparedResource := c.prepareResourceForUpstream(ctx, gvr, upstreamNamespace, upstreamClusterName, downstreamResource)
		if err != nil {
			return false, err
		}

		// quick path: status unchanged, only update main resource
		if !dirtyStatus {
			preparedResource.SetAnnotations(addResourceVersionAnnotation(downstreamRV, preparedResource.GetAnnotations()))
			existingResource, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Get(ctx, preparedResource.GetName(), metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			preparedResource.SetResourceVersion(existingResource.GetResourceVersion())
			_, err = upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Update(ctx, preparedResource, metav1.UpdateOptions{})
			// If the downstream resource is marked for deletion, let's requeue it to manage the deletion timestamp
			return markedForDeletionDownstream, err
		}

		// slow path: status changed => we need 3 steps
		// 1. update main resource

		preparedResource.SetAnnotations(addResourceVersionAnnotation(resourceVersionUpstream, preparedResource.GetAnnotations()))
		existingResource, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Get(ctx, preparedResource.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		preparedResource.SetResourceVersion(existingResource.GetResourceVersion())
		updatedResource, err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Update(ctx, preparedResource, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}

		// 2. update the status as a distinct action,
		preparedResource.SetResourceVersion(updatedResource.GetResourceVersion())
		updatedResource, err = upstreamClient.Resource(gvr).Namespace(upstreamNamespace).UpdateStatus(ctx, preparedResource, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}

		// 3. finally update the main resource again to set the resource version annotation to the value of the downstream resource version.
		preparedResource.SetAnnotations(addResourceVersionAnnotation(downstreamRV, preparedResource.GetAnnotations()))
		preparedResource.SetResourceVersion(updatedResource.GetResourceVersion())
		_, err = upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Update(ctx, preparedResource, metav1.UpdateOptions{})
		// If the downstream resource is marked for deletion, let's requeue it to manage the deletion timestamp
		return markedForDeletionDownstream, err
	}

	if downstreamResource.GetDeletionTimestamp() != nil {
		if err := upstreamClient.Resource(gvr).Namespace(upstreamNamespace).Delete(ctx, upstreamName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	return false, nil
}

func (c *reconciler) prepareResourceForUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNS string, upstreamLogicalCluster logicalcluster.Name, downstreamObj *unstructured.Unstructured) *unstructured.Unstructured {
	// Make a deepcopy
	resourceToUpsync := downstreamObj.DeepCopy()
	annotations := resourceToUpsync.GetAnnotations()
	if annotations != nil {
		delete(annotations, shared.NamespaceLocatorAnnotation)
		resourceToUpsync.SetAnnotations(annotations)
	}
	labels := resourceToUpsync.GetLabels()
	if labels != nil {
		delete(labels, workloadv1alpha1.InternalDownstreamClusterLabel)
		resourceToUpsync.SetLabels(labels)
	}
	resourceToUpsync.SetNamespace(upstreamNS)
	resourceToUpsync.SetUID("")
	resourceToUpsync.SetResourceVersion("")
	resourceToUpsync.SetManagedFields(nil)
	resourceToUpsync.SetDeletionTimestamp(nil)
	resourceToUpsync.SetDeletionGracePeriodSeconds(nil)
	resourceToUpsync.SetOwnerReferences(nil)
	resourceToUpsync.SetFinalizers([]string{shared.SyncerFinalizerNamePrefix + c.syncTargetKey})

	return resourceToUpsync
}

func addResourceVersionAnnotation(resourceVersion string, annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	annotations[ResourceVersionAnnotation] = resourceVersion
	return annotations
}
