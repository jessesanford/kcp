/*
Copyright The KCP Authors.

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

package policies

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
	"github.com/kcp-dev/kcp/pkg/logging"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

const (
	ControllerName = "tmc-scaling-policy-manager"
	
	// Policy validation constants
	defaultValidationTimeout = 30 * time.Second
	maxPolicyRetries        = 3
)

// ScalingPolicySpec defines the desired state of a scaling policy
type ScalingPolicySpec struct {
	// Target defines the resource to be scaled
	Target ScalingTarget `json:"target"`
	
	// Triggers define conditions that trigger scaling
	Triggers []ScalingTrigger `json:"triggers"`
	
	// Constraints define scaling boundaries
	Constraints ScalingConstraints `json:"constraints"`
	
	// Behavior defines scaling behavior parameters
	Behavior *ScalingBehavior `json:"behavior,omitempty"`
}

// ScalingTarget defines the target resource for scaling
type ScalingTarget struct {
	// APIVersion of the target resource
	APIVersion string `json:"apiVersion"`
	
	// Kind of the target resource
	Kind string `json:"kind"`
	
	// Name of the target resource
	Name string `json:"name"`
	
	// Namespace of the target resource (if namespaced)
	Namespace string `json:"namespace,omitempty"`
}

// ScalingTrigger defines conditions that trigger scaling actions
type ScalingTrigger struct {
	// Type of trigger (cpu, memory, custom, schedule)
	Type string `json:"type"`
	
	// Threshold defines the trigger threshold
	Threshold *ScalingThreshold `json:"threshold,omitempty"`
	
	// Schedule defines schedule-based triggers
	Schedule *ScheduleTrigger `json:"schedule,omitempty"`
}

// ScalingThreshold defines threshold conditions
type ScalingThreshold struct {
	// Metric name
	Metric string `json:"metric"`
	
	// Target value
	TargetValue string `json:"targetValue"`
	
	// Comparison operator (>, <, >=, <=, ==)
	Operator string `json:"operator"`
}

// ScheduleTrigger defines schedule-based scaling
type ScheduleTrigger struct {
	// Cron expression
	Cron string `json:"cron"`
	
	// Target replica count
	Replicas *int32 `json:"replicas,omitempty"`
}

// ScalingConstraints defines scaling boundaries and limits
type ScalingConstraints struct {
	// Minimum replica count
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	
	// Maximum replica count
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
	
	// Maximum scale up rate
	MaxScaleUp *int32 `json:"maxScaleUp,omitempty"`
	
	// Maximum scale down rate
	MaxScaleDown *int32 `json:"maxScaleDown,omitempty"`
}

// ScalingBehavior defines advanced scaling behavior
type ScalingBehavior struct {
	// Scale up behavior
	ScaleUp *ScalingPolicyBehavior `json:"scaleUp,omitempty"`
	
	// Scale down behavior
	ScaleDown *ScalingPolicyBehavior `json:"scaleDown,omitempty"`
}

// ScalingPolicyBehavior defines behavior parameters
type ScalingPolicyBehavior struct {
	// Stabilization window duration
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`
	
	// Policy selection strategy
	SelectPolicy *string `json:"selectPolicy,omitempty"`
	
	// Policies list
	Policies []ScalingPolicy `json:"policies,omitempty"`
}

// ScalingPolicy defines a scaling policy rule
type ScalingPolicy struct {
	// Type of policy (Percent, Pods)
	Type string `json:"type"`
	
	// Value of the policy
	Value int32 `json:"value"`
	
	// Period over which policy is evaluated
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`
}

// Manager manages scaling policies and their lifecycle
type Manager struct {
	kcpClusterClient kcpclientset.ClusterInterface
	
	// Policy cache and indexing
	policyCache   map[string]*ScalingPolicySpec
	cacheMutex    sync.RWMutex
	
	// Validation and lifecycle
	validator     *PolicyValidator
	
	// Workqueue for policy updates
	queue workqueue.RateLimitingInterface
	
	// Feature gate check
	featureGateEnabled bool
}

// PolicyValidator handles policy validation
type PolicyValidator struct {
	timeout time.Duration
}

// NewManager creates a new scaling policy manager
func NewManager(
	kcpClusterClient kcpclientset.ClusterInterface,
	informerFactory kcpinformers.SharedInformerFactory,
) (*Manager, error) {
	logger := klog.Background().WithValues("controller", ControllerName)
	
	// Check if TMC scaling feature is enabled
	featureEnabled := features.DefaultFeatureGate.Enabled(features.TMCScaling)
	if !featureEnabled {
		logger.V(2).Info("TMC Scaling feature is disabled, policy manager will be inactive")
	}
	
	manager := &Manager{
		kcpClusterClient:   kcpClusterClient,
		policyCache:        make(map[string]*ScalingPolicySpec),
		validator:          &PolicyValidator{timeout: defaultValidationTimeout},
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		featureGateEnabled: featureEnabled,
	}
	
	logger.V(2).Info("Scaling policy manager initialized", "featureEnabled", featureEnabled)
	return manager, nil
}

// Start starts the policy manager
func (m *Manager) Start(ctx context.Context, workers int) error {
	logger := logging.WithObject(klog.FromContext(ctx), nil).WithValues("controller", ControllerName)
	
	if !m.featureGateEnabled {
		logger.V(2).Info("TMC Scaling feature disabled, policy manager not starting")
		<-ctx.Done()
		return nil
	}
	
	defer runtime.HandleCrash()
	defer m.queue.ShutDown()
	
	logger.Info("Starting policy manager", "workers", workers)
	
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, m.runWorker, time.Second)
	}
	
	logger.Info("Policy manager started")
	<-ctx.Done()
	logger.Info("Policy manager shutting down")
	
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (m *Manager) runWorker(ctx context.Context) {
	for m.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (m *Manager) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := m.queue.Get()
	if shutdown {
		return false
	}
	
	defer m.queue.Done(obj)
	
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		m.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return true
	}
	
	if err := m.syncHandler(ctx, key); err == nil {
		m.queue.Forget(obj)
	} else {
		runtime.HandleError(fmt.Errorf("error syncing policy %q: %v", key, err))
		m.queue.AddRateLimited(key)
	}
	
	return true
}

// syncHandler processes a single policy update
func (m *Manager) syncHandler(ctx context.Context, key string) error {
	logger := logging.WithObject(klog.FromContext(ctx), nil).WithValues("controller", ControllerName, "key", key)
	
	logger.V(4).Info("Syncing scaling policy")
	
	// Extract logical cluster and policy name from key
	clusterName, policyName, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		return err
	}
	
	logger = logger.WithValues("cluster", clusterName, "policy", policyName)
	
	// Get policy from cache
	m.cacheMutex.RLock()
	policy, exists := m.policyCache[key]
	m.cacheMutex.RUnlock()
	
	if !exists {
		logger.V(4).Info("Policy not found in cache, may have been deleted")
		return nil
	}
	
	// Validate policy
	if err := m.validator.ValidatePolicy(ctx, policy); err != nil {
		logger.Error(err, "Policy validation failed")
		return err
	}
	
	logger.V(4).Info("Policy sync completed successfully")
	return nil
}

// AddPolicy adds a policy to the manager
func (m *Manager) AddPolicy(clusterName logicalcluster.Name, policyName string, spec *ScalingPolicySpec) error {
	if !m.featureGateEnabled {
		return fmt.Errorf("TMC scaling feature is disabled")
	}
	
	key := kcpcache.ToClusterAwareKey(clusterName.String(), "", policyName)
	
	// Validate policy before adding
	if err := m.validator.ValidatePolicy(context.Background(), spec); err != nil {
		return fmt.Errorf("policy validation failed: %w", err)
	}
	
	m.cacheMutex.Lock()
	m.policyCache[key] = spec
	m.cacheMutex.Unlock()
	
	// Enqueue for processing
	m.queue.Add(key)
	
	klog.V(4).InfoS("Policy added", "cluster", clusterName, "policy", policyName)
	return nil
}

// RemovePolicy removes a policy from the manager
func (m *Manager) RemovePolicy(clusterName logicalcluster.Name, policyName string) {
	key := kcpcache.ToClusterAwareKey(clusterName.String(), "", policyName)
	
	m.cacheMutex.Lock()
	delete(m.policyCache, key)
	m.cacheMutex.Unlock()
	
	klog.V(4).InfoS("Policy removed", "cluster", clusterName, "policy", policyName)
}

// GetPolicy retrieves a policy by cluster and name
func (m *Manager) GetPolicy(clusterName logicalcluster.Name, policyName string) (*ScalingPolicySpec, bool) {
	key := kcpcache.ToClusterAwareKey(clusterName.String(), "", policyName)
	
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()
	
	policy, exists := m.policyCache[key]
	return policy, exists
}

// ListPolicies lists all policies for a given cluster
func (m *Manager) ListPolicies(clusterName logicalcluster.Name) []*ScalingPolicySpec {
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()
	
	var policies []*ScalingPolicySpec
	clusterPrefix := clusterName.String() + ":"
	
	for key, policy := range m.policyCache {
		if key != "" && len(key) > len(clusterPrefix) && key[:len(clusterPrefix)] == clusterPrefix {
			policies = append(policies, policy)
		}
	}
	
	return policies
}

// ValidatePolicy validates a scaling policy specification
func (v *PolicyValidator) ValidatePolicy(ctx context.Context, spec *ScalingPolicySpec) error {
	// Validate target
	if err := v.validateTarget(&spec.Target); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}
	
	// Validate triggers
	if len(spec.Triggers) == 0 {
		return fmt.Errorf("at least one trigger must be specified")
	}
	
	for i, trigger := range spec.Triggers {
		if err := v.validateTrigger(&trigger); err != nil {
			return fmt.Errorf("invalid trigger %d: %w", i, err)
		}
	}
	
	// Validate constraints
	if err := v.validateConstraints(&spec.Constraints); err != nil {
		return fmt.Errorf("invalid constraints: %w", err)
	}
	
	// Validate behavior if specified
	if spec.Behavior != nil {
		if err := v.validateBehavior(spec.Behavior); err != nil {
			return fmt.Errorf("invalid behavior: %w", err)
		}
	}
	
	return nil
}

// validateTarget validates a scaling target
func (v *PolicyValidator) validateTarget(target *ScalingTarget) error {
	if target.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if target.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if target.Name == "" {
		return fmt.Errorf("name is required")
	}
	
	// TODO: Add more sophisticated target validation
	// - Check if the target resource type supports scaling
	// - Validate that the target exists (if needed)
	
	return nil
}

// validateTrigger validates a scaling trigger
func (v *PolicyValidator) validateTrigger(trigger *ScalingTrigger) error {
	supportedTypes := []string{"cpu", "memory", "custom", "schedule"}
	validType := false
	for _, t := range supportedTypes {
		if trigger.Type == t {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("unsupported trigger type: %s", trigger.Type)
	}
	
	// Validate threshold for metric-based triggers
	if trigger.Type != "schedule" {
		if trigger.Threshold == nil {
			return fmt.Errorf("threshold is required for metric-based triggers")
		}
		if err := v.validateThreshold(trigger.Threshold); err != nil {
			return err
		}
	}
	
	// Validate schedule for schedule-based triggers
	if trigger.Type == "schedule" {
		if trigger.Schedule == nil {
			return fmt.Errorf("schedule is required for schedule triggers")
		}
		if err := v.validateSchedule(trigger.Schedule); err != nil {
			return err
		}
	}
	
	return nil
}

// validateThreshold validates a scaling threshold
func (v *PolicyValidator) validateThreshold(threshold *ScalingThreshold) error {
	if threshold.Metric == "" {
		return fmt.Errorf("metric is required")
	}
	if threshold.TargetValue == "" {
		return fmt.Errorf("targetValue is required")
	}
	
	validOperators := []string{">", "<", ">=", "<=", "=="}
	validOperator := false
	for _, op := range validOperators {
		if threshold.Operator == op {
			validOperator = true
			break
		}
	}
	if !validOperator {
		return fmt.Errorf("invalid operator: %s", threshold.Operator)
	}
	
	return nil
}

// validateSchedule validates a schedule trigger
func (v *PolicyValidator) validateSchedule(schedule *ScheduleTrigger) error {
	if schedule.Cron == "" {
		return fmt.Errorf("cron expression is required")
	}
	
	// TODO: Add cron expression parsing and validation
	
	if schedule.Replicas != nil && *schedule.Replicas < 0 {
		return fmt.Errorf("replicas cannot be negative")
	}
	
	return nil
}

// validateConstraints validates scaling constraints
func (v *PolicyValidator) validateConstraints(constraints *ScalingConstraints) error {
	if constraints.MinReplicas != nil && *constraints.MinReplicas < 0 {
		return fmt.Errorf("minReplicas cannot be negative")
	}
	
	if constraints.MaxReplicas != nil && *constraints.MaxReplicas < 0 {
		return fmt.Errorf("maxReplicas cannot be negative")
	}
	
	if constraints.MinReplicas != nil && constraints.MaxReplicas != nil {
		if *constraints.MinReplicas > *constraints.MaxReplicas {
			return fmt.Errorf("minReplicas cannot be greater than maxReplicas")
		}
	}
	
	if constraints.MaxScaleUp != nil && *constraints.MaxScaleUp < 0 {
		return fmt.Errorf("maxScaleUp cannot be negative")
	}
	
	if constraints.MaxScaleDown != nil && *constraints.MaxScaleDown < 0 {
		return fmt.Errorf("maxScaleDown cannot be negative")
	}
	
	return nil
}

// validateBehavior validates scaling behavior configuration
func (v *PolicyValidator) validateBehavior(behavior *ScalingBehavior) error {
	if behavior.ScaleUp != nil {
		if err := v.validatePolicyBehavior(behavior.ScaleUp); err != nil {
			return fmt.Errorf("invalid scaleUp behavior: %w", err)
		}
	}
	
	if behavior.ScaleDown != nil {
		if err := v.validatePolicyBehavior(behavior.ScaleDown); err != nil {
			return fmt.Errorf("invalid scaleDown behavior: %w", err)
		}
	}
	
	return nil
}

// validatePolicyBehavior validates policy behavior parameters
func (v *PolicyValidator) validatePolicyBehavior(behavior *ScalingPolicyBehavior) error {
	if behavior.StabilizationWindowSeconds != nil && *behavior.StabilizationWindowSeconds < 0 {
		return fmt.Errorf("stabilizationWindowSeconds cannot be negative")
	}
	
	if behavior.SelectPolicy != nil {
		validPolicies := []string{"Min", "Max", "Disabled"}
		valid := false
		for _, p := range validPolicies {
			if *behavior.SelectPolicy == p {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid selectPolicy: %s", *behavior.SelectPolicy)
		}
	}
	
	for i, policy := range behavior.Policies {
		if err := v.validateScalingPolicy(&policy); err != nil {
			return fmt.Errorf("invalid policy %d: %w", i, err)
		}
	}
	
	return nil
}

// validateScalingPolicy validates individual scaling policy rules
func (v *PolicyValidator) validateScalingPolicy(policy *ScalingPolicy) error {
	if policy.Type != "Percent" && policy.Type != "Pods" {
		return fmt.Errorf("invalid policy type: %s", policy.Type)
	}
	
	if policy.Value <= 0 {
		return fmt.Errorf("policy value must be positive")
	}
	
	if policy.PeriodSeconds != nil && *policy.PeriodSeconds <= 0 {
		return fmt.Errorf("periodSeconds must be positive")
	}
	
	return nil
}