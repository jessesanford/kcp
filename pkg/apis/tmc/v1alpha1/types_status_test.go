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

package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadStatusAggregatorValidation(t *testing.T) {
	tests := []struct {
		name       string
		aggregator *WorkloadStatusAggregator
		wantErr    bool
	}{
		{
			name: "valid basic status aggregator",
			aggregator: &WorkloadStatusAggregator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-app-status",
					Namespace: "default",
				},
				Spec: WorkloadStatusAggregatorSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web"},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"env": "prod"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid aggregator with custom fields",
			aggregator: &WorkloadStatusAggregator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment-status",
					Namespace: "apps",
				},
				Spec: WorkloadStatusAggregatorSpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-a", "cluster-b"},
					},
					StatusFields: []StatusFieldSelector{
						{
							FieldPath:       "status.replicas",
							AggregationType: StatusSumAggregation,
							DisplayName:     "Total Replicas",
						},
						{
							FieldPath:       "status.readyReplicas",
							AggregationType: StatusSumAggregation,
							DisplayName:     "Ready Replicas",
						},
					},
					UpdateInterval: &metav1.Duration{Duration: 30 * time.Second},
				},
			},
			wantErr: false,
		},
		{
			name: "valid aggregator with multiple field types",
			aggregator: &WorkloadStatusAggregator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-status",
					Namespace: "services",
				},
				Spec: WorkloadStatusAggregatorSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"tier": "backend"},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west", "us-east"},
					},
					StatusFields: []StatusFieldSelector{
						{
							FieldPath:       "status.replicas",
							AggregationType: StatusSumAggregation,
						},
						{
							FieldPath:       "status.phase",
							AggregationType: StatusFirstNonEmptyAggregation,
						},
						{
							FieldPath:       "status.message",
							AggregationType: StatusConcatAggregation,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate required fields
			if tt.aggregator.Spec.WorkloadSelector.LabelSelector == nil &&
				len(tt.aggregator.Spec.WorkloadSelector.WorkloadTypes) == 0 {
				if !tt.wantErr {
					t.Errorf("Expected valid aggregator but WorkloadSelector is empty")
				}
				return
			}

			// Validate status fields
			for _, field := range tt.aggregator.Spec.StatusFields {
				if field.FieldPath == "" {
					if !tt.wantErr {
						t.Errorf("Expected valid aggregator but StatusField has empty FieldPath")
					}
					return
				}

				validAggregationTypes := []StatusAggregationType{
					StatusSumAggregation, StatusMaxAggregation, StatusMinAggregation,
					StatusAverageAggregation, StatusFirstNonEmptyAggregation, StatusConcatAggregation,
				}
				found := false
				for _, validType := range validAggregationTypes {
					if field.AggregationType == validType {
						found = true
						break
					}
				}
				if !found {
					if !tt.wantErr {
						t.Errorf("Expected valid aggregator but AggregationType %s is invalid", field.AggregationType)
					}
					return
				}
			}

			if tt.wantErr {
				t.Errorf("Expected validation error but aggregator passed validation")
			}
		})
	}
}

func TestWorkloadOverallStatusCalculation(t *testing.T) {
	tests := []struct {
		name           string
		totalReady     int32
		totalCount     int32
		expectedStatus WorkloadOverallStatus
	}{
		{
			name:           "all workloads ready",
			totalReady:     10,
			totalCount:     10,
			expectedStatus: AllReadyStatus,
		},
		{
			name:           "mostly ready (90%)",
			totalReady:     9,
			totalCount:     10,
			expectedStatus: MostlyReadyStatus,
		},
		{
			name:           "exactly 80% ready",
			totalReady:     8,
			totalCount:     10,
			expectedStatus: MostlyReadyStatus,
		},
		{
			name:           "partially ready (50%)",
			totalReady:     5,
			totalCount:     10,
			expectedStatus: PartiallyReadyStatus,
		},
		{
			name:           "exactly 20% ready",
			totalReady:     2,
			totalCount:     10,
			expectedStatus: PartiallyReadyStatus,
		},
		{
			name:           "few ready (10%)",
			totalReady:     1,
			totalCount:     10,
			expectedStatus: NotReadyStatus,
		},
		{
			name:           "none ready",
			totalReady:     0,
			totalCount:     10,
			expectedStatus: NotReadyStatus,
		},
		{
			name:           "no workloads",
			totalReady:     0,
			totalCount:     0,
			expectedStatus: UnknownStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualStatus WorkloadOverallStatus

			if tt.totalCount == 0 {
				actualStatus = UnknownStatus
			} else {
				readyPercentage := float64(tt.totalReady) / float64(tt.totalCount) * 100

				switch {
				case readyPercentage == 100:
					actualStatus = AllReadyStatus
				case readyPercentage >= 80:
					actualStatus = MostlyReadyStatus
				case readyPercentage >= 20:
					actualStatus = PartiallyReadyStatus
				default:
					actualStatus = NotReadyStatus
				}
			}

			if actualStatus != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s (ready: %d, total: %d)",
					tt.expectedStatus, actualStatus, tt.totalReady, tt.totalCount)
			}
		})
	}
}

