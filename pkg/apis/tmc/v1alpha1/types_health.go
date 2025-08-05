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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// WorkloadHealthPolicy represents health monitoring and policy configuration for workload placement.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type WorkloadHealthPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadHealthPolicySpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadHealthPolicyStatus `json:"status,omitempty"`
}

// WorkloadHealthPolicySpec defines the desired state of WorkloadHealthPolicy.
type WorkloadHealthPolicySpec struct {
	// WorkloadSelector selects the workloads this health policy applies to
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines which clusters to apply health policies
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// HealthChecks defines the health check configurations
	HealthChecks []HealthCheckConfig `json:"healthChecks"`

	// FailurePolicy defines how to handle health check failures
	// +kubebuilder:default="Quarantine"
	// +optional
	FailurePolicy HealthFailurePolicy `json:"failurePolicy,omitempty"`

	// RecoveryPolicy defines how to handle recovery from failures
	// +optional
	RecoveryPolicy *HealthRecoveryPolicy `json:"recoveryPolicy,omitempty"`

	// GlobalTimeout defines the global timeout for all health checks
	// +kubebuilder:default="300s"
	// +optional
	GlobalTimeout metav1.Duration `json:"globalTimeout,omitempty"`
}

// HealthCheckConfig defines a single health check configuration
type HealthCheckConfig struct {
	// Name is the health check name
	Name string `json:"name"`

	// Type defines the health check type
	// +kubebuilder:validation:Enum=HTTP;TCP;GRPC;Command;Kubernetes
	Type HealthCheckType `json:"type"`

	// Interval defines how often to perform the health check
	// +kubebuilder:default="30s"
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout defines the health check timeout
	// +kubebuilder:default="10s"
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// SuccessThreshold defines consecutive successes required
	// +kubebuilder:default=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// FailureThreshold defines consecutive failures required
	// +kubebuilder:default=3
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// HTTPCheck defines HTTP-specific health check configuration
	// +optional
	HTTPCheck *HTTPHealthCheck `json:"httpCheck,omitempty"`

	// TCPCheck defines TCP-specific health check configuration
	// +optional
	TCPCheck *TCPHealthCheck `json:"tcpCheck,omitempty"`

	// GRPCCheck defines GRPC-specific health check configuration
	// +optional
	GRPCCheck *GRPCHealthCheck `json:"grpcCheck,omitempty"`

	// CommandCheck defines command-based health check configuration
	// +optional
	CommandCheck *CommandHealthCheck `json:"commandCheck,omitempty"`

	// KubernetesCheck defines Kubernetes-native health check configuration
	// +optional
	KubernetesCheck *KubernetesHealthCheck `json:"kubernetesCheck,omitempty"`

	// Weight defines the importance of this health check (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`
}

// HealthCheckType represents the type of health check
// +kubebuilder:validation:Enum=HTTP;TCP;GRPC;Command;Kubernetes
type HealthCheckType string

const (
	// HealthCheckTypeHTTP uses HTTP requests for health checks
	HealthCheckTypeHTTP HealthCheckType = "HTTP"

	// HealthCheckTypeTCP uses TCP connections for health checks
	HealthCheckTypeTCP HealthCheckType = "TCP"

	// HealthCheckTypeGRPC uses GRPC calls for health checks
	HealthCheckTypeGRPC HealthCheckType = "GRPC"

	// HealthCheckTypeCommand executes commands for health checks
	HealthCheckTypeCommand HealthCheckType = "Command"

	// HealthCheckTypeKubernetes uses Kubernetes probes
	HealthCheckTypeKubernetes HealthCheckType = "Kubernetes"
)

// HTTPHealthCheck defines HTTP-specific health check configuration
type HTTPHealthCheck struct {
	// URL is the HTTP endpoint to check
	URL string `json:"url"`

	// Method defines the HTTP method
	// +kubebuilder:validation:Enum=GET;POST;PUT;HEAD
	// +kubebuilder:default="GET"
	// +optional
	Method string `json:"method,omitempty"`

	// Headers defines HTTP headers to send
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// ExpectedStatusCodes defines expected HTTP status codes
	// +optional
	ExpectedStatusCodes []int `json:"expectedStatusCodes,omitempty"`

	// ExpectedResponseBody defines expected response body pattern
	// +optional
	ExpectedResponseBody string `json:"expectedResponseBody,omitempty"`

	// InsecureTLS allows insecure TLS connections
	// +optional
	InsecureTLS bool `json:"insecureTls,omitempty"`
}

