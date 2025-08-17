# Implementation Instructions: WebSocket Connection Manager

## Branch: `feature/phase7-syncer-impl/p7w4-websocket`

## Overview
This branch implements the WebSocket connection manager that establishes and maintains a persistent bidirectional connection between the syncer and KCP. It handles connection lifecycle, reconnection, message routing, and connection health monitoring.

**Target Size**: ~600 lines  
**Complexity**: High  
**Priority**: Critical (enables real-time sync)

## Dependencies
- **Phase 5 APIs**: Connection interfaces
- **Phase 6 Infrastructure**: Virtual workspace tunneling
- **Wave 1-3**: All sync components use this connection
- **External**: gorilla/websocket library

## Files to Create

### 1. WebSocket Manager Core (~250 lines)
**File**: `pkg/reconciler/workload/syncer/tunnel/manager.go`
- Connection manager struct
- Connection establishment
- Message routing
- Graceful shutdown

### 2. Connection Handler (~150 lines)
**File**: `pkg/reconciler/workload/syncer/tunnel/connection.go`
- WebSocket connection wrapper
- Read/write pumps
- Message framing
- Error handling

### 3. Reconnection Logic (~100 lines)
**File**: `pkg/reconciler/workload/syncer/tunnel/reconnect.go`
- Exponential backoff
- Connection state tracking
- Automatic reconnection
- Circuit breaker

### 4. Message Protocol (~50 lines)
**File**: `pkg/reconciler/workload/syncer/tunnel/protocol.go`
- Message types definition
- Protocol versioning
- Message serialization
- Header management

### 5. Connection Tests (~50 lines)
**File**: `pkg/reconciler/workload/syncer/tunnel/manager_test.go`
- Connection tests
- Reconnection tests
- Message routing tests

## Step-by-Step Implementation Guide

### Step 1: Create Package Structure
```bash
mkdir -p pkg/reconciler/workload/syncer/tunnel
```

### Step 2: Define Protocol Types
Create `protocol.go` with:

```go
package tunnel

import (
    "encoding/json"
    "time"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
    // Control messages
    MessageTypeHandshake   MessageType = "handshake"
    MessageTypePing        MessageType = "ping"
    MessageTypePong        MessageType = "pong"
    MessageTypeClose       MessageType = "close"
    
    // Sync messages
    MessageTypeResource    MessageType = "resource"
    MessageTypeStatus      MessageType = "status"
    MessageTypeEvent       MessageType = "event"
    MessageTypeCommand     MessageType = "command"
)

// Message represents a WebSocket message
type Message struct {
    ID        string          `json:"id"`
    Type      MessageType     `json:"type"`
    Timestamp time.Time       `json:"timestamp"`
    Payload   json.RawMessage `json:"payload,omitempty"`
    Error     string          `json:"error,omitempty"`
}

// HandshakePayload contains handshake information
type HandshakePayload struct {
    SyncTarget    string            `json:"syncTarget"`
    Version       string            `json:"version"`
    Capabilities  []string          `json:"capabilities"`
    Token         string            `json:"token,omitempty"`
    Metadata      map[string]string `json:"metadata,omitempty"`
}

// ResourcePayload contains resource sync data
type ResourcePayload struct {
    Operation string          `json:"operation"` // create, update, delete
    GVR       string          `json:"gvr"`
    Namespace string          `json:"namespace"`
    Name      string          `json:"name"`
    Object    json.RawMessage `json:"object"`
}
```

### Step 3: Implement Connection Manager
Create `manager.go` with:

