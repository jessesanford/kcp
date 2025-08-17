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

package cel

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/kcp-dev/logicalcluster/v3"
)

// PlacementContextBuilder provides a fluent interface for building placement contexts.
type PlacementContextBuilder struct {
	context *PlacementContext
}

// NewPlacementContextBuilder creates a new placement context builder.
func NewPlacementContextBuilder() *PlacementContextBuilder {
	return &PlacementContextBuilder{
		context: &PlacementContext{
			Variables: make(map[string]interface{}),
		},
	}
}

// WithWorkspace adds workspace context to the placement context.
func (b *PlacementContextBuilder) WithWorkspace(workspace *WorkspaceContext) *PlacementContextBuilder {
	b.context.Workspace = workspace
	return b
}

// WithRequest adds request context to the placement context.
func (b *PlacementContextBuilder) WithRequest(request *RequestContext) *PlacementContextBuilder {
	b.context.Request = request
	return b
}

// WithResources adds resource context to the placement context.
func (b *PlacementContextBuilder) WithResources(resources *ResourceContext) *PlacementContextBuilder {
	b.context.Resources = resources
	return b
}

// WithVariable adds a custom variable to the placement context.
func (b *PlacementContextBuilder) WithVariable(key string, value interface{}) *PlacementContextBuilder {
	if b.context.Variables == nil {
		b.context.Variables = make(map[string]interface{})
	}
	b.context.Variables[key] = value
	return b
}

// WithVariables adds multiple custom variables to the placement context.
func (b *PlacementContextBuilder) WithVariables(vars map[string]interface{}) *PlacementContextBuilder {
	if b.context.Variables == nil {
		b.context.Variables = make(map[string]interface{})
	}
	for k, v := range vars {
		b.context.Variables[k] = v
	}
	return b
}

// Build returns the constructed placement context.
func (b *PlacementContextBuilder) Build() *PlacementContext {
	return b.context
}

// WorkspaceContextBuilder provides a fluent interface for building workspace contexts.
type WorkspaceContextBuilder struct {
	context *WorkspaceContext
}

// NewWorkspaceContextBuilder creates a new workspace context builder.
func NewWorkspaceContextBuilder(name logicalcluster.Name) *WorkspaceContextBuilder {
	return &WorkspaceContextBuilder{
		context: &WorkspaceContext{
			Name:          name,
			Labels:        make(map[string]string),
			Annotations:   make(map[string]string),
			LastHeartbeat: time.Now(),
		},
	}
}

// WithLabels adds labels to the workspace context.
func (b *WorkspaceContextBuilder) WithLabels(labels map[string]string) *WorkspaceContextBuilder {
	if b.context.Labels == nil {
		b.context.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.context.Labels[k] = v
	}
	return b
}

// WithLabel adds a single label to the workspace context.
func (b *WorkspaceContextBuilder) WithLabel(key, value string) *WorkspaceContextBuilder {
	if b.context.Labels == nil {
		b.context.Labels = make(map[string]string)
	}
	b.context.Labels[key] = value
	return b
}

// WithAnnotations adds annotations to the workspace context.
func (b *WorkspaceContextBuilder) WithAnnotations(annotations map[string]string) *WorkspaceContextBuilder {
	if b.context.Annotations == nil {
		b.context.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		b.context.Annotations[k] = v
	}
	return b
}

// WithReady sets the ready status of the workspace.
func (b *WorkspaceContextBuilder) WithReady(ready bool) *WorkspaceContextBuilder {
	b.context.Ready = ready
	return b
}

// WithRegion sets the region of the workspace.
func (b *WorkspaceContextBuilder) WithRegion(region string) *WorkspaceContextBuilder {
	b.context.Region = region
	return b
}

// WithZone sets the zone of the workspace.
func (b *WorkspaceContextBuilder) WithZone(zone string) *WorkspaceContextBuilder {
	b.context.Zone = zone
	return b
}

// WithLastHeartbeat sets the last heartbeat time.
func (b *WorkspaceContextBuilder) WithLastHeartbeat(heartbeat time.Time) *WorkspaceContextBuilder {
	b.context.LastHeartbeat = heartbeat
	return b
}

// Build returns the constructed workspace context.
func (b *WorkspaceContextBuilder) Build() *WorkspaceContext {
	return b.context
}

// RequestContextBuilder provides a fluent interface for building request contexts.
type RequestContextBuilder struct {
	context *RequestContext
}

