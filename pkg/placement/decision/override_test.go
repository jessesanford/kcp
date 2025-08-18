package decision

import (
	"context"
	"errors"
	"testing"
	"time"

	placementv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/placement/v1alpha1"
	kcpfakeclientset "github.com/kcp-dev/kcp/pkg/client/clientset/versioned/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

// Mock implementations for testing

type mockOverrideMetricsCollector struct {
	recordedOverrides []ActiveOverride
	recordedLatencies []time.Duration
	recordedErrors    []string
	emergencyOverrides []SeverityLevel
}

func (m *mockOverrideMetricsCollector) RecordOverride(ctx context.Context, override *ActiveOverride) {
	m.recordedOverrides = append(m.recordedOverrides, *override)
}

func (m *mockOverrideMetricsCollector) RecordOverrideLatency(duration time.Duration) {
	m.recordedLatencies = append(m.recordedLatencies, duration)
}

func (m *mockOverrideMetricsCollector) RecordOverrideError(errorType string) {
	m.recordedErrors = append(m.recordedErrors, errorType)
}

func (m *mockOverrideMetricsCollector) RecordEmergencyOverride(severity SeverityLevel) {
	m.emergencyOverrides = append(m.emergencyOverrides, severity)
}

type mockOverrideAuthChecker struct {
	canCreateOverride  bool
	canApproveOverride bool
	requiresApproval   bool
	createError        error
	approveError       error
}

func (m *mockOverrideAuthChecker) CanCreateOverride(ctx context.Context, user string, policy *OverridePolicy) (bool, error) {
	return m.canCreateOverride, m.createError
}

func (m *mockOverrideAuthChecker) CanApproveOverride(ctx context.Context, user string, override *ActiveOverride) (bool, error) {
	return m.canApproveOverride, m.approveError
}

func (m *mockOverrideAuthChecker) RequiresApproval(ctx context.Context, override *ActiveOverride) bool {
	return m.requiresApproval
}

type mockOverrideValidator struct {
	validateOverrideError error
	validatePolicyError   error
}

func (m *mockOverrideValidator) ValidateOverride(ctx context.Context, override *ActiveOverride) error {
	return m.validateOverrideError
}

func (m *mockOverrideValidator) ValidatePolicy(ctx context.Context, policy *OverridePolicy) error {
	return m.validatePolicyError
}

type mockOverrideNotifier struct {
	createdNotifications   []ActiveOverride
	approvedNotifications  []ActiveOverride
	expiredNotifications   []ActiveOverride
	emergencyNotifications []ActiveOverride
	notificationErrors     map[string]error
}

func newMockOverrideNotifier() *mockOverrideNotifier {
	return &mockOverrideNotifier{
		notificationErrors: make(map[string]error),
	}
}

func (m *mockOverrideNotifier) NotifyOverrideCreated(ctx context.Context, override *ActiveOverride) error {
	m.createdNotifications = append(m.createdNotifications, *override)
	return m.notificationErrors["created"]
}

func (m *mockOverrideNotifier) NotifyOverrideApproved(ctx context.Context, override *ActiveOverride) error {
	m.approvedNotifications = append(m.approvedNotifications, *override)
	return m.notificationErrors["approved"]
}

func (m *mockOverrideNotifier) NotifyOverrideExpired(ctx context.Context, override *ActiveOverride) error {
	m.expiredNotifications = append(m.expiredNotifications, *override)
	return m.notificationErrors["expired"]
}

func (m *mockOverrideNotifier) NotifyEmergencyOverride(ctx context.Context, override *ActiveOverride) error {
	m.emergencyNotifications = append(m.emergencyNotifications, *override)
	return m.notificationErrors["emergency"]
}

func (m *mockOverrideNotifier) setError(notification string, err error) {
	m.notificationErrors[notification] = err
}

// Test helper functions

func createTestOverridePlacement() *placementv1alpha1.WorkloadPlacement {
	return &placementv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"environment": "production",
				"team":        "platform",
			},
		},
		Spec: placementv1alpha1.WorkloadPlacementSpec{
			Strategy: placementv1alpha1.PlacementStrategyBestFit,
		},
	}
}

