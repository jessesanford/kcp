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

package interfaces

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// ConnectionID uniquely identifies a tunnel connection
type ConnectionID string

// ConnectionPoolConfig configures connection pool behavior
type ConnectionPoolConfig struct {
	// MaxConnections limits the total number of concurrent connections
	MaxConnections int
	
	// MaxIdleConnections limits idle connections in the pool
	MaxIdleConnections int
	
	// MaxConnectionAge sets maximum lifetime for a connection
	MaxConnectionAge time.Duration
	
	// IdleTimeout sets timeout for idle connections before removal
	IdleTimeout time.Duration
	
	// HealthCheckInterval specifies how often to check connection health
	HealthCheckInterval time.Duration
	
	// EnableMetrics controls whether to collect detailed metrics
	EnableMetrics bool
}

// ConnectionInfo provides details about a managed connection
type ConnectionInfo struct {
	// ID uniquely identifies this connection
	ID ConnectionID
	
	// Workspace associates the connection with a logical cluster
	Workspace logicalcluster.Name
	
	// Protocol indicates the tunneling protocol in use
	Protocol TunnelProtocol
	
	// State shows current connection state
	State TunnelState
	
	// CreatedAt tracks when connection was created
	CreatedAt time.Time
	
	// LastUsedAt tracks most recent activity
	LastUsedAt time.Time
	
	// RemoteAddr contains the remote endpoint address
	RemoteAddr string
	
	// Stats provides usage statistics for this connection
	Stats TunnelStats
}

// PoolStats provides statistics about the connection pool
type PoolStats struct {
	// ActiveConnections counts currently active connections
	ActiveConnections int
	
	// IdleConnections counts connections in idle state
	IdleConnections int
	
	// TotalConnections is the total number of managed connections
	TotalConnections int
	
	// ConnectionsCreated tracks total connections created since startup
	ConnectionsCreated uint64
	
	// ConnectionsClosed tracks total connections closed since startup
	ConnectionsClosed uint64
	
	// FailedConnections tracks connection failures since startup
	FailedConnections uint64
	
	// PoolHits tracks successful connection reuse from pool
	PoolHits uint64
	
	// PoolMisses tracks requests that required new connections
	PoolMisses uint64
}

// ConnectionFilter allows filtering connections by various criteria
type ConnectionFilter struct {
	// Workspace filters by logical cluster (empty matches all)
	Workspace logicalcluster.Name
	
	// Protocol filters by tunnel protocol (empty matches all)
	Protocol TunnelProtocol
	
	// State filters by connection state (empty matches all)
	State TunnelState
	
	// MaxAge filters connections older than specified duration
	MaxAge time.Duration
	
	// MinIdleTime filters connections idle for specified duration
	MinIdleTime time.Duration
}

// ConnectionManager handles the lifecycle of tunnel connections including
// connection pooling, load balancing, health monitoring, and failover.
//
// The manager provides workspace-aware connection management with support
// for multiple protocols and automatic cleanup of stale connections.
// All operations are thread-safe for concurrent access.
type ConnectionManager interface {
	// GetConnection retrieves or creates a connection for the specified workspace
	// and protocol. Returns an existing healthy connection from the pool if
	// available, otherwise creates a new connection.
	//
	// The connection is automatically marked as in-use and will not be
	// considered for idle cleanup until returned to the pool.
	GetConnection(ctx context.Context, workspace logicalcluster.Name, protocol TunnelProtocol) (Tunnel, error)
	
	// ReturnConnection returns a connection to the pool for potential reuse.
	// The connection should be returned when no longer actively used.
	//
	// If the connection is unhealthy or the pool is full, it will be closed.
	// This method is idempotent - returning the same connection multiple
	// times is safe but has no additional effect.
	ReturnConnection(connection Tunnel) error
	
	// RemoveConnection permanently removes a connection from management.
	// The connection will be closed and cannot be returned to the pool.
	//
	// This should be called when a connection is known to be unusable
	// or when implementing custom connection lifecycle policies.
	RemoveConnection(connectionID ConnectionID) error
	
	// ListConnections returns information about currently managed connections.
	// Connections can be filtered using the provided filter criteria.
	//
	// Returns a snapshot of connection information at call time.
	ListConnections(filter ConnectionFilter) ([]ConnectionInfo, error)
	
	// GetConnectionInfo returns detailed information about a specific connection.
	// Returns ErrConnectionNotFound if the connection is not managed by this pool.
	GetConnectionInfo(connectionID ConnectionID) (ConnectionInfo, error)
	
	// HealthCheck performs health checks on all managed connections.
	// Unhealthy connections are automatically removed from the pool.
	//
	// Returns the number of connections that failed health checks.
	HealthCheck(ctx context.Context) (int, error)
	
	// Cleanup removes idle connections and connections that exceed maximum age.
	// This is typically called periodically by a background goroutine.
	//
	// Returns the number of connections that were cleaned up.
	Cleanup() (int, error)
	
	// Stats returns current statistics about the connection pool
	Stats() PoolStats
	
	// SetConfig updates the connection pool configuration.
	// Changes take effect for new connections and the next cleanup cycle.
	//
	// Returns an error if the configuration is invalid.
	SetConfig(config ConnectionPoolConfig) error
	
	// GetConfig returns the current connection pool configuration
	GetConfig() ConnectionPoolConfig
	
	// Close gracefully shuts down the connection manager.
	// All managed connections are closed and resources are freed.
	//
	// The manager cannot be used after calling Close.
	Close() error
}

