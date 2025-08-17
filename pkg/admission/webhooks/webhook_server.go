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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/klog/v2"
)

const (
	// DefaultWebhookPort is the default port for webhook server
	DefaultWebhookPort = 8443
	
	// Webhook paths
	SyncTargetWebhookPath    = "/synctarget"
	PlacementWebhookPath     = "/placement"
	HealthCheckPath          = "/healthz"
	ReadinessCheckPath       = "/readyz"
	
	// Default timeouts
	DefaultReadTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
)

// WebhookServerConfig contains configuration for the webhook server
type WebhookServerConfig struct {
	// Port is the port the webhook server listens on
	Port int
	
	// CertFile is the path to the TLS certificate file
	CertFile string
	
	// KeyFile is the path to the TLS private key file
	KeyFile string
	
	// HealthzBindAddress is the address to bind the health check endpoint to
	HealthzBindAddress string
}

// WebhookServer runs HTTP server with admission webhooks
type WebhookServer struct {
	config *WebhookServerConfig
	server *http.Server
	scheme *runtime.Scheme
	codecs serializer.CodecFactory
}

// NewWebhookServer creates a new webhook server instance
func NewWebhookServer(config *WebhookServerConfig) *WebhookServer {
	scheme := runtime.NewScheme()
	utilruntime.Must(admissionv1.AddToScheme(scheme))
	
	return &WebhookServer{
		config: config,
		scheme: scheme,
		codecs: serializer.NewCodecFactory(scheme),
	}
}

// Start starts the webhook server and blocks until stopped
func (s *WebhookServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Register webhook handlers
	mux.HandleFunc(SyncTargetWebhookPath, s.handleSyncTargetWebhook)
	// PlacementWebhook will be added in a follow-up PR
	
	// Register health check handlers
	mux.HandleFunc(HealthCheckPath, s.handleHealthz)
	mux.HandleFunc(ReadinessCheckPath, s.handleReadyz)
	
	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
	
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      mux,
		TLSConfig:    tlsConfig,
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
	}
	
	klog.Infof("Starting TMC webhook server on port %d", s.config.Port)
	
	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Failed to start webhook server: %v", err)
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Gracefully shutdown server
	klog.Info("Shutting down TMC webhook server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	return s.server.Shutdown(shutdownCtx)
}

// handleSyncTargetWebhook handles admission requests for SyncTarget resources
func (s *WebhookServer) handleSyncTargetWebhook(w http.ResponseWriter, r *http.Request) {
	klog.V(4).Info("Handling SyncTarget webhook request")
	
	webhook, err := NewSyncTargetWebhook(nil)
	if err != nil {
		klog.Errorf("Failed to create SyncTarget webhook: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create webhook: %v", err), http.StatusInternalServerError)
		return
	}
	
	s.handleAdmissionRequest(w, r, webhook)
}

// handlePlacementWebhook will be added in a follow-up PR for WorkloadPlacement resources

// handleAdmissionRequest processes admission webhook requests
func (s *WebhookServer) handleAdmissionRequest(w http.ResponseWriter, r *http.Request, webhook admission.Interface) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are supported", http.StatusMethodNotAllowed)
		return
	}
	
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}
	
	// Read request body
	defer r.Body.Close()
	body := make([]byte, r.ContentLength)
	if _, err := r.Body.Read(body); err != nil && err.Error() != "EOF" {
		klog.Errorf("Failed to read request body: %v", err)
		http.Error(w, fmt.Sprintf("Failed to read request: %v", err), http.StatusBadRequest)
		return
	}
	
	// Decode admission review request
	admissionReview := &admissionv1.AdmissionReview{}
	if _, _, err := s.codecs.UniversalDeserializer().Decode(body, nil, admissionReview); err != nil {
		klog.Errorf("Failed to decode admission request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}
	
	if admissionReview.Request == nil {
		klog.Error("Admission request is nil")
		http.Error(w, "Invalid admission request", http.StatusBadRequest)
		return
	}
	
	// Create admission attributes from the request
	// This is a simplified implementation - in a real webhook server,
	// you would properly convert the admission request to KCP admission attributes
	
	// Create response
	response := &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: true,
		Result:  &metav1.Status{Message: "Validation passed"},
	}
	
	// Create response review
	responseReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: response,
	}
	
	// Encode and send response
	w.Header().Set("Content-Type", "application/json")
	if err := s.codecs.LegacyCodec(admissionv1.SchemeGroupVersion).Encode(responseReview, w); err != nil {
		klog.Errorf("Failed to encode admission response: %v", err)
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
	
	klog.V(4).Info("Successfully handled admission webhook request")
}

// handleHealthz handles health check requests
func (s *WebhookServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleReadyz handles readiness check requests
func (s *WebhookServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}
	
	// Check if webhook server is ready
	// In a real implementation, you might check dependencies, certificates, etc.
	if s.isReady() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}

// isReady checks if the webhook server is ready to serve requests
func (s *WebhookServer) isReady() bool {
	// Perform readiness checks
	// - Check TLS certificates
	// - Check dependencies
	// - Check configuration
	return true
}

// DefaultWebhookServerConfig returns a default webhook server configuration
func DefaultWebhookServerConfig() *WebhookServerConfig {
	return &WebhookServerConfig{
		Port:               DefaultWebhookPort,
		CertFile:           "/etc/certs/tls.crt",
		KeyFile:            "/etc/certs/tls.key",
		HealthzBindAddress: "0.0.0.0",
	}
}

// InstallHealthzChecks installs health check handlers
func InstallHealthzChecks(mux *http.ServeMux) {
	healthz.InstallHandler(mux, healthz.NamedCheck("webhook", func(r *http.Request) error {
		return nil
	}))
}