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
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

// Connection wraps a WebSocket connection with thread-safe read/write operations
type Connection struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	readMu    sync.Mutex
	closed    bool
	closedMu  sync.RWMutex
	closeOnce sync.Once
}

// NewConnection creates a new connection wrapper
func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{
		conn: conn,
	}
}

// WriteMessage writes a message to the WebSocket connection
func (c *Connection) WriteMessage(msg *Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.isClosed() {
		return websocket.ErrCloseSent
	}

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	// Marshal message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Write message as text frame
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// ReadMessage reads a message from the WebSocket connection
func (c *Connection) ReadMessage() (*Message, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if c.isClosed() {
		return nil, websocket.ErrCloseSent
	}

	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return nil, err
	}

	// Read message from WebSocket
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	// Only handle text messages
	if msgType != websocket.TextMessage {
		return nil, websocket.ErrReadLimit
	}

	// Unmarshal message from JSON
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// WritePing writes a ping control message
func (c *Connection) WritePing() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.isClosed() {
		return websocket.ErrCloseSent
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

// WritePong writes a pong control message in response to a ping
func (c *Connection) WritePong(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.isClosed() {
		return websocket.ErrCloseSent
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.PongMessage, data)
}

// SetPingHandler sets the ping handler for the connection
func (c *Connection) SetPingHandler(h func(appData string) error) {
	c.conn.SetPingHandler(h)
}

// SetPongHandler sets the pong handler for the connection
func (c *Connection) SetPongHandler(h func(appData string) error) {
	c.conn.SetPongHandler(h)
}

// SetCloseHandler sets the close handler for the connection
func (c *Connection) SetCloseHandler(h func(code int, text string) error) {
	c.conn.SetCloseHandler(h)
}

// EnableCompression enables per-message deflate compression
func (c *Connection) EnableCompression() {
	c.conn.EnableWriteCompression(true)
}

// DisableCompression disables per-message deflate compression
func (c *Connection) DisableCompression() {
	c.conn.EnableWriteCompression(false)
}

// Close closes the WebSocket connection gracefully
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()

		// Send close message
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		c.writeMu.Lock()
		if writeErr := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); writeErr != nil {
			klog.V(2).Info("Failed to set write deadline for close", "error", writeErr)
		}
		if writeErr := c.conn.WriteMessage(websocket.CloseMessage, closeMsg); writeErr != nil {
			klog.V(2).Info("Failed to write close message", "error", writeErr)
		}
		c.writeMu.Unlock()

		// Close underlying connection
		err = c.conn.Close()
	})
	return err
}

// CloseWithError closes the connection with an error code and message
func (c *Connection) CloseWithError(code int, text string) error {
	var err error
	c.closeOnce.Do(func() {
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()

		// Send close message with error code
		closeMsg := websocket.FormatCloseMessage(code, text)
		c.writeMu.Lock()
		if writeErr := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); writeErr != nil {
			klog.V(2).Info("Failed to set write deadline for close", "error", writeErr)
		}
		if writeErr := c.conn.WriteMessage(websocket.CloseMessage, closeMsg); writeErr != nil {
			klog.V(2).Info("Failed to write close message", "error", writeErr)
		}
		c.writeMu.Unlock()

		// Close underlying connection
		err = c.conn.Close()
	})
	return err
}

// IsClosed returns whether the connection is closed
func (c *Connection) IsClosed() bool {
	return c.isClosed()
}

// isClosed returns whether the connection is closed (internal method)
func (c *Connection) isClosed() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()
	return c.closed
}

// LocalAddr returns the local network address
func (c *Connection) LocalAddr() string {
	return c.conn.LocalAddr().String()
}

// RemoteAddr returns the remote network address
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}