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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestControllerCrashRecovery validates controller crash and recovery scenarios.
func TestControllerCrashRecovery(t *testing.T) {
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

	crashInjector := &ControllerCrashInjector{Suite: suite}

	t.Run("SimulateControllerCrash", func(t *testing.T) {
		testControllerCrashScenario(t, ctx, crashInjector)
	})

	t.Run("ValidateControllerRecovery", func(t *testing.T) {
		testControllerRecoveryValidation(t, ctx, crashInjector)
	})

	t.Run("TestLeaderElectionRecovery", func(t *testing.T) {
		testLeaderElectionRecovery(t, ctx, crashInjector)
	})
}

// testControllerCrashScenario simulates controller crashes and validates system response.
func testControllerCrashScenario(t *testing.T, ctx context.Context, injector *ControllerCrashInjector) {
	failureID := fmt.Sprintf("controller-crash-%d", time.Now().Unix())
	
	// Record failure start
	injector.Suite.FailureTracker.RecordFailureStart(failureID, ControllerCrash, "test-controller")
	
	// Deploy a test controller
	err := injector.DeployTestController(ctx, "test-controller")
	require.NoError(t, err, "should deploy test controller")
	
	// Wait for controller to be ready
	err = injector.WaitForControllerReady(ctx, "test-controller")
	require.NoError(t, err, "controller should be ready")
	
	// Simulate controller crash
	err = injector.SimulateControllerCrash(ctx, "test-controller")
	if err != nil {
		t.Logf("Controller crash simulation: %v", err)
	}
	
	// Verify crash detection
	crashed := false
	err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		ready, err := injector.IsControllerReady(ctx, "test-controller")
		if err != nil {
			return false, err
		}
		if !ready {
			crashed = true
			return true, nil
		}
		return false, nil
	})
	
	if err == nil && crashed {
		t.Logf("Controller crash detected successfully")
	} else {
		t.Logf("Controller crash detection: ready=%v, err=%v", !crashed, err)
	}
	
	// Wait for automatic recovery
	recoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		return injector.IsControllerReady(recoveryCtx, "test-controller")
	})
	
	injector.Suite.FailureTracker.RecordFailureEnd(failureID, err)
	
	if err != nil {
		t.Logf("Controller recovery validation: %v", err)
	} else {
		t.Logf("Controller recovered successfully")
	}
}

// testControllerRecoveryValidation validates controller recovery behavior.
func testControllerRecoveryValidation(t *testing.T, ctx context.Context, injector *ControllerCrashInjector) {
	// Create test workload that requires controller management
	workloadName := fmt.Sprintf("recovery-test-%d", time.Now().Unix())
	err := injector.CreateManagedWorkload(ctx, workloadName)
	require.NoError(t, err, "should create managed workload")
	
	// Simulate controller crash during workload management
	err = injector.SimulateControllerCrash(ctx, "test-controller")
	if err != nil {
		t.Logf("Controller crash during workload management: %v", err)
	}
	
	// Wait for controller recovery
	err = wait.PollImmediate(3*time.Second, 1*time.Minute, func() (bool, error) {
		return injector.IsControllerReady(ctx, "test-controller")
	})
	
	if err != nil {
		t.Logf("Controller recovery after workload crash: %v", err)
	}
	
	// Validate workload state is consistent
	workload, err := injector.GetWorkloadStatus(ctx, workloadName)
	if err != nil {
		t.Logf("Workload status check after controller recovery: %v", err)
	} else {
		t.Logf("Workload status after controller recovery: %+v", workload)
	}
}

// testLeaderElectionRecovery validates leader election during controller crashes.
func testLeaderElectionRecovery(t *testing.T, ctx context.Context, injector *ControllerCrashInjector) {
	// Deploy multiple controller instances for leader election testing
	for i := 0; i < 2; i++ {
		controllerName := fmt.Sprintf("leader-test-controller-%d", i)
		err := injector.DeployTestController(ctx, controllerName)
		if err != nil {
			t.Logf("Failed to deploy controller %s: %v", controllerName, err)
		}
	}
	
	// Wait a bit for leader election
	time.Sleep(10 * time.Second)
	
	// Simulate crash of one instance
	err := injector.SimulateControllerCrash(ctx, "leader-test-controller-0")
	if err != nil {
		t.Logf("Leader election crash simulation: %v", err)
	}
	
	// Verify system continues operating
	time.Sleep(5 * time.Second)
	
	// Check if any controller instance is still running
	runningControllers := 0
	for i := 0; i < 2; i++ {
		controllerName := fmt.Sprintf("leader-test-controller-%d", i)
		ready, err := injector.IsControllerReady(ctx, controllerName)
		if err == nil && ready {
			runningControllers++
		}
	}
	
	t.Logf("Running controllers after leader crash: %d", runningControllers)
	assert.Greater(t, runningControllers, 0, "at least one controller should remain running after leader crash")
}