```go
package tunnel

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/gorilla/websocket"
    workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
    "k8s.io/klog/v2"
)

// Manager manages WebSocket connections to KCP
type Manager struct {
    url         string
    syncTarget  *workloadv1alpha1.SyncTarget
    token       string
    
    conn        *Connection
    mu          sync.RWMutex
    
    // Message handling
    handlers    map[MessageType]MessageHandler
    outgoing    chan *Message
    incoming    chan *Message
    
    // Connection state
    connected   bool
    connecting  bool
    lastError   error
    
    // Reconnection
    reconnector *Reconnector
    
    // Lifecycle
    ctx         context.Context
    cancel      context.CancelFunc
    wg          sync.WaitGroup
}

// MessageHandler processes incoming messages
type MessageHandler func(ctx context.Context, msg *Message) error

// NewManager creates a new WebSocket manager
func NewManager(url string, syncTarget *workloadv1alpha1.SyncTarget, token string) *Manager {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Manager{
        url:        url,
        syncTarget: syncTarget,
        token:      token,
        handlers:   make(map[MessageType]MessageHandler),
        outgoing:   make(chan *Message, 100),
        incoming:   make(chan *Message, 100),
        reconnector: NewReconnector(),
        ctx:        ctx,
        cancel:     cancel,
    }
}

// Connect establishes WebSocket connection to KCP
func (m *Manager) Connect(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if m.connected {
        return nil
    }
    
    if m.connecting {
        return fmt.Errorf("connection already in progress")
    }
    
    m.connecting = true
    defer func() { m.connecting = false }()
    
    logger := klog.FromContext(ctx)
    logger.Info("Establishing WebSocket connection", "url", m.url)
    
    // Create WebSocket connection
    dialer := websocket.Dialer{
        HandshakeTimeout: 30 * time.Second,
        ReadBufferSize:   1024 * 1024, // 1MB
        WriteBufferSize:  1024 * 1024, // 1MB
    }
    
    headers := http.Header{
        "Authorization": []string{fmt.Sprintf("Bearer %s", m.token)},
        "X-SyncTarget":  []string{m.syncTarget.Name},
        "X-Workspace":   []string{m.syncTarget.GetClusterName().String()},
    }
    
    conn, resp, err := dialer.DialContext(ctx, m.url, headers)
    if err != nil {
        if resp != nil {
            logger.Error(err, "Failed to connect", "status", resp.StatusCode)
        }
        m.lastError = err
        return fmt.Errorf("websocket dial failed: %w", err)
    }
    
    // Wrap connection
    m.conn = NewConnection(conn)
    
    // Perform handshake
    if err := m.performHandshake(ctx); err != nil {
        m.conn.Close()
        m.conn = nil
        return fmt.Errorf("handshake failed: %w", err)
    }
    
    // Start message pumps
    m.wg.Add(3)
    go m.readPump()
    go m.writePump()
    go m.processMessages()
    
    m.connected = true
    logger.Info("WebSocket connection established")
    
    return nil
}

// performHandshake exchanges initial handshake messages
func (m *Manager) performHandshake(ctx context.Context) error {
    handshake := &Message{
        ID:        uuid.New().String(),
        Type:      MessageTypeHandshake,
        Timestamp: time.Now(),
        Payload: mustMarshal(HandshakePayload{
            SyncTarget:   m.syncTarget.Name,
            Version:      "v1alpha1",
            Capabilities: []string{"sync", "status", "events"},
            Metadata: map[string]string{
                "cluster": m.syncTarget.Spec.KubeConfig.Cluster,
            },
        }),
    }
    
    // Send handshake
    if err := m.conn.WriteMessage(handshake); err != nil {
        return err
    }
    
    // Wait for response
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    response, err := m.conn.ReadMessage()
    if err != nil {
        return err
    }
    
    if response.Type != MessageTypeHandshake {
        return fmt.Errorf("unexpected response type: %s", response.Type)
    }
    
    if response.Error != "" {
        return fmt.Errorf("handshake error: %s", response.Error)
    }
    
    return nil
}

// RegisterHandler registers a message handler
func (m *Manager) RegisterHandler(msgType MessageType, handler MessageHandler) {
    m.handlers[msgType] = handler
}

// Send sends a message through the WebSocket
func (m *Manager) Send(msg *Message) error {
    if !m.IsConnected() {
        return fmt.Errorf("not connected")
    }
    
    select {
    case m.outgoing <- msg:
        return nil
    case <-time.After(5 * time.Second):
        return fmt.Errorf("send timeout")
    }
}

// readPump reads messages from WebSocket
func (m *Manager) readPump() {
    defer m.wg.Done()
    defer m.handleDisconnect()
    
    for {
        select {
        case <-m.ctx.Done():
            return
        default:
            msg, err := m.conn.ReadMessage()
            if err != nil {
                if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                    klog.Error(err, "WebSocket read error")
                }
                return
            }
            
            select {
            case m.incoming <- msg:
            case <-m.ctx.Done():
                return
            }
        }
    }
}

// writePump writes messages to WebSocket
func (m *Manager) writePump() {
    defer m.wg.Done()
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-m.ctx.Done():
            return
            
        case msg := <-m.outgoing:
            if err := m.conn.WriteMessage(msg); err != nil {
                klog.Error(err, "Failed to write message")
                return
            }
            
        case <-ticker.C:
            // Send ping to keep connection alive
            ping := &Message{
                ID:        uuid.New().String(),
                Type:      MessageTypePing,
                Timestamp: time.Now(),
            }
            if err := m.conn.WriteMessage(ping); err != nil {
                return
            }
        }
    }
}

// processMessages handles incoming messages
func (m *Manager) processMessages() {
    defer m.wg.Done()
    
    for {
        select {
        case <-m.ctx.Done():
            return
            
        case msg := <-m.incoming:
            m.handleMessage(msg)
        }
    }
}
```

