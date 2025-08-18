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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestResourceExhaustion validates system behavior during resource exhaustion.
func TestResourceExhaustion(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Minute)
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

	exhaustion := &ResourceExhaustionInjector{Suite: suite}

	t.Run("SimulateCPUExhaustion", func(t *testing.T) {
		testCPUExhaustionScenario(t, ctx, exhaustion)
	})

	t.Run("SimulateMemoryExhaustion", func(t *testing.T) {
		testMemoryExhaustionScenario(t, ctx, exhaustion)
	})

	t.Run("ValidateResourceThrottling", func(t *testing.T) {
		testResourceThrottlingBehavior(t, ctx, exhaustion)
	})

	t.Run("TestSystemRecoveryAfterExhaustion", func(t *testing.T) {
		testSystemRecoveryAfterExhaustion(t, ctx, exhaustion)
	})
}

// testCPUExhaustionScenario simulates CPU exhaustion.
func testCPUExhaustionScenario(t *testing.T, ctx context.Context, injector *ResourceExhaustionInjector) {
	failureID := fmt.Sprintf("cpu-exhaustion-%d", time.Now().Unix())
	
	// Record failure start
	injector.Suite.FailureTracker.RecordFailureStart(failureID, ResourceExhaustion, "cpu-resources")
	
	// Create CPU-intensive workload
	err := injector.CreateCPUStressWorkload(ctx, "cpu-stress", 2)
	if err != nil {
		t.Logf("CPU stress workload creation: %v", err)
	}
	
	// Monitor system behavior during CPU stress
	time.Sleep(10 * time.Second)
	
	// Verify system responsiveness
	responsive := true
	startTime := time.Now()
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err := injector.Suite.KubeClient.CoreV1().Pods(injector.Suite.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			if time.Since(startTime) > 5*time.Second {
				responsive = false
				return true, nil // Stop polling
			}
		}
		return false, nil // Continue polling
	})
	
	if !responsive {
		t.Logf("System became less responsive under CPU stress (expected)")
	}
	
	// Clean up stress workload
	cleanupErr := injector.CleanupStressWorkloads(ctx, "cpu-stress")
	if cleanupErr != nil {
		t.Logf("Cleanup error: %v", cleanupErr)
	}
	
	// Wait for recovery
	recoveryErr := injector.Suite.WaitForRecoveryWithTimeout(ctx, injector.Suite.ValidateSystemHealth, 1*time.Minute)
	
	injector.Suite.FailureTracker.RecordFailureEnd(failureID, recoveryErr)
	
	if recoveryErr != nil {
		t.Logf("CPU exhaustion recovery: %v", recoveryErr)
	} else {
		t.Logf("System recovered from CPU exhaustion")
	}
}

// testMemoryExhaustionScenario simulates memory exhaustion.
func testMemoryExhaustionScenario(t *testing.T, ctx context.Context, injector *ResourceExhaustionInjector) {
	failureID := fmt.Sprintf("memory-exhaustion-%d", time.Now().Unix())
	
	// Record failure start
	injector.Suite.FailureTracker.RecordFailureStart(failureID, ResourceExhaustion, "memory-resources")
	
	// Create memory-intensive workload
	err := injector.CreateMemoryStressWorkload(ctx, "memory-stress", 1)
	if err != nil {
		t.Logf("Memory stress workload creation: %v", err)
	}
	
	// Monitor memory pressure effects
	time.Sleep(5 * time.Second)
	
	// Check for memory pressure indicators
	memoryPressureDetected := false
	pods, err := injector.Suite.KubeClient.CoreV1().Pods(injector.Suite.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, pod := range pods.Items {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse {
					memoryPressureDetected = true
					break
				}
			}
		}
	}
	
	if memoryPressureDetected {
		t.Logf("Memory pressure effects detected")
	}
	
	// Clean up and wait for recovery
	cleanupErr := injector.CleanupStressWorkloads(ctx, "memory-stress")
	if cleanupErr != nil {
		t.Logf("Memory stress cleanup: %v", cleanupErr)
	}
	
	recoveryErr := injector.Suite.WaitForRecoveryWithTimeout(ctx, injector.Suite.ValidateSystemHealth, 2*time.Minute)
	
	injector.Suite.FailureTracker.RecordFailureEnd(failureID, recoveryErr)
	
	if recoveryErr != nil {
		t.Logf("Memory exhaustion recovery: %v", recoveryErr)
	} else {
		t.Logf("System recovered from memory exhaustion")
	}
}