// ConnectionEventType represents types of connection lifecycle events
type ConnectionEventType string

const (
	// ConnectionEventCreated fires when a new connection is created
	ConnectionEventCreated ConnectionEventType = "Created"
	
	// ConnectionEventConnected fires when connection establishment succeeds  
	ConnectionEventConnected ConnectionEventType = "Connected"
	
	// ConnectionEventDisconnected fires when connection is lost unexpectedly
	ConnectionEventDisconnected ConnectionEventType = "Disconnected"
	
	// ConnectionEventReconnecting fires when reconnection attempt starts
	ConnectionEventReconnecting ConnectionEventType = "Reconnecting"
	
	// ConnectionEventReconnected fires when reconnection succeeds
	ConnectionEventReconnected ConnectionEventType = "Reconnected"
	
	// ConnectionEventClosed fires when connection is permanently closed
	ConnectionEventClosed ConnectionEventType = "Closed"
	
	// ConnectionEventHealthCheckFailed fires when health check fails
	ConnectionEventHealthCheckFailed ConnectionEventType = "HealthCheckFailed"
	
	// ConnectionEventReturnedToPool fires when connection returns to pool
	ConnectionEventReturnedToPool ConnectionEventType = "ReturnedToPool"
	
	// ConnectionEventRemovedFromPool fires when connection leaves pool
	ConnectionEventRemovedFromPool ConnectionEventType = "RemovedFromPool"
)

// ConnectionEvent represents a connection lifecycle event
type ConnectionEvent struct {
	// Type specifies the event type
	Type ConnectionEventType
	
	// ConnectionID identifies the connection
	ConnectionID ConnectionID
	
	// Workspace associates the event with a logical cluster
	Workspace logicalcluster.Name
	
	// Timestamp records when the event occurred
	Timestamp time.Time
	
	// Error contains error information for failure events
	Error error
	
	// Metadata contains additional event-specific information
	Metadata map[string]interface{}
}

// ConnectionEventHandler processes connection lifecycle events
type ConnectionEventHandler interface {
	// HandleConnectionEvent processes a connection event.
	// Handlers should not block as this may impact connection management performance.
	HandleConnectionEvent(event ConnectionEvent)
}

// ConnectionManagerFactory creates connection manager instances
type ConnectionManagerFactory interface {
	// CreateConnectionManager creates a new connection manager with the specified configuration.
	// The manager is ready to use immediately after creation.
	CreateConnectionManager(config ConnectionPoolConfig, factory TunnelFactory) (ConnectionManager, error)
	
	// AddEventHandler registers an event handler for connection lifecycle events.
	// Multiple handlers can be registered and will all receive events.
	AddEventHandler(handler ConnectionEventHandler)
	
	// RemoveEventHandler unregisters a previously added event handler
	RemoveEventHandler(handler ConnectionEventHandler)
}

// Common connection manager errors
var (
	// ErrConnectionNotFound indicates a connection ID was not found in the pool
	ErrConnectionNotFound = fmt.Errorf("connection not found")
	
	// ErrPoolFull indicates the connection pool has reached capacity
	ErrPoolFull = fmt.Errorf("connection pool is full")
	
	// ErrConnectionUnhealthy indicates a connection failed health checks
	ErrConnectionUnhealthy = fmt.Errorf("connection is unhealthy")
	
	// ErrManagerClosed indicates the connection manager has been closed
	ErrManagerClosed = fmt.Errorf("connection manager is closed")
	
	// ErrInvalidPoolConfig indicates the pool configuration is invalid
	ErrInvalidPoolConfig = fmt.Errorf("invalid connection pool configuration")
)