### Step 4: Implement Connection Wrapper
Create `connection.go` with:

1. **Connection struct**:
   - WebSocket connection wrapper
   - Read/write mutexes
   - Message serialization
   - Deadline management

2. **Read/write methods**:
   - Thread-safe operations
   - Message framing
   - Error recovery
   - Timeout handling

3. **Connection health**:
   - Ping/pong handling
   - Deadline updates
   - Error tracking

### Step 5: Implement Reconnection Logic
Create `reconnect.go` with:

1. **Reconnector struct**:
```go
type Reconnector struct {
    attempts      int
    maxAttempts   int
    baseDelay     time.Duration
    maxDelay      time.Duration
    factor        float64
    jitter        float64
}
```

2. **Exponential backoff**:
   - Calculate delay
   - Add jitter
   - Track attempts
   - Circuit breaking

3. **Reconnection loop**:
   - Monitor connection
   - Trigger reconnection
   - Restore state
   - Resume operations

### Step 6: Add Advanced Features

1. **Message compression**:
   - Enable compression
   - Compress large payloads
   - Decompress on receive

2. **Connection pooling**:
   - Multiple connections
   - Load balancing
   - Failover support

3. **Metrics and monitoring**:
   - Message counters
   - Latency tracking
   - Error rates
   - Connection duration

### Step 7: Add Comprehensive Tests
Create test files covering:

1. **Connection tests**:
   - Successful connection
   - Connection failures
   - Handshake protocol

2. **Message tests**:
   - Message routing
   - Handler execution
   - Error handling

3. **Reconnection tests**:
   - Automatic reconnection
   - Backoff behavior
   - State restoration

## Testing Requirements

### Unit Tests:
- Connection establishment
- Message serialization
- Handler registration
- Reconnection logic
- Error scenarios

### Integration Tests:
- Full connection lifecycle
- Message exchange
- Reconnection under load
- Network failure simulation

## Validation Checklist

- [ ] Connection establishes successfully
- [ ] Handshake protocol works correctly
- [ ] Messages route to handlers
- [ ] Reconnection works automatically
- [ ] Ping/pong keeps connection alive
- [ ] Graceful shutdown implemented
- [ ] Comprehensive logging
- [ ] Metrics exposed
- [ ] Tests achieve >70% coverage
- [ ] Code follows KCP patterns
- [ ] Feature flags integrated
- [ ] Under 600 lines (excluding tests)

## Common Pitfalls to Avoid

1. **Don't block on message sends** - use buffered channels
2. **Handle partial writes** - WebSocket may fragment
3. **Clean up on disconnect** - prevent goroutine leaks
4. **Implement heartbeat** - detect stale connections
5. **Add connection limits** - prevent resource exhaustion

## Integration Notes

This component:
- Provides transport for all sync operations
- Used by Waves 1-3 components
- Coordinates with Wave 4 heartbeat
- Critical for real-time sync

Should provide:
- Reliable message delivery
- Automatic reconnection
- Connection metrics
- Message routing

## Success Criteria

The implementation is complete when:
1. WebSocket connection establishes reliably
2. Messages are delivered bidirectionally
3. Reconnection works automatically
4. Connection remains stable under load
5. All tests pass
6. Can handle 1000+ messages per second