# Admission Webhooks Implementation Instructions

## Overview
This branch implements admission webhooks for TMC resources, providing validation and mutation capabilities for WorkloadPlacement, SyncTarget, and ClusterRegistration resources. These webhooks ensure data integrity and apply defaults.

**Branch**: `feature/tmc-completion/p6w3-webhooks`  
**Estimated Lines**: 550 lines  
**Wave**: 1  
**Dependencies**: Phase 5 APIs must be complete  

## Dependencies

### Required Before Starting
- Phase 5 APIs complete (TMC types defined)
- Core webhook framework available
- Admission control infrastructure

### Blocks These Features
- None - independent component that can run in parallel with other Wave 1 work

## Files to Create/Modify

### Primary Implementation Files (550 lines total)

1. **pkg/admission/webhooks/workloadplacement_webhook.go** (150 lines)
   - WorkloadPlacement validation
   - Resource requirement validation
   - Placement policy validation

2. **pkg/admission/webhooks/synctarget_webhook.go** (130 lines)
   - SyncTarget validation
   - Connection validation
   - Capacity validation

3. **pkg/admission/webhooks/clusterregistration_webhook.go** (120 lines)
   - ClusterRegistration validation
   - Registration approval logic
   - Credential validation

4. **pkg/admission/webhooks/webhook_server.go** (100 lines)
   - Webhook server setup
   - TLS configuration
   - Handler registration

5. **pkg/admission/webhooks/helpers.go** (50 lines)
   - Common validation utilities
   - Response builders
   - Error formatters

### Test Files (not counted in line limit)

1. **pkg/admission/webhooks/workloadplacement_webhook_test.go**
2. **pkg/admission/webhooks/synctarget_webhook_test.go**
3. **pkg/admission/webhooks/clusterregistration_webhook_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Webhook Server (Hour 1-2)

```go
// pkg/admission/webhooks/webhook_server.go
package webhooks

import (
    "context"
    "crypto/tls"
    "fmt"
    "net/http"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    admissionv1 "k8s.io/api/admission/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/serializer"
    "k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
    "k8s.io/klog/v2"
    "sigs.k8s.io/controller-runtime/pkg/webhook"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookServer serves admission webhooks
type WebhookServer struct {
    // Server configuration
    server     *http.Server
    port       int
    certFile   string
    keyFile    string
    
    // Handlers
    handlers   map[string]admission.Handler
    
    // Decoders
    decoder    *admission.Decoder
    scheme     *runtime.Scheme
    codecs     serializer.CodecFactory
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
    Port      int
    CertFile  string
    KeyFile   string
    Namespace string
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(config *WebhookConfig) (*WebhookServer, error) {
    scheme := runtime.NewScheme()
    if err := tmcv1alpha1.AddToScheme(scheme); err != nil {
        return nil, fmt.Errorf("failed to add TMC scheme: %w", err)
    }
    
    codecs := serializer.NewCodecFactory(scheme)
    decoder := admission.NewDecoder(scheme)
    
    ws := &WebhookServer{
        port:     config.Port,
        certFile: config.CertFile,
        keyFile:  config.KeyFile,
        handlers: make(map[string]admission.Handler),
        decoder:  decoder,
        scheme:   scheme,
        codecs:   codecs,
    }
    
    // Register webhook handlers
    ws.registerHandlers()
    
    // Setup HTTP server
    mux := http.NewServeMux()
    ws.setupRoutes(mux)
    
    ws.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", config.Port),
        Handler: mux,
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }
    
    return ws, nil
}

// registerHandlers registers all webhook handlers
func (ws *WebhookServer) registerHandlers() {
    // WorkloadPlacement webhooks
    ws.handlers["/validate-workloadplacement"] = &WorkloadPlacementValidator{
        decoder: ws.decoder,
    }
    ws.handlers["/mutate-workloadplacement"] = &WorkloadPlacementMutator{
        decoder: ws.decoder,
    }
    
    // SyncTarget webhooks
    ws.handlers["/validate-synctarget"] = &SyncTargetValidator{
        decoder: ws.decoder,
    }
    ws.handlers["/mutate-synctarget"] = &SyncTargetMutator{
        decoder: ws.decoder,
    }
    
    // ClusterRegistration webhooks
    ws.handlers["/validate-clusterregistration"] = &ClusterRegistrationValidator{
        decoder: ws.decoder,
    }
    ws.handlers["/mutate-clusterregistration"] = &ClusterRegistrationMutator{
        decoder: ws.decoder,
    }
}

// setupRoutes sets up HTTP routes
func (ws *WebhookServer) setupRoutes(mux *http.ServeMux) {
    for path, handler := range ws.handlers {
        mux.HandleFunc(path, ws.handleAdmission(handler))
    }
    
    // Health check endpoint
    mux.HandleFunc("/healthz", ws.handleHealth)
    
    // Ready check endpoint
    mux.HandleFunc("/readyz", ws.handleReady)
}

// handleAdmission creates an HTTP handler for admission requests
func (ws *WebhookServer) handleAdmission(handler admission.Handler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var body []byte
        if r.Body != nil {
            defer r.Body.Close()
            body, _ = io.ReadAll(r.Body)
        }
        
        // Verify content type
        contentType := r.Header.Get("Content-Type")
        if contentType != "application/json" {
            http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
            return
        }
        
        // Decode admission review
        obj, gvk, err := ws.codecs.UniversalDeserializer().Decode(body, nil, nil)
        if err != nil {
            klog.Errorf("Failed to decode admission review: %v", err)
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        var responseObj runtime.Object
        switch *gvk {
        case admissionv1.SchemeGroupVersion.WithKind("AdmissionReview"):
            review := obj.(*admissionv1.AdmissionReview)
            
            // Create admission request
            req := admission.Request{
                AdmissionRequest: *review.Request,
            }
            
            // Handle the request
            response := handler.Handle(context.Background(), req)
            
            // Build admission review response
            review.Response = &admissionv1.AdmissionResponse{
                UID:     review.Request.UID,
                Allowed: response.Allowed,
                Result:  response.Result,
            }
            
            if len(response.Patches) > 0 {
                review.Response.Patch = response.Patches
                patchType := admissionv1.PatchTypeJSONPatch
                review.Response.PatchType = &patchType
            }
            
            responseObj = review
            
        default:
            http.Error(w, fmt.Sprintf("Unsupported group version kind: %v", gvk), http.StatusBadRequest)
            return
        }
        
        // Encode response
        respBytes, err := runtime.Encode(ws.codecs.LegacyCodec(admissionv1.SchemeGroupVersion), responseObj)
        if err != nil {
            klog.Errorf("Failed to encode response: %v", err)
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        // Write response
        w.Header().Set("Content-Type", "application/json")
        w.Write(respBytes)
    }
}

// Start starts the webhook server
func (ws *WebhookServer) Start(ctx context.Context) error {
    klog.Infof("Starting webhook server on port %d", ws.port)
    
    go func() {
        <-ctx.Done()
        klog.Info("Shutting down webhook server")
        ws.server.Shutdown(context.Background())
    }()
    
    return ws.server.ListenAndServeTLS(ws.certFile, ws.keyFile)
}

// handleHealth handles health check requests
func (ws *WebhookServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// handleReady handles readiness check requests
func (ws *WebhookServer) handleReady(w http.ResponseWriter, r *http.Request) {
    // Check if all handlers are registered
    if len(ws.handlers) == 0 {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("Not ready"))
        return
    }
    
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("Ready"))
}
```

