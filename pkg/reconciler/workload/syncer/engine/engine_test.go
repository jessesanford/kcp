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

package engine

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes/scheme"

	kcpfake "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestNewEngine(t *testing.T) {
	kcpClient := kcpfake.NewSimpleClientset()
	downstreamClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	kcpInformerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, time.Minute)
	downstreamInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(downstreamClient, time.Minute)

	tests := map[string]struct {
		config      *EngineConfig
		expectError bool
	}{
		"default config": {
			config:      nil,
			expectError: false,
		},
		"custom config": {
			config: &EngineConfig{
				WorkerCount:     10,
				ResyncPeriod:    5 * time.Minute,
				MaxRetries:      5,
				RateLimitPerSec: 50,
				QueueDepth:      2000,
				EnableProfiling: true,
			},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			engine := NewEngine(kcpClient, downstreamClient, kcpInformerFactory, downstreamInformerFactory, tc.config)
			
			if engine == nil {
				t.Fatal("Expected engine to be created, got nil")
			}
			
			if engine.config == nil {
				t.Fatal("Expected config to be set")
			}
			
			if tc.config == nil {
				// Should use default config
				defaultConfig := DefaultEngineConfig()
				if engine.config.WorkerCount != defaultConfig.WorkerCount {
					t.Errorf("Expected worker count %d, got %d", defaultConfig.WorkerCount, engine.config.WorkerCount)
				}
			} else {
				if engine.config.WorkerCount != tc.config.WorkerCount {
					t.Errorf("Expected worker count %d, got %d", tc.config.WorkerCount, engine.config.WorkerCount)
				}
			}
		})
	}
}

func TestEngineRegisterResourceSyncer(t *testing.T) {
	engine := createTestEngine()

	testGVR := schema.GroupVersionResource{
		Group:    "test.io",
		Version:  "v1",
		Resource: "testresources",
	}

	tests := map[string]struct {
		gvr         schema.GroupVersionResource
		expectError bool
	}{
		"valid registration": {
			gvr:         testGVR,
			expectError: false,
		},
		"duplicate registration": {
			gvr:         testGVR,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := engine.RegisterResourceSyncer(tc.gvr)
			
			if tc.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestEngineGetStatus(t *testing.T) {
	engine := createTestEngine()
	
	status := engine.GetStatus()
	if status == nil {
		t.Fatal("Expected status to be returned")
	}
	
	if status.Connected {
		t.Error("Expected Connected to be false initially")
	}
	
	if status.SyncedResources == nil {
		t.Error("Expected SyncedResources map to be initialized")
	}
	
	if status.PendingResources == nil {
		t.Error("Expected PendingResources map to be initialized")
	}
	
	if status.FailedResources == nil {
		t.Error("Expected FailedResources map to be initialized")
	}
}

func TestSyncItem(t *testing.T) {
	now := metav1.Now()
	testGVR := schema.GroupVersionResource{
		Group:    "test.io",
		Version:  "v1",
		Resource: "testresources",
	}

	item := &SyncItem{
		GVR:       testGVR,
		Key:       "default/test-object",
		Action:    ActionAdd,
		Object:    "test-object",
		Retries:   0,
		Timestamp: now,
	}

	if item.GVR != testGVR {
		t.Errorf("Expected GVR %v, got %v", testGVR, item.GVR)
	}
	
	if item.Key != "default/test-object" {
		t.Errorf("Expected key 'default/test-object', got %s", item.Key)
	}
	
	if item.Action != ActionAdd {
		t.Errorf("Expected action %s, got %s", ActionAdd, item.Action)
	}
}

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()
	
	if config == nil {
		t.Fatal("Expected config to be created")
	}
	
	if config.WorkerCount <= 0 {
		t.Error("Expected positive worker count")
	}
	
	if config.ResyncPeriod <= 0 {
		t.Error("Expected positive resync period")
	}
	
	if config.MaxRetries <= 0 {
		t.Error("Expected positive max retries")
	}
	
	if config.QueueDepth <= 0 {
		t.Error("Expected positive queue depth")
	}
}

func TestNewResourceSyncer(t *testing.T) {
	engine := createTestEngine()
	testGVR := schema.GroupVersionResource{
		Group:    "test.io",
		Version:  "v1",
		Resource: "testresources",
	}

	tests := map[string]struct {
		engine      *Engine
		expectError bool
	}{
		"valid engine": {
			engine:      engine,
			expectError: false,
		},
		"nil engine": {
			engine:      nil,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncer, err := NewResourceSyncer(testGVR, tc.engine)
			
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if syncer != nil {
					t.Error("Expected syncer to be nil when error occurs")
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
				if syncer == nil {
					t.Fatal("Expected syncer to be created")
				}
				if syncer.gvr != testGVR {
					t.Errorf("Expected GVR %v, got %v", testGVR, syncer.gvr)
				}
			}
		})
	}
}

// Helper function to create a test engine
func createTestEngine() *Engine {
	kcpClient := kcpfake.NewSimpleClientset()
	downstreamClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	kcpInformerFactory := kcpinformers.NewSharedInformerFactory(kcpClient, time.Minute)
	downstreamInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(downstreamClient, time.Minute)
	
	return NewEngine(kcpClient, downstreamClient, kcpInformerFactory, downstreamInformerFactory, nil)
}

func TestEngineLifecycle(t *testing.T) {
	engine := createTestEngine()
	
	// Test Stop without Start (should not panic)
	engine.Stop()
	
	// Test multiple stops (should not panic)
	engine.Stop()
}