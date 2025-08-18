package decision

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpfakeclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

// Mock implementations for testing

type mockMetricsCollector struct {
	recordedDecisions []DecisionRecord
	recordedLatencies []time.Duration
	recordedErrors    []string
	recordedViolations []PolicyViolation
}

func (m *mockMetricsCollector) RecordDecision(ctx context.Context, record *DecisionRecord) {
	m.recordedDecisions = append(m.recordedDecisions, *record)
}

func (m *mockMetricsCollector) RecordDecisionLatency(duration time.Duration) {
	m.recordedLatencies = append(m.recordedLatencies, duration)
}

func (m *mockMetricsCollector) RecordDecisionError(errorType string) {
	m.recordedErrors = append(m.recordedErrors, errorType)
}

func (m *mockMetricsCollector) RecordPolicyViolation(violation PolicyViolation) {
	m.recordedViolations = append(m.recordedViolations, violation)
}

type mockDecisionStorage struct {
	decisions map[string]*DecisionRecord
	errors    map[string]error
}

func newMockDecisionStorage() *mockDecisionStorage {
	return &mockDecisionStorage{
		decisions: make(map[string]*DecisionRecord),
		errors:    make(map[string]error),
	}
}

func (m *mockDecisionStorage) StoreDecision(ctx context.Context, record *DecisionRecord) error {
	if err, exists := m.errors["store"]; exists {
		return err
	}
	m.decisions[record.ID] = record
	return nil
}

func (m *mockDecisionStorage) GetDecision(ctx context.Context, id string) (*DecisionRecord, error) {
	if err, exists := m.errors["get"]; exists {
		return nil, err
	}
	if record, exists := m.decisions[id]; exists {
		return record, nil
	}
	return nil, errors.New("decision not found")
}

func (m *mockDecisionStorage) ListDecisions(ctx context.Context, placement string) ([]*DecisionRecord, error) {
	if err, exists := m.errors["list"]; exists {
		return nil, err
	}
	var results []*DecisionRecord
	for _, record := range m.decisions {
		if record.Placement.Name == placement {
			results = append(results, record)
		}
	}
	return results, nil
}

func (m *mockDecisionStorage) DeleteDecision(ctx context.Context, id string) error {
	if err, exists := m.errors["delete"]; exists {
		return err
	}
	delete(m.decisions, id)
	return nil
}

func (m *mockDecisionStorage) PurgeOldDecisions(ctx context.Context, before time.Time) error {
	if err, exists := m.errors["purge"]; exists {
		return err
	}
	for id, record := range m.decisions {
		if record.Timestamp.Before(before) {
			delete(m.decisions, id)
		}
	}
	return nil
}

func (m *mockDecisionStorage) setError(operation string, err error) {
	m.errors[operation] = err
}

type mockDecisionValidator struct {
	validateDecisionError    error
	validateConstraintsError error
}

func (m *mockDecisionValidator) ValidateDecision(
	ctx context.Context,
	placement *placementv1alpha1.WorkloadPlacement,
	decision *placementv1alpha1.PlacementDecision,
) error {
	return m.validateDecisionError
}

func (m *mockDecisionValidator) ValidateConstraints(
	ctx context.Context,
	constraints *placementv1alpha1.SchedulingConstraint,
) error {
	return m.validateConstraintsError
}

// Test helper functions

func createTestPlacement() *placementv1alpha1.WorkloadPlacement {
	return &placementv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "test-namespace",
		},
		Spec: placementv1alpha1.WorkloadPlacementSpec{
			Strategy: placementv1alpha1.PlacementStrategyBestFit,
			ResourceRequirements: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			Constraints: &placementv1alpha1.SchedulingConstraint{
				MinClusters: pointer.Int32(1),
				MaxClusters: pointer.Int32(3),
			},
		},
	}
}

func createTestDecision() *placementv1alpha1.PlacementDecision {
	return &placementv1alpha1.PlacementDecision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-decision",
		},
		Spec: placementv1alpha1.PlacementDecisionSpec{
			WorkloadPlacement: "test-placement",
			Clusters: []placementv1alpha1.ClusterDecision{
				{
					ClusterName: "cluster-1",
					Weight:      100,
				},
				{
					ClusterName: "cluster-2",
					Weight:      50,
				},
			},
		},
	}
}

