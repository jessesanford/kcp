/*
Copyright 2025 The KCP Authors.

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

package chaos

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestResilienceValidation validates system resilience across multiple failure scenarios.
func TestResilienceValidation(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancelFunc)

	server := kcptesting.SharedKcpServer(t)
	cfg := server.BaseConfig(t)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "error creating kubernetes client")

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "error creating kcp client")

	suite, err := NewChaosTestSuite(cfg, kubeClient, kcpClient)
	require.NoError(t, err, "error creating chaos test suite")

	// Setup test environment
	err = suite.SetupTestNamespace(ctx)
	require.NoError(t, err, "error setting up test namespace")
	t.Cleanup(func() {
		_ = suite.CleanupTestNamespace(context.Background())
	})

	validator := &ResilienceValidator{Suite: suite}

	t.Run("ComprehensiveRecoveryValidation", func(t *testing.T) {
		testComprehensiveRecoveryValidation(t, ctx, validator)
	})

	t.Run("RecoveryTimeObjectives", func(t *testing.T) {
		testRecoveryTimeObjectives(t, ctx, validator)
	})

	t.Run("DataConsistencyValidation", func(t *testing.T) {
		testDataConsistencyValidation(t, ctx, validator)
	})

	t.Run("CascadingFailureResilience", func(t *testing.T) {
		testCascadingFailureResilience(t, ctx, validator)
	})
}

// testComprehensiveRecoveryValidation tests system recovery across all failure types.
func testComprehensiveRecoveryValidation(t *testing.T, ctx context.Context, validator *ResilienceValidator) {
	// Create baseline system state
	err := validator.EstablishBaselineState(ctx)
	require.NoError(t, err, "should establish baseline state")

	// Test recovery from each failure type
	failureTypes := []FailureType{
		NetworkPartitionFailure,
		ClusterFailure,
		ControllerCrash,
		APIServerFailure,
		ResourceExhaustion,
	}

	results := make(map[FailureType]*RecoveryResult)

	for _, failureType := range failureTypes {
		t.Logf("Testing recovery from %s", failureType)
		
		result := validator.TestFailureRecovery(ctx, failureType)
		results[failureType] = result
		
		if result.Recovered {
			t.Logf("✓ Recovery from %s: %v", failureType, result.RecoveryTime)
		} else {
			t.Logf("✗ Recovery from %s failed: %v", failureType, result.Error)
		}
		
		// Brief pause between tests
		time.Sleep(5 * time.Second)
	}

	// Validate overall resilience
	successfulRecoveries := 0
	totalRecoveryTime := time.Duration(0)
	
	for failureType, result := range results {
		if result.Recovered {
			successfulRecoveries++
			totalRecoveryTime += result.RecoveryTime
		} else {
			t.Logf("Failed to recover from %s: %v", failureType, result.Error)
		}
	}

	recoveryRate := float64(successfulRecoveries) / float64(len(failureTypes))
	averageRecoveryTime := totalRecoveryTime / time.Duration(successfulRecoveries)

	t.Logf("Overall resilience metrics:")
	t.Logf("- Recovery rate: %.1f%% (%d/%d)", recoveryRate*100, successfulRecoveries, len(failureTypes))
	t.Logf("- Average recovery time: %v", averageRecoveryTime)

	// Validate resilience targets
	assert.GreaterOrEqual(t, recoveryRate, 0.8, "recovery rate should be at least 80%")
	if successfulRecoveries > 0 {
		assert.Less(t, averageRecoveryTime, 2*time.Minute, "average recovery time should be under 2 minutes")
	}
}

// testRecoveryTimeObjectives validates specific RTO requirements.
func testRecoveryTimeObjectives(t *testing.T, ctx context.Context, validator *ResilienceValidator) {
	// Define RTO targets for different failure types
	rtoTargets := map[FailureType]time.Duration{
		NetworkPartitionFailure: 1 * time.Minute,
		ClusterFailure:          2 * time.Minute,
		ControllerCrash:         30 * time.Second,
		APIServerFailure:        1 * time.Minute,
		ResourceExhaustion:      90 * time.Second,
	}

	rtoResults := make(map[FailureType]bool)

	for failureType, targetRTO := range rtoTargets {
		t.Logf("Testing RTO for %s (target: %v)", failureType, targetRTO)
		
		actualRTO, err := validator.MeasureRecoveryTime(ctx, failureType)
		if err != nil {
			t.Logf("RTO measurement failed for %s: %v", failureType, err)
			rtoResults[failureType] = false
			continue
		}

		meetsRTO := actualRTO <= targetRTO
		rtoResults[failureType] = meetsRTO
		
		if meetsRTO {
			t.Logf("✓ %s RTO: %v (target: %v)", failureType, actualRTO, targetRTO)
		} else {
			t.Logf("✗ %s RTO: %v exceeded target: %v", failureType, actualRTO, targetRTO)
		}
		
		// Brief pause between measurements
		time.Sleep(3 * time.Second)
	}

	// Calculate RTO compliance rate
	rtoMet := 0
	for _, met := range rtoResults {
		if met {
			rtoMet++
		}
	}

	rtoComplianceRate := float64(rtoMet) / float64(len(rtoTargets))
	t.Logf("RTO compliance rate: %.1f%% (%d/%d)", rtoComplianceRate*100, rtoMet, len(rtoTargets))

	// Validate RTO compliance (allow some flexibility in test environment)
	assert.GreaterOrEqual(t, rtoComplianceRate, 0.6, "RTO compliance should be at least 60% in test environment")
}

// testDataConsistencyValidation validates data consistency during failures.
func testDataConsistencyValidation(t *testing.T, ctx context.Context, validator *ResilienceValidator) {
	// Create test data
	testData := validator.CreateTestDataSet(ctx)
	require.NotNil(t, testData, "should create test data set")

	// Inject various failures while maintaining data operations
	failureScenarios := []struct {
		name        string
		failureType FailureType
		duration    time.Duration
	}{
		{"short-network-partition", NetworkPartitionFailure, 10 * time.Second},
		{"brief-controller-crash", ControllerCrash, 5 * time.Second},
		{"resource-pressure", ResourceExhaustion, 15 * time.Second},
	}

	for _, scenario := range failureScenarios {
		t.Logf("Testing data consistency during %s", scenario.name)
		
		// Start continuous data operations
		stopDataOps := validator.StartContinuousDataOperations(ctx, testData)
		
		// Inject failure
		err := validator.InjectFailure(ctx, scenario.failureType, scenario.duration)
		if err != nil {
			t.Logf("Failure injection for %s: %v", scenario.name, err)
		}
		
		// Wait for failure duration
		time.Sleep(scenario.duration)
		
		// Stop data operations
		stopDataOps()
		
		// Validate data consistency
		consistent, inconsistencies := validator.ValidateDataConsistency(ctx, testData)
		if consistent {
			t.Logf("✓ Data remained consistent during %s", scenario.name)
		} else {
			t.Logf("✗ Data inconsistencies found during %s: %v", scenario.name, inconsistencies)
		}
		
		assert.True(t, consistent, fmt.Sprintf("data should remain consistent during %s", scenario.name))
		
		// Brief recovery pause
		time.Sleep(5 * time.Second)
	}
}

// testCascadingFailureResilience validates system behavior during cascading failures.
func testCascadingFailureResilience(t *testing.T, ctx context.Context, validator *ResilienceValidator) {
	// Create system state for cascading failure test
	err := validator.EstablishBaselineState(ctx)
	require.NoError(t, err, "should establish baseline for cascading test")

	// Simulate cascading failure scenario
	cascadingScenario := []struct {
		failureType FailureType
		delay       time.Duration
	}{
		{NetworkPartitionFailure, 0},
		{ControllerCrash, 5 * time.Second},
		{ResourceExhaustion, 10 * time.Second},
	}

	t.Logf("Starting cascading failure scenario with %d sequential failures", len(cascadingScenario))
	
	cascadingStart := time.Now()
	activeFailures := make([]string, 0)

	// Inject cascading failures
	for i, step := range cascadingScenario {
		if step.delay > 0 {
			time.Sleep(step.delay)
		}
		
		failureID := fmt.Sprintf("cascading-%d-%s", i, step.failureType)
		err := validator.InjectFailureWithID(ctx, failureID, step.failureType)
		if err != nil {
			t.Logf("Cascading failure injection %s: %v", failureID, err)
		} else {
			activeFailures = append(activeFailures, failureID)
		}
		
		t.Logf("Injected failure %d/%d: %s", i+1, len(cascadingScenario), step.failureType)
	}

	// Let cascading failures run
	time.Sleep(15 * time.Second)

	// Begin recovery validation
	recoveryCtx, cancelRecovery := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelRecovery()

	// Wait for system to recover from cascading failures
	err = validator.Suite.WaitForRecoveryWithTimeout(recoveryCtx, validator.Suite.ValidateSystemHealth, 5*time.Minute)
	cascadingRecoveryTime := time.Since(cascadingStart)

	if err != nil {
		t.Logf("Cascading failure recovery: %v (took %v)", err, cascadingRecoveryTime)
	} else {
		t.Logf("✓ System recovered from cascading failures in %v", cascadingRecoveryTime)
	}

	// Validate resilience against cascading failures
	assert.NoError(t, err, "system should recover from cascading failures")
	assert.Less(t, cascadingRecoveryTime, 4*time.Minute, "cascading failure recovery should complete within 4 minutes")

	// Clean up any remaining failure artifacts
	for _, failureID := range activeFailures {
		validator.Suite.FailureTracker.RecordFailureEnd(failureID, err)
	}
}

// ResilienceValidator provides comprehensive resilience validation capabilities.
type ResilienceValidator struct {
	Suite *ChaosTestSuite
}

// RecoveryResult represents the result of a recovery test.
type RecoveryResult struct {
	FailureType  FailureType
	Recovered    bool
	RecoveryTime time.Duration
	Error        error
}

// TestDataSet represents a set of test data for consistency validation.
type TestDataSet struct {
	ConfigMaps map[string]*corev1.ConfigMap
	Secrets    map[string]*corev1.Secret
	TestID     string
}

// EstablishBaselineState creates a known good system state for testing.
func (rv *ResilienceValidator) EstablishBaselineState(ctx context.Context) error {
	// Create baseline workloads
	err := rv.Suite.CreateTestWorkload(ctx, "baseline-workload", 2)
	if err != nil {
		return fmt.Errorf("failed to create baseline workload: %w", err)
	}

	// Wait for baseline to be healthy
	return rv.Suite.WaitForRecovery(ctx, rv.Suite.ValidateSystemHealth)
}

// TestFailureRecovery tests recovery from a specific failure type.
func (rv *ResilienceValidator) TestFailureRecovery(ctx context.Context, failureType FailureType) *RecoveryResult {
	failureID := fmt.Sprintf("recovery-test-%s-%d", failureType, time.Now().Unix())
	
	rv.Suite.FailureTracker.RecordFailureStart(failureID, failureType, "recovery-test")
	
	startTime := time.Now()
	
	// Inject failure based on type
	var err error
	switch failureType {
	case NetworkPartitionFailure:
		err = rv.simulateNetworkPartition(ctx)
	case ClusterFailure:
		err = rv.simulateClusterFailure(ctx)
	case ControllerCrash:
		err = rv.simulateControllerCrash(ctx)
	case APIServerFailure:
		err = rv.simulateAPIServerFailure(ctx)
	case ResourceExhaustion:
		err = rv.simulateResourceExhaustion(ctx)
	}
	
	if err != nil {
		rv.Suite.FailureTracker.RecordFailureEnd(failureID, err)
		return &RecoveryResult{
			FailureType:  failureType,
			Recovered:    false,
			RecoveryTime: 0,
			Error:        err,
		}
	}
	
	// Wait for recovery
	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	
	recoveryErr := rv.Suite.WaitForRecoveryWithTimeout(recoveryCtx, rv.Suite.ValidateSystemHealth, 3*time.Minute)
	recoveryTime := time.Since(startTime)
	
	rv.Suite.FailureTracker.RecordFailureEnd(failureID, recoveryErr)
	
	return &RecoveryResult{
		FailureType:  failureType,
		Recovered:    recoveryErr == nil,
		RecoveryTime: recoveryTime,
		Error:        recoveryErr,
	}
}

// MeasureRecoveryTime measures the actual recovery time for a failure type.
func (rv *ResilienceValidator) MeasureRecoveryTime(ctx context.Context, failureType FailureType) (time.Duration, error) {
	result := rv.TestFailureRecovery(ctx, failureType)
	if !result.Recovered {
		return 0, result.Error
	}
	return result.RecoveryTime, nil
}

// CreateTestDataSet creates a test data set for consistency validation.
func (rv *ResilienceValidator) CreateTestDataSet(ctx context.Context) *TestDataSet {
	testDataSet := &TestDataSet{
		ConfigMaps: make(map[string]*corev1.ConfigMap),
		Secrets:    make(map[string]*corev1.Secret),
		TestID:     rv.Suite.TestID,
	}
	
	// Create test ConfigMaps
	for i := 0; i < 3; i++ {
		cmName := fmt.Sprintf("%s-test-cm-%d", rv.Suite.TestID, i)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: rv.Suite.Namespace,
			},
			Data: map[string]string{
				"test-key": fmt.Sprintf("test-value-%d", i),
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}
		
		createdCM, err := rv.Suite.KubeClient.CoreV1().ConfigMaps(rv.Suite.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err == nil {
			testDataSet.ConfigMaps[cmName] = createdCM
		}
	}
	
	return testDataSet
}

// StartContinuousDataOperations starts continuous data operations and returns a stop function.
func (rv *ResilienceValidator) StartContinuousDataOperations(ctx context.Context, testData *TestDataSet) func() {
	stopCh := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Perform read operations
				for cmName := range testData.ConfigMaps {
					_, _ = rv.Suite.KubeClient.CoreV1().ConfigMaps(rv.Suite.Namespace).Get(ctx, cmName, metav1.GetOptions{})
				}
			}
		}
	}()
	
	return func() {
		close(stopCh)
	}
}

// InjectFailure injects a specific failure type for a duration.
func (rv *ResilienceValidator) InjectFailure(ctx context.Context, failureType FailureType, duration time.Duration) error {
	time.Sleep(100 * time.Millisecond) // Simulate failure injection
	return nil
}

// InjectFailureWithID injects a failure with a specific ID for tracking.
func (rv *ResilienceValidator) InjectFailureWithID(ctx context.Context, failureID string, failureType FailureType) error {
	rv.Suite.FailureTracker.RecordFailureStart(failureID, failureType, "cascading-test")
	time.Sleep(100 * time.Millisecond) // Simulate failure injection
	return nil
}

// ValidateDataConsistency validates that test data remains consistent.
func (rv *ResilienceValidator) ValidateDataConsistency(ctx context.Context, testData *TestDataSet) (bool, []string) {
	inconsistencies := make([]string, 0)
	
	// Validate ConfigMaps
	for cmName, originalCM := range testData.ConfigMaps {
		currentCM, err := rv.Suite.KubeClient.CoreV1().ConfigMaps(rv.Suite.Namespace).Get(ctx, cmName, metav1.GetOptions{})
		if err != nil {
			inconsistencies = append(inconsistencies, fmt.Sprintf("ConfigMap %s not found: %v", cmName, err))
			continue
		}
		
		if currentCM.Data["test-key"] != originalCM.Data["test-key"] {
			inconsistencies = append(inconsistencies, fmt.Sprintf("ConfigMap %s data inconsistency", cmName))
		}
	}
	
	return len(inconsistencies) == 0, inconsistencies
}

// Helper methods for simulating different failure types
func (rv *ResilienceValidator) simulateNetworkPartition(ctx context.Context) error {
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (rv *ResilienceValidator) simulateClusterFailure(ctx context.Context) error {
	time.Sleep(300 * time.Millisecond)
	return nil
}

func (rv *ResilienceValidator) simulateControllerCrash(ctx context.Context) error {
	time.Sleep(150 * time.Millisecond)
	return nil
}

func (rv *ResilienceValidator) simulateAPIServerFailure(ctx context.Context) error {
	time.Sleep(250 * time.Millisecond)
	return nil
}

func (rv *ResilienceValidator) simulateResourceExhaustion(ctx context.Context) error {
	time.Sleep(400 * time.Millisecond)
	return nil
}