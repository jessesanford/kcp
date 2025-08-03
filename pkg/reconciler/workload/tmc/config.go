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

package tmc

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
)

// TMCConfig represents the complete configuration for the TMC system
type TMCConfig struct {
	// Core configuration
	Core CoreConfig `json:"core" yaml:"core"`

	// Component configurations
	Placement        PlacementConfig        `json:"placement" yaml:"placement"`
	Sync             SyncConfig             `json:"sync" yaml:"sync"`
	Migration        MigrationConfig        `json:"migration" yaml:"migration"`
	VirtualWorkspace VirtualWorkspaceConfig `json:"virtualWorkspace" yaml:"virtualWorkspace"`

	// System configurations
	Health   HealthConfig   `json:"health" yaml:"health"`
	Metrics  MetricsConfig  `json:"metrics" yaml:"metrics"`
	Tracing  TracingConfig  `json:"tracing" yaml:"tracing"`
	Recovery RecoveryConfig `json:"recovery" yaml:"recovery"`

	// Resource configurations
	Resources ResourcesConfig `json:"resources" yaml:"resources"`

	// Validation and metadata
	Version  string            `json:"version" yaml:"version"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// CoreConfig contains core TMC system configuration
type CoreConfig struct {
	// Service configuration
	ServiceName    string `json:"serviceName" yaml:"serviceName" default:"kcp-tmc"`
	ServiceVersion string `json:"serviceVersion" yaml:"serviceVersion" default:"v1.0.0"`
	Environment    string `json:"environment" yaml:"environment" default:"production"`

	// Operational settings
	LogLevel        string `json:"logLevel" yaml:"logLevel" default:"info" validate:"oneof=debug info warn error"`
	EnableProfiling bool   `json:"enableProfiling" yaml:"enableProfiling" default:"false"`

	// Concurrency and performance
	MaxConcurrentOperations int           `json:"maxConcurrentOperations" yaml:"maxConcurrentOperations" default:"10" validate:"min=1,max=100"`
	OperationTimeout        time.Duration `json:"operationTimeout" yaml:"operationTimeout" default:"5m"`
	WorkerPoolSize          int           `json:"workerPoolSize" yaml:"workerPoolSize" default:"5" validate:"min=1,max=50"`

	// Feature flags
	FeatureFlags map[string]bool `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

// PlacementConfig contains placement-specific configuration
type PlacementConfig struct {
	// Controller settings
	Enabled                 bool          `json:"enabled" yaml:"enabled" default:"true"`
	SyncInterval            time.Duration `json:"syncInterval" yaml:"syncInterval" default:"30s"`
	MaxConcurrentPlacements int           `json:"maxConcurrentPlacements" yaml:"maxConcurrentPlacements" default:"5" validate:"min=1,max=20"`

	// Placement behavior
	DefaultStrategy   string        `json:"defaultStrategy" yaml:"defaultStrategy" default:"roundrobin" validate:"oneof=roundrobin weighted capacity priority"`
	RebalanceEnabled  bool          `json:"rebalanceEnabled" yaml:"rebalanceEnabled" default:"true"`
	RebalanceInterval time.Duration `json:"rebalanceInterval" yaml:"rebalanceInterval" default:"5m"`

	// Constraints and limits
	MaxClustersPerPlacement int     `json:"maxClustersPerPlacement" yaml:"maxClustersPerPlacement" default:"10" validate:"min=1,max=100"`
	MinHealthyRatio         float64 `json:"minHealthyRatio" yaml:"minHealthyRatio" default:"0.5" validate:"min=0,max=1"`

	// Resource preferences
	PreferredZones  []string `json:"preferredZones,omitempty" yaml:"preferredZones,omitempty"`
	AvoidanceLabels []string `json:"avoidanceLabels,omitempty" yaml:"avoidanceLabels,omitempty"`

	// Advanced settings
	Advanced PlacementAdvancedConfig `json:"advanced" yaml:"advanced"`
}

// PlacementAdvancedConfig contains advanced placement settings
type PlacementAdvancedConfig struct {
	EnableAntiAffinity     bool          `json:"enableAntiAffinity" yaml:"enableAntiAffinity" default:"false"`
	PlacementTimeout       time.Duration `json:"placementTimeout" yaml:"placementTimeout" default:"2m"`
	RetryBackoffMultiplier float64       `json:"retryBackoffMultiplier" yaml:"retryBackoffMultiplier" default:"2.0"`
	MaxRetryDelay          time.Duration `json:"maxRetryDelay" yaml:"maxRetryDelay" default:"5m"`
}

// SyncConfig contains sync-specific configuration
type SyncConfig struct {
	// Controller settings
	Enabled      bool          `json:"enabled" yaml:"enabled" default:"true"`
	SyncInterval time.Duration `json:"syncInterval" yaml:"syncInterval" default:"15s"`

	// Sync behavior
	MaxSyncAttempts int           `json:"maxSyncAttempts" yaml:"maxSyncAttempts" default:"3" validate:"min=1,max=10"`
	SyncTimeout     time.Duration `json:"syncTimeout" yaml:"syncTimeout" default:"30s"`
	BatchSize       int           `json:"batchSize" yaml:"batchSize" default:"50" validate:"min=1,max=1000"`

	// Resource handling
	SupportedResources []string `json:"supportedResources,omitempty" yaml:"supportedResources,omitempty"`
	ExcludedResources  []string `json:"excludedResources,omitempty" yaml:"excludedResources,omitempty"`
	ConflictResolution string   `json:"conflictResolution" yaml:"conflictResolution" default:"last-writer-wins" validate:"oneof=last-writer-wins first-writer-wins manual"`

	// Performance tuning
	MaxSyncWorkers   int `json:"maxSyncWorkers" yaml:"maxSyncWorkers" default:"10" validate:"min=1,max=50"`
	SyncBacklogLimit int `json:"syncBacklogLimit" yaml:"syncBacklogLimit" default:"1000" validate:"min=100,max=10000"`
}

// MigrationConfig contains migration-specific configuration
type MigrationConfig struct {
	// Migration settings
	Enabled                 bool          `json:"enabled" yaml:"enabled" default:"true"`
	MaxConcurrentMigrations int           `json:"maxConcurrentMigrations" yaml:"maxConcurrentMigrations" default:"3" validate:"min=1,max=10"`
	MigrationTimeout        time.Duration `json:"migrationTimeout" yaml:"migrationTimeout" default:"30m"`

	// Migration strategies
	DefaultStrategy string        `json:"defaultStrategy" yaml:"defaultStrategy" default:"live" validate:"oneof=live snapshot halt-resume"`
	EnableRollback  bool          `json:"enableRollback" yaml:"enableRollback" default:"true"`
	RollbackTimeout time.Duration `json:"rollbackTimeout" yaml:"rollbackTimeout" default:"10m"`

	// Data transfer
	DataTransferBatchSize int           `json:"dataTransferBatchSize" yaml:"dataTransferBatchSize" default:"100" validate:"min=1,max=1000"`
	DataTransferTimeout   time.Duration `json:"dataTransferTimeout" yaml:"dataTransferTimeout" default:"5m"`
	CompressionEnabled    bool          `json:"compressionEnabled" yaml:"compressionEnabled" default:"true"`

	// Validation and safety
	PreMigrationValidation  bool     `json:"preMigrationValidation" yaml:"preMigrationValidation" default:"true"`
	PostMigrationValidation bool     `json:"postMigrationValidation" yaml:"postMigrationValidation" default:"true"`
	SafetyChecks            []string `json:"safetyChecks,omitempty" yaml:"safetyChecks,omitempty"`
}

// VirtualWorkspaceConfig contains virtual workspace configuration
type VirtualWorkspaceConfig struct {
	// Virtual workspace settings
	Enabled              bool          `json:"enabled" yaml:"enabled" default:"true"`
	MaxVirtualWorkspaces int           `json:"maxVirtualWorkspaces" yaml:"maxVirtualWorkspaces" default:"100" validate:"min=1,max=1000"`
	SyncInterval         time.Duration `json:"syncInterval" yaml:"syncInterval" default:"30s"`

	// Aggregation settings
	AggregationEnabled   bool          `json:"aggregationEnabled" yaml:"aggregationEnabled" default:"true"`
	AggregationInterval  time.Duration `json:"aggregationInterval" yaml:"aggregationInterval" default:"60s"`
	DefaultMergeStrategy string        `json:"defaultMergeStrategy" yaml:"defaultMergeStrategy" default:"union" validate:"oneof=union intersection priority latest"`

	// Projection settings
	ProjectionEnabled     bool          `json:"projectionEnabled" yaml:"projectionEnabled" default:"true"`
	ProjectionInterval    time.Duration `json:"projectionInterval" yaml:"projectionInterval" default:"90s"`
	DefaultProjectionMode string        `json:"defaultProjectionMode" yaml:"defaultProjectionMode" default:"selective" validate:"oneof=all selective conditional"`

	// Resource limits
	MaxResourcesPerWorkspace int `json:"maxResourcesPerWorkspace" yaml:"maxResourcesPerWorkspace" default:"10000" validate:"min=100,max=100000"`
	MaxClustersPerWorkspace  int `json:"maxClustersPerWorkspace" yaml:"maxClustersPerWorkspace" default:"20" validate:"min=1,max=100"`
}

// HealthConfig contains health monitoring configuration
type HealthConfig struct {
	// Health monitoring
	Enabled       bool          `json:"enabled" yaml:"enabled" default:"true"`
	CheckInterval time.Duration `json:"checkInterval" yaml:"checkInterval" default:"30s"`
	HealthTimeout time.Duration `json:"healthTimeout" yaml:"healthTimeout" default:"10s"`

	// Thresholds
	DegradedThreshold  time.Duration `json:"degradedThreshold" yaml:"degradedThreshold" default:"2m"`
	UnhealthyThreshold time.Duration `json:"unhealthyThreshold" yaml:"unhealthyThreshold" default:"5m"`

	// Health providers
	RegisterSystemProviders  bool `json:"registerSystemProviders" yaml:"registerSystemProviders" default:"true"`
	RegisterClusterProviders bool `json:"registerClusterProviders" yaml:"registerClusterProviders" default:"true"`

	// Alerting
	AlertingEnabled bool               `json:"alertingEnabled" yaml:"alertingEnabled" default:"false"`
	AlertThresholds map[string]float64 `json:"alertThresholds,omitempty" yaml:"alertThresholds,omitempty"`
}

// MetricsConfig contains metrics collection configuration
type MetricsConfig struct {
	// Metrics collection
	Enabled            bool          `json:"enabled" yaml:"enabled" default:"true"`
	CollectionInterval time.Duration `json:"collectionInterval" yaml:"collectionInterval" default:"30s"`

	// Prometheus settings
	PrometheusEnabled bool   `json:"prometheusEnabled" yaml:"prometheusEnabled" default:"true"`
	PrometheusPort    int    `json:"prometheusPort" yaml:"prometheusPort" default:"8080" validate:"min=1024,max=65535"`
	PrometheusPath    string `json:"prometheusPath" yaml:"prometheusPath" default:"/metrics"`

	// Metric retention
	RetentionPeriod time.Duration `json:"retentionPeriod" yaml:"retentionPeriod" default:"24h"`

	// Custom metrics
	EnableCustomMetrics bool   `json:"enableCustomMetrics" yaml:"enableCustomMetrics" default:"true"`
	MetricNamespace     string `json:"metricNamespace" yaml:"metricNamespace" default:"tmc"`
}

// TracingConfig contains distributed tracing configuration
type TracingConfig struct {
	// Tracing settings
	Enabled        bool   `json:"enabled" yaml:"enabled" default:"false"`
	ServiceName    string `json:"serviceName" yaml:"serviceName" default:"kcp-tmc"`
	ServiceVersion string `json:"serviceVersion" yaml:"serviceVersion" default:"v1.0.0"`

	// Sampling
	SamplingRate float64 `json:"samplingRate" yaml:"samplingRate" default:"0.1" validate:"min=0,max=1"`

	// Jaeger configuration
	JaegerEnabled  bool   `json:"jaegerEnabled" yaml:"jaegerEnabled" default:"false"`
	JaegerEndpoint string `json:"jaegerEndpoint" yaml:"jaegerEndpoint" default:"http://localhost:14268/api/traces"`

	// OTLP configuration
	OTLPEnabled  bool   `json:"otlpEnabled" yaml:"otlpEnabled" default:"false"`
	OTLPEndpoint string `json:"otlpEndpoint" yaml:"otlpEndpoint" default:"localhost:4317"`
}

// RecoveryConfig contains error recovery configuration
type RecoveryConfig struct {
	// Recovery settings
	Enabled                 bool          `json:"enabled" yaml:"enabled" default:"true"`
	MaxConcurrentRecoveries int           `json:"maxConcurrentRecoveries" yaml:"maxConcurrentRecoveries" default:"5" validate:"min=1,max=20"`
	RecoveryTimeout         time.Duration `json:"recoveryTimeout" yaml:"recoveryTimeout" default:"10m"`
	HealthCheckInterval     time.Duration `json:"healthCheckInterval" yaml:"healthCheckInterval" default:"30s"`

	// Retry configuration
	DefaultMaxRetries    int           `json:"defaultMaxRetries" yaml:"defaultMaxRetries" default:"5" validate:"min=1,max=20"`
	DefaultInitialDelay  time.Duration `json:"defaultInitialDelay" yaml:"defaultInitialDelay" default:"1s"`
	DefaultMaxDelay      time.Duration `json:"defaultMaxDelay" yaml:"defaultMaxDelay" default:"30s"`
	DefaultBackoffFactor float64       `json:"defaultBackoffFactor" yaml:"defaultBackoffFactor" default:"2.0" validate:"min=1.0,max=10.0"`

	// Circuit breaker
	CircuitBreakerEnabled   bool          `json:"circuitBreakerEnabled" yaml:"circuitBreakerEnabled" default:"true"`
	CircuitBreakerThreshold int           `json:"circuitBreakerThreshold" yaml:"circuitBreakerThreshold" default:"5" validate:"min=1,max=50"`
	CircuitBreakerTimeout   time.Duration `json:"circuitBreakerTimeout" yaml:"circuitBreakerTimeout" default:"60s"`
}

// ResourcesConfig contains resource-specific configuration
type ResourcesConfig struct {
	// Resource limits
	MaxResourcesPerCluster  int `json:"maxResourcesPerCluster" yaml:"maxResourcesPerCluster" default:"10000" validate:"min=100,max=1000000"`
	MaxNamespacesPerCluster int `json:"maxNamespacesPerCluster" yaml:"maxNamespacesPerCluster" default:"1000" validate:"min=10,max=10000"`

	// Resource types
	SupportedGVKs []string `json:"supportedGVKs,omitempty" yaml:"supportedGVKs,omitempty"`
	ExcludedGVKs  []string `json:"excludedGVKs,omitempty" yaml:"excludedGVKs,omitempty"`

	// Resource handling
	TransformationPolicies map[string]interface{} `json:"transformationPolicies,omitempty" yaml:"transformationPolicies,omitempty"`
	ValidationPolicies     map[string]interface{} `json:"validationPolicies,omitempty" yaml:"validationPolicies,omitempty"`

	// Performance
	ResourceCacheSize int           `json:"resourceCacheSize" yaml:"resourceCacheSize" default:"10000" validate:"min=100,max=100000"`
	ResourceCacheTTL  time.Duration `json:"resourceCacheTTL" yaml:"resourceCacheTTL" default:"5m"`
}

// DefaultTMCConfig returns a TMC configuration with all default values
func DefaultTMCConfig() *TMCConfig {
	config := &TMCConfig{
		Version: "v1.0.0",
		Metadata: map[string]string{
			"created": time.Now().Format(time.RFC3339),
		},
	}

	// Apply defaults using reflection
	applyDefaults(reflect.ValueOf(config).Elem())

	return config
}

// Validate validates the TMC configuration
func (c *TMCConfig) Validate() error {
	validator := NewConfigValidator()
	return validator.Validate(c)
}

// ApplyDefaults applies default values to unset configuration fields
func (c *TMCConfig) ApplyDefaults() {
	applyDefaults(reflect.ValueOf(c).Elem())
}

// ConfigValidator provides configuration validation functionality
type ConfigValidator struct {
	errors []error
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		errors: make([]error, 0),
	}
}

