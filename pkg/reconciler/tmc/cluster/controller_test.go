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

package cluster

import (
	"testing"
)

// TestConstants verifies that basic constants are defined correctly
func TestConstants(t *testing.T) {
	if ClusterReadyCondition != "Ready" {
		t.Errorf("Expected ClusterReadyCondition to be 'Ready', got %q", ClusterReadyCondition)
	}
	
	if ClusterConnectivityCondition != "Connectivity" {
		t.Errorf("Expected ClusterConnectivityCondition to be 'Connectivity', got %q", ClusterConnectivityCondition)
	}
	
	if ClusterHealthCondition != "Health" {
		t.Errorf("Expected ClusterHealthCondition to be 'Health', got %q", ClusterHealthCondition)
	}
}

// TestReconcileStatus verifies reconcile status constants
func TestReconcileStatus(t *testing.T) {
	if reconcileStatusContinue != 0 {
		t.Errorf("Expected reconcileStatusContinue to be 0, got %d", reconcileStatusContinue)
	}
	
	if reconcileStatusStop != 1 {
		t.Errorf("Expected reconcileStatusStop to be 1, got %d", reconcileStatusStop)
	}
}