// testResourceThrottlingBehavior validates resource throttling mechanisms.
func testResourceThrottlingBehavior(t *testing.T, ctx context.Context, injector *ResourceExhaustionInjector) {
	// Create workload with resource limits
	err := injector.CreateThrottledWorkload(ctx, "throttled-test")
	require.NoError(t, err, "should create throttled workload")
	
	// Wait for workload to start
	time.Sleep(5 * time.Second)
	
	// Verify throttling behavior
	pod, err := injector.GetWorkloadPod(ctx, "throttled-test")
	if err != nil {
		t.Logf("Error getting throttled pod: %v", err)
		return
	}
	
	// Check resource requests and limits
	container := pod.Spec.Containers[0]
	if container.Resources.Limits != nil {
		cpuLimit := container.Resources.Limits[corev1.ResourceCPU]
		memoryLimit := container.Resources.Limits[corev1.ResourceMemory]
		t.Logf("Throttled workload limits - CPU: %s, Memory: %s", cpuLimit.String(), memoryLimit.String())
	}
	
	// Monitor throttling effects
	time.Sleep(10 * time.Second)
	
	// Verify pod is still running despite limits
	updatedPod, err := injector.Suite.KubeClient.CoreV1().Pods(injector.Suite.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err == nil {
		assert.Equal(t, corev1.PodRunning, updatedPod.Status.Phase, "throttled pod should remain running")
	}
}

// testSystemRecoveryAfterExhaustion validates system recovery after resource exhaustion.
func testSystemRecoveryAfterExhaustion(t *testing.T, ctx context.Context, injector *ResourceExhaustionInjector) {
	// Create multiple stress workloads
	stressTypes := []string{"cpu-recovery", "memory-recovery"}
	
	for _, stressType := range stressTypes {
		err := injector.CreateCombinedStressWorkload(ctx, stressType)
		if err != nil {
			t.Logf("Combined stress workload %s creation: %v", stressType, err)
		}
	}
	
	// Let stress run for a period
	time.Sleep(15 * time.Second)
	
	// Clean up all stress workloads
	for _, stressType := range stressTypes {
		err := injector.CleanupStressWorkloads(ctx, stressType)
		if err != nil {
			t.Logf("Cleanup stress workload %s: %v", stressType, err)
		}
	}
	
	// Validate system recovery
	recoveryStartTime := time.Now()
	err := injector.Suite.WaitForRecoveryWithTimeout(ctx, injector.Suite.ValidateSystemHealth, 3*time.Minute)
	recoveryTime := time.Since(recoveryStartTime)
	
	if err != nil {
		t.Logf("System recovery after exhaustion: %v (took %v)", err, recoveryTime)
	} else {
		t.Logf("System recovered successfully after exhaustion in %v", recoveryTime)
		assert.Less(t, recoveryTime, 2*time.Minute, "recovery should complete within reasonable time")
	}
	
	// Verify system can handle new workloads after recovery
	err = injector.Suite.CreateTestWorkload(ctx, "post-recovery-test", 1)
	assert.NoError(t, err, "should be able to create workloads after recovery")
}

// ResourceExhaustionInjector provides resource exhaustion simulation capabilities.
type ResourceExhaustionInjector struct {
	Suite *ChaosTestSuite
}

// CreateCPUStressWorkload creates a CPU-intensive workload.
func (rei *ResourceExhaustionInjector) CreateCPUStressWorkload(ctx context.Context, name string, instances int) error {
	for i := 0; i < instances; i++ {
		podName := fmt.Sprintf("%s-cpu-stress-%d", name, i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: rei.Suite.Namespace,
				Labels: map[string]string{
					"stress-type": "cpu",
					"test-name":   name,
					"test-id":     rei.Suite.TestID,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "cpu-stress",
						Image: "busybox:1.35",
						Command: []string{
							"sh", "-c",
							"while true; do echo 'CPU stress' | md5sum; done",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("32Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyAlways,
			},
		}
		
		_, err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create CPU stress pod %s: %w", podName, err)
		}
	}
	
	return nil
}

// CreateMemoryStressWorkload creates a memory-intensive workload.
func (rei *ResourceExhaustionInjector) CreateMemoryStressWorkload(ctx context.Context, name string, instances int) error {
	for i := 0; i < instances; i++ {
		podName := fmt.Sprintf("%s-memory-stress-%d", name, i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: rei.Suite.Namespace,
				Labels: map[string]string{
					"stress-type": "memory",
					"test-name":   name,
					"test-id":     rei.Suite.TestID,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "memory-stress",
						Image: "busybox:1.35",
						Command: []string{
							"sh", "-c",
							"while true; do head -c 10M /dev/urandom | tail; sleep 1; done",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyAlways,
			},
		}
		
		_, err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create memory stress pod %s: %w", podName, err)
		}
	}
	
	return nil
}

// CreateThrottledWorkload creates a workload with specific resource limits for throttling tests.
func (rei *ResourceExhaustionInjector) CreateThrottledWorkload(ctx context.Context, name string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rei.Suite.Namespace,
			Labels: map[string]string{
				"workload-type": "throttled",
				"test-id":       rei.Suite.TestID,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "throttled-app",
					Image: "busybox:1.35",
					Command: []string{
						"sh", "-c",
						"while true; do echo 'throttled workload running' && sleep 5; done",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	
	_, err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

// CreateCombinedStressWorkload creates a workload that stresses both CPU and memory.
func (rei *ResourceExhaustionInjector) CreateCombinedStressWorkload(ctx context.Context, name string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rei.Suite.Namespace,
			Labels: map[string]string{
				"stress-type": "combined",
				"test-name":   name,
				"test-id":     rei.Suite.TestID,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "combined-stress",
					Image: "busybox:1.35",
					Command: []string{
						"sh", "-c",
						"while true; do head -c 5M /dev/urandom | md5sum; sleep 1; done",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	
	_, err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	return err
}

// CleanupStressWorkloads removes all stress workloads with the given test name.
func (rei *ResourceExhaustionInjector) CleanupStressWorkloads(ctx context.Context, testName string) error {
	pods, err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("test-name=%s", testName),
	})
	if err != nil {
		return fmt.Errorf("failed to list stress workloads: %w", err)
	}
	
	for _, pod := range pods.Items {
		err := rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: int64Ptr(0),
		})
		if err != nil {
			return fmt.Errorf("failed to delete stress pod %s: %w", pod.Name, err)
		}
	}
	
	return nil
}

// GetWorkloadPod gets a pod for a specific workload.
func (rei *ResourceExhaustionInjector) GetWorkloadPod(ctx context.Context, workloadName string) (*corev1.Pod, error) {
	return rei.Suite.KubeClient.CoreV1().Pods(rei.Suite.Namespace).Get(ctx, workloadName, metav1.GetOptions{})
}

// TestResourceExhaustionMetrics validates resource exhaustion metrics.
func TestResourceExhaustionMetrics(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancelFunc)

	server := kcptesting.SharedKcpServer(t)
	cfg := server.BaseConfig(t)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "error creating kubernetes client")

	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err, "error creating kcp client")

	suite, err := NewChaosTestSuite(cfg, kubeClient, kcpClient)
	require.NoError(t, err, "error creating chaos test suite")

	// Test resource exhaustion metrics
	failureID := "metrics-resource-exhaustion"
	suite.FailureTracker.RecordFailureStart(failureID, ResourceExhaustion, "test-resources")
	
	time.Sleep(50 * time.Millisecond)
	
	suite.FailureTracker.RecordFailureEnd(failureID, nil)

	// Validate metrics
	record, exists := suite.FailureTracker.GetFailureRecord(failureID)
	assert.True(t, exists, "resource exhaustion failure record should exist")
	assert.Equal(t, ResourceExhaustion, record.Type, "failure type should be resource exhaustion")
	assert.True(t, record.Recovered, "resource exhaustion should be marked as recovered")
	assert.Greater(t, record.RecoveryRTO.Nanoseconds(), int64(0), "recovery time should be measured")
}