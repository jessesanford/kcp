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

package webhooks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apiserver/pkg/admission"
)

func TestNewWebhookServer(t *testing.T) {
	config := &WebhookServerConfig{
		Port:               8443,
		CertFile:           "/test/cert.pem",
		KeyFile:            "/test/key.pem",
		HealthzBindAddress: "0.0.0.0",
	}
	
	server := NewWebhookServer(config)
	
	if server == nil {
		t.Fatal("Expected webhook server instance, got nil")
	}
	
	if server.config != config {
		t.Errorf("Expected config %v, got %v", config, server.config)
	}
	
	if server.scheme == nil {
		t.Error("Expected scheme to be initialized")
	}
	
	// Verify codecs are initialized by checking they can create a codec
	if server.codecs.LegacyCodec(admissionv1.SchemeGroupVersion) == nil {
		t.Error("Expected codecs to be initialized")
	}
}

func TestDefaultWebhookServerConfig(t *testing.T) {
	config := DefaultWebhookServerConfig()
	
	if config.Port != DefaultWebhookPort {
		t.Errorf("Expected port %d, got %d", DefaultWebhookPort, config.Port)
	}
	
	if config.CertFile != "/etc/certs/tls.crt" {
		t.Errorf("Expected cert file '/etc/certs/tls.crt', got %s", config.CertFile)
	}
	
	if config.KeyFile != "/etc/certs/tls.key" {
		t.Errorf("Expected key file '/etc/certs/tls.key', got %s", config.KeyFile)
	}
	
	if config.HealthzBindAddress != "0.0.0.0" {
		t.Errorf("Expected healthz bind address '0.0.0.0', got %s", config.HealthzBindAddress)
	}
}

func TestWebhookServer_handleHealthz(t *testing.T) {
	server := NewWebhookServer(DefaultWebhookServerConfig())
	
	tests := map[string]struct {
		method         string
		expectedStatus int
		expectedBody   string
	}{
		"GET request succeeds": {
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		"POST request fails": {
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Only GET requests are supported\n",
		},
		"PUT request fails": {
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Only GET requests are supported\n",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/healthz", nil)
			recorder := httptest.NewRecorder()
			
			server.handleHealthz(recorder, req)
			
			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}
			
			if recorder.Body.String() != tc.expectedBody {
				t.Errorf("Expected body %q, got %q", tc.expectedBody, recorder.Body.String())
			}
		})
	}
}

func TestWebhookServer_handleReadyz(t *testing.T) {
	server := NewWebhookServer(DefaultWebhookServerConfig())
	
	tests := map[string]struct {
		method         string
		expectedStatus int
		expectedBody   string
	}{
		"GET request succeeds": {
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "ready",
		},
		"POST request fails": {
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Only GET requests are supported\n",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/readyz", nil)
			recorder := httptest.NewRecorder()
			
			server.handleReadyz(recorder, req)
			
			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}
			
			if recorder.Body.String() != tc.expectedBody {
				t.Errorf("Expected body %q, got %q", tc.expectedBody, recorder.Body.String())
			}
		})
	}
}

func TestWebhookServer_isReady(t *testing.T) {
	server := NewWebhookServer(DefaultWebhookServerConfig())
	
	// Current implementation always returns true
	if !server.isReady() {
		t.Error("Expected server to be ready")
	}
}

func TestWebhookServer_handleAdmissionRequest(t *testing.T) {
	server := NewWebhookServer(DefaultWebhookServerConfig())
	
	// Create a mock webhook that implements admission.Interface
	mockWebhook := &mockAdmissionWebhook{}
	
	tests := map[string]struct {
		method         string
		contentType    string
		body           []byte
		expectedStatus int
	}{
		"valid POST request": {
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           createValidAdmissionReview(t),
			expectedStatus: http.StatusOK,
		},
		"invalid method": {
			method:         http.MethodGet,
			contentType:    "application/json",
			body:           createValidAdmissionReview(t),
			expectedStatus: http.StatusMethodNotAllowed,
		},
		"invalid content type": {
			method:         http.MethodPost,
			contentType:    "text/plain",
			body:           createValidAdmissionReview(t),
			expectedStatus: http.StatusBadRequest,
		},
		"invalid JSON body": {
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           []byte(`invalid json`),
			expectedStatus: http.StatusBadRequest,
		},
		"empty admission request": {
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           createEmptyAdmissionReview(t),
			expectedStatus: http.StatusBadRequest,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/webhook", bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			recorder := httptest.NewRecorder()
			
			server.handleAdmissionRequest(recorder, req, mockWebhook)
			
			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}
			
			// For successful requests, verify response structure
			if tc.expectedStatus == http.StatusOK {
				var response admissionv1.AdmissionReview
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}
				
				if response.Response == nil {
					t.Error("Expected response to have Response field")
				}
				
				if !response.Response.Allowed {
					t.Error("Expected response to be allowed")
				}
			}
		})
	}
}

func TestWebhookServerTLSConfig(t *testing.T) {
	// Create temporary certificate files
	certFile, keyFile := createTempTLSFiles(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)
	
	config := &WebhookServerConfig{
		Port:               8443,
		CertFile:           certFile,
		KeyFile:            keyFile,
		HealthzBindAddress: "0.0.0.0",
	}
	
	server := NewWebhookServer(config)
	
	// Test that server can be created and would use proper TLS config
	// We can't easily test the actual TLS functionality without starting the server,
	// but we can verify the configuration is set up correctly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// This will timeout because the server starts successfully in the background
	// but the context timeout will cause it to shutdown gracefully  
	err := server.Start(ctx)
	// Either no error (successful start/stop) or context timeout is acceptable
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("Expected no error or context timeout, got: %v", err)
	}
}

// Helper functions

func createValidAdmissionReview(t *testing.T) []byte {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: uuid.NewUUID(),
			Kind: metav1.GroupVersionKind{
				Group:   "workload.kcp.io",
				Version: "v1alpha1",
				Kind:    "SyncTarget",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "workload.kcp.io",
				Version:  "v1alpha1",
				Resource: "synctargets",
			},
			Name:      "test-synctarget",
			Namespace: "default",
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: []byte(`{"apiVersion":"workload.kcp.io/v1alpha1","kind":"SyncTarget","metadata":{"name":"test-synctarget"}}`),
			},
		},
	}
	
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Failed to marshal admission review: %v", err)
	}
	
	return data
}

func createEmptyAdmissionReview(t *testing.T) []byte {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		// Request is nil
	}
	
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("Failed to marshal admission review: %v", err)
	}
	
	return data
}

func createTempTLSFiles(t *testing.T) (certFile, keyFile string) {
	// Generate a private key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	
	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	
	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}
	
	// Create temporary certificate file
	certTempFile, err := ioutil.TempFile("", "cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp cert file: %v", err)
	}
	defer certTempFile.Close()
	
	if err := pem.Encode(certTempFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}); err != nil {
		t.Fatalf("Failed to write certificate: %v", err)
	}
	
	// Create temporary key file
	keyTempFile, err := ioutil.TempFile("", "key-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp key file: %v", err)
	}
	defer keyTempFile.Close()
	
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}
	
	if err := pem.Encode(keyTempFile, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	}); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}
	
	return certTempFile.Name(), keyTempFile.Name()
}

// mockAdmissionWebhook is a simple mock that implements admission.Interface
type mockAdmissionWebhook struct{}

func (m *mockAdmissionWebhook) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	return nil
}

func (m *mockAdmissionWebhook) Handles(operation admission.Operation) bool {
	return true
}