func TestStatusAggregationTypes(t *testing.T) {
	// Test aggregation type constants
	if StatusSumAggregation != "Sum" {
		t.Errorf("StatusSumAggregation constant has wrong value: %s", StatusSumAggregation)
	}
	if StatusMaxAggregation != "Max" {
		t.Errorf("StatusMaxAggregation constant has wrong value: %s", StatusMaxAggregation)
	}
	if StatusMinAggregation != "Min" {
		t.Errorf("StatusMinAggregation constant has wrong value: %s", StatusMinAggregation)
	}
	if StatusAverageAggregation != "Average" {
		t.Errorf("StatusAverageAggregation constant has wrong value: %s", StatusAverageAggregation)
	}
	if StatusFirstNonEmptyAggregation != "FirstNonEmpty" {
		t.Errorf("StatusFirstNonEmptyAggregation constant has wrong value: %s", StatusFirstNonEmptyAggregation)
	}
	if StatusConcatAggregation != "Concat" {
		t.Errorf("StatusConcatAggregation constant has wrong value: %s", StatusConcatAggregation)
	}

	// Test overall status constants
	if AllReadyStatus != "AllReady" {
		t.Errorf("AllReadyStatus constant has wrong value: %s", AllReadyStatus)
	}
	if MostlyReadyStatus != "MostlyReady" {
		t.Errorf("MostlyReadyStatus constant has wrong value: %s", MostlyReadyStatus)
	}
	if PartiallyReadyStatus != "PartiallyReady" {
		t.Errorf("PartiallyReadyStatus constant has wrong value: %s", PartiallyReadyStatus)
	}
	if NotReadyStatus != "NotReady" {
		t.Errorf("NotReadyStatus constant has wrong value: %s", NotReadyStatus)
	}
	if UnknownStatus != "Unknown" {
		t.Errorf("UnknownStatus constant has wrong value: %s", UnknownStatus)
	}
}

func TestClusterWorkloadStatusValidation(t *testing.T) {
	status := ClusterWorkloadStatus{
		ClusterName:   "test-cluster",
		WorkloadCount: 5,
		ReadyCount:    4,
		LastSeen:      metav1.Now(),
		Reachable:     true,
	}

	// Validate ready count doesn't exceed workload count
	if status.ReadyCount > status.WorkloadCount {
		t.Errorf("ReadyCount (%d) should not exceed WorkloadCount (%d)",
			status.ReadyCount, status.WorkloadCount)
	}

	// Validate cluster name is not empty
	if status.ClusterName == "" {
		t.Errorf("ClusterName should not be empty")
	}

	// Validate counts are non-negative
	if status.WorkloadCount < 0 {
		t.Errorf("WorkloadCount should be non-negative, got %d", status.WorkloadCount)
	}
	if status.ReadyCount < 0 {
		t.Errorf("ReadyCount should be non-negative, got %d", status.ReadyCount)
	}

	// Validate last seen time is reasonable
	timeSinceLastSeen := time.Since(status.LastSeen.Time)
	if timeSinceLastSeen < 0 {
		t.Errorf("LastSeen is in the future")
	}
}