// Validate validates a TMC configuration
func (v *ConfigValidator) Validate(config *TMCConfig) error {
	v.errors = make([]error, 0)

	// Validate core configuration
	v.validateCore(&config.Core)

	// Validate component configurations
	v.validatePlacement(&config.Placement)
	v.validateSync(&config.Sync)
	v.validateMigration(&config.Migration)
	v.validateVirtualWorkspace(&config.VirtualWorkspace)

	// Validate system configurations
	v.validateHealth(&config.Health)
	v.validateMetrics(&config.Metrics)
	v.validateTracing(&config.Tracing)
	v.validateRecovery(&config.Recovery)
	v.validateResources(&config.Resources)

	// Return combined errors
	if len(v.errors) > 0 {
		return fmt.Errorf("configuration validation failed: %v", v.errors)
	}

	return nil
}

func (v *ConfigValidator) validateCore(config *CoreConfig) {
	if config.ServiceName == "" {
		v.addError("core.serviceName cannot be empty")
	}

	if config.ServiceVersion == "" {
		v.addError("core.serviceVersion cannot be empty")
	}

	if config.MaxConcurrentOperations <= 0 {
		v.addError("core.maxConcurrentOperations must be positive")
	}

	if config.OperationTimeout <= 0 {
		v.addError("core.operationTimeout must be positive")
	}

	if config.WorkerPoolSize <= 0 {
		v.addError("core.workerPoolSize must be positive")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, config.LogLevel) {
		v.addError(fmt.Sprintf("core.logLevel must be one of: %v", validLogLevels))
	}
}