// NewRequestContextBuilder creates a new request context builder.
func NewRequestContextBuilder(name, namespace string) *RequestContextBuilder {
	return &RequestContextBuilder{
		context: &RequestContext{
			Name:      name,
			Namespace: namespace,
			Labels:    make(map[string]string),
			CreatedAt: time.Now(),
		},
	}
}

// WithSourceWorkspace sets the source workspace for the request.
func (b *RequestContextBuilder) WithSourceWorkspace(workspace logicalcluster.Name) *RequestContextBuilder {
	b.context.SourceWorkspace = workspace
	return b
}

// WithLabels adds labels to the request context.
func (b *RequestContextBuilder) WithLabels(labels map[string]string) *RequestContextBuilder {
	if b.context.Labels == nil {
		b.context.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.context.Labels[k] = v
	}
	return b
}

// WithRequirements sets the resource requirements for the request.
func (b *RequestContextBuilder) WithRequirements(requirements *ResourceRequirements) *RequestContextBuilder {
	b.context.Requirements = requirements
	return b
}

// WithPriority sets the priority for the request.
func (b *RequestContextBuilder) WithPriority(priority int32) *RequestContextBuilder {
	b.context.Priority = priority
	return b
}

// WithCreatedAt sets the creation time for the request.
func (b *RequestContextBuilder) WithCreatedAt(createdAt time.Time) *RequestContextBuilder {
	b.context.CreatedAt = createdAt
	return b
}

// Build returns the constructed request context.
func (b *RequestContextBuilder) Build() *RequestContext {
	return b.context
}

// ResourceContextBuilder provides a fluent interface for building resource contexts.
type ResourceContextBuilder struct {
	context *ResourceContext
}

// NewResourceContextBuilder creates a new resource context builder.
func NewResourceContextBuilder() *ResourceContextBuilder {
	return &ResourceContextBuilder{
		context: &ResourceContext{},
	}
}

// WithTotalCapacity sets the total resource capacity.
func (b *ResourceContextBuilder) WithTotalCapacity(capacity *ResourceCapacity) *ResourceContextBuilder {
	b.context.TotalCapacity = capacity
	return b
}

// WithAvailableCapacity sets the available resource capacity.
func (b *ResourceContextBuilder) WithAvailableCapacity(capacity *ResourceCapacity) *ResourceContextBuilder {
	b.context.AvailableCapacity = capacity
	return b
}

// WithCurrentUtilization sets the current resource utilization.
func (b *ResourceContextBuilder) WithCurrentUtilization(utilization *ResourceUtilization) *ResourceContextBuilder {
	b.context.CurrentUtilization = utilization
	return b
}

// WithReservedResources sets the reserved resources.
func (b *ResourceContextBuilder) WithReservedResources(reserved *ResourceCapacity) *ResourceContextBuilder {
	b.context.ReservedResources = reserved
	return b
}

// Build returns the constructed resource context.
func (b *ResourceContextBuilder) Build() *ResourceContext {
	return b.context
}

// ResourceCapacityBuilder provides a fluent interface for building resource capacity.
type ResourceCapacityBuilder struct {
	capacity *ResourceCapacity
}

// NewResourceCapacityBuilder creates a new resource capacity builder.
func NewResourceCapacityBuilder() *ResourceCapacityBuilder {
	return &ResourceCapacityBuilder{
		capacity: &ResourceCapacity{
			CustomResources: make(map[string]resource.Quantity),
			LastUpdated:     time.Now(),
		},
	}
}

// WithCPU sets the CPU capacity.
func (b *ResourceCapacityBuilder) WithCPU(cpu resource.Quantity) *ResourceCapacityBuilder {
	b.capacity.CPU = cpu
	return b
}

// WithMemory sets the memory capacity.
func (b *ResourceCapacityBuilder) WithMemory(memory resource.Quantity) *ResourceCapacityBuilder {
	b.capacity.Memory = memory
	return b
}

// WithStorage sets the storage capacity.
func (b *ResourceCapacityBuilder) WithStorage(storage resource.Quantity) *ResourceCapacityBuilder {
	b.capacity.Storage = storage
	return b
}

// WithCustomResource adds a custom resource capacity.
func (b *ResourceCapacityBuilder) WithCustomResource(name string, quantity resource.Quantity) *ResourceCapacityBuilder {
	if b.capacity.CustomResources == nil {
		b.capacity.CustomResources = make(map[string]resource.Quantity)
	}
	b.capacity.CustomResources[name] = quantity
	return b
}

// WithLastUpdated sets the last updated time.
func (b *ResourceCapacityBuilder) WithLastUpdated(lastUpdated time.Time) *ResourceCapacityBuilder {
	b.capacity.LastUpdated = lastUpdated
	return b
}

