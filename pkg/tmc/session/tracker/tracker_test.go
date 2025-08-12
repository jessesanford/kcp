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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Test Set and Get
	info := &SessionInfo{
		SessionID:      "test-session",
		LogicalCluster: logicalcluster.Name("test-cluster"),
		PlacementName:  "test-placement",
		State:          SessionStateActive,
		CreatedAt:      time.Now(),
		LastSeen:       time.Now(),
		ExpiresAt:      time.Now().Add(time.Hour),
		Metadata:       map[string]string{"key": "value"},
	}

	err := store.Set(ctx, info.SessionID, info)
	require.NoError(t, err)

	retrieved, err := store.Get(ctx, info.SessionID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, info.SessionID, retrieved.SessionID)
	assert.Equal(t, info.LogicalCluster, retrieved.LogicalCluster)
	assert.Equal(t, info.PlacementName, retrieved.PlacementName)
	assert.Equal(t, info.State, retrieved.State)
	assert.Equal(t, info.Metadata["key"], retrieved.Metadata["key"])

	// Test List
	sessions, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, info.SessionID, sessions[0].SessionID)

	// Test Size
	size, err := store.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, size)

	// Test Delete
	err = store.Delete(ctx, info.SessionID)
	require.NoError(t, err)

	retrieved, err = store.Get(ctx, info.SessionID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Test Clear
	_ = store.Set(ctx, "session1", info)
	_ = store.Set(ctx, "session2", info)
	
	size, err = store.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, size)

	err = store.Clear(ctx)
	require.NoError(t, err)

	size, err = store.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, size)
}

func TestMemoryStoreValidation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	tests := map[string]struct {
		sessionID string
		info      *SessionInfo
		wantError bool
	}{
		"empty session ID": {
			sessionID: "",
			info:      &SessionInfo{},
			wantError: true,
		},
		"nil session info": {
			sessionID: "test",
			info:      nil,
			wantError: true,
		},
		"valid session": {
			sessionID: "test",
			info: &SessionInfo{
				SessionID: "test",
				State:     SessionStateActive,
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := store.Set(ctx, tc.sessionID, tc.info)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSessionTracker(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		DefaultSessionTTL: time.Hour,
		CleanupInterval:   time.Minute,
		MaxSessions:       10,
	}
	store := NewMemoryStore()
	tracker := NewSessionTracker(config, store)

	// Test StartSession
	info := &SessionInfo{
		SessionID:      "test-session",
		LogicalCluster: logicalcluster.Name("test-cluster"),
		PlacementName:  "test-placement",
	}

	err := tracker.StartSession(ctx, info)
	require.NoError(t, err)

	// Test GetSession
	retrieved, err := tracker.GetSession(ctx, info.SessionID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, info.SessionID, retrieved.SessionID)
	assert.Equal(t, SessionStateActive, retrieved.State)
	assert.False(t, retrieved.CreatedAt.IsZero())
	assert.False(t, retrieved.LastSeen.IsZero())
	assert.False(t, retrieved.ExpiresAt.IsZero())

	// Test UpdateSession
	updates := &SessionInfo{
		State: SessionStateInactive,
		Metadata: map[string]string{
			"updated": "true",
		},
	}

	err = tracker.UpdateSession(ctx, info.SessionID, updates)
	require.NoError(t, err)

	retrieved, err = tracker.GetSession(ctx, info.SessionID)
	require.NoError(t, err)
	assert.Equal(t, SessionStateInactive, retrieved.State)
	assert.Equal(t, "true", retrieved.Metadata["updated"])

	// Test ListSessions
	sessions, err := tracker.ListSessions(ctx, "")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// Test filtered list
	sessions, err = tracker.ListSessions(ctx, logicalcluster.Name("test-cluster"))
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	sessions, err = tracker.ListSessions(ctx, logicalcluster.Name("other-cluster"))
	require.NoError(t, err)
	assert.Len(t, sessions, 0)

	// Test ExpireSession
	err = tracker.ExpireSession(ctx, info.SessionID)
	require.NoError(t, err)

	retrieved, err = tracker.GetSession(ctx, info.SessionID)
	require.NoError(t, err)
	assert.Equal(t, SessionStateExpired, retrieved.State)

	// Test TerminateSession
	err = tracker.TerminateSession(ctx, info.SessionID)
	require.NoError(t, err)

	retrieved, err = tracker.GetSession(ctx, info.SessionID)
	require.NoError(t, err)
	assert.Equal(t, SessionStateTerminating, retrieved.State)
}

func TestSessionTrackerValidation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	tracker := NewSessionTracker(nil, store)

	tests := map[string]struct {
		operation func() error
		wantError bool
	}{
		"start session with nil info": {
			operation: func() error {
				return tracker.StartSession(ctx, nil)
			},
			wantError: true,
		},
		"start session with empty ID": {
			operation: func() error {
				return tracker.StartSession(ctx, &SessionInfo{})
			},
			wantError: true,
		},
		"update session with empty ID": {
			operation: func() error {
				return tracker.UpdateSession(ctx, "", &SessionInfo{})
			},
			wantError: true,
		},
		"update session with nil updates": {
			operation: func() error {
				return tracker.UpdateSession(ctx, "test", nil)
			},
			wantError: true,
		},
		"get session with empty ID": {
			operation: func() error {
				_, err := tracker.GetSession(ctx, "")
				return err
			},
			wantError: true,
		},
		"update non-existent session": {
			operation: func() error {
				return tracker.UpdateSession(ctx, "non-existent", &SessionInfo{})
			},
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.operation()
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSessionCleanup(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		DefaultSessionTTL: time.Millisecond * 10,
		CleanupInterval:   time.Millisecond * 5,
		MaxSessions:       10,
	}
	store := NewMemoryStore()
	tracker := NewSessionTracker(config, store)

	// Create sessions with different states
	now := time.Now()
	sessions := []*SessionInfo{
		{
			SessionID: "active-expired",
			State:     SessionStateActive,
			ExpiresAt: now.Add(-time.Minute), // Expired
		},
		{
			SessionID: "inactive-expired", 
			State:     SessionStateInactive,
			ExpiresAt: now.Add(-time.Minute), // Expired
		},
		{
			SessionID: "already-expired",
			State:     SessionStateExpired,
			ExpiresAt: now.Add(-time.Hour), // Long expired
		},
		{
			SessionID: "terminating",
			State:     SessionStateTerminating,
			ExpiresAt: now.Add(-time.Hour), // Long expired
		},
		{
			SessionID: "active-valid",
			State:     SessionStateActive,
			ExpiresAt: now.Add(time.Hour), // Valid
		},
	}

	for _, session := range sessions {
		err := tracker.StartSession(ctx, session)
		require.NoError(t, err)
	}

	// Verify all sessions exist
	allSessions, err := tracker.ListSessions(ctx, "")
	require.NoError(t, err)
	assert.Len(t, allSessions, 5)

	// Run cleanup
	err = tracker.Cleanup(ctx)
	require.NoError(t, err)

	// Verify expired sessions were removed
	remainingSessions, err := tracker.ListSessions(ctx, "")
	require.NoError(t, err)
	assert.Len(t, remainingSessions, 1)
	assert.Equal(t, "active-valid", remainingSessions[0].SessionID)
}