/*
Copyright 2022 The KCP Authors.

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

package tunnel

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"k8s.io/klog/v2"
)

// Manager manages WebSocket connections to KCP
type Manager struct {
	url        string
	syncTarget *workloadv1alpha1.SyncTarget
	token      string

	conn *Connection
	mu   sync.RWMutex

	// Message handling
	handlers map[MessageType]MessageHandler
	outgoing chan *Message
	incoming chan *Message

	// Connection state
	connected  bool
	connecting bool
	lastError  error

	// Reconnection
	reconnector *Reconnector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MessageHandler processes incoming messages
type MessageHandler func(ctx context.Context, msg *Message) error

// NewManager creates a new WebSocket manager
func NewManager(url string, syncTarget *workloadv1alpha1.SyncTarget, token string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
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

	// Register built-in handlers
	manager.RegisterHandler(MessageTypePing, manager.handlePing)
	manager.RegisterHandler(MessageTypePong, manager.handlePong)

	return manager
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
		"X-Workspace":   []string{m.syncTarget.Namespace},
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
	m.conn.EnableCompression()

	// Set up connection handlers
	m.setupConnectionHandlers()

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
	m.reconnector.RecordSuccess()
	logger.Info("WebSocket connection established")

	return nil
}

// setupConnectionHandlers sets up WebSocket control handlers
func (m *Manager) setupConnectionHandlers() {
	m.conn.SetPingHandler(func(appData string) error {
		klog.V(4).Info("Received ping from server")
		return m.conn.WritePong([]byte(appData))
	})

	m.conn.SetPongHandler(func(appData string) error {
		klog.V(4).Info("Received pong from server")
		return nil
	})

	m.conn.SetCloseHandler(func(code int, text string) error {
		klog.Info("Server initiated connection close", "code", code, "text", text)
		return nil
	})
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
				"cluster": m.syncTarget.Name,
			},
		}),
	}

	// Send handshake
	if err := m.conn.WriteMessage(handshake); err != nil {
		return err
	}

	// Wait for response directly from connection
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[msgType] = handler
}

// Send sends a message through the WebSocket
func (m *Manager) Send(msg *Message) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Set timestamp if not already set
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Generate ID if not already set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	select {
	case m.outgoing <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

// SendResource sends a resource sync message
func (m *Manager) SendResource(operation, gvr, namespace, name string, object []byte) error {
	msg := &Message{
		Type: MessageTypeResource,
		Payload: mustMarshal(ResourcePayload{
			Operation: operation,
			GVR:       gvr,
			Namespace: namespace,
			Name:      name,
			Object:    object,
		}),
	}
	return m.Send(msg)
}

// SendStatus sends a status update message
func (m *Manager) SendStatus(resourceID string, status []byte) error {
	msg := &Message{
		Type: MessageTypeStatus,
		Payload: mustMarshal(StatusPayload{
			ResourceID: resourceID,
			Status:     status,
		}),
	}
	return m.Send(msg)
}

// IsConnected returns whether the connection is active
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected && m.conn != nil && !m.conn.IsClosed()
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
			if err := m.conn.WritePing(); err != nil {
				klog.V(2).Info("Failed to send ping", "error", err)
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

// handleMessage routes messages to registered handlers
func (m *Manager) handleMessage(msg *Message) {
	handler, exists := m.handlers[msg.Type]
	if !exists {
		klog.V(2).Info("No handler for message type", "type", msg.Type, "id", msg.ID)
		return
	}

	if err := handler(m.ctx, msg); err != nil {
		klog.Error(err, "Message handler error", "type", msg.Type, "id", msg.ID)
	}
}

// handlePing processes ping messages
func (m *Manager) handlePing(ctx context.Context, msg *Message) error {
	pong := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypePong,
		Timestamp: time.Now(),
	}
	return m.Send(pong)
}

// handlePong processes pong messages
func (m *Manager) handlePong(ctx context.Context, msg *Message) error {
	klog.V(4).Info("Received pong message", "id", msg.ID)
	return nil
}

// handleDisconnect handles disconnection and triggers reconnection
func (m *Manager) handleDisconnect() {
	m.mu.Lock()
	m.connected = false
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.mu.Unlock()

	klog.Info("WebSocket connection lost, will attempt to reconnect")
	m.reconnector.RecordFailure()

	// Attempt reconnection in background
	go m.reconnectLoop()
}

// reconnectLoop continuously attempts to reconnect
func (m *Manager) reconnectLoop() {
	for {
		if !m.reconnector.ShouldRetry() {
			if m.reconnector.IsCircuitOpen() {
				klog.Warning("Circuit breaker open, stopping reconnection attempts")
				return
			}
		}

		delay := m.reconnector.NextDelay()
		klog.Info("Attempting reconnection", "attempt", m.reconnector.GetAttempts(), "delay", delay)

		timer := time.NewTimer(delay)
		select {
		case <-m.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if err := m.Connect(m.ctx); err != nil {
			klog.Error(err, "Reconnection failed", "attempts", m.reconnector.GetAttempts())
			continue
		}

		klog.Info("Reconnection successful")
		return
	}
}

// Close closes the manager and all connections
func (m *Manager) Close() error {
	m.cancel()

	m.mu.Lock()
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.connected = false
	m.mu.Unlock()

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for goroutines to finish")
	}
}