func (v *ConfigValidator) validatePlacement(config *PlacementConfig) {
	if config.MaxConcurrentPlacements <= 0 {
		v.addError("placement.maxConcurrentPlacements must be positive")
	}

	if config.SyncInterval <= 0 {
		v.addError("placement.syncInterval must be positive")
	}

	if config.MaxClustersPerPlacement <= 0 {
		v.addError("placement.maxClustersPerPlacement must be positive")
	}

	if config.MinHealthyRatio < 0 || config.MinHealthyRatio > 1 {
		v.addError("placement.minHealthyRatio must be between 0 and 1")
	}

	validStrategies := []string{"roundrobin", "weighted", "capacity", "priority"}
	if !contains(validStrategies, config.DefaultStrategy) {
		v.addError(fmt.Sprintf("placement.defaultStrategy must be one of: %v", validStrategies))
	}
}

func (v *ConfigValidator) validateSync(config *SyncConfig) {
	if config.MaxSyncAttempts <= 0 {
		v.addError("sync.maxSyncAttempts must be positive")
	}

	if config.SyncTimeout <= 0 {
		v.addError("sync.syncTimeout must be positive")
	}

	if config.BatchSize <= 0 {
		v.addError("sync.batchSize must be positive")
	}

	if config.MaxSyncWorkers <= 0 {
		v.addError("sync.maxSyncWorkers must be positive")
	}

	validConflictResolutions := []string{"last-writer-wins", "first-writer-wins", "manual"}
	if !contains(validConflictResolutions, config.ConflictResolution) {
		v.addError(fmt.Sprintf("sync.conflictResolution must be one of: %v", validConflictResolutions))
	}
}