func createTestCandidates() []*CandidateTarget {
	return []*CandidateTarget{
		{
			SyncTarget: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1",
				},
			},
			Workspace: "workspace-1",
			Score:     95.0,
			Reasons:   []string{"high availability"},
		},
		{
			SyncTarget: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-2",
				},
			},
			Workspace: "workspace-2",
			Score:     80.0,
			Reasons:   []string{"adequate resources"},
			Violations: []PolicyViolation{
				{
					Policy:   "resource-policy",
					Rule:     "cpu-limit",
					Message:  "CPU usage above 80%",
					Severity: ViolationSeverityWarning,
				},
			},
		},
	}
}

// Test cases

func TestNewDecisionRecorder(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	if recorder.kcpClient == nil {
		t.Error("Expected kcpClient to be set")
	}
	if recorder.eventRecorder == nil {
		t.Error("Expected eventRecorder to be set")
	}
	if recorder.storage == nil {
		t.Error("Expected storage to be set")
	}
	if recorder.metricsCollector == nil {
		t.Error("Expected metricsCollector to be set")
	}
	if recorder.decisionHistory == nil {
		t.Error("Expected decisionHistory to be initialized")
	}
	if recorder.maxHistorySize != 100 {
		t.Errorf("Expected maxHistorySize to be 100, got %d", recorder.maxHistorySize)
	}
}

func TestRecordDecision(t *testing.T) {
	tests := []struct {
		name           string
		placement      *placementv1alpha1.WorkloadPlacement
		decision       *placementv1alpha1.PlacementDecision
		candidates     []*CandidateTarget
		duration       time.Duration
		expectedStatus DecisionStatus
		expectError    bool
	}{
		{
			name:           "successful decision",
			placement:      createTestPlacement(),
			decision:       createTestDecision(),
			candidates:     createTestCandidates(),
			duration:       time.Millisecond * 100,
			expectedStatus: DecisionStatusScheduled,
			expectError:    false,
		},
		{
			name:       "decision with policy violations",
			placement:  createTestPlacement(),
			decision:   createTestDecision(),
			candidates: createTestCandidates(),
			duration:   time.Millisecond * 150,
			expectedStatus: DecisionStatusScheduled, // Warning-level violations don't reject
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpfakeclientset.NewSimpleClientset()
			eventRecorder := record.NewFakeRecorder(100)
			storage := newMockDecisionStorage()
			metrics := &mockMetricsCollector{}

			recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

			err := recorder.RecordDecision(context.TODO(), tt.placement, tt.decision, tt.candidates, tt.duration)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check that decision was stored
			history := recorder.GetDecisionHistory(tt.placement.Name)
			if len(history) != 1 {
				t.Errorf("Expected 1 decision in history, got %d", len(history))
			} else {
				record := history[0]
				if record.Status != tt.expectedStatus {
					t.Errorf("Expected status %s, got %s", tt.expectedStatus, record.Status)
				}
				if record.Duration != tt.duration {
					t.Errorf("Expected duration %v, got %v", tt.duration, record.Duration)
				}
			}

			// Check metrics were recorded
			if len(metrics.recordedDecisions) != 1 {
				t.Errorf("Expected 1 recorded decision, got %d", len(metrics.recordedDecisions))
			}
			if len(metrics.recordedLatencies) != 1 {
				t.Errorf("Expected 1 recorded latency, got %d", len(metrics.recordedLatencies))
			}

			// Check storage
			if len(storage.decisions) != 1 {
				t.Errorf("Expected 1 decision in storage, got %d", len(storage.decisions))
			}
		})
	}
}

func TestRecordFailedDecision(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	candidates := createTestCandidates()
	testErr := errors.New("no feasible targets")
	duration := time.Millisecond * 200

	err := recorder.RecordFailedDecision(context.TODO(), placement, testErr, candidates, duration)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check history
	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) != 1 {
		t.Errorf("Expected 1 decision in history, got %d", len(history))
	} else {
		record := history[0]
		if record.Status != DecisionStatusFailed {
			t.Errorf("Expected status %s, got %s", DecisionStatusFailed, record.Status)
		}
		if record.Error != testErr.Error() {
			t.Errorf("Expected error %q, got %q", testErr.Error(), record.Error)
		}
	}

	// Check metrics
	if len(metrics.recordedErrors) != 1 {
		t.Errorf("Expected 1 recorded error, got %d", len(metrics.recordedErrors))
	}
	if metrics.recordedErrors[0] != "no_feasible" {
		t.Errorf("Expected error type 'no_feasible', got %s", metrics.recordedErrors[0])
	}
}

