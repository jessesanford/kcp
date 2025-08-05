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

package options

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestNewOptions(t *testing.T) {
	opts, err := NewOptions()
	if err != nil {
		t.Fatalf("Failed to create options: %v", err)
	}

	if opts == nil {
		t.Fatal("Expected non-nil options")
	}

	// Check default values
	if opts.SyncPeriod != 30*time.Second {
		t.Errorf("Expected default sync period 30s, got %v", opts.SyncPeriod)
	}

	if opts.WorkerCount != 2 {
		t.Errorf("Expected default worker count 2, got %d", opts.WorkerCount)
	}

	if opts.LeaderElection {
		t.Error("Expected leader election disabled by default")
	}

	if opts.LeaderElectionNamespace != "kcp-system" {
		t.Errorf("Expected default leader election namespace 'kcp-system', got %q", opts.LeaderElectionNamespace)
	}

	if opts.MetricsBindAddress != ":8080" {
		t.Errorf("Expected default metrics bind address ':8080', got %q", opts.MetricsBindAddress)
	}
}

func TestAddFlags(t *testing.T) {
	opts, err := NewOptions()
	if err != nil {
		t.Fatalf("Failed to create options: %v", err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	err = opts.AddFlags(flags)
	if err != nil {
		t.Fatalf("Failed to add flags: %v", err)
	}

	// Test that flags were added
	expectedFlags := []string{
		"kubeconfig",
		"master",
		"namespace",
		"sync-period",
		"worker-count",
		"enable-leader-election",
		"leader-election-namespace",
		"leader-election-id",
		"metrics-bind-address",
		"health-probe-bind-address",
	}

	for _, flagName := range expectedFlags {
		if flags.Lookup(flagName) == nil {
			t.Errorf("Expected flag %q to be added", flagName)
		}
	}
}

func TestFlagParsing(t *testing.T) {
	opts, err := NewOptions()
	if err != nil {
		t.Fatalf("Failed to create options: %v", err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(flags)

	// Parse test flags
	args := []string{
		"--namespace=test-ns",
		"--sync-period=45s",
		"--worker-count=5",
		"--enable-leader-election=true",
		"--metrics-bind-address=:9090",
	}

	err = flags.Parse(args)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Check parsed values
	if opts.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got %q", opts.Namespace)
	}

	if opts.SyncPeriod != 45*time.Second {
		t.Errorf("Expected sync period 45s, got %v", opts.SyncPeriod)
	}

	if opts.WorkerCount != 5 {
		t.Errorf("Expected worker count 5, got %d", opts.WorkerCount)
	}

	if !opts.LeaderElection {
		t.Error("Expected leader election to be enabled")
	}

	if opts.MetricsBindAddress != ":9090" {
		t.Errorf("Expected metrics bind address ':9090', got %q", opts.MetricsBindAddress)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options",
			options: &Options{
				SyncPeriod:              30 * time.Second,
				WorkerCount:             2,
				LeaderElection:          false,
				LeaderElectionNamespace: "kcp-system",
				LeaderElectionID:        "test-leader",
			},
			wantErr: false,
		},
		{
			name: "invalid sync period - zero",
			options: &Options{
				SyncPeriod:  0,
				WorkerCount: 2,
			},
			wantErr: true,
			errMsg:  "sync-period must be positive",
		},
		{
			name: "invalid sync period - negative",
			options: &Options{
				SyncPeriod:  -1 * time.Second,
				WorkerCount: 2,
			},
			wantErr: true,
			errMsg:  "sync-period must be positive",
		},
		{
			name: "invalid worker count - zero",
			options: &Options{
				SyncPeriod:  30 * time.Second,
				WorkerCount: 0,
			},
			wantErr: true,
			errMsg:  "worker-count must be positive",
		},
		{
			name: "invalid worker count - negative",
			options: &Options{
				SyncPeriod:  30 * time.Second,
				WorkerCount: -1,
			},
			wantErr: true,
			errMsg:  "worker-count must be positive",
		},
		{
			name: "invalid worker count - too high",
			options: &Options{
				SyncPeriod:  30 * time.Second,
				WorkerCount: 51,
			},
			wantErr: true,
			errMsg:  "worker-count must be <= 50",
		},
		{
			name: "leader election enabled without namespace",
			options: &Options{
				SyncPeriod:              30 * time.Second,
				WorkerCount:             2,
				LeaderElection:          true,
				LeaderElectionNamespace: "",
				LeaderElectionID:        "test-leader",
			},
			wantErr: true,
			errMsg:  "leader-election-namespace must be specified",
		},
		{
			name: "leader election enabled without ID",
			options: &Options{
				SyncPeriod:              30 * time.Second,
				WorkerCount:             2,
				LeaderElection:          true,
				LeaderElectionNamespace: "kcp-system",
				LeaderElectionID:        "",
			},
			wantErr: true,
			errMsg:  "leader-election-id must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestComplete(t *testing.T) {
	opts, err := NewOptions()
	if err != nil {
		t.Fatalf("Failed to create options: %v", err)
	}

	// Set a non-existent kubeconfig path
	opts.Kubeconfig = "/non/existent/path"

	err = opts.Complete()
	if err != nil {
		t.Errorf("Complete should not fail, got: %v", err)
	}

	// Should clear non-existent kubeconfig to use in-cluster config
	if opts.Kubeconfig != "" {
		t.Errorf("Expected kubeconfig to be cleared for non-existent path, got %q", opts.Kubeconfig)
	}
}

func TestString(t *testing.T) {
	opts := &Options{
		Namespace:      "test-ns",
		SyncPeriod:     45 * time.Second,
		WorkerCount:    5,
		LeaderElection: true,
	}

	str := opts.String()
	expectedSubstrings := []string{
		"test-ns",
		"45s",
		"5",
		"true",
	}

	for _, substr := range expectedSubstrings {
		if !containsString(str, substr) {
			t.Errorf("Expected string representation to contain %q, got %q", substr, str)
		}
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsString(s[1:], substr) || (len(s) > 0 && s[:len(substr)] == substr))
}