func createTestOverrideDecision() *placementv1alpha1.PlacementDecision {
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
					Weight:      75,
				},
			},
		},
	}
}

func createTestOverridePolicy() *OverridePolicy {
	return &OverridePolicy{
		Name:      "test-policy",
		Namespace: "test-namespace",
		Priority:  100,
		Enabled:   true,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"environment": "production",
			},
		},
		Rules: []OverrideRule{
			{
				Name:      "cluster-selection-override",
				Type:      OverrideRuleTypeClusterSelection,
				Target:    OverrideTargetClusters,
				Operation: OverrideOperationReplace,
				Value:     []string{"cluster-3", "cluster-4"},
				Priority:  100,
			},
		},
		SeverityLevel: SeverityLevelMedium,
	}
}

func createTestOverrideSpec(emergency bool) *OverrideSpec {
	return &OverrideSpec{
		Reason:    "Emergency maintenance required",
		Emergency: emergency,
		Source:    "api",
		TTL:       pointer.Duration(2 * time.Hour),
	}
}

// Test cases

func TestNewDecisionOverride(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	override := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	if override.kcpClient == nil {
		t.Error("Expected kcpClient to be set")
	}
	if override.eventRecorder == nil {
		t.Error("Expected eventRecorder to be set")
	}
	if override.metricsCollector == nil {
		t.Error("Expected metricsCollector to be set")
	}
	if override.authChecker == nil {
		t.Error("Expected authChecker to be set")
	}
	if override.validator == nil {
		t.Error("Expected validator to be set")
	}
	if override.notifier == nil {
		t.Error("Expected notifier to be set")
	}
	if override.overridePolicies == nil {
		t.Error("Expected overridePolicies to be initialized")
	}
	if override.activeOverrides == nil {
		t.Error("Expected activeOverrides to be initialized")
	}
	if override.maxHistorySize != 1000 {
		t.Errorf("Expected maxHistorySize to be 1000, got %d", override.maxHistorySize)
	}
	if override.defaultTTL != 24*time.Hour {
		t.Errorf("Expected defaultTTL to be 24h, got %v", override.defaultTTL)
	}
}