func TestWorkloadStatusAggregatorStatusCalculations(t *testing.T) {
	status := WorkloadStatusAggregatorStatus{
		LastUpdateTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
		TotalWorkloads: &[]int32{15}[0],
		ReadyWorkloads: &[]int32{12}[0],
		OverallStatus:  MostlyReadyStatus,
		ClusterStatus: map[string]ClusterWorkloadStatus{
			"cluster-a": {
				ClusterName:   "cluster-a",
				WorkloadCount: 8,
				ReadyCount:    7,
				LastSeen:      metav1.Now(),
				Reachable:     true,
			},
			"cluster-b": {
				ClusterName:   "cluster-b",
				WorkloadCount: 7,
				ReadyCount:    5,
				LastSeen:      metav1.Now(),
				Reachable:     true,
			},
		},
		AggregatedFields: map[string]string{
			"totalReplicas": "30",
			"readyReplicas": "24",
		},
	}

	// Validate total workload calculation
	expectedTotal := int32(8 + 7)
	if *status.TotalWorkloads != expectedTotal {
		t.Errorf("Expected total workloads %d, got %d", expectedTotal, *status.TotalWorkloads)
	}

	// Validate ready workload calculation
	expectedReady := int32(7 + 5)
	if *status.ReadyWorkloads != expectedReady {
		t.Errorf("Expected ready workloads %d, got %d", expectedReady, *status.ReadyWorkloads)
	}

	// Validate overall status matches ready percentage
	readyPercentage := float64(*status.ReadyWorkloads) / float64(*status.TotalWorkloads) * 100
	if readyPercentage >= 80 && status.OverallStatus != MostlyReadyStatus {
		t.Errorf("Expected MostlyReadyStatus for %f%% ready, got %s", readyPercentage, status.OverallStatus)
	}

	// Validate cluster status consistency
	for clusterName, clusterStatus := range status.ClusterStatus {
		if clusterStatus.ClusterName != clusterName {
			t.Errorf("Cluster status key (%s) doesn't match ClusterName (%s)",
				clusterName, clusterStatus.ClusterName)
		}

		if clusterStatus.ReadyCount > clusterStatus.WorkloadCount {
			t.Errorf("Cluster %s: ReadyCount (%d) > WorkloadCount (%d)",
				clusterName, clusterStatus.ReadyCount, clusterStatus.WorkloadCount)
		}
	}

	// Validate aggregated fields contain expected values
	if totalReplicas, ok := status.AggregatedFields["totalReplicas"]; ok {
		if totalReplicas != "30" {
			t.Errorf("Expected totalReplicas '30', got %v", totalReplicas)
		}
	} else {
		t.Errorf("AggregatedFields missing totalReplicas")
	}
}

func TestWorkloadStatusSummaryValidation(t *testing.T) {
	summary := WorkloadStatusSummary{
		WorkloadRef: WorkloadReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "web-app",
			Namespace:  "default",
		},
		ClusterName:        "cluster-a",
		Ready:              true,
		Phase:              "Running",
		Message:            "Deployment is ready",
		LastTransitionTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
		Conditions: []WorkloadCondition{
			{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				Reason:             "MinimumReplicasAvailable",
				Message:            "Deployment has minimum availability",
				LastTransitionTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			},
		},
	}

	// Validate workload reference
	if summary.WorkloadRef.Name == "" {
		t.Errorf("WorkloadRef.Name should not be empty")
	}
	if summary.WorkloadRef.APIVersion == "" {
		t.Errorf("WorkloadRef.APIVersion should not be empty")
	}
	if summary.WorkloadRef.Kind == "" {
		t.Errorf("WorkloadRef.Kind should not be empty")
	}

	// Validate cluster name
	if summary.ClusterName == "" {
		t.Errorf("ClusterName should not be empty")
	}

	// Validate conditions
	for _, condition := range summary.Conditions {
		if condition.Type == "" {
			t.Errorf("Condition Type should not be empty")
		}

		validStatuses := []metav1.ConditionStatus{
			metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown,
		}
		found := false
		for _, validStatus := range validStatuses {
			if condition.Status == validStatus {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Condition has invalid status: %s", condition.Status)
		}
	}

	// Validate transition times are reasonable
	if summary.LastTransitionTime != nil {
		timeSinceTransition := time.Since(summary.LastTransitionTime.Time)
		if timeSinceTransition < 0 {
			t.Errorf("LastTransitionTime is in the future")
		}
	}
}
