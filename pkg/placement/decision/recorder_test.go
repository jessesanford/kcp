/*
Copyright 2023 The KCP Authors.

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

package decision

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/record"

	"github.com/kcp-dev/logicalcluster/v3"
)

// MockDecisionStorage implements DecisionStorage for testing.
type MockDecisionStorage struct {
	records           map[string]*DecisionRecord
	attempts          map[string]*DecisionAttempt
	storeError        error
	storeAttemptError error
	queryError        error
	metricsError      error
	purgeError        error
	purgeCount        int
}

func NewMockDecisionStorage() *MockDecisionStorage {
	return &MockDecisionStorage{
		records:  make(map[string]*DecisionRecord),
		attempts: make(map[string]*DecisionAttempt),
	}
}

func (m *MockDecisionStorage) Store(ctx context.Context, record *DecisionRecord) error {
	if m.storeError != nil {
		return m.storeError
	}
	m.records[record.DecisionID] = record
	return nil
}

func (m *MockDecisionStorage) StoreAttempt(ctx context.Context, attempt *DecisionAttempt) error {
	if m.storeAttemptError != nil {
		return m.storeAttemptError
	}
	m.attempts[attempt.ID] = attempt
	return nil
}

func (m *MockDecisionStorage) Query(ctx context.Context, query *HistoryQuery) ([]*DecisionRecord, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	
	var results []*DecisionRecord
	for _, record := range m.records {
		if m.matchesQuery(record, query) {
			results = append(results, record)
		}
	}
	
	// Apply limit
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}
	
	return results, nil
}

func (m *MockDecisionStorage) GetMetrics(ctx context.Context, timeRange TimeRange) (*DecisionMetrics, error) {
	if m.metricsError != nil {
		return nil, m.metricsError
	}
	
	metrics := &DecisionMetrics{
		TimeRange:           timeRange,
		DecisionsByStatus:   make(map[DecisionStatus]int64),
		WorkspaceUtilization: make(map[logicalcluster.Name]*WorkspaceUtilizationMetrics),
	}
	
	for _, record := range m.records {
		if record.Decision != nil {
			metrics.TotalDecisions++
			metrics.DecisionsByStatus[record.Decision.Status]++
			
			if record.Decision.Status == DecisionStatusComplete {
				metrics.SuccessfulDecisions++
			} else if record.Decision.Status == DecisionStatusError {
				metrics.FailedDecisions++
			}
		}
	}
	
	return metrics, nil
}

func (m *MockDecisionStorage) Purge(ctx context.Context, policy *RetentionPolicy) (int, error) {
	if m.purgeError != nil {
		return 0, m.purgeError
	}
	return m.purgeCount, nil
}

func (m *MockDecisionStorage) matchesQuery(record *DecisionRecord, query *HistoryQuery) bool {
	if query.RequestID != "" && record.RequestID != query.RequestID {
		return false
	}
	if len(query.DecisionIDs) > 0 {
		found := false
		for _, id := range query.DecisionIDs {
			if record.DecisionID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(query.StatusFilter) > 0 && record.Decision != nil {
		found := false
		for _, status := range query.StatusFilter {
			if record.Decision.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// MockEventRecorder implements record.EventRecorder for testing.
type MockEventRecorder struct {
	events []MockEvent
}

type MockEvent struct {
	Object    interface{}
	EventType string
	Reason    string
	Message   string
}

func NewMockEventRecorder() *MockEventRecorder {
	return &MockEventRecorder{}
}

func (m *MockEventRecorder) Event(object interface{}, eventtype, reason, message string) {
	m.events = append(m.events, MockEvent{
		Object:    object,
		EventType: eventtype,
		Reason:    reason,
		Message:   message,
	})
}

func (m *MockEventRecorder) Eventf(object interface{}, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (m *MockEventRecorder) AnnotatedEventf(object interface{}, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func TestNewDecisionRecorder(t *testing.T) {
	tests := map[string]struct {
		storage       DecisionStorage
		eventRecorder record.EventRecorder
		config        *RecorderConfig
		wantError     bool
		errorContains string
	}{
		"valid configuration": {
			storage:       NewMockDecisionStorage(),
			eventRecorder: NewMockEventRecorder(),
			config:        DefaultRecorderConfig(),
			wantError:     false,
		},
		"nil storage": {
			storage:       nil,
			eventRecorder: NewMockEventRecorder(),
			config:        DefaultRecorderConfig(),
			wantError:     true,
			errorContains: "decision storage cannot be nil",
		},
		"nil event recorder": {
			storage:       NewMockDecisionStorage(),
			eventRecorder: nil,
			config:        DefaultRecorderConfig(),
			wantError:     true,
			errorContains: "event recorder cannot be nil",
		},
		"nil config uses default": {
			storage:       NewMockDecisionStorage(),
			eventRecorder: NewMockEventRecorder(),
			config:        nil,
			wantError:     false,
		},
		"invalid config": {
			storage:       NewMockDecisionStorage(),
			eventRecorder: NewMockEventRecorder(),
			config: &RecorderConfig{
				Version:         "", // Invalid: empty version
				DefaultTTL:      time.Hour,
				CleanupInterval: time.Hour,
				CleanupTimeout:  time.Minute,
			},
			wantError:     true,
			errorContains: "invalid recorder configuration",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			recorder, err := NewDecisionRecorder(tc.storage, tc.eventRecorder, tc.config)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, recorder)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, recorder)
			}
		})
	}
}

func TestDecisionRecorder_RecordDecision(t *testing.T) {
	tests := map[string]struct {
		decision      *PlacementDecision
		storageError  error
		wantError     bool
		errorContains string
	}{
		"successful decision recording": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
				SelectedWorkspaces: []*WorkspacePlacement{
					{
						Workspace:  logicalcluster.Name("root:test"),
						FinalScore: 95.0,
					},
				},
				DecisionRationale: DecisionRationale{
					Summary: "Selected workspace based on resource availability",
				},
				DecisionTime:     time.Now(),
				DecisionDuration: 100 * time.Millisecond,
			},
			wantError: false,
		},
		"nil decision": {
			decision:      nil,
			wantError:     true,
			errorContains: "decision cannot be nil",
		},
		"storage error": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
			},
			storageError:  errors.New("storage failure"),
			wantError:     true,
			errorContains: "failed to store decision record",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			storage.storeError = tc.storageError
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			err = recorder.RecordDecision(ctx, tc.decision)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				
				// Verify the decision was stored
				assert.Len(t, storage.records, 1)
				stored := storage.records[tc.decision.ID]
				assert.Equal(t, tc.decision.ID, stored.DecisionID)
				assert.Equal(t, tc.decision.RequestID, stored.RequestID)
				assert.Equal(t, tc.decision, stored.Decision)
			}
		})
	}
}

func TestDecisionRecorder_RecordDecisionAttempt(t *testing.T) {
	tests := map[string]struct {
		attempt       *DecisionAttempt
		storageError  error
		wantError     bool
		errorContains string
	}{
		"successful attempt recording": {
			attempt: &DecisionAttempt{
				ID:        "attempt-1",
				RequestID: "request-1",
				StartTime: time.Now(),
				EndTime:   time.Now().Add(100 * time.Millisecond),
				Duration:  100 * time.Millisecond,
				Success:   true,
				Phase:     PhaseFinalization,
				Workspace: logicalcluster.Name("root:test"),
			},
			wantError: false,
		},
		"nil attempt": {
			attempt:       nil,
			wantError:     true,
			errorContains: "decision attempt cannot be nil",
		},
		"storage error": {
			attempt: &DecisionAttempt{
				ID:        "attempt-1",
				RequestID: "request-1",
				Success:   false,
			},
			storageError:  errors.New("attempt storage failure"),
			wantError:     true,
			errorContains: "failed to store decision attempt",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			storage.storeAttemptError = tc.storageError
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			err = recorder.RecordDecisionAttempt(ctx, tc.attempt)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				
				// Verify the attempt was stored
				assert.Len(t, storage.attempts, 1)
				stored := storage.attempts[tc.attempt.ID]
				assert.Equal(t, tc.attempt.ID, stored.ID)
				assert.Equal(t, tc.attempt.RequestID, stored.RequestID)
			}
		})
	}
}

func TestDecisionRecorder_QueryDecisionHistory(t *testing.T) {
	tests := map[string]struct {
		query         *HistoryQuery
		setupRecords  func(*MockDecisionStorage)
		queryError    error
		wantError     bool
		errorContains string
		wantCount     int
	}{
		"successful query": {
			query: &HistoryQuery{
				RequestID: "request-1",
				Limit:     10,
			},
			setupRecords: func(storage *MockDecisionStorage) {
				storage.records["decision-1"] = &DecisionRecord{
					DecisionID: "decision-1",
					RequestID:  "request-1",
					Decision:   &PlacementDecision{Status: DecisionStatusComplete},
				}
				storage.records["decision-2"] = &DecisionRecord{
					DecisionID: "decision-2",
					RequestID:  "request-2",
					Decision:   &PlacementDecision{Status: DecisionStatusComplete},
				}
			},
			wantCount: 1,
			wantError: false,
		},
		"nil query": {
			query:         nil,
			wantError:     true,
			errorContains: "history query cannot be nil",
		},
		"invalid query": {
			query: &HistoryQuery{
				Limit: -1, // Invalid
			},
			wantError:     true,
			errorContains: "invalid history query",
		},
		"storage query error": {
			query: &HistoryQuery{
				RequestID: "request-1",
			},
			queryError:    errors.New("query failure"),
			wantError:     true,
			errorContains: "failed to query decision history",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			if tc.setupRecords != nil {
				tc.setupRecords(storage)
			}
			storage.queryError = tc.queryError
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			records, err := recorder.QueryDecisionHistory(ctx, tc.query)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, records)
			} else {
				require.NoError(t, err)
				assert.Len(t, records, tc.wantCount)
			}
		})
	}
}

func TestDecisionRecorder_GetDecisionMetrics(t *testing.T) {
	tests := map[string]struct {
		timeRange     TimeRange
		setupRecords  func(*MockDecisionStorage)
		metricsError  error
		wantError     bool
		errorContains string
	}{
		"successful metrics retrieval": {
			timeRange: TimeRange{
				Start: time.Now().Add(-time.Hour),
				End:   time.Now(),
			},
			setupRecords: func(storage *MockDecisionStorage) {
				storage.records["decision-1"] = &DecisionRecord{
					Decision: &PlacementDecision{Status: DecisionStatusComplete},
				}
			},
			wantError: false,
		},
		"invalid time range": {
			timeRange: TimeRange{
				Start: time.Now(),
				End:   time.Now().Add(-time.Hour), // End before start
			},
			wantError:     true,
			errorContains: "invalid time range",
		},
		"storage metrics error": {
			timeRange: TimeRange{
				Start: time.Now().Add(-time.Hour),
				End:   time.Now(),
			},
			metricsError:  errors.New("metrics failure"),
			wantError:     true,
			errorContains: "failed to get decision metrics",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			if tc.setupRecords != nil {
				tc.setupRecords(storage)
			}
			storage.metricsError = tc.metricsError
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			metrics, err := recorder.GetDecisionMetrics(ctx, tc.timeRange)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, metrics)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, metrics)
				assert.Equal(t, tc.timeRange, metrics.TimeRange)
			}
		})
	}
}

func TestDecisionRecorder_PurgeOldRecords(t *testing.T) {
	tests := map[string]struct {
		retentionPolicy *RetentionPolicy
		purgeCount      int
		purgeError      error
		wantError       bool
		errorContains   string
	}{
		"successful purge": {
			retentionPolicy: &RetentionPolicy{
				DefaultTTL:            24 * time.Hour,
				SuccessfulDecisionTTL: 7 * 24 * time.Hour,
				FailedDecisionTTL:     30 * 24 * time.Hour,
				AttemptTTL:            24 * time.Hour,
				MaxRecords:            1000,
				PurgeInterval:         time.Hour,
			},
			purgeCount: 5,
			wantError:  false,
		},
		"invalid retention policy": {
			retentionPolicy: &RetentionPolicy{
				DefaultTTL: -time.Hour, // Invalid
			},
			wantError:     true,
			errorContains: "invalid retention policy",
		},
		"storage purge error": {
			retentionPolicy: &RetentionPolicy{
				DefaultTTL:            24 * time.Hour,
				SuccessfulDecisionTTL: 7 * 24 * time.Hour,
				FailedDecisionTTL:     30 * 24 * time.Hour,
				AttemptTTL:            24 * time.Hour,
				MaxRecords:            1000,
				PurgeInterval:         time.Hour,
			},
			purgeError:    errors.New("purge failure"),
			wantError:     true,
			errorContains: "failed to purge old records",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			storage.purgeCount = tc.purgeCount
			storage.purgeError = tc.purgeError
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			err = recorder.PurgeOldRecords(ctx, tc.retentionPolicy)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDecisionRecorder_EmitDecisionEvent(t *testing.T) {
	tests := map[string]struct {
		decision      *PlacementDecision
		eventType     DecisionEventType
		reason        string
		message       string
		wantError     bool
		errorContains string
	}{
		"successful event emission": {
			decision: &PlacementDecision{
				ID:        "decision-1",
				RequestID: "request-1",
				Status:    DecisionStatusComplete,
			},
			eventType: DecisionEventTypeNormal,
			reason:    "DecisionRecorded",
			message:   "Decision recorded successfully",
			wantError: false,
		},
		"nil decision": {
			decision:      nil,
			eventType:     DecisionEventTypeNormal,
			reason:        "DecisionRecorded",
			message:       "Decision recorded successfully",
			wantError:     true,
			errorContains: "decision cannot be nil",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewMockDecisionStorage()
			eventRecorder := NewMockEventRecorder()
			
			recorder, err := NewDecisionRecorder(storage, eventRecorder, DefaultRecorderConfig())
			require.NoError(t, err)
			
			ctx := context.Background()
			err = recorder.EmitDecisionEvent(ctx, tc.decision, tc.eventType, tc.reason, tc.message)
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRetentionPolicy_Validate(t *testing.T) {
	tests := map[string]struct {
		policy        *RetentionPolicy
		wantError     bool
		errorContains string
	}{
		"valid policy": {
			policy: &RetentionPolicy{
				DefaultTTL:            24 * time.Hour,
				SuccessfulDecisionTTL: 7 * 24 * time.Hour,
				FailedDecisionTTL:     30 * 24 * time.Hour,
				AttemptTTL:            24 * time.Hour,
				MaxRecords:            1000,
				PurgeInterval:         time.Hour,
			},
			wantError: false,
		},
		"invalid default TTL": {
			policy: &RetentionPolicy{
				DefaultTTL: 0, // Invalid
				SuccessfulDecisionTTL: 7 * 24 * time.Hour,
				FailedDecisionTTL:     30 * 24 * time.Hour,
				AttemptTTL:            24 * time.Hour,
				MaxRecords:            1000,
				PurgeInterval:         time.Hour,
			},
			wantError:     true,
			errorContains: "default TTL must be positive",
		},
		"invalid max records": {
			policy: &RetentionPolicy{
				DefaultTTL:            24 * time.Hour,
				SuccessfulDecisionTTL: 7 * 24 * time.Hour,
				FailedDecisionTTL:     30 * 24 * time.Hour,
				AttemptTTL:            24 * time.Hour,
				MaxRecords:            0, // Invalid
				PurgeInterval:         time.Hour,
			},
			wantError:     true,
			errorContains: "max records must be positive",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.policy.Validate()
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRecorderConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config        *RecorderConfig
		wantError     bool
		errorContains string
	}{
		"valid config": {
			config: &RecorderConfig{
				Version:         "1.0.0",
				DefaultTTL:      24 * time.Hour,
				CleanupInterval: time.Hour,
				CleanupTimeout:  5 * time.Minute,
			},
			wantError: false,
		},
		"empty version": {
			config: &RecorderConfig{
				Version:         "", // Invalid
				DefaultTTL:      24 * time.Hour,
				CleanupInterval: time.Hour,
				CleanupTimeout:  5 * time.Minute,
			},
			wantError:     true,
			errorContains: "version cannot be empty",
		},
		"invalid cleanup interval": {
			config: &RecorderConfig{
				Version:         "1.0.0",
				DefaultTTL:      24 * time.Hour,
				CleanupInterval: 0, // Invalid
				CleanupTimeout:  5 * time.Minute,
			},
			wantError:     true,
			errorContains: "cleanup interval must be positive",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.config.Validate()
			
			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultRecorderConfig(t *testing.T) {
	config := DefaultRecorderConfig()
	
	assert.NotEmpty(t, config.Version)
	assert.True(t, config.DefaultTTL > 0)
	assert.True(t, config.CleanupInterval > 0)
	assert.True(t, config.CleanupTimeout > 0)
	assert.NotNil(t, config.DefaultRetentionPolicy)
	
	// Validate the default config
	err := config.Validate()
	assert.NoError(t, err)
}