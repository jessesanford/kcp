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
	"time"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	// Control messages
	MessageTypeHandshake MessageType = "handshake"
	MessageTypePing      MessageType = "ping"
	MessageTypePong      MessageType = "pong"
	MessageTypeClose     MessageType = "close"

	// Sync messages
	MessageTypeResource MessageType = "resource"
	MessageTypeStatus   MessageType = "status"
	MessageTypeEvent    MessageType = "event"
	MessageTypeCommand  MessageType = "command"
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
	SyncTarget   string            `json:"syncTarget"`
	Version      string            `json:"version"`
	Capabilities []string          `json:"capabilities"`
	Token        string            `json:"token,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ResourcePayload contains resource sync data
type ResourcePayload struct {
	Operation string          `json:"operation"` // create, update, delete
	GVR       string          `json:"gvr"`
	Namespace string          `json:"namespace"`
	Name      string          `json:"name"`
	Object    json.RawMessage `json:"object"`
}

// StatusPayload contains status update information
type StatusPayload struct {
	ResourceID string          `json:"resourceId"`
	Status     json.RawMessage `json:"status"`
	Conditions json.RawMessage `json:"conditions,omitempty"`
}

// EventPayload contains event information
type EventPayload struct {
	Type      string          `json:"type"`
	Object    json.RawMessage `json:"object"`
	Reason    string          `json:"reason"`
	Message   string          `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
}

// mustMarshal marshals data to JSON and panics on error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}