### Step 2: Implement WorkloadPlacement Webhook (Hour 3-4)

```go
// pkg/admission/webhooks/workloadplacement_webhook.go
package webhooks

import (
    "context"
    "fmt"
    "net/http"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    admissionv1 "k8s.io/api/admission/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/klog/v2"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WorkloadPlacementValidator validates WorkloadPlacement resources
type WorkloadPlacementValidator struct {
    decoder *admission.Decoder
}

// Handle handles validation requests
func (v *WorkloadPlacementValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
    placement := &tmcv1alpha1.WorkloadPlacement{}
    
    err := v.decoder.Decode(req, placement)
    if err != nil {
        klog.Errorf("Failed to decode WorkloadPlacement: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Validating WorkloadPlacement %s/%s", placement.Namespace, placement.Name)
    
    // Perform validation
    if err := v.validateWorkloadPlacement(placement); err != nil {
        klog.V(2).Infof("WorkloadPlacement %s/%s validation failed: %v", placement.Namespace, placement.Name, err)
        return admission.Denied(err.Error())
    }
    
    return admission.Allowed("")
}

// validateWorkloadPlacement validates a WorkloadPlacement
func (v *WorkloadPlacementValidator) validateWorkloadPlacement(placement *tmcv1alpha1.WorkloadPlacement) error {
    // Validate workload reference
    if err := v.validateWorkloadRef(placement); err != nil {
        return fmt.Errorf("invalid workload reference: %w", err)
    }
    
    // Validate target clusters
    if err := v.validateTargetClusters(placement); err != nil {
        return fmt.Errorf("invalid target clusters: %w", err)
    }
    
    // Validate resource requirements
    if err := v.validateResourceRequirements(placement); err != nil {
        return fmt.Errorf("invalid resource requirements: %w", err)
    }
    
    // Validate placement policy
    if err := v.validatePlacementPolicy(placement); err != nil {
        return fmt.Errorf("invalid placement policy: %w", err)
    }
    
    return nil
}

// validateWorkloadRef validates workload reference
func (v *WorkloadPlacementValidator) validateWorkloadRef(placement *tmcv1alpha1.WorkloadPlacement) error {
    if placement.Spec.Workload == nil {
        return fmt.Errorf("workload reference is required")
    }
    
    if placement.Spec.Workload.APIVersion == "" {
        return fmt.Errorf("workload APIVersion is required")
    }
    
    if placement.Spec.Workload.Kind == "" {
        return fmt.Errorf("workload Kind is required")
    }
    
    if placement.Spec.Workload.Name == "" {
        return fmt.Errorf("workload Name is required")
    }
    
    // Validate supported workload types
    supportedKinds := []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob"}
    supported := false
    for _, kind := range supportedKinds {
        if placement.Spec.Workload.Kind == kind {
            supported = true
            break
        }
    }
    
    if !supported {
        return fmt.Errorf("workload kind %s is not supported", placement.Spec.Workload.Kind)
    }
    
    return nil
}

// validateTargetClusters validates target clusters
func (v *WorkloadPlacementValidator) validateTargetClusters(placement *tmcv1alpha1.WorkloadPlacement) error {
    if len(placement.Spec.TargetClusters) == 0 && placement.Spec.ClusterSelector == nil {
        return fmt.Errorf("either targetClusters or clusterSelector must be specified")
    }
    
    // Validate cluster references
    for i, target := range placement.Spec.TargetClusters {
        if target.Name == "" {
            return fmt.Errorf("targetClusters[%d].name is required", i)
        }
    }
    
    // Validate cluster selector
    if placement.Spec.ClusterSelector != nil {
        if _, err := metav1.LabelSelectorAsSelector(placement.Spec.ClusterSelector); err != nil {
            return fmt.Errorf("invalid clusterSelector: %w", err)
        }
    }
    
    return nil
}

// validateResourceRequirements validates resource requirements
func (v *WorkloadPlacementValidator) validateResourceRequirements(placement *tmcv1alpha1.WorkloadPlacement) error {
    if placement.Spec.ResourceRequirements == nil {
        return nil // Optional
    }
    
    req := placement.Spec.ResourceRequirements
    
    // Validate CPU
    if req.CPU != nil {
        if req.CPU.Sign() <= 0 {
            return fmt.Errorf("CPU must be positive")
        }
        
        // Check reasonable limits
        maxCPU := resource.MustParse("1000")
        if req.CPU.Cmp(maxCPU) > 0 {
            return fmt.Errorf("CPU exceeds maximum allowed (%s)", maxCPU.String())
        }
    }
    
    // Validate Memory
    if req.Memory != nil {
        if req.Memory.Sign() <= 0 {
            return fmt.Errorf("Memory must be positive")
        }
        
        // Check reasonable limits
        maxMemory := resource.MustParse("10Ti")
        if req.Memory.Cmp(maxMemory) > 0 {
            return fmt.Errorf("Memory exceeds maximum allowed (%s)", maxMemory.String())
        }
    }
    
    // Validate Storage
    if req.Storage != nil {
        if req.Storage.Sign() <= 0 {
            return fmt.Errorf("Storage must be positive")
        }
        
        // Check reasonable limits
        maxStorage := resource.MustParse("100Ti")
        if req.Storage.Cmp(maxStorage) > 0 {
            return fmt.Errorf("Storage exceeds maximum allowed (%s)", maxStorage.String())
        }
    }
    
    return nil
}

// validatePlacementPolicy validates placement policy
func (v *WorkloadPlacementValidator) validatePlacementPolicy(placement *tmcv1alpha1.WorkloadPlacement) error {
    if placement.Spec.Placement == nil {
        return nil // Optional
    }
    
    policy := placement.Spec.Placement
    
    // Validate spread constraints
    if policy.SpreadConstraints != nil {
        for i, constraint := range policy.SpreadConstraints {
            if constraint.TopologyKey == "" {
                return fmt.Errorf("spreadConstraints[%d].topologyKey is required", i)
            }
            
            if constraint.MaxSkew < 1 {
                return fmt.Errorf("spreadConstraints[%d].maxSkew must be >= 1", i)
            }
        }
    }
    
    // Validate affinity
    if policy.Affinity != nil {
        // Validate required affinity
        if policy.Affinity.RequiredDuringScheduling != nil {
            for i, term := range policy.Affinity.RequiredDuringScheduling {
                if len(term.MatchExpressions) == 0 && len(term.MatchLabels) == 0 {
                    return fmt.Errorf("affinity.requiredDuringScheduling[%d] must have matchExpressions or matchLabels", i)
                }
            }
        }
    }
    
    return nil
}

// WorkloadPlacementMutator mutates WorkloadPlacement resources
type WorkloadPlacementMutator struct {
    decoder *admission.Decoder
}

// Handle handles mutation requests
func (m *WorkloadPlacementMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
    placement := &tmcv1alpha1.WorkloadPlacement{}
    
    err := m.decoder.Decode(req, placement)
    if err != nil {
        klog.Errorf("Failed to decode WorkloadPlacement: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Mutating WorkloadPlacement %s/%s", placement.Namespace, placement.Name)
    
    // Apply mutations
    m.mutateWorkloadPlacement(placement)
    
    // Create patches
    marshaledPlacement, err := json.Marshal(placement)
    if err != nil {
        return admission.Errored(http.StatusInternalServerError, err)
    }
    
    return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPlacement)
}

// mutateWorkloadPlacement applies mutations to a WorkloadPlacement
func (m *WorkloadPlacementMutator) mutateWorkloadPlacement(placement *tmcv1alpha1.WorkloadPlacement) {
    // Set default labels
    if placement.Labels == nil {
        placement.Labels = make(map[string]string)
    }
    
    if _, exists := placement.Labels["tmc.kcp.dev/managed"]; !exists {
        placement.Labels["tmc.kcp.dev/managed"] = "true"
    }
    
    // Set default annotations
    if placement.Annotations == nil {
        placement.Annotations = make(map[string]string)
    }
    
    if _, exists := placement.Annotations["tmc.kcp.dev/created-by"]; !exists {
        placement.Annotations["tmc.kcp.dev/created-by"] = "webhook"
    }
    
    // Set default resource requirements if not specified
    if placement.Spec.ResourceRequirements == nil {
        placement.Spec.ResourceRequirements = &tmcv1alpha1.ResourceRequirements{}
    }
    
    if placement.Spec.ResourceRequirements.CPU == nil {
        defaultCPU := resource.MustParse("100m")
        placement.Spec.ResourceRequirements.CPU = &defaultCPU
    }
    
    if placement.Spec.ResourceRequirements.Memory == nil {
        defaultMemory := resource.MustParse("128Mi")
        placement.Spec.ResourceRequirements.Memory = &defaultMemory
    }
    
    // Set default placement policy
    if placement.Spec.Placement == nil {
        placement.Spec.Placement = &tmcv1alpha1.PlacementPolicy{
            ReplicaCount: 1,
        }
    }
    
    if placement.Spec.Placement.ReplicaCount == 0 {
        placement.Spec.Placement.ReplicaCount = 1
    }
}
```