func TestApplyOverride(t *testing.T) {
	tests := []struct {
		name                 string
		placement            *placementv1alpha1.WorkloadPlacement
		decision             *placementv1alpha1.PlacementDecision
		spec                 *OverrideSpec
		policy               *OverridePolicy
		userID               string
		canCreate            bool
		createError          error
		requiresApproval     bool
		validateError        error
		expectedStatus       OverrideStatus
		expectedClusters     []string
		expectError          bool
	}{
		{
			name:             "successful override",
			placement:        createTestOverridePlacement(),
			decision:         createTestOverrideDecision(),
			spec:             createTestOverrideSpec(false),
			policy:           createTestOverridePolicy(),
			userID:           "test-user",
			canCreate:        true,
			requiresApproval: false,
			expectedStatus:   OverrideStatusPending,
			expectedClusters: []string{"cluster-3", "cluster-4"},
			expectError:      false,
		},
		{
			name:             "emergency override",
			placement:        createTestOverridePlacement(),
			decision:         createTestOverrideDecision(),
			spec:             createTestOverrideSpec(true),
			policy:           createTestOverridePolicy(),
			userID:           "test-user",
			canCreate:        true,
			requiresApproval: false,
			expectedStatus:   OverrideStatusPending,
			expectedClusters: []string{"cluster-3", "cluster-4"},
			expectError:      false,
		},
		{
			name:             "override requiring approval",
			placement:        createTestOverridePlacement(),
			decision:         createTestOverrideDecision(),
			spec:             createTestOverrideSpec(false),
			policy:           createTestOverridePolicy(),
			userID:           "test-user",
			canCreate:        true,
			requiresApproval: true,
			expectedStatus:   OverrideStatusPending,
			expectError:      false,
		},
		{
			name:        "unauthorized user",
			placement:   createTestOverridePlacement(),
			decision:    createTestOverrideDecision(),
			spec:        createTestOverrideSpec(false),
			policy:      createTestOverridePolicy(),
			userID:      "unauthorized-user",
			canCreate:   false,
			expectError: true,
		},
		{
			name:          "authorization check error",
			placement:     createTestOverridePlacement(),
			decision:      createTestOverrideDecision(),
			spec:          createTestOverrideSpec(false),
			policy:        createTestOverridePolicy(),
			userID:        "test-user",
			createError:   errors.New("auth service unavailable"),
			expectError:   true,
		},
		{
			name:          "validation error",
			placement:     createTestOverridePlacement(),
			decision:      createTestOverrideDecision(),
			spec:          createTestOverrideSpec(false),
			policy:        createTestOverridePolicy(),
			userID:        "test-user",
			canCreate:     true,
			validateError: errors.New("invalid override"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kcpClient := kcpfakeclientset.NewSimpleClientset()
			eventRecorder := record.NewFakeRecorder(100)
			metrics := &mockOverrideMetricsCollector{}
			auth := &mockOverrideAuthChecker{
				canCreateOverride: tt.canCreate,
				createError:       tt.createError,
				requiresApproval:  tt.requiresApproval,
			}
			validator := &mockOverrideValidator{
				validateOverrideError: tt.validateError,
			}
			notifier := newMockOverrideNotifier()

			override := NewDecisionOverride(
				kcpClient.Cluster(""),
				eventRecorder,
				metrics,
				auth,
				validator,
				notifier,
			)

			// Add the policy
			override.overridePolicies[tt.policy.Name] = tt.policy

			modifiedDecision, activeOverride, err := override.ApplyOverride(
				context.TODO(),
				tt.placement,
				tt.decision,
				tt.spec,
				tt.userID,
			)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if modifiedDecision == nil {
					t.Error("Expected modified decision but got nil")
				}
				if activeOverride == nil {
					t.Error("Expected active override but got nil")
				} else {
					if activeOverride.Status != tt.expectedStatus {
						t.Errorf("Expected status %s, got %s", tt.expectedStatus, activeOverride.Status)
					}
					if activeOverride.Emergency != tt.spec.Emergency {
						t.Errorf("Expected emergency %v, got %v", tt.spec.Emergency, activeOverride.Emergency)
					}
					if activeOverride.UserID != tt.userID {
						t.Errorf("Expected user ID %s, got %s", tt.userID, activeOverride.UserID)
					}
				}

				// Check cluster selection was applied
				if len(tt.expectedClusters) > 0 && modifiedDecision != nil {
					if len(modifiedDecision.Spec.Clusters) != len(tt.expectedClusters) {
						t.Errorf("Expected %d clusters, got %d", len(tt.expectedClusters), len(modifiedDecision.Spec.Clusters))
					} else {
						for i, expected := range tt.expectedClusters {
							if modifiedDecision.Spec.Clusters[i].ClusterName != expected {
								t.Errorf("Expected cluster %s at index %d, got %s", expected, i, modifiedDecision.Spec.Clusters[i].ClusterName)
							}
						}
					}
				}

				// Check metrics were recorded
				if len(metrics.recordedOverrides) != 1 {
					t.Errorf("Expected 1 recorded override, got %d", len(metrics.recordedOverrides))
				}
				if tt.spec.Emergency && len(metrics.emergencyOverrides) != 1 {
					t.Errorf("Expected 1 emergency override recorded, got %d", len(metrics.emergencyOverrides))
				}

				// Check notifications were sent
				if tt.spec.Emergency {
					if len(notifier.emergencyNotifications) != 1 {
						t.Errorf("Expected 1 emergency notification, got %d", len(notifier.emergencyNotifications))
					}
				} else {
					if len(notifier.createdNotifications) != 1 {
						t.Errorf("Expected 1 created notification, got %d", len(notifier.createdNotifications))
					}
				}
			}
		})
	}
}

