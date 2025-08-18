/*
Copyright 2025 The KCP Authors.

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

package framework

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
)

// WaitForCondition waits for a condition to be met with timeout.
func WaitForCondition(t kcptesting.TestingT, condition func() bool, timeout time.Duration) error {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		return condition(), nil
	})
}

// AssertResourceExists verifies resource exists.
func AssertResourceExists(t kcptesting.TestingT, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string, timeout time.Duration) {
	t.Helper()
	
	err := WaitForCondition(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		var resourceClient dynamic.ResourceInterface
		if namespace != "" {
			resourceClient = client.Resource(gvr).Namespace(namespace)
		} else {
			resourceClient = client.Resource(gvr)
		}
		
		_, err := resourceClient.Get(ctx, name, metav1.GetOptions{})
		return err == nil
	}, timeout)
	
	if err != nil {
		t.Fatalf("Resource %s/%s %s not found within %v: %v", gvr.String(), namespace, name, timeout, err)
	}
}

// AssertResourceDeleted verifies resource is deleted.
func AssertResourceDeleted(t kcptesting.TestingT, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string, timeout time.Duration) {
	t.Helper()
	
	err := WaitForCondition(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		var resourceClient dynamic.ResourceInterface
		if namespace != "" {
			resourceClient = client.Resource(gvr).Namespace(namespace)
		} else {
			resourceClient = client.Resource(gvr)
		}
		
		_, err := resourceClient.Get(ctx, name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, timeout)
	
	if err != nil {
		t.Fatalf("Resource %s/%s %s still exists after %v: %v", gvr.String(), namespace, name, timeout, err)
	}
}