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

package endpoints

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"

	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestNewHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(fakeClient, 0)
	authorizer := &fakeAuthorizer{}

	// Test valid config
	handler, err := NewHandler(&HandlerConfig{
		ClusterClient:   fakeClient,
		InformerFactory: informerFactory,
		Authorizer:      authorizer,
		Scheme:          scheme,
	})
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if handler == nil {
		t.Error("Expected handler but got nil")
	}
	if handler.Ready {
		t.Error("Handler should not be ready before Start() is called")
	}

	// Test missing cluster client
	_, err = NewHandler(&HandlerConfig{
		InformerFactory: informerFactory,
		Authorizer:      authorizer,
		Scheme:          scheme,
	})
	if err == nil {
		t.Error("Expected error for missing cluster client")
	}
}

func TestHandlerServeHTTP_NotReady(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleClientset()
	informerFactory := kcpinformers.NewSharedInformerFactory(fakeClient, 0)
	authorizer := &fakeAuthorizer{}

	handler, err := NewHandler(&HandlerConfig{
		ClusterClient:   fakeClient,
		InformerFactory: informerFactory,
		Authorizer:      authorizer,
		Scheme:          scheme,
	})
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Handler should not be ready initially
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pods", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d but got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHandlerValidation(t *testing.T) {
	handler := &Handler{}

	// Test valid API discovery request
	result := handler.isValidVirtualWorkspaceRequest(&request.RequestInfo{
		IsResourceRequest: false,
		Path:              "/api",
	})
	if !result {
		t.Error("Expected valid API discovery request")
	}

	// Test valid resource request
	result = handler.isValidVirtualWorkspaceRequest(&request.RequestInfo{
		IsResourceRequest: true,
		APIGroup:          "",
		APIVersion:        "v1",
		Resource:          "pods",
	})
	if !result {
		t.Error("Expected valid resource request")
	}

	// Test invalid resource request
	result = handler.isValidVirtualWorkspaceRequest(&request.RequestInfo{
		IsResourceRequest: true,
		APIGroup:          "",
		APIVersion:        "",
		Resource:          "pods",
	})
	if result {
		t.Error("Expected invalid resource request")
	}
}

// fakeAuthorizer is a test implementation of the authorizer interface
type fakeAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
}

func (a *fakeAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	if a.decision == 0 {
		return authorizer.DecisionAllow, "", nil
	}
	return a.decision, a.reason, a.err
}