func TestGetDecisionHistory(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Record multiple decisions
	for i := 0; i < 3; i++ {
		err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
		if err != nil {
			t.Fatalf("Failed to record decision %d: %v", i, err)
		}
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) != 3 {
		t.Errorf("Expected 3 decisions in history, got %d", len(history))
	}

	// Test with non-existent placement
	emptyHistory := recorder.GetDecisionHistory("non-existent")
	if len(emptyHistory) != 0 {
		t.Errorf("Expected empty history for non-existent placement, got %d decisions", len(emptyHistory))
	}
}

func TestGetLatestDecision(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Test with no decisions
	latest := recorder.GetLatestDecision(placement.Name)
	if latest != nil {
		t.Error("Expected nil for placement with no decisions")
	}

	// Record decisions
	for i := 0; i < 3; i++ {
		err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
		if err != nil {
			t.Fatalf("Failed to record decision %d: %v", i, err)
		}
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	latest = recorder.GetLatestDecision(placement.Name)
	if latest == nil {
		t.Error("Expected latest decision, got nil")
	} else {
		history := recorder.GetDecisionHistory(placement.Name)
		if latest.ID != history[len(history)-1].ID {
			t.Error("Latest decision ID doesn't match last in history")
		}
	}
}

func TestGetDecisionByID(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Record a decision
	err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
	if err != nil {
		t.Fatalf("Failed to record decision: %v", err)
	}

	// Get the ID from history
	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) == 0 {
		t.Fatal("No decisions in history")
	}
	recordID := history[0].ID

	// Test retrieval by ID
	retrieved, err := recorder.GetDecisionByID(context.TODO(), recordID)
	if err != nil {
		t.Errorf("Failed to get decision by ID: %v", err)
	}
	if retrieved == nil {
		t.Error("Expected decision, got nil")
	} else if retrieved.ID != recordID {
		t.Errorf("Expected ID %s, got %s", recordID, retrieved.ID)
	}

	// Test non-existent ID
	_, err = recorder.GetDecisionByID(context.TODO(), "non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent ID")
	}
}

func TestPurgeOldDecisions(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)
	recorder.retentionTime = time.Hour // Set short retention for testing

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Record some decisions and manually set old timestamps
	err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
	if err != nil {
		t.Fatalf("Failed to record decision: %v", err)
	}

	// Manually set an old timestamp in history
	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) > 0 {
		// Make it older than retention time
		recorder.historyMutex.Lock()
		recorder.decisionHistory[placement.Name][0].Timestamp = time.Now().Add(-2 * time.Hour)
		recorder.historyMutex.Unlock()

		// Also add to storage with old timestamp
		oldRecord := *history[0]
		oldRecord.Timestamp = time.Now().Add(-2 * time.Hour)
		storage.decisions["old-decision"] = &oldRecord
	}

	// Record a recent decision
	err = recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
	if err != nil {
		t.Fatalf("Failed to record recent decision: %v", err)
	}

	// Purge old decisions
	err = recorder.PurgeOldDecisions(context.TODO())
	if err != nil {
		t.Errorf("Failed to purge old decisions: %v", err)
	}

	// Check that old decisions were removed
	history = recorder.GetDecisionHistory(placement.Name)
	if len(history) != 1 {
		t.Errorf("Expected 1 decision after purge, got %d", len(history))
	}

	// Check storage was purged (old decision should be removed)
	if len(storage.decisions) < 1 {
		t.Error("Expected at least 1 decision in storage after purge")
	}
}

func TestHistorySizeLimit(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)
	recorder.maxHistorySize = 3 // Set small limit for testing

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Record more decisions than the limit
	for i := 0; i < 5; i++ {
		err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
		if err != nil {
			t.Fatalf("Failed to record decision %d: %v", i, err)
		}
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) != 3 {
		t.Errorf("Expected history size to be limited to 3, got %d", len(history))
	}
}