func (v *ConfigValidator) validateMigration(config *MigrationConfig) {
	if config.MaxConcurrentMigrations <= 0 {
		v.addError("migration.maxConcurrentMigrations must be positive")
	}

	if config.MigrationTimeout <= 0 {
		v.addError("migration.migrationTimeout must be positive")
	}

	if config.DataTransferBatchSize <= 0 {
		v.addError("migration.dataTransferBatchSize must be positive")
	}

	validStrategies := []string{"live", "snapshot", "halt-resume"}
	if !contains(validStrategies, config.DefaultStrategy) {
		v.addError(fmt.Sprintf("migration.defaultStrategy must be one of: %v", validStrategies))
	}
}

func (v *ConfigValidator) validateVirtualWorkspace(config *VirtualWorkspaceConfig) {
	if config.MaxVirtualWorkspaces <= 0 {
		v.addError("virtualWorkspace.maxVirtualWorkspaces must be positive")
	}

	if config.MaxResourcesPerWorkspace <= 0 {
		v.addError("virtualWorkspace.maxResourcesPerWorkspace must be positive")
	}

	if config.MaxClustersPerWorkspace <= 0 {
		v.addError("virtualWorkspace.maxClustersPerWorkspace must be positive")
	}

	validMergeStrategies := []string{"union", "intersection", "priority", "latest"}
	if !contains(validMergeStrategies, config.DefaultMergeStrategy) {
		v.addError(fmt.Sprintf("virtualWorkspace.defaultMergeStrategy must be one of: %v", validMergeStrategies))
	}

	validProjectionModes := []string{"all", "selective", "conditional"}
	if !contains(validProjectionModes, config.DefaultProjectionMode) {
		v.addError(fmt.Sprintf("virtualWorkspace.defaultProjectionMode must be one of: %v", validProjectionModes))
	}
}

