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

package syncer

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// SyncTargetInterface defines the interface for interacting with SyncTarget resources.
// This interface abstracts the underlying client implementation to allow for testing and mocking.
type SyncTargetInterface interface {
	// List returns a list of SyncTargets that match the selector.
	List(ctx context.Context, opts metav1.ListOptions) (*workloadv1alpha1.SyncTargetList, error)
	
	// Get retrieves a specific SyncTarget by name.
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*workloadv1alpha1.SyncTarget, error)
	
	// UpdateStatus updates the status subresource of a SyncTarget.
	UpdateStatus(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget, opts metav1.UpdateOptions) (*workloadv1alpha1.SyncTarget, error)
	
	// Watch returns a watch interface for SyncTarget resources.
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}