func TestDecisionValidator(t *testing.T) {
	tests := []struct {
		name        string
		placement   *placementv1alpha1.WorkloadPlacement
		decision    *placementv1alpha1.PlacementDecision
		expectError bool
	}{
		{
			name:        "valid decision",
			placement:   createTestPlacement(),
			decision:    createTestDecision(),
			expectError: false,
		},
		{
			name:      "mismatched placement name",
			placement: createTestPlacement(),
			decision: &placementv1alpha1.PlacementDecision{
				Spec: placementv1alpha1.PlacementDecisionSpec{
					WorkloadPlacement: "different-name",
				},
			},
			expectError: true,
		},
		{
			name: "too few clusters",
			placement: &placementv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: placementv1alpha1.WorkloadPlacementSpec{
					Constraints: &placementv1alpha1.SchedulingConstraint{
						MinClusters: pointer.Int32(3),
					},
				},
			},
			decision: &placementv1alpha1.PlacementDecision{
				Spec: placementv1alpha1.PlacementDecisionSpec{
					WorkloadPlacement: "test-placement",
					Clusters: []placementv1alpha1.ClusterDecision{
						{ClusterName: "cluster-1"},
					},
				},
			},
			expectError: true,
		},
		{
			name: "too many clusters",
			placement: &placementv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: placementv1alpha1.WorkloadPlacementSpec{
					Constraints: &placementv1alpha1.SchedulingConstraint{
						MaxClusters: pointer.Int32(1),
					},
				},
			},
			decision: &placementv1alpha1.PlacementDecision{
				Spec: placementv1alpha1.PlacementDecisionSpec{
					WorkloadPlacement: "test-placement",
					Clusters: []placementv1alpha1.ClusterDecision{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
				},
			},
			expectError: true,
		},
	}

	validator := NewDecisionValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDecision(context.TODO(), tt.placement, tt.decision)
			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestConstraintsValidator(t *testing.T) {
	tests := []struct {
		name        string
		constraints *placementv1alpha1.SchedulingConstraint
		expectError bool
	}{
		{
			name:        "nil constraints",
			constraints: nil,
			expectError: false,
		},
		{
			name: "valid constraints",
			constraints: &placementv1alpha1.SchedulingConstraint{
				MinClusters: pointer.Int32(1),
				MaxClusters: pointer.Int32(3),
			},
			expectError: false,
		},
		{
			name: "invalid constraints - min > max",
			constraints: &placementv1alpha1.SchedulingConstraint{
				MinClusters: pointer.Int32(5),
				MaxClusters: pointer.Int32(3),
			},
			expectError: true,
		},
	}

	validator := NewDecisionValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateConstraints(context.TODO(), tt.constraints)
			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestErrorClassification(t *testing.T) {
	recorder := &DecisionRecorder{}

	tests := []struct {
		error    error
		expected string
	}{
		{errors.New("no candidates available"), "no_candidates"},
		{errors.New("no feasible targets found"), "no_feasible"},
		{errors.New("policy violation detected"), "policy_violation"},
		{errors.New("insufficient resources"), "insufficient_resources"},
		{errors.New("unknown error type"), "unknown"},
	}

	for _, tt := range tests {
		result := recorder.classifyError(tt.error)
		if result != tt.expected {
			t.Errorf("Expected error classification %s, got %s for error: %v",
				tt.expected, result, tt.error)
		}
	}
}

func TestStorageIntegration(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Test storage error handling
	storage.setError("store", errors.New("storage unavailable"))

	err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
	// Should not fail even if storage fails
	if err != nil {
		t.Errorf("Expected no error despite storage failure, got: %v", err)
	}

	// Check that decision was still recorded in memory
	history := recorder.GetDecisionHistory(placement.Name)
	if len(history) != 1 {
		t.Errorf("Expected 1 decision in history despite storage failure, got %d", len(history))
	}
}

// Benchmark tests

func BenchmarkRecordDecision(b *testing.B) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(1000)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
		if err != nil {
			b.Fatalf("Failed to record decision: %v", err)
		}
	}
}

func BenchmarkGetDecisionHistory(b *testing.B) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(1000)
	storage := newMockDecisionStorage()
	metrics := &mockMetricsCollector{}

	recorder := NewDecisionRecorder(kcpClient.Cluster(""), eventRecorder, storage, metrics)

	placement := createTestPlacement()
	decision := createTestDecision()
	candidates := createTestCandidates()

	// Pre-populate with some decisions
	for i := 0; i < 100; i++ {
		err := recorder.RecordDecision(context.TODO(), placement, decision, candidates, time.Millisecond*100)
		if err != nil {
			b.Fatalf("Failed to record decision: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = recorder.GetDecisionHistory(placement.Name)
	}
}