// Build returns the constructed resource capacity.
func (b *ResourceCapacityBuilder) Build() *ResourceCapacity {
	return b.capacity
}

// ResourceRequirementsBuilder provides a fluent interface for building resource requirements.
type ResourceRequirementsBuilder struct {
	requirements *ResourceRequirements
}

// NewResourceRequirementsBuilder creates a new resource requirements builder.
func NewResourceRequirementsBuilder() *ResourceRequirementsBuilder {
	return &ResourceRequirementsBuilder{
		requirements: &ResourceRequirements{
			CustomResources: make(map[string]resource.Quantity),
		},
	}
}

// WithCPU sets the CPU requirement.
func (b *ResourceRequirementsBuilder) WithCPU(cpu resource.Quantity) *ResourceRequirementsBuilder {
	b.requirements.CPU = cpu
	return b
}

// WithMemory sets the memory requirement.
func (b *ResourceRequirementsBuilder) WithMemory(memory resource.Quantity) *ResourceRequirementsBuilder {
	b.requirements.Memory = memory
	return b
}

// WithStorage sets the storage requirement.
func (b *ResourceRequirementsBuilder) WithStorage(storage resource.Quantity) *ResourceRequirementsBuilder {
	b.requirements.Storage = storage
	return b
}

// WithCustomResource adds a custom resource requirement.
func (b *ResourceRequirementsBuilder) WithCustomResource(name string, quantity resource.Quantity) *ResourceRequirementsBuilder {
	if b.requirements.CustomResources == nil {
		b.requirements.CustomResources = make(map[string]resource.Quantity)
	}
	b.requirements.CustomResources[name] = quantity
	return b
}

// Build returns the constructed resource requirements.
func (b *ResourceRequirementsBuilder) Build() *ResourceRequirements {
	return b.requirements
}

// ContextValidator provides validation for placement contexts.
type ContextValidator struct{}

// NewContextValidator creates a new context validator.
func NewContextValidator() *ContextValidator {
	return &ContextValidator{}
}

// ValidatePlacementContext validates a placement context.
func (v *ContextValidator) ValidatePlacementContext(ctx *PlacementContext) error {
	if ctx == nil {
		return fmt.Errorf("placement context cannot be nil")
	}

	if ctx.Workspace != nil {
		if err := v.validateWorkspaceContext(ctx.Workspace); err != nil {
			return fmt.Errorf("invalid workspace context: %w", err)
		}
	}

	if ctx.Request != nil {
		if err := v.validateRequestContext(ctx.Request); err != nil {
			return fmt.Errorf("invalid request context: %w", err)
		}
	}

	if ctx.Resources != nil {
		if err := v.validateResourceContext(ctx.Resources); err != nil {
			return fmt.Errorf("invalid resource context: %w", err)
		}
	}

	return nil
}

// validateWorkspaceContext validates a workspace context.
func (v *ContextValidator) validateWorkspaceContext(ctx *WorkspaceContext) error {
	if ctx.Name.Empty() {
		return fmt.Errorf("workspace name cannot be empty")
	}

	// Validate labels
	for key := range ctx.Labels {
		if key == "" {
			return fmt.Errorf("workspace label key cannot be empty")
		}
	}

	return nil
}

// validateRequestContext validates a request context.
func (v *ContextValidator) validateRequestContext(ctx *RequestContext) error {
	if ctx.Name == "" {
		return fmt.Errorf("request name cannot be empty")
	}

	if ctx.Namespace == "" {
		return fmt.Errorf("request namespace cannot be empty")
	}

	if ctx.Priority < 0 {
		return fmt.Errorf("request priority cannot be negative")
	}

	return nil
}

// validateResourceContext validates a resource context.
func (v *ContextValidator) validateResourceContext(ctx *ResourceContext) error {
	if ctx.TotalCapacity != nil && ctx.AvailableCapacity != nil {
		// Available capacity should not exceed total capacity
		if ctx.AvailableCapacity.CPU.Cmp(ctx.TotalCapacity.CPU) > 0 {
			return fmt.Errorf("available CPU capacity exceeds total capacity")
		}
		if ctx.AvailableCapacity.Memory.Cmp(ctx.TotalCapacity.Memory) > 0 {
			return fmt.Errorf("available memory capacity exceeds total capacity")
		}
		if ctx.AvailableCapacity.Storage.Cmp(ctx.TotalCapacity.Storage) > 0 {
			return fmt.Errorf("available storage capacity exceeds total capacity")
		}
	}

	return nil
}