### Step 3: Implement SyncTarget Webhook (Hour 5)

```go
// pkg/admission/webhooks/synctarget_webhook.go
package webhooks

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/klog/v2"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SyncTargetValidator validates SyncTarget resources
type SyncTargetValidator struct {
    decoder *admission.Decoder
}

// Handle handles validation requests
func (v *SyncTargetValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
    syncTarget := &tmcv1alpha1.SyncTarget{}
    
    err := v.decoder.Decode(req, syncTarget)
    if err != nil {
        klog.Errorf("Failed to decode SyncTarget: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Validating SyncTarget %s", syncTarget.Name)
    
    // Perform validation
    if err := v.validateSyncTarget(syncTarget); err != nil {
        klog.V(2).Infof("SyncTarget %s validation failed: %v", syncTarget.Name, err)
        return admission.Denied(err.Error())
    }
    
    return admission.Allowed("")
}

// validateSyncTarget validates a SyncTarget
func (v *SyncTargetValidator) validateSyncTarget(st *tmcv1alpha1.SyncTarget) error {
    // Validate KubeConfig
    if err := v.validateKubeConfig(st); err != nil {
        return fmt.Errorf("invalid kubeconfig: %w", err)
    }
    
    // Validate Cell
    if err := v.validateCell(st); err != nil {
        return fmt.Errorf("invalid cell: %w", err)
    }
    
    // Validate labels
    if err := v.validateLabels(st); err != nil {
        return fmt.Errorf("invalid labels: %w", err)
    }
    
    // Validate capacity if specified
    if st.Spec.ExpectedCapacity != nil {
        if err := v.validateCapacity(st.Spec.ExpectedCapacity); err != nil {
            return fmt.Errorf("invalid expected capacity: %w", err)
        }
    }
    
    return nil
}

// validateKubeConfig validates the kubeconfig
func (v *SyncTargetValidator) validateKubeConfig(st *tmcv1alpha1.SyncTarget) error {
    if st.Spec.KubeConfig == "" && st.Spec.KubeConfigSecret == nil {
        return fmt.Errorf("either kubeConfig or kubeConfigSecret must be specified")
    }
    
    if st.Spec.KubeConfig != "" && st.Spec.KubeConfigSecret != nil {
        return fmt.Errorf("only one of kubeConfig or kubeConfigSecret can be specified")
    }
    
    // If using secret reference, validate it
    if st.Spec.KubeConfigSecret != nil {
        if st.Spec.KubeConfigSecret.Name == "" {
            return fmt.Errorf("kubeConfigSecret.name is required")
        }
        
        if st.Spec.KubeConfigSecret.Key == "" {
            return fmt.Errorf("kubeConfigSecret.key is required")
        }
    }
    
    // Basic validation of kubeconfig content if inline
    if st.Spec.KubeConfig != "" {
        if !strings.Contains(st.Spec.KubeConfig, "clusters:") {
            return fmt.Errorf("kubeConfig appears to be invalid (missing clusters)")
        }
        
        if !strings.Contains(st.Spec.KubeConfig, "users:") {
            return fmt.Errorf("kubeConfig appears to be invalid (missing users)")
        }
    }
    
    return nil
}

// validateCell validates the cell
func (v *SyncTargetValidator) validateCell(st *tmcv1alpha1.SyncTarget) error {
    if st.Spec.Cell == "" {
        return nil // Optional
    }
    
    // Validate cell format (e.g., region-zone format)
    parts := strings.Split(st.Spec.Cell, "-")
    if len(parts) < 2 {
        return fmt.Errorf("cell must be in format 'region-zone' (e.g., 'us-west-1a')")
    }
    
    // Validate allowed regions
    validRegions := []string{"us-west", "us-east", "eu-west", "eu-central", "ap-south", "ap-northeast"}
    regionValid := false
    for _, valid := range validRegions {
        if strings.HasPrefix(st.Spec.Cell, valid) {
            regionValid = true
            break
        }
    }
    
    if !regionValid {
        return fmt.Errorf("invalid region in cell %s", st.Spec.Cell)
    }
    
    return nil
}

// validateLabels validates labels
func (v *SyncTargetValidator) validateLabels(st *tmcv1alpha1.SyncTarget) error {
    // Check for required labels
    requiredLabels := []string{
        "tmc.kcp.dev/cluster-type",
    }
    
    for _, label := range requiredLabels {
        if _, exists := st.Labels[label]; !exists {
            return fmt.Errorf("required label %s is missing", label)
        }
    }
    
    // Validate cluster type
    clusterType := st.Labels["tmc.kcp.dev/cluster-type"]
    validTypes := []string{"physical", "virtual", "edge", "cloud"}
    typeValid := false
    for _, valid := range validTypes {
        if clusterType == valid {
            typeValid = true
            break
        }
    }
    
    if !typeValid {
        return fmt.Errorf("invalid cluster type: %s", clusterType)
    }
    
    return nil
}

// validateCapacity validates capacity specifications
func (v *SyncTargetValidator) validateCapacity(capacity *tmcv1alpha1.ResourceCapacity) error {
    // Validate CPU
    if capacity.CPU.Sign() <= 0 {
        return fmt.Errorf("CPU capacity must be positive")
    }
    
    // Validate Memory
    if capacity.Memory.Sign() <= 0 {
        return fmt.Errorf("Memory capacity must be positive")
    }
    
    // Validate Storage
    if capacity.Storage.Sign() <= 0 {
        return fmt.Errorf("Storage capacity must be positive")
    }
    
    // Validate Pods
    if capacity.Pods <= 0 {
        return fmt.Errorf("Pods capacity must be positive")
    }
    
    // Validate Nodes
    if capacity.Nodes <= 0 {
        return fmt.Errorf("Nodes capacity must be positive")
    }
    
    // Validate reasonable limits
    maxNodes := int32(10000)
    if capacity.Nodes > maxNodes {
        return fmt.Errorf("Nodes capacity exceeds maximum (%d)", maxNodes)
    }
    
    return nil
}

// SyncTargetMutator mutates SyncTarget resources
type SyncTargetMutator struct {
    decoder *admission.Decoder
}

// Handle handles mutation requests
func (m *SyncTargetMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
    syncTarget := &tmcv1alpha1.SyncTarget{}
    
    err := m.decoder.Decode(req, syncTarget)
    if err != nil {
        klog.Errorf("Failed to decode SyncTarget: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Mutating SyncTarget %s", syncTarget.Name)
    
    // Apply mutations
    m.mutateSyncTarget(syncTarget)
    
    // Create patches
    marshaledSyncTarget, err := json.Marshal(syncTarget)
    if err != nil {
        return admission.Errored(http.StatusInternalServerError, err)
    }
    
    return admission.PatchResponseFromRaw(req.Object.Raw, marshaledSyncTarget)
}

// mutateSyncTarget applies mutations to a SyncTarget
func (m *SyncTargetMutator) mutateSyncTarget(st *tmcv1alpha1.SyncTarget) {
    // Set default labels
    if st.Labels == nil {
        st.Labels = make(map[string]string)
    }
    
    // Add management label
    if _, exists := st.Labels["tmc.kcp.dev/managed"]; !exists {
        st.Labels["tmc.kcp.dev/managed"] = "true"
    }
    
    // Set default cluster type if not specified
    if _, exists := st.Labels["tmc.kcp.dev/cluster-type"]; !exists {
        st.Labels["tmc.kcp.dev/cluster-type"] = "physical"
    }
    
    // Set default annotations
    if st.Annotations == nil {
        st.Annotations = make(map[string]string)
    }
    
    // Add creation timestamp annotation
    if _, exists := st.Annotations["tmc.kcp.dev/registered-at"]; !exists {
        st.Annotations["tmc.kcp.dev/registered-at"] = time.Now().Format(time.RFC3339)
    }
    
    // Set default expected capacity if not specified
    if st.Spec.ExpectedCapacity == nil {
        st.Spec.ExpectedCapacity = &tmcv1alpha1.ResourceCapacity{
            CPU:     resource.MustParse("8"),
            Memory:  resource.MustParse("32Gi"),
            Storage: resource.MustParse("100Gi"),
            Pods:    110,
            Nodes:   1,
        }
    }
    
    // Set default sync interval
    if st.Spec.SyncInterval == nil {
        defaultInterval := metav1.Duration{Duration: 30 * time.Second}
        st.Spec.SyncInterval = &defaultInterval
    }
}
```

