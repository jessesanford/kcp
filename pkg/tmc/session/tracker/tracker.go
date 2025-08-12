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
	"sync"
	"time"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// SessionState represents the state of a placement session
type SessionState string

const (
	// SessionStateActive indicates an active placement session
	SessionStateActive SessionState = "Active"
	// SessionStateInactive indicates an inactive placement session
	SessionStateInactive SessionState = "Inactive"
	// SessionStateExpired indicates an expired placement session
	SessionStateExpired SessionState = "Expired"
	// SessionStateTerminating indicates a session being terminated
	SessionStateTerminating SessionState = "Terminating"
)

// SessionInfo contains information about a tracked session
type SessionInfo struct {
	// SessionID is the unique identifier for this session
	SessionID string
	// LogicalCluster is the logical cluster where this session exists
	LogicalCluster logicalcluster.Name
	// PlacementName is the name of the placement this session tracks
	PlacementName string
	// State is the current state of the session
	State SessionState
	// CreatedAt is when this session was first created
	CreatedAt time.Time
	// LastSeen is when this session was last observed
	LastSeen time.Time
	// ExpiresAt is when this session will expire if not renewed
	ExpiresAt time.Time
	// Metadata contains additional session metadata
	Metadata map[string]string
}

// SessionTracker manages placement session tracking and lifecycle
type SessionTracker interface {
	// StartSession begins tracking a new placement session
	StartSession(ctx context.Context, info *SessionInfo) error
	// UpdateSession updates an existing session's information
	UpdateSession(ctx context.Context, sessionID string, updates *SessionInfo) error
	// GetSession retrieves information about a specific session
	GetSession(ctx context.Context, sessionID string) (*SessionInfo, error)
	// ListSessions returns all sessions, optionally filtered by logical cluster
	ListSessions(ctx context.Context, cluster logicalcluster.Name) ([]*SessionInfo, error)
	// ExpireSession marks a session as expired
	ExpireSession(ctx context.Context, sessionID string) error
	// TerminateSession terminates a session
	TerminateSession(ctx context.Context, sessionID string) error
	// Cleanup removes expired sessions
	Cleanup(ctx context.Context) error
	// Start begins the session tracker background processes
	Start(ctx context.Context) error
	// Stop shuts down the session tracker
	Stop(ctx context.Context) error
}

// Config contains configuration for the session tracker
type Config struct {
	// DefaultSessionTTL is the default time-to-live for sessions
	DefaultSessionTTL time.Duration
	// CleanupInterval is how often to run cleanup operations
	CleanupInterval time.Duration
	// MaxSessions is the maximum number of sessions to track
	MaxSessions int
	// Logger is the logger to use
	Logger logr.Logger
}

// DefaultConfig returns a default configuration for the session tracker
func DefaultConfig() *Config {
	return &Config{
		DefaultSessionTTL: 30 * time.Minute,
		CleanupInterval:   5 * time.Minute,
		MaxSessions:       1000,
		Logger:            klog.Background(),
	}
}

// sessionTracker implements SessionTracker interface
type sessionTracker struct {
	config *Config
	store  Store
	logger logr.Logger

	// Synchronization
	mu    sync.RWMutex
	queue workqueue.RateLimitingInterface

	// Background process control
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSessionTracker creates a new session tracker with the given configuration and store
func NewSessionTracker(config *Config, store Store) SessionTracker {
	if config == nil {
		config = DefaultConfig()
	}

	tracker := &sessionTracker{
		config: config,
		store:  store,
		logger: config.Logger.WithName("session-tracker"),
		queue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		done:   make(chan struct{}),
	}

	return tracker
}

// StartSession begins tracking a new placement session
func (st *sessionTracker) StartSession(ctx context.Context, info *SessionInfo) error {
	if info == nil {
		return fmt.Errorf("session info cannot be nil")
	}
	if info.SessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Set default values
	now := time.Now()
	if info.CreatedAt.IsZero() {
		info.CreatedAt = now
	}
	if info.LastSeen.IsZero() {
		info.LastSeen = now
	}
	if info.ExpiresAt.IsZero() {
		info.ExpiresAt = now.Add(st.config.DefaultSessionTTL)
	}
	if info.State == "" {
		info.State = SessionStateActive
	}
	if info.Metadata == nil {
		info.Metadata = make(map[string]string)
	}

	// Store the session
	if err := st.store.Set(ctx, info.SessionID, info); err != nil {
		return fmt.Errorf("failed to store session %s: %w", info.SessionID, err)
	}

	st.logger.Info("Started tracking session", 
		"sessionID", info.SessionID,
		"cluster", info.LogicalCluster,
		"placement", info.PlacementName,
		"expiresAt", info.ExpiresAt)

	return nil
}

// UpdateSession updates an existing session's information
func (st *sessionTracker) UpdateSession(ctx context.Context, sessionID string, updates *SessionInfo) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if updates == nil {
		return fmt.Errorf("updates cannot be nil")
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Get existing session
	existing, err := st.store.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session %s: %w", sessionID, err)
	}
	if existing == nil {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Apply updates
	updated := *existing
	updated.LastSeen = time.Now()

	if updates.State != "" {
		updated.State = updates.State
	}
	if !updates.ExpiresAt.IsZero() {
		updated.ExpiresAt = updates.ExpiresAt
	}
	if updates.Metadata != nil {
		if updated.Metadata == nil {
			updated.Metadata = make(map[string]string)
		}
		for k, v := range updates.Metadata {
			updated.Metadata[k] = v
		}
	}

	// Store updated session
	if err := st.store.Set(ctx, sessionID, &updated); err != nil {
		return fmt.Errorf("failed to update session %s: %w", sessionID, err)
	}

	st.logger.V(2).Info("Updated session", 
		"sessionID", sessionID,
		"state", updated.State,
		"lastSeen", updated.LastSeen)

	return nil
}

// GetSession retrieves information about a specific session
func (st *sessionTracker) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	st.mu.RLock()
	defer st.mu.RUnlock()

	session, err := st.store.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session %s: %w", sessionID, err)
	}

	return session, nil
}