// ContextConversions provides utilities for converting between different context types.
type ContextConversions struct{}

// ToMap converts a placement context to a map suitable for CEL evaluation.
func (c *ContextConversions) ToMap(ctx *PlacementContext) map[string]interface{} {
	result := make(map[string]interface{})

	if ctx.Workspace != nil {
		result["workspace"] = c.workspaceToMap(ctx.Workspace)
	}

	if ctx.Request != nil {
		result["request"] = c.requestToMap(ctx.Request)
	}

	if ctx.Resources != nil {
		result["resources"] = c.resourcesToMap(ctx.Resources)
	}

	// Add custom variables
	for k, v := range ctx.Variables {
		result[k] = v
	}

	return result
}

// workspaceToMap converts a workspace context to a map.
func (c *ContextConversions) workspaceToMap(ctx *WorkspaceContext) map[string]interface{} {
	return map[string]interface{}{
		"name":          ctx.Name.String(),
		"labels":        ctx.Labels,
		"annotations":   ctx.Annotations,
		"ready":         ctx.Ready,
		"lastHeartbeat": ctx.LastHeartbeat.Unix(),
		"region":        ctx.Region,
		"zone":          ctx.Zone,
	}
}

// requestToMap converts a request context to a map.
func (c *ContextConversions) requestToMap(ctx *RequestContext) map[string]interface{} {
	result := map[string]interface{}{
		"name":            ctx.Name,
		"namespace":       ctx.Namespace,
		"sourceWorkspace": ctx.SourceWorkspace.String(),
		"labels":          ctx.Labels,
		"priority":        ctx.Priority,
		"createdAt":       ctx.CreatedAt.Unix(),
	}

	if ctx.Requirements != nil {
		result["requirements"] = c.requirementsToMap(ctx.Requirements)
	}

	return result
}

// resourcesToMap converts a resource context to a map.
func (c *ContextConversions) resourcesToMap(ctx *ResourceContext) map[string]interface{} {
	result := make(map[string]interface{})

	if ctx.TotalCapacity != nil {
		result["totalCapacity"] = c.capacityToMap(ctx.TotalCapacity)
	}

	if ctx.AvailableCapacity != nil {
		result["availableCapacity"] = c.capacityToMap(ctx.AvailableCapacity)
	}

	if ctx.CurrentUtilization != nil {
		result["currentUtilization"] = c.utilizationToMap(ctx.CurrentUtilization)
	}

	return result
}

// capacityToMap converts resource capacity to a map.
func (c *ContextConversions) capacityToMap(capacity *ResourceCapacity) map[string]interface{} {
	result := map[string]interface{}{
		"cpu":         resourceQuantityToFloat64(capacity.CPU),
		"memory":      resourceQuantityToFloat64(capacity.Memory),
		"storage":     resourceQuantityToFloat64(capacity.Storage),
		"lastUpdated": capacity.LastUpdated.Unix(),
	}

	if len(capacity.CustomResources) > 0 {
		custom := make(map[string]float64)
		for name, quantity := range capacity.CustomResources {
			custom[name] = resourceQuantityToFloat64(quantity)
		}
		result["custom"] = custom
	}

	return result
}

// requirementsToMap converts resource requirements to a map.
func (c *ContextConversions) requirementsToMap(requirements *ResourceRequirements) map[string]interface{} {
	result := map[string]interface{}{
		"cpu":     resourceQuantityToFloat64(requirements.CPU),
		"memory":  resourceQuantityToFloat64(requirements.Memory),
		"storage": resourceQuantityToFloat64(requirements.Storage),
	}

	if len(requirements.CustomResources) > 0 {
		custom := make(map[string]float64)
		for name, quantity := range requirements.CustomResources {
			custom[name] = resourceQuantityToFloat64(quantity)
		}
		result["custom"] = custom
	}

	return result
}

// utilizationToMap converts resource utilization to a map.
func (c *ContextConversions) utilizationToMap(utilization *ResourceUtilization) map[string]interface{} {
	result := map[string]interface{}{
		"cpu":     resourceQuantityToFloat64(utilization.CPU),
		"memory":  resourceQuantityToFloat64(utilization.Memory),
		"storage": resourceQuantityToFloat64(utilization.Storage),
	}

	if len(utilization.CustomResources) > 0 {
		custom := make(map[string]float64)
		for name, quantity := range utilization.CustomResources {
			custom[name] = resourceQuantityToFloat64(quantity)
		}
		result["custom"] = custom
	}

	return result
}