func TestApproveOverride(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{
		canCreateOverride:  true,
		canApproveOverride: true,
		requiresApproval:   true,
	}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	// Create an override that requires approval
	placement := createTestOverridePlacement()
	decision := createTestOverrideDecision()
	spec := createTestOverrideSpec(false)
	policy := createTestOverridePolicy()
	userID := "test-user"

	manager.overridePolicies[policy.Name] = policy

	_, activeOverride, err := manager.ApplyOverride(context.TODO(), placement, decision, spec, userID)
	if err != nil {
		t.Fatalf("Failed to create override: %v", err)
	}

	if activeOverride.Status != OverrideStatusPending {
		t.Errorf("Expected status Pending, got %s", activeOverride.Status)
	}

	// Approve the override
	approverID := "approver-user"
	err = manager.ApproveOverride(context.TODO(), activeOverride.ID, approverID)
	if err != nil {
		t.Errorf("Failed to approve override: %v", err)
	}

	// Check status was updated
	approved, err := manager.GetOverride(activeOverride.ID)
	if err != nil {
		t.Fatalf("Failed to get override: %v", err)
	}
	if approved.Status != OverrideStatusActive {
		t.Errorf("Expected status Active after approval, got %s", approved.Status)
	}
	if len(approved.ApprovedBy) != 1 || approved.ApprovedBy[0] != approverID {
		t.Errorf("Expected approver %s, got %v", approverID, approved.ApprovedBy)
	}
	if approved.ApprovedAt == nil {
		t.Error("Expected ApprovedAt to be set")
	}

	// Check notification was sent
	if len(notifier.approvedNotifications) != 1 {
		t.Errorf("Expected 1 approval notification, got %d", len(notifier.approvedNotifications))
	}
}

func TestRejectOverride(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{
		canCreateOverride:  true,
		canApproveOverride: true,
		requiresApproval:   true,
	}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	// Create an override that requires approval
	placement := createTestOverridePlacement()
	decision := createTestOverrideDecision()
	spec := createTestOverrideSpec(false)
	policy := createTestOverridePolicy()
	userID := "test-user"

	manager.overridePolicies[policy.Name] = policy

	_, activeOverride, err := manager.ApplyOverride(context.TODO(), placement, decision, spec, userID)
	if err != nil {
		t.Fatalf("Failed to create override: %v", err)
	}

	// Reject the override
	rejecterID := "rejecter-user"
	rejectReason := "Insufficient justification"
	err = manager.RejectOverride(context.TODO(), activeOverride.ID, rejecterID, rejectReason)
	if err != nil {
		t.Errorf("Failed to reject override: %v", err)
	}

	// Check status was updated
	rejected, err := manager.GetOverride(activeOverride.ID)
	if err != nil {
		t.Fatalf("Failed to get override: %v", err)
	}
	if rejected.Status != OverrideStatusRejected {
		t.Errorf("Expected status Rejected, got %s", rejected.Status)
	}
	if len(rejected.RejectedBy) != 1 || rejected.RejectedBy[0] != rejecterID {
		t.Errorf("Expected rejecter %s, got %v", rejecterID, rejected.RejectedBy)
	}
	if rejected.RejectedAt == nil {
		t.Error("Expected RejectedAt to be set")
	}
}

func TestRevertOverride(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{
		canCreateOverride: true,
		requiresApproval:  false,
	}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	// Create an active override
	placement := createTestOverridePlacement()
	decision := createTestOverrideDecision()
	spec := createTestOverrideSpec(false)
	policy := createTestOverridePolicy()
	userID := "test-user"

	manager.overridePolicies[policy.Name] = policy

	_, activeOverride, err := manager.ApplyOverride(context.TODO(), placement, decision, spec, userID)
	if err != nil {
		t.Fatalf("Failed to create override: %v", err)
	}

	// Mark as active (since no approval required)
	activeOverride.Status = OverrideStatusActive

	// Revert the override
	reverterID := "reverter-user"
	err = manager.RevertOverride(context.TODO(), activeOverride.ID, reverterID)
	if err != nil {
		t.Errorf("Failed to revert override: %v", err)
	}

	// Check status was updated
	reverted, err := manager.GetOverride(activeOverride.ID)
	if err != nil {
		t.Fatalf("Failed to get override: %v", err)
	}
	if reverted.Status != OverrideStatusReverted {
		t.Errorf("Expected status Reverted, got %s", reverted.Status)
	}
}