### Step 4: Implement ClusterRegistration Webhook (Hour 6)

```go
// pkg/admission/webhooks/clusterregistration_webhook.go
package webhooks

import (
    "context"
    "fmt"
    "net/http"
    "regexp"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "k8s.io/klog/v2"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ClusterRegistrationValidator validates ClusterRegistration resources
type ClusterRegistrationValidator struct {
    decoder *admission.Decoder
}

// Handle handles validation requests
func (v *ClusterRegistrationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
    registration := &tmcv1alpha1.ClusterRegistration{}
    
    err := v.decoder.Decode(req, registration)
    if err != nil {
        klog.Errorf("Failed to decode ClusterRegistration: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Validating ClusterRegistration %s", registration.Name)
    
    // Perform validation
    if err := v.validateClusterRegistration(registration); err != nil {
        klog.V(2).Infof("ClusterRegistration %s validation failed: %v", registration.Name, err)
        return admission.Denied(err.Error())
    }
    
    return admission.Allowed("")
}

// validateClusterRegistration validates a ClusterRegistration
func (v *ClusterRegistrationValidator) validateClusterRegistration(cr *tmcv1alpha1.ClusterRegistration) error {
    // Validate cluster name
    if err := v.validateClusterName(cr); err != nil {
        return fmt.Errorf("invalid cluster name: %w", err)
    }
    
    // Validate location
    if err := v.validateLocation(cr); err != nil {
        return fmt.Errorf("invalid location: %w", err)
    }
    
    // Validate cluster type
    if err := v.validateClusterType(cr); err != nil {
        return fmt.Errorf("invalid cluster type: %w", err)
    }
    
    // Validate contact information
    if err := v.validateContactInfo(cr); err != nil {
        return fmt.Errorf("invalid contact information: %w", err)
    }
    
    // Validate kubeconfig if provided
    if cr.Spec.KubeConfig != "" {
        if err := v.validateKubeConfig(cr.Spec.KubeConfig); err != nil {
            return fmt.Errorf("invalid kubeconfig: %w", err)
        }
    }
    
    return nil
}

// validateClusterName validates the cluster name
func (v *ClusterRegistrationValidator) validateClusterName(cr *tmcv1alpha1.ClusterRegistration) error {
    // Validate name format (DNS-1123)
    nameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
    if !nameRegex.MatchString(cr.Name) {
        return fmt.Errorf("name must be a valid DNS-1123 subdomain")
    }
    
    // Check name length
    if len(cr.Name) > 63 {
        return fmt.Errorf("name must be no more than 63 characters")
    }
    
    // Check for reserved names
    reservedNames := []string{"kcp", "system", "default", "kube-system"}
    for _, reserved := range reservedNames {
        if cr.Name == reserved {
            return fmt.Errorf("name %s is reserved", reserved)
        }
    }
    
    return nil
}

// validateLocation validates the location
func (v *ClusterRegistrationValidator) validateLocation(cr *tmcv1alpha1.ClusterRegistration) error {
    if cr.Spec.Location == "" {
        return fmt.Errorf("location is required")
    }
    
    // Validate location format
    locationRegex := regexp.MustCompile(`^[a-z]{2,3}-[a-z]+-\d+[a-z]?$`)
    if !locationRegex.MatchString(cr.Spec.Location) {
        return fmt.Errorf("location must be in format 'region-zone-number' (e.g., 'us-west-1a')")
    }
    
    return nil
}

// validateClusterType validates the cluster type
func (v *ClusterRegistrationValidator) validateClusterType(cr *tmcv1alpha1.ClusterRegistration) error {
    if cr.Spec.Type == "" {
        return nil // Optional
    }
    
    validTypes := []string{"physical", "virtual", "edge", "cloud", "hybrid"}
    valid := false
    for _, t := range validTypes {
        if cr.Spec.Type == t {
            valid = true
            break
        }
    }
    
    if !valid {
        return fmt.Errorf("type must be one of: %v", validTypes)
    }
    
    return nil
}

// validateContactInfo validates contact information
func (v *ClusterRegistrationValidator) validateContactInfo(cr *tmcv1alpha1.ClusterRegistration) error {
    if cr.Spec.ContactInfo == nil {
        return nil // Optional
    }
    
    contact := cr.Spec.ContactInfo
    
    // Validate email if provided
    if contact.Email != "" {
        emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
        if !emailRegex.MatchString(contact.Email) {
            return fmt.Errorf("invalid email format")
        }
    }
    
    // Validate team name
    if contact.Team != "" {
        if len(contact.Team) > 100 {
            return fmt.Errorf("team name must be no more than 100 characters")
        }
    }
    
    return nil
}

// validateKubeConfig validates kubeconfig content
func (v *ClusterRegistrationValidator) validateKubeConfig(kubeconfig string) error {
    // Basic validation
    if len(kubeconfig) < 100 {
        return fmt.Errorf("kubeconfig appears too short to be valid")
    }
    
    if len(kubeconfig) > 1048576 { // 1MB limit
        return fmt.Errorf("kubeconfig exceeds maximum size (1MB)")
    }
    
    // Check for required sections
    requiredSections := []string{"clusters:", "contexts:", "users:"}
    for _, section := range requiredSections {
        if !strings.Contains(kubeconfig, section) {
            return fmt.Errorf("kubeconfig missing required section: %s", section)
        }
    }
    
    return nil
}

// ClusterRegistrationMutator mutates ClusterRegistration resources
type ClusterRegistrationMutator struct {
    decoder *admission.Decoder
}

// Handle handles mutation requests
func (m *ClusterRegistrationMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
    registration := &tmcv1alpha1.ClusterRegistration{}
    
    err := m.decoder.Decode(req, registration)
    if err != nil {
        klog.Errorf("Failed to decode ClusterRegistration: %v", err)
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    klog.V(2).Infof("Mutating ClusterRegistration %s", registration.Name)
    
    // Apply mutations
    m.mutateClusterRegistration(registration)
    
    // Create patches
    marshaledRegistration, err := json.Marshal(registration)
    if err != nil {
        return admission.Errored(http.StatusInternalServerError, err)
    }
    
    return admission.PatchResponseFromRaw(req.Object.Raw, marshaledRegistration)
}

// mutateClusterRegistration applies mutations to a ClusterRegistration
func (m *ClusterRegistrationMutator) mutateClusterRegistration(cr *tmcv1alpha1.ClusterRegistration) {
    // Set default labels
    if cr.Labels == nil {
        cr.Labels = make(map[string]string)
    }
    
    // Add location label
    if cr.Spec.Location != "" {
        cr.Labels["tmc.kcp.dev/location"] = cr.Spec.Location
    }
    
    // Add type label
    if cr.Spec.Type != "" {
        cr.Labels["tmc.kcp.dev/type"] = cr.Spec.Type
    } else {
        cr.Labels["tmc.kcp.dev/type"] = "physical" // Default type
    }
    
    // Set default annotations
    if cr.Annotations == nil {
        cr.Annotations = make(map[string]string)
    }
    
    // Add registration timestamp
    if _, exists := cr.Annotations["tmc.kcp.dev/registered-at"]; !exists {
        cr.Annotations["tmc.kcp.dev/registered-at"] = time.Now().Format(time.RFC3339)
    }
    
    // Set default approval status
    if _, exists := cr.Annotations["tmc.kcp.dev/auto-approve"]; !exists {
        // Auto-approve if from trusted source
        if m.isTrustedSource(cr) {
            cr.Annotations["tmc.kcp.dev/auto-approve"] = "true"
        } else {
            cr.Annotations["tmc.kcp.dev/auto-approve"] = "false"
        }
    }
    
    // Set default contact info if not provided
    if cr.Spec.ContactInfo == nil {
        cr.Spec.ContactInfo = &tmcv1alpha1.ContactInfo{
            Team: "unassigned",
        }
    }
}

// isTrustedSource checks if registration is from a trusted source
func (m *ClusterRegistrationMutator) isTrustedSource(cr *tmcv1alpha1.ClusterRegistration) bool {
    // Check for trusted annotation
    if cr.Annotations != nil {
        if trusted, exists := cr.Annotations["tmc.kcp.dev/trusted-source"]; exists && trusted == "true" {
            return true
        }
    }
    
    // Check for trusted namespace
    trustedNamespaces := []string{"kcp-system", "tmc-system"}
    for _, ns := range trustedNamespaces {
        if cr.Namespace == ns {
            return true
        }
    }
    
    return false
}
```