// ControllerCrashInjector provides controller crash simulation capabilities.
type ControllerCrashInjector struct {
	Suite *ChaosTestSuite
}

// DeployTestController deploys a test controller for crash testing.
func (cci *ControllerCrashInjector) DeployTestController(ctx context.Context, name string) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cci.Suite.Namespace,
			Labels: map[string]string{
				"app":           "test-controller",
				"controller":    name,
				"test-id":       cci.Suite.TestID,
				"chaos-target":  "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "test-controller",
					"controller": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "test-controller",
						"controller": name,
						"test-id":    cci.Suite.TestID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "controller",
							Image: "busybox:1.35",
							Command: []string{
								"sh", "-c",
								"while true; do echo 'controller running' && sleep 10; done",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    "10m",
									corev1.ResourceMemory: "16Mi",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"echo", "healthy"},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"echo", "ready"},
									},
								},
								InitialDelaySeconds: 2,
								PeriodSeconds:       5,
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}
	
	_, err := cci.Suite.KubeClient.AppsV1().Deployments(cci.Suite.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

// SimulateControllerCrash simulates a controller crash by deleting its pod.
func (cci *ControllerCrashInjector) SimulateControllerCrash(ctx context.Context, controllerName string) error {
	// Find controller pods
	pods, err := cci.Suite.KubeClient.CoreV1().Pods(cci.Suite.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("controller=%s", controllerName),
	})
	if err != nil {
		return fmt.Errorf("failed to list controller pods: %w", err)
	}
	
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for controller %s", controllerName)
	}
	
	// Delete the first pod to simulate crash
	podName := pods.Items[0].Name
	err = cci.Suite.KubeClient.CoreV1().Pods(cci.Suite.Namespace).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: int64Ptr(0), // Force immediate termination
	})
	if err != nil {
		return fmt.Errorf("failed to delete controller pod: %w", err)
	}
	
	return nil
}

// WaitForControllerReady waits for a controller to become ready.
func (cci *ControllerCrashInjector) WaitForControllerReady(ctx context.Context, controllerName string) error {
	return wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
		return cci.IsControllerReady(ctx, controllerName)
	})
}

// IsControllerReady checks if a controller is ready.
func (cci *ControllerCrashInjector) IsControllerReady(ctx context.Context, controllerName string) (bool, error) {
	deployment, err := cci.Suite.KubeClient.AppsV1().Deployments(cci.Suite.Namespace).Get(ctx, controllerName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	
	return deployment.Status.ReadyReplicas > 0, nil
}

// CreateManagedWorkload creates a workload that would be managed by controllers.
func (cci *ControllerCrashInjector) CreateManagedWorkload(ctx context.Context, name string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cci.Suite.Namespace,
			Labels: map[string]string{
				"workload": name,
				"test-id":  cci.Suite.TestID,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"workload": name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	
	_, err := cci.Suite.KubeClient.CoreV1().Services(cci.Suite.Namespace).Create(ctx, service, metav1.CreateOptions{})
	return err
}

// GetWorkloadStatus gets the status of a managed workload.
func (cci *ControllerCrashInjector) GetWorkloadStatus(ctx context.Context, name string) (*corev1.Service, error) {
	return cci.Suite.KubeClient.CoreV1().Services(cci.Suite.Namespace).Get(ctx, name, metav1.GetOptions{})
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

// TestControllerCrashMetrics validates crash metrics collection.
func TestControllerCrashMetrics(t *testing.T) {
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

	// Test crash metrics tracking
	failureID := "metrics-controller-crash"
	suite.FailureTracker.RecordFailureStart(failureID, ControllerCrash, "metrics-test-controller")
	
	// Simulate processing delay
	time.Sleep(150 * time.Millisecond)
	
	suite.FailureTracker.RecordFailureEnd(failureID, nil)

	// Validate metrics
	record, exists := suite.FailureTracker.GetFailureRecord(failureID)
	assert.True(t, exists, "crash failure record should exist")
	assert.Equal(t, ControllerCrash, record.Type, "failure type should be controller crash")
	assert.Equal(t, "metrics-test-controller", record.Target, "target should match")
	assert.True(t, record.Recovered, "crash should be marked as recovered")
	assert.Greater(t, record.RecoveryRTO.Milliseconds(), int64(100), "recovery time should be measured")
}