// ListSessions returns all sessions, optionally filtered by logical cluster
func (st *sessionTracker) ListSessions(ctx context.Context, cluster logicalcluster.Name) ([]*SessionInfo, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	sessions, err := st.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	// Filter by cluster if specified
	if cluster != "" {
		filtered := make([]*SessionInfo, 0, len(sessions))
		for _, session := range sessions {
			if session.LogicalCluster == cluster {
				filtered = append(filtered, session)
			}
		}
		return filtered, nil
	}

	return sessions, nil
}

// ExpireSession marks a session as expired
func (st *sessionTracker) ExpireSession(ctx context.Context, sessionID string) error {
	return st.updateSessionState(ctx, sessionID, SessionStateExpired)
}

// TerminateSession terminates a session
func (st *sessionTracker) TerminateSession(ctx context.Context, sessionID string) error {
	return st.updateSessionState(ctx, sessionID, SessionStateTerminating)
}

// updateSessionState is a helper to update session state
func (st *sessionTracker) updateSessionState(ctx context.Context, sessionID string, state SessionState) error {
	updates := &SessionInfo{State: state}
	return st.UpdateSession(ctx, sessionID, updates)
}

// Cleanup removes expired sessions
func (st *sessionTracker) Cleanup(ctx context.Context) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	sessions, err := st.store.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions for cleanup: %w", err)
	}

	removed := 0
	for _, session := range sessions {
		shouldRemove := false

		switch session.State {
		case SessionStateExpired, SessionStateTerminating:
			// Remove expired or terminating sessions after grace period
			if now.After(session.ExpiresAt.Add(time.Minute)) {
				shouldRemove = true
			}
		case SessionStateActive, SessionStateInactive:
			// Remove sessions that have expired
			if now.After(session.ExpiresAt) {
				shouldRemove = true
			}
		}

		if shouldRemove {
			if err := st.store.Delete(ctx, session.SessionID); err != nil {
				st.logger.Error(err, "Failed to remove expired session", "sessionID", session.SessionID)
				continue
			}
			removed++
			st.logger.V(1).Info("Removed expired session", 
				"sessionID", session.SessionID,
				"state", session.State,
				"expiredAt", session.ExpiresAt)
		}
	}

	if removed > 0 {
		st.logger.Info("Cleanup completed", "removedSessions", removed)
	}

	return nil
}

// Start begins the session tracker background processes
func (st *sessionTracker) Start(ctx context.Context) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.ctx != nil {
		return fmt.Errorf("session tracker is already started")
	}

	st.ctx, st.cancel = context.WithCancel(ctx)
	
	// Start cleanup worker
	go st.runCleanup()

	st.logger.Info("Session tracker started")
	return nil
}

// Stop shuts down the session tracker
func (st *sessionTracker) Stop(ctx context.Context) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.ctx == nil {
		return nil // Already stopped
	}

	st.cancel()
	st.queue.ShutDown()

	// Wait for background processes to stop
	select {
	case <-st.done:
		st.logger.Info("Session tracker stopped")
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for session tracker to stop: %w", ctx.Err())
	}

	st.ctx = nil
	st.cancel = nil

	return nil
}

// runCleanup runs periodic cleanup of expired sessions
func (st *sessionTracker) runCleanup() {
	defer close(st.done)

	ticker := time.NewTicker(st.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-st.ctx.Done():
			return
		case <-ticker.C:
			if err := st.Cleanup(st.ctx); err != nil {
				st.logger.Error(err, "Failed to run session cleanup")
			}
		}
	}
}