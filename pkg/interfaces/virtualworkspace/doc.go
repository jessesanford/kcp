/*
Package virtualworkspace defines interfaces for implementing virtual workspaces
that project APIs from physical clusters into KCP workspaces.

Virtual workspaces provide a unified API surface across multiple locations,
enabling transparent access to resources regardless of their physical location.

Core Interfaces:

- VirtualWorkspace: Main interface for virtual workspace implementation
- APIProjector: Projects APIs into virtual workspace
- RequestRouter: Routes requests to appropriate locations
- ResponseAggregator: Aggregates responses from multiple locations
- Authenticator/Authorizer: Handles authentication and authorization

Architecture:

Virtual workspaces act as API gateways that:
1. Project APIs from discovered resources at locations
2. Route incoming requests to appropriate physical clusters
3. Aggregate responses from multiple locations
4. Handle authentication and authorization

Usage Example:

	import (
	    "github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace"
	    "github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace/projection"
	)

	// Create virtual workspace
	vw := virtualworkspace.New(config)

	// Register locations
	vw.RegisterLocation(locationInfo)

	// Start serving
	vw.Start(ctx)

For more information, see the TMC virtual workspace documentation.
*/
package virtualworkspace