func (v *ConfigValidator) validateHealth(config *HealthConfig) {
	if config.CheckInterval <= 0 {
		v.addError("health.checkInterval must be positive")
	}

	if config.HealthTimeout <= 0 {
		v.addError("health.healthTimeout must be positive")
	}

	if config.DegradedThreshold <= 0 {
		v.addError("health.degradedThreshold must be positive")
	}

	if config.UnhealthyThreshold <= 0 {
		v.addError("health.unhealthyThreshold must be positive")
	}
}

func (v *ConfigValidator) validateMetrics(config *MetricsConfig) {
	if config.CollectionInterval <= 0 {
		v.addError("metrics.collectionInterval must be positive")
	}

	if config.PrometheusPort <= 0 || config.PrometheusPort > 65535 {
		v.addError("metrics.prometheusPort must be between 1 and 65535")
	}

	if config.PrometheusPath == "" {
		v.addError("metrics.prometheusPath cannot be empty")
	}

	if config.RetentionPeriod <= 0 {
		v.addError("metrics.retentionPeriod must be positive")
	}
}

func (v *ConfigValidator) validateTracing(config *TracingConfig) {
	if config.SamplingRate < 0 || config.SamplingRate > 1 {
		v.addError("tracing.samplingRate must be between 0 and 1")
	}

	if config.Enabled && config.ServiceName == "" {
		v.addError("tracing.serviceName cannot be empty when tracing is enabled")
	}
}