// TCPHealthCheck defines TCP-specific health check configuration
type TCPHealthCheck struct {
	// Host is the TCP host to connect to
	Host string `json:"host"`

	// Port is the TCP port to connect to
	Port int32 `json:"port"`

	// SendData defines data to send after connection
	// +optional
	SendData string `json:"sendData,omitempty"`

	// ExpectedResponse defines expected response pattern
	// +optional
	ExpectedResponse string `json:"expectedResponse,omitempty"`
}

// GRPCHealthCheck defines GRPC-specific health check configuration
type GRPCHealthCheck struct {
	// Address is the GRPC server address
	Address string `json:"address"`

	// Service defines the service name to check
	// +optional
	Service string `json:"service,omitempty"`

	// InsecureTLS allows insecure TLS connections
	// +optional
	InsecureTLS bool `json:"insecureTls,omitempty"`
}

// CommandHealthCheck defines command-based health check configuration
type CommandHealthCheck struct {
	// Command is the command to execute
	Command []string `json:"command"`

	// WorkingDirectory defines the working directory
	// +optional
	WorkingDirectory string `json:"workingDirectory,omitempty"`

	// Environment defines environment variables
	// +optional
	Environment map[string]string `json:"environment,omitempty"`

	// ExpectedExitCode defines the expected exit code
	// +kubebuilder:default=0
	// +optional
	ExpectedExitCode int32 `json:"expectedExitCode,omitempty"`
}

// KubernetesHealthCheck defines Kubernetes-native health check configuration
type KubernetesHealthCheck struct {
	// ProbeType defines the type of Kubernetes probe
	// +kubebuilder:validation:Enum=Readiness;Liveness;Startup
	ProbeType KubernetesProbeType `json:"probeType"`

	// Selector defines the pod selector
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// ContainerName defines the specific container to check
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// RequiredHealthyPods defines minimum healthy pods required
	// +kubebuilder:default=1
	// +optional
	RequiredHealthyPods int32 `json:"requiredHealthyPods,omitempty"`
}

// KubernetesProbeType represents Kubernetes probe types
// +kubebuilder:validation:Enum=Readiness;Liveness;Startup
type KubernetesProbeType string

const (
	// KubernetesProbeTypeReadiness checks readiness probes
	KubernetesProbeTypeReadiness KubernetesProbeType = "Readiness"

	// KubernetesProbeTypeLiveness checks liveness probes
	KubernetesProbeTypeLiveness KubernetesProbeType = "Liveness"

	// KubernetesProbeTypeStartup checks startup probes
	KubernetesProbeTypeStartup KubernetesProbeType = "Startup"
)

// HealthFailurePolicy represents failure handling policies
// +kubebuilder:validation:Enum=Ignore;Quarantine;Remove;Alert
type HealthFailurePolicy string

const (
	// HealthFailurePolicyIgnore ignores health failures
	HealthFailurePolicyIgnore HealthFailurePolicy = "Ignore"

	// HealthFailurePolicyQuarantine quarantines unhealthy workloads
	HealthFailurePolicyQuarantine HealthFailurePolicy = "Quarantine"

	// HealthFailurePolicyRemove removes unhealthy workloads
	HealthFailurePolicyRemove HealthFailurePolicy = "Remove"

	// HealthFailurePolicyAlert sends alerts for unhealthy workloads
	HealthFailurePolicyAlert HealthFailurePolicy = "Alert"
)

