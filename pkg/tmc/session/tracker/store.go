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

package tracker

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Store defines the interface for session state storage
type Store interface {
	// Set stores a session with the given ID
	Set(ctx context.Context, sessionID string, info *SessionInfo) error
	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*SessionInfo, error)
	// Delete removes a session by ID
	Delete(ctx context.Context, sessionID string) error
	// List returns all stored sessions
	List(ctx context.Context) ([]*SessionInfo, error)
	// Clear removes all sessions
	Clear(ctx context.Context) error
	// Size returns the number of stored sessions
	Size(ctx context.Context) (int, error)
}

// memoryStore implements Store interface with in-memory storage
type memoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionInfo
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() Store {
	return &memoryStore{
		sessions: make(map[string]*SessionInfo),
	}
}

// Set stores a session with the given ID
func (ms *memoryStore) Set(ctx context.Context, sessionID string, info *SessionInfo) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if info == nil {
		return fmt.Errorf("session info cannot be nil")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Create a deep copy to avoid external modifications
	sessionCopy := *info
	if info.Metadata != nil {
		sessionCopy.Metadata = make(map[string]string, len(info.Metadata))
		for k, v := range info.Metadata {
			sessionCopy.Metadata[k] = v
		}
	}

	ms.sessions[sessionID] = &sessionCopy
	return nil
}

// Get retrieves a session by ID
func (ms *memoryStore) Get(ctx context.Context, sessionID string) (*SessionInfo, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	session, exists := ms.sessions[sessionID]
	if !exists {
		return nil, nil
	}

	// Return a copy to avoid external modifications
	sessionCopy := *session
	if session.Metadata != nil {
		sessionCopy.Metadata = make(map[string]string, len(session.Metadata))
		for k, v := range session.Metadata {
			sessionCopy.Metadata[k] = v
		}
	}

	return &sessionCopy, nil
}

// Delete removes a session by ID
func (ms *memoryStore) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.sessions, sessionID)
	return nil
}

// List returns all stored sessions
func (ms *memoryStore) List(ctx context.Context) ([]*SessionInfo, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	sessions := make([]*SessionInfo, 0, len(ms.sessions))
	
	for _, session := range ms.sessions {
		// Create a copy to avoid external modifications
		sessionCopy := *session
		if session.Metadata != nil {
			sessionCopy.Metadata = make(map[string]string, len(session.Metadata))
			for k, v := range session.Metadata {
				sessionCopy.Metadata[k] = v
			}
		}
		sessions = append(sessions, &sessionCopy)
	}

	// Sort by creation time for consistent ordering
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// Clear removes all sessions
func (ms *memoryStore) Clear(ctx context.Context) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.sessions = make(map[string]*SessionInfo)
	return nil
}

// Size returns the number of stored sessions
func (ms *memoryStore) Size(ctx context.Context) (int, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return len(ms.sessions), nil
}