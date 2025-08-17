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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockWebSocketServer creates a test WebSocket server
func mockWebSocketServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		if handler != nil {
			handler(conn)
		}
	}))
}

func TestNewManager(t *testing.T) {
	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-target",
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: "root:test",
			},
		},
		Spec: workloadv1alpha1.SyncTargetSpec{},
	}

	manager := NewManager("ws://localhost:8080", syncTarget, "test-token")

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.url != "ws://localhost:8080" {
		t.Errorf("Expected URL ws://localhost:8080, got %s", manager.url)
	}

	if manager.token != "test-token" {
		t.Errorf("Expected token test-token, got %s", manager.token)
	}

	if manager.syncTarget != syncTarget {
		t.Error("SyncTarget not set correctly")
	}

	if manager.IsConnected() {
		t.Error("Manager should not be connected initially")
	}
}

func TestManager_ConnectSuccess(t *testing.T) {
	// Create mock server that responds to handshake
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		// Read handshake message
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("Failed to read handshake: %v", err)
			return
		}

		if msg.Type != MessageTypeHandshake {
			t.Errorf("Expected handshake message, got %s", msg.Type)
			return
		}

		// Send handshake response
		response := &Message{
			ID:        msg.ID,
			Type:      MessageTypeHandshake,
			Timestamp: time.Now(),
		}
		if err := conn.WriteJSON(response); err != nil {
			t.Errorf("Failed to send handshake response: %v", err)
		}

		// Keep connection alive for test
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-target",
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: "root:test",
			},
		},
		Spec: workloadv1alpha1.SyncTargetSpec{},
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	manager := NewManager(wsURL, syncTarget, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !manager.IsConnected() {
		t.Error("Manager should be connected after successful connect")
	}

	manager.Close()
}

func TestManager_SendMessage(t *testing.T) {
	// Create mock server that echoes messages
	received := make(chan Message, 1)
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		// Handle handshake
		var handshake Message
		conn.ReadJSON(&handshake)
		response := &Message{
			ID:        handshake.ID,
			Type:      MessageTypeHandshake,
			Timestamp: time.Now(),
		}
		conn.WriteJSON(response)

		// Read test message
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("Failed to read message: %v", err)
			return
		}
		received <- msg
	})
	defer server.Close()

	syncTarget := &workloadv1alpha1.SyncTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-target",
			Annotations: map[string]string{
				logicalcluster.AnnotationKey: "root:test",
			},
		},
		Spec: workloadv1alpha1.SyncTargetSpec{},
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	manager := NewManager(wsURL, syncTarget, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect first
	if err := manager.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send test message
	testMsg := &Message{
		Type: MessageTypeCommand,
		Payload: mustMarshal(map[string]string{
			"test": "data",
		}),
	}

	if err := manager.Send(testMsg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify message received
	select {
	case msg := <-received:
		if msg.Type != MessageTypeCommand {
			t.Errorf("Expected command message, got %s", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Message not received within timeout")
	}

	manager.Close()
}

func TestReconnector(t *testing.T) {
	reconnector := NewReconnector()

	// Test initial state
	if !reconnector.ShouldRetry() {
		t.Error("Should retry initially")
	}

	if reconnector.GetAttempts() != 0 {
		t.Errorf("Expected 0 attempts initially, got %d", reconnector.GetAttempts())
	}

	// Test delay calculation
	delay1 := reconnector.NextDelay()
	if delay1 < 500*time.Millisecond || delay1 > 2*time.Second {
		t.Errorf("First delay should be ~1s (with jitter), got %v", delay1)
	}

	delay2 := reconnector.NextDelay()
	if delay2 < delay1 {
		t.Error("Delay should increase with attempts")
	}

	// Test success reset
	reconnector.RecordSuccess()
	if reconnector.GetAttempts() != 0 {
		t.Error("Attempts should be reset after success")
	}

	// Test circuit breaker
	for i := 0; i < 6; i++ {
		reconnector.RecordFailure()
	}

	if !reconnector.IsCircuitOpen() {
		t.Error("Circuit should be open after max failures")
	}
}