### Step 5: Implement Helper Functions (Hour 7)

```go
// pkg/admission/webhooks/helpers.go
package webhooks

import (
    "fmt"
    
    admissionv1 "k8s.io/api/admission/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildAllowedResponse builds an allowed admission response
func BuildAllowedResponse(message string) *admissionv1.AdmissionResponse {
    return &admissionv1.AdmissionResponse{
        Allowed: true,
        Result: &metav1.Status{
            Message: message,
        },
    }
}

// BuildDeniedResponse builds a denied admission response
func BuildDeniedResponse(reason string) *admissionv1.AdmissionResponse {
    return &admissionv1.AdmissionResponse{
        Allowed: false,
        Result: &metav1.Status{
            Message: reason,
            Code:    403,
        },
    }
}

// BuildErrorResponse builds an error admission response
func BuildErrorResponse(err error) *admissionv1.AdmissionResponse {
    return &admissionv1.AdmissionResponse{
        Allowed: false,
        Result: &metav1.Status{
            Message: err.Error(),
            Code:    500,
        },
    }
}

// ValidateNamespace validates namespace constraints
func ValidateNamespace(namespace string, allowedNamespaces []string) error {
    if len(allowedNamespaces) == 0 {
        return nil // No restrictions
    }
    
    for _, allowed := range allowedNamespaces {
        if namespace == allowed {
            return nil
        }
    }
    
    return fmt.Errorf("namespace %s is not allowed", namespace)
}

// ValidateWorkspace validates workspace constraints
func ValidateWorkspace(workspace string) error {
    if workspace == "" {
        return fmt.Errorf("workspace is required")
    }
    
    // Check for system workspaces
    systemWorkspaces := []string{"root", "system"}
    for _, sys := range systemWorkspaces {
        if workspace == sys {
            return fmt.Errorf("cannot modify system workspace %s", workspace)
        }
    }
    
    return nil
}

// IsUpdateValid checks if an update is valid
func IsUpdateValid(old, new interface{}) error {
    // This is a placeholder for update validation logic
    // Specific implementations would check immutable fields
    return nil
}
```