// HealthRecoveryPolicy defines recovery behavior from health failures
type HealthRecoveryPolicy struct {
	// AutoRecovery enables automatic recovery
	// +kubebuilder:default=true
	// +optional
	AutoRecovery bool `json:"autoRecovery,omitempty"`

	// RecoveryDelay defines delay before attempting recovery
	// +kubebuilder:default="60s"
	// +optional
	RecoveryDelay metav1.Duration `json:"recoveryDelay,omitempty"`

	// MaxRecoveryAttempts defines maximum recovery attempts
	// +kubebuilder:default=3
	// +optional
	MaxRecoveryAttempts int32 `json:"maxRecoveryAttempts,omitempty"`

	// RecoveryThreshold defines health threshold for recovery
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=80
	// +optional
	RecoveryThreshold int32 `json:"recoveryThreshold,omitempty"`
}

// WorkloadHealthPolicyStatus represents the observed state of WorkloadHealthPolicy
type WorkloadHealthPolicyStatus struct {
	// Conditions represent the latest available observations of the health policy state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current health policy phase
	// +kubebuilder:default="Active"
	// +optional
	Phase HealthPolicyPhase `json:"phase,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// OverallHealthScore represents the overall health score (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	OverallHealthScore *int32 `json:"overallHealthScore,omitempty"`

	// HealthCheckResults contains the results of health checks
	// +optional
	HealthCheckResults []HealthCheckResult `json:"healthCheckResults,omitempty"`

	// AffectedWorkloads contains workloads affected by this policy
	// +optional
	AffectedWorkloads []HealthWorkloadReference `json:"affectedWorkloads,omitempty"`

	// Message provides additional information about the health policy state
	// +optional
	Message string `json:"message,omitempty"`
}

// HealthPolicyPhase represents the current phase of health policy
// +kubebuilder:validation:Enum=Active;Suspended;Failed;Unknown
type HealthPolicyPhase string

const (
	// HealthPolicyPhaseActive indicates policy is active
	HealthPolicyPhaseActive HealthPolicyPhase = "Active"

	// HealthPolicyPhaseSuspended indicates policy is suspended
	HealthPolicyPhaseSuspended HealthPolicyPhase = "Suspended"

	// HealthPolicyPhaseFailed indicates policy failed
	HealthPolicyPhaseFailed HealthPolicyPhase = "Failed"

	// HealthPolicyPhaseUnknown indicates unknown policy state
	HealthPolicyPhaseUnknown HealthPolicyPhase = "Unknown"
)

// HealthCheckResult represents a health check result
type HealthCheckResult struct {
	// Name is the health check name
	Name string `json:"name"`

	// Status is the current health status
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown;Degraded
	Status HealthStatus `json:"status"`

	// Score is the health score (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score int32 `json:"score"`

	// LastCheckTime is when the check was last performed
	LastCheckTime metav1.Time `json:"lastCheckTime"`

	// ConsecutiveSuccesses is the number of consecutive successes
	ConsecutiveSuccesses int32 `json:"consecutiveSuccesses"`

	// ConsecutiveFailures is the number of consecutive failures
	ConsecutiveFailures int32 `json:"consecutiveFailures"`

	// Message provides additional information about the health check
	// +optional
	Message string `json:"message,omitempty"`
}

// HealthStatus represents health status
// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown;Degraded
type HealthStatus string

const (
	// HealthStatusHealthy indicates healthy status
	HealthStatusHealthy HealthStatus = "Healthy"

	// HealthStatusUnhealthy indicates unhealthy status
	HealthStatusUnhealthy HealthStatus = "Unhealthy"

	// HealthStatusUnknown indicates unknown status
	HealthStatusUnknown HealthStatus = "Unknown"

	// HealthStatusDegraded indicates degraded status
	HealthStatusDegraded HealthStatus = "Degraded"
)

// HealthWorkloadReference extends WorkloadReference with health-specific information
type HealthWorkloadReference struct {
	// WorkloadReference embeds the basic workload reference
	WorkloadReference `json:",inline"`

	// ClusterName is the cluster where the workload is placed
	ClusterName string `json:"clusterName"`

	// HealthStatus is the current health status
	HealthStatus HealthStatus `json:"healthStatus"`

	// LastHealthUpdate is when health was last updated
	LastHealthUpdate metav1.Time `json:"lastHealthUpdate"`
}

// WorkloadHealthPolicyList is a list of WorkloadHealthPolicy resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadHealthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadHealthPolicy `json:"items"`
}