func (v *ConfigValidator) validateRecovery(config *RecoveryConfig) {
	if config.MaxConcurrentRecoveries <= 0 {
		v.addError("recovery.maxConcurrentRecoveries must be positive")
	}

	if config.RecoveryTimeout <= 0 {
		v.addError("recovery.recoveryTimeout must be positive")
	}

	if config.DefaultMaxRetries <= 0 {
		v.addError("recovery.defaultMaxRetries must be positive")
	}

	if config.DefaultBackoffFactor < 1.0 {
		v.addError("recovery.defaultBackoffFactor must be at least 1.0")
	}
}

func (v *ConfigValidator) validateResources(config *ResourcesConfig) {
	if config.MaxResourcesPerCluster <= 0 {
		v.addError("resources.maxResourcesPerCluster must be positive")
	}

	if config.MaxNamespacesPerCluster <= 0 {
		v.addError("resources.maxNamespacesPerCluster must be positive")
	}

	if config.ResourceCacheSize <= 0 {
		v.addError("resources.resourceCacheSize must be positive")
	}

	if config.ResourceCacheTTL <= 0 {
		v.addError("resources.resourceCacheTTL must be positive")
	}

	// Validate GVKs format
	for _, gvk := range config.SupportedGVKs {
		if !isValidGVK(gvk) {
			v.addError(fmt.Sprintf("invalid GVK format: %s", gvk))
		}
	}

	for _, gvk := range config.ExcludedGVKs {
		if !isValidGVK(gvk) {
			v.addError(fmt.Sprintf("invalid GVK format: %s", gvk))
		}
	}
}

