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

/*
Package workspace provides interfaces and types for virtual workspace management in KCP.

This package defines the core abstractions for creating and managing virtual workspaces that
provide isolated views of cluster resources. Virtual workspaces enable fine-grained access
control and resource isolation while maintaining efficient operations across logical clusters.

Key Components:

WorkspaceProvider - Core interface for creating and managing virtual workspace instances.
Implementations handle workspace lifecycle, authentication integration, and resource projection.

WorkspaceCache - Caching layer interface for virtual workspace metadata and state.
Provides performance optimization for frequent workspace operations and lookups.

Core Design Principles:

1. Workspace Isolation: Each virtual workspace maintains strict boundaries between different
   users, organizations, or logical resource groups.

2. KCP Integration: Seamless integration with KCP's logical cluster architecture,
   workspace-aware clients, and multi-tenancy patterns.

3. Performance: Optimized for high-frequency operations through intelligent caching
   and efficient resource projection techniques.

4. Extensibility: Interface-driven design allows for multiple implementations
   and deployment-specific customizations.

Usage Example:

	provider := NewWorkspaceProvider(config)
	workspace, err := provider.GetWorkspace(ctx, workspaceRef)
	if err != nil {
		return fmt.Errorf("failed to access workspace: %w", err)
	}
	
	// Use workspace for resource operations
	client := workspace.Client()
	objects, err := client.List(ctx, &corev1.PodList{})

This package follows KCP's architectural patterns for workspace management and integrates
with the broader virtual workspace ecosystem for comprehensive resource virtualization.
*/
package workspace