func TestGetActiveOverrides(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{canCreateOverride: true}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	// Add some test overrides with different statuses
	manager.activeOverrides["active-1"] = &ActiveOverride{
		ID:     "active-1",
		Status: OverrideStatusActive,
	}
	manager.activeOverrides["pending-1"] = &ActiveOverride{
		ID:     "pending-1",
		Status: OverrideStatusPending,
	}
	manager.activeOverrides["active-2"] = &ActiveOverride{
		ID:     "active-2",
		Status: OverrideStatusActive,
	}
	manager.activeOverrides["expired-1"] = &ActiveOverride{
		ID:     "expired-1",
		Status: OverrideStatusExpired,
	}

	active := manager.GetActiveOverrides()
	if len(active) != 2 {
		t.Errorf("Expected 2 active overrides, got %d", len(active))
	}

	// Check that only active ones are returned
	activeIDs := make(map[string]bool)
	for _, override := range active {
		activeIDs[override.ID] = true
		if override.Status != OverrideStatusActive {
			t.Errorf("Expected active status for %s, got %s", override.ID, override.Status)
		}
	}

	if !activeIDs["active-1"] || !activeIDs["active-2"] {
		t.Error("Expected to find active-1 and active-2 in results")
	}
}

func TestCleanupExpiredOverrides(t *testing.T) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	now := time.Now()
	placement := createTestOverridePlacement()

	// Add expired override
	expiredTime := now.Add(-1 * time.Hour)
	manager.activeOverrides["expired-1"] = &ActiveOverride{
		ID:        "expired-1",
		Status:    OverrideStatusActive,
		ExpiresAt: &expiredTime,
		Placement: placement,
	}

	// Add future override
	futureTime := now.Add(1 * time.Hour)
	manager.activeOverrides["future-1"] = &ActiveOverride{
		ID:        "future-1",
		Status:    OverrideStatusActive,
		ExpiresAt: &futureTime,
		Placement: placement,
	}

	// Add override with no expiration
	manager.activeOverrides["no-expiry-1"] = &ActiveOverride{
		ID:        "no-expiry-1",
		Status:    OverrideStatusActive,
		Placement: placement,
	}

	err := manager.CleanupExpiredOverrides(context.TODO())
	if err != nil {
		t.Errorf("Failed to cleanup expired overrides: %v", err)
	}

	// Check that expired override was marked as expired
	expired, _ := manager.GetOverride("expired-1")
	if expired.Status != OverrideStatusExpired {
		t.Errorf("Expected expired override to have status Expired, got %s", expired.Status)
	}

	// Check that future override is still active
	future, _ := manager.GetOverride("future-1")
	if future.Status != OverrideStatusActive {
		t.Errorf("Expected future override to remain Active, got %s", future.Status)
	}

	// Check that no-expiry override is still active
	noExpiry, _ := manager.GetOverride("no-expiry-1")
	if noExpiry.Status != OverrideStatusActive {
		t.Errorf("Expected no-expiry override to remain Active, got %s", noExpiry.Status)
	}

	// Check notification was sent for expired override
	if len(notifier.expiredNotifications) != 1 {
		t.Errorf("Expected 1 expiry notification, got %d", len(notifier.expiredNotifications))
	}
}

// Benchmark tests

func BenchmarkApplyOverride(b *testing.B) {
	kcpClient := kcpfakeclientset.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(1000)
	metrics := &mockOverrideMetricsCollector{}
	auth := &mockOverrideAuthChecker{canCreateOverride: true}
	validator := &mockOverrideValidator{}
	notifier := newMockOverrideNotifier()

	manager := NewDecisionOverride(
		kcpClient.Cluster(""),
		eventRecorder,
		metrics,
		auth,
		validator,
		notifier,
	)

	placement := createTestOverridePlacement()
	decision := createTestOverrideDecision()
	spec := createTestOverrideSpec(false)
	policy := createTestOverridePolicy()

	manager.overridePolicies[policy.Name] = policy

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := manager.ApplyOverride(context.TODO(), placement, decision, spec, "test-user")
		if err != nil {
			b.Fatalf("Failed to apply override: %v", err)
		}
	}
}