## Testing Requirements

### Unit Tests

1. **Webhook Server Tests**
   - Test server initialization
   - Test handler registration
   - Test TLS configuration
   - Test health endpoints

2. **WorkloadPlacement Webhook Tests**
   - Test validation logic
   - Test mutation logic
   - Test error handling
   - Test edge cases

3. **SyncTarget Webhook Tests**
   - Test validation rules
   - Test default values
   - Test kubeconfig validation

4. **ClusterRegistration Webhook Tests**
   - Test name validation
   - Test location validation
   - Test auto-approval logic

5. **Helper Function Tests**
   - Test response builders
   - Test validation utilities

### Integration Tests

1. **End-to-End Webhook Tests**
   - Test complete admission flow
   - Test with real resources
   - Test rejection scenarios
   - Test mutation application

2. **Multi-Resource Tests**
   - Test cross-resource validation
   - Test dependency validation

## KCP Patterns to Follow

### Admission Control
- Follow Kubernetes admission patterns
- Implement proper validation
- Apply sensible defaults
- Handle errors gracefully

### Security
- Validate all inputs
- Prevent privilege escalation
- Check workspace boundaries
- Enforce quotas

### Performance
- Keep validations fast
- Avoid external calls in webhooks
- Cache frequently used data

## Integration Points

### With Controllers
- Webhooks validate before controllers process
- Apply defaults that controllers expect
- Ensure consistency

### With API Server
- Register webhooks properly
- Handle admission reviews
- Return proper responses

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 550 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Webhooks validate correctly

### Functionality Complete
- [ ] Server starts and serves
- [ ] All webhooks registered
- [ ] Validation working
- [ ] Mutation applied
- [ ] TLS configured

### Integration Ready
- [ ] Webhooks accessible
- [ ] Proper responses returned
- [ ] Error handling robust
- [ ] Logging comprehensive

### Documentation Complete
- [ ] Validation rules documented
- [ ] Mutation behavior documented
- [ ] Configuration documented
- [ ] Error codes documented

## Commit Message Template
```
feat(webhooks): implement admission webhooks for TMC resources

- Add webhook server with TLS support
- Implement WorkloadPlacement validation and mutation
- Implement SyncTarget validation and defaults
- Implement ClusterRegistration validation and auto-approval
- Add comprehensive validation rules
- Ensure workspace isolation in validations

Part of TMC Phase 6 Wave 3 implementation
Independent component - no dependencies
```

## Next Steps
After this branch is complete:
1. Admission control will be active
2. Resources will be validated
3. Defaults will be applied automatically