func (v *ConfigValidator) addError(message string) {
	v.errors = append(v.errors, fmt.Errorf(message))
}

// Helper functions

func applyDefaults(value reflect.Value) {
	if !value.IsValid() || !value.CanSet() {
		return
	}

	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := valueType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle struct fields recursively
		if field.Kind() == reflect.Struct {
			applyDefaults(field)
			continue
		}

		// Apply default if field is zero value and has default tag
		if field.IsZero() {
			defaultTag := fieldType.Tag.Get("default")
			if defaultTag != "" {
				setFieldFromTag(field, defaultTag)
			}
		}
	}
}

func setFieldFromTag(field reflect.Value, defaultTag string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultTag)
	case reflect.Bool:
		if val, err := strconv.ParseBool(defaultTag); err == nil {
			field.SetBool(val)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			if val, err := time.ParseDuration(defaultTag); err == nil {
				field.SetInt(int64(val))
			}
		} else {
			if val, err := strconv.ParseInt(defaultTag, 10, 64); err == nil {
				field.SetInt(val)
			}
		}
	case reflect.Float32, reflect.Float64:
		if val, err := strconv.ParseFloat(defaultTag, 64); err == nil {
			field.SetFloat(val)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isValidGVK(gvk string) bool {
	parts := strings.Split(gvk, "/")
	if len(parts) != 3 {
		return false
	}

	group, version, kind := parts[0], parts[1], parts[2]

	// Validate each part
	if kind == "" {
		return false
	}

	if version == "" {
		return false
	}

	// Group can be empty for core resources
	if group != "" {
		errs := validation.IsDNS1123Subdomain(group)
		if len(errs) > 0 {
			return false
		}
	}

	return true
}

// ConfigManager manages TMC configuration lifecycle
type ConfigManager struct {
	config    *TMCConfig
	validator *ConfigValidator
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config:    DefaultTMCConfig(),
		validator: NewConfigValidator(),
	}
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *TMCConfig {
	return cm.config
}

// LoadConfig loads configuration from a source and validates it
func (cm *ConfigManager) LoadConfig(config *TMCConfig) error {
	// Apply defaults first
	config.ApplyDefaults()

	// Validate configuration
	if err := cm.validator.Validate(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	cm.config = config
	return nil
}

// UpdateConfig updates specific configuration sections
func (cm *ConfigManager) UpdateConfig(updates map[string]interface{}) error {
	// Create a copy of current config
	updatedConfig := *cm.config

	// Apply updates using reflection
	if err := applyUpdates(reflect.ValueOf(&updatedConfig).Elem(), updates); err != nil {
		return fmt.Errorf("failed to apply config updates: %w", err)
	}

	// Validate updated configuration
	if err := cm.validator.Validate(&updatedConfig); err != nil {
		return fmt.Errorf("updated configuration validation failed: %w", err)
	}

	cm.config = &updatedConfig
	return nil
}

func applyUpdates(value reflect.Value, updates map[string]interface{}) error {
	for key, updateValue := range updates {
		if err := setNestedField(value, key, updateValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", key, err)
		}
	}
	return nil
}

func setNestedField(value reflect.Value, path string, newValue interface{}) error {
	parts := strings.Split(path, ".")
	current := value

	// Navigate to the field
	for i, part := range parts {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return fmt.Errorf("cannot navigate through non-struct field")
		}

		field := current.FieldByName(part)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found", part)
		}

		if i == len(parts)-1 {
			// Set the final field
			return setFieldValue(field, newValue)
		}

		current = field
	}

	return nil
}

func setFieldValue(field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	valueToSet := reflect.ValueOf(value)
	if !valueToSet.Type().AssignableTo(field.Type()) {
		return fmt.Errorf("value type %v is not assignable to field type %v", valueToSet.Type(), field.Type())
	}

	field.Set(valueToSet)
	return nil
}
