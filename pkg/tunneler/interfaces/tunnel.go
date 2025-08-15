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
	"io"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TunnelProtocol defines the supported tunnel protocols
type TunnelProtocol string

const (
	// TunnelProtocolWebSocket represents WebSocket-based tunneling
	TunnelProtocolWebSocket TunnelProtocol = "websocket"
	// TunnelProtocolGRPC represents gRPC-based tunneling  
	TunnelProtocolGRPC TunnelProtocol = "grpc"
	// TunnelProtocolHTTP represents HTTP-based tunneling
	TunnelProtocolHTTP TunnelProtocol = "http"
)

// TunnelState represents the current state of a tunnel
type TunnelState string

const (
	// TunnelStateConnecting indicates tunnel is establishing connection
	TunnelStateConnecting TunnelState = "Connecting"
	// TunnelStateConnected indicates tunnel is successfully connected
	TunnelStateConnected TunnelState = "Connected"
	// TunnelStateReconnecting indicates tunnel is attempting to reconnect
	TunnelStateReconnecting TunnelState = "Reconnecting"
	// TunnelStateDisconnected indicates tunnel is disconnected
	TunnelStateDisconnected TunnelState = "Disconnected"
	// TunnelStateClosed indicates tunnel is permanently closed
	TunnelStateClosed TunnelState = "Closed"
)

// TunnelOptions contains configuration options for creating a tunnel
type TunnelOptions struct {
	// Protocol specifies the tunneling protocol to use
	Protocol TunnelProtocol
	
	// Workspace identifies the logical cluster this tunnel operates in
	Workspace logicalcluster.Name
	
	// MaxReconnectAttempts limits automatic reconnection attempts
	MaxReconnectAttempts int
	
	// ReconnectInterval specifies delay between reconnection attempts
	ReconnectInterval time.Duration
	
	// ReadTimeout sets the read operation timeout
	ReadTimeout time.Duration
	
	// WriteTimeout sets the write operation timeout
	WriteTimeout time.Duration
	
	// BufferSize configures internal buffer sizes
	BufferSize int
	
	// EnableCompression enables data compression if supported by protocol
	EnableCompression bool
	
	// Headers contains protocol-specific headers (e.g., for HTTP/WebSocket)
	Headers map[string]string
}

// TunnelStats provides statistical information about tunnel usage
type TunnelStats struct {
	// BytesReceived tracks total bytes received through tunnel
	BytesReceived uint64
	
	// BytesSent tracks total bytes sent through tunnel  
	BytesSent uint64
	
	// MessagesReceived tracks total messages received
	MessagesReceived uint64
	
	// MessagesSent tracks total messages sent
	MessagesSent uint64
	
	// ConnectionCount tracks total connection attempts
	ConnectionCount uint64
	
	// ReconnectionCount tracks reconnection attempts
	ReconnectionCount uint64
	
	// LastError contains the most recent error encountered
	LastError error
	
	// ConnectedAt tracks when connection was established
	ConnectedAt time.Time
	
	// DisconnectedAt tracks when connection was lost
	DisconnectedAt time.Time
}

// Tunnel represents a bidirectional communication channel that supports
// multiple protocols and provides workspace-aware tunneling capabilities.
// 
// Implementations must be thread-safe and support concurrent read/write operations.
// Tunnels provide automatic reconnection, backpressure handling, and comprehensive
// statistical tracking for monitoring and debugging purposes.
type Tunnel interface {
	// Connect establishes the tunnel connection using the configured protocol.
	// Returns an error if connection fails or times out.
	//
	// This method is idempotent - calling Connect on an already connected
	// tunnel should return nil without side effects.
	Connect(ctx context.Context) error
	
	// Close gracefully shuts down the tunnel and releases all resources.
	// Any pending operations will be cancelled and return appropriate errors.
	//
	// Close is safe to call multiple times and from multiple goroutines.
	Close() error
	
	// Send transmits data through the tunnel. Implementations should handle
	// backpressure by blocking until data can be sent or context is cancelled.
	//
	// Returns io.ErrClosedPipe if tunnel is closed, context.Canceled if
	// context is cancelled, or other protocol-specific errors.
	Send(ctx context.Context, data []byte) error
	
	// Receive reads data from the tunnel. This is a blocking operation that
	// returns when data is available or an error occurs.
	//
	// Returns io.EOF when tunnel is closed gracefully, io.ErrClosedPipe for
	// unexpected closures, or other protocol-specific errors.
	Receive(ctx context.Context) ([]byte, error)
	
	// SendStream returns a writer for streaming data through the tunnel.
	// The writer supports Write operations until the returned context
	// is cancelled or an error occurs.
	//
	// Callers must close the returned writer to properly terminate the stream.
	SendStream(ctx context.Context) (io.WriteCloser, error)
	
	// ReceiveStream returns a reader for streaming data from the tunnel.
	// The reader supports Read operations until EOF or an error occurs.
	//
	// Callers should close the returned reader when done to free resources.
	ReceiveStream(ctx context.Context) (io.ReadCloser, error)
	
	// State returns the current connection state of the tunnel
	State() TunnelState
	
	// Stats returns current statistical information about tunnel usage.
	// The returned struct contains a snapshot of current metrics.
	Stats() TunnelStats
	
	// SetReconnectEnabled controls whether automatic reconnection is enabled.
	// When enabled, the tunnel will attempt to reconnect on connection loss.
	SetReconnectEnabled(enabled bool)
	
	// Ping sends a keep-alive message to verify connection health.
	// Returns nil if tunnel is healthy, or an error indicating the issue.
	Ping(ctx context.Context) error
	
	// LocalAddr returns the local network address for protocol-specific debugging
	LocalAddr() string
	
	// RemoteAddr returns the remote network address for protocol-specific debugging  
	RemoteAddr() string
	
	// Protocol returns the tunneling protocol in use
	Protocol() TunnelProtocol
	
	// Workspace returns the logical cluster this tunnel is associated with
	Workspace() logicalcluster.Name
}

// TunnelFactory creates tunnel instances with specific configurations.
// Implementations should validate options and return appropriate errors
// for invalid configurations.
type TunnelFactory interface {
	// CreateTunnel creates a new tunnel instance with the specified options.
	// The tunnel is created in disconnected state - callers must call Connect.
	//
	// Returns ErrUnsupportedProtocol for unsupported protocols or other
	// configuration errors for invalid options.
	CreateTunnel(opts TunnelOptions) (Tunnel, error)
	
	// SupportedProtocols returns a list of protocols supported by this factory
	SupportedProtocols() []TunnelProtocol
	
	// ValidateOptions validates tunnel options without creating a tunnel.
	// Useful for configuration validation in admission controllers.
	ValidateOptions(opts TunnelOptions) error
}

// Common errors that tunnel implementations should use
var (
	// ErrUnsupportedProtocol indicates the requested protocol is not supported
	ErrUnsupportedProtocol = fmt.Errorf("unsupported tunnel protocol")
	
	// ErrTunnelClosed indicates operation failed because tunnel is closed
	ErrTunnelClosed = fmt.Errorf("tunnel is closed")
	
	// ErrInvalidConfiguration indicates tunnel options are invalid
	ErrInvalidConfiguration = fmt.Errorf("invalid tunnel configuration")
	
	// ErrConnectionFailed indicates tunnel connection could not be established
	ErrConnectionFailed = fmt.Errorf("tunnel connection failed")
	
	// ErrReconnectExhausted indicates reconnection attempts have been exhausted
	ErrReconnectExhausted = fmt.Errorf("tunnel reconnection attempts exhausted")
)