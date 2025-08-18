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
	"math/rand"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

const (
	// ChaosTestNamespace is the namespace used for chaos testing resources
	ChaosTestNamespace = "chaos-tests"
	
	// TestDataPrefix is the prefix for all chaos test resources
	TestDataPrefix = "ct-"
	
	// DefaultRecoveryTimeout is the default timeout for recovery validation
	DefaultRecoveryTimeout = 5 * time.Minute
	
	// DefaultPollingInterval is the default interval for polling operations
	DefaultPollingInterval = 1 * time.Second
)

// ChaosTestSuite provides utilities for chaos testing scenarios.
type ChaosTestSuite struct {
	Config         *rest.Config
	KubeClient     kubernetes.Interface
	KcpClient      kcpclientset.ClusterInterface
	Namespace      string
	TestID         string
	FailureTracker *FailureTracker
}

// NewChaosTestSuite creates a new chaos testing suite.
func NewChaosTestSuite(config *rest.Config, kubeClient kubernetes.Interface, kcpClient kcpclientset.ClusterInterface) (*ChaosTestSuite, error) {
	testID := fmt.Sprintf("%s%d", TestDataPrefix, rand.Int31())
	
	suite := &ChaosTestSuite{
		Config:         config,
		KubeClient:     kubeClient,
		KcpClient:      kcpClient,
		Namespace:      ChaosTestNamespace,
		TestID:         testID,
		FailureTracker: NewFailureTracker(),
	}
	
	return suite, nil
}

// SetupTestNamespace creates and configures the chaos test namespace.
func (s *ChaosTestSuite) SetupTestNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Namespace,
			Labels: map[string]string{
				"chaos-testing": "true",
				"test-id":       s.TestID,
			},
		},
	}
	
	_, err := s.KubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create test namespace: %w", err)
	}
	
	return nil
}

// CleanupTestNamespace removes the chaos test namespace and all resources.
func (s *ChaosTestSuite) CleanupTestNamespace(ctx context.Context) error {
	err := s.KubeClient.CoreV1().Namespaces().Delete(ctx, s.Namespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete test namespace: %w", err)
	}
	
	// Wait for namespace to be fully deleted
	return wait.PollImmediate(DefaultPollingInterval, DefaultRecoveryTimeout, func() (bool, error) {
		_, err := s.KubeClient.CoreV1().Namespaces().Get(ctx, s.Namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// FailureTracker tracks active failures and their recovery status.
type FailureTracker struct {
	mu       sync.RWMutex
	failures map[string]*FailureRecord
}

// FailureRecord represents a single failure injection and its recovery status.
type FailureRecord struct {
	ID          string
	Type        FailureType
	Target      string
	StartTime   time.Time
	EndTime     *time.Time
	RecoveryRTO time.Duration
	Recovered   bool
	Error       error
}

// FailureType represents different types of chaos failures.
type FailureType string

const (
	NetworkPartitionFailure FailureType = "network-partition"
	ClusterFailure         FailureType = "cluster-failure"
	ControllerCrash        FailureType = "controller-crash"
	APIServerFailure       FailureType = "apiserver-failure"
	ResourceExhaustion     FailureType = "resource-exhaustion"
)

// NewFailureTracker creates a new failure tracker.
func NewFailureTracker() *FailureTracker {
	return &FailureTracker{
		failures: make(map[string]*FailureRecord),
	}
}

// RecordFailureStart records the start of a failure injection.
func (ft *FailureTracker) RecordFailureStart(id string, failureType FailureType, target string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	
	ft.failures[id] = &FailureRecord{
		ID:        id,
		Type:      failureType,
		Target:    target,
		StartTime: time.Now(),
		Recovered: false,
	}
}

// RecordFailureEnd records the end of a failure injection.
func (ft *FailureTracker) RecordFailureEnd(id string, err error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	
	if record, exists := ft.failures[id]; exists {
		now := time.Now()
		record.EndTime = &now
		record.RecoveryRTO = now.Sub(record.StartTime)
		record.Error = err
		record.Recovered = err == nil
	}
}

// GetFailureRecord returns a failure record by ID.
func (ft *FailureTracker) GetFailureRecord(id string) (*FailureRecord, bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	
	record, exists := ft.failures[id]
	return record, exists
}

// GetAllFailures returns all failure records.
func (ft *FailureTracker) GetAllFailures() map[string]*FailureRecord {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	
	result := make(map[string]*FailureRecord)
	for k, v := range ft.failures {
		result[k] = v
	}
	return result
}

// WaitForRecovery waits for a system to recover from failure with timeout.
func (s *ChaosTestSuite) WaitForRecovery(ctx context.Context, healthCheck func(context.Context) error) error {
	return s.WaitForRecoveryWithTimeout(ctx, healthCheck, DefaultRecoveryTimeout)
}

// WaitForRecoveryWithTimeout waits for recovery with a custom timeout.
func (s *ChaosTestSuite) WaitForRecoveryWithTimeout(ctx context.Context, healthCheck func(context.Context) error, timeout time.Duration) error {
	return wait.PollImmediate(DefaultPollingInterval, timeout, func() (bool, error) {
		err := healthCheck(ctx)
		if err != nil {
			return false, nil // Continue polling
		}
		return true, nil // Recovery successful
	})
}

// CreateTestWorkload creates a test workload for chaos testing.
func (s *ChaosTestSuite) CreateTestWorkload(ctx context.Context, name string, replicas int32) error {
	deployment := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", s.TestID, name),
			Namespace: s.Namespace,
			Labels: map[string]string{
				"app":        name,
				"test-id":    s.TestID,
				"chaos-test": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "busybox:1.35",
					Command: []string{"sleep", "3600"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    "10m",
							corev1.ResourceMemory: "16Mi",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	
	_, err := s.KubeClient.CoreV1().Pods(s.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

// ValidateSystemHealth performs basic system health checks.
func (s *ChaosTestSuite) ValidateSystemHealth(ctx context.Context) error {
	// Check if test namespace is accessible
	_, err := s.KubeClient.CoreV1().Namespaces().Get(ctx, s.Namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("namespace health check failed: %w", err)
	}
	
	// Check if test pods are running
	pods, err := s.KubeClient.CoreV1().Pods(s.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"test-id": s.TestID,
		}).String(),
	})
	if err != nil {
		return fmt.Errorf("pod health check failed: %w", err)
	}
	
	// Verify at least one pod is running
	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}
	
	if runningPods == 0 {
		return fmt.Errorf("no running pods found in test namespace")
	}
	
	return nil
}