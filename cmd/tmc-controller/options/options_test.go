// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package options

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	t.Run("creates default options", func(t *testing.T) {
		opts := NewOptions()
		
		assert.NotNil(t, opts)
		assert.NotNil(t, opts.ClusterKubeconfigs)
		assert.Equal(t, 30*time.Second, opts.ResyncPeriod)
		assert.Equal(t, 5, opts.WorkerCount)
		assert.Equal(t, 30*time.Second, opts.ShutdownTimeout)
		assert.Equal(t, 8080, opts.MetricsPort)
		assert.Equal(t, 8081, opts.HealthPort)
		assert.Empty(t, opts.KCPKubeconfig)
		assert.Empty(t, opts.Workspace)
		assert.Empty(t, opts.ClusterKubeconfigs)
	})
}

func TestOptions_AddFlags(t *testing.T) {
	t.Run("adds all flags correctly", func(t *testing.T) {
		opts := NewOptions()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		
		opts.AddFlags(fs)
		
		// Verify all expected flags are present
		expectedFlags := []string{
			"kcp-kubeconfig",
			"workspace",
			"cluster-kubeconfigs",
			"resync-period",
			"worker-count",
			"shutdown-timeout",
			"metrics-port",
			"health-port",
		}
		
		for _, flagName := range expectedFlags {
			flag := fs.Lookup(flagName)
			assert.NotNil(t, flag, "Flag %s should be present", flagName)
		}
	})
	
	t.Run("flag values can be parsed", func(t *testing.T) {
		opts := NewOptions()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		
		opts.AddFlags(fs)
		
		// Test parsing string flags
		args := []string{
			"--kcp-kubeconfig=/path/to/kcp/config",
			"--workspace=root:production",
			"--cluster-kubeconfigs=cluster1=/path/to/cluster1,cluster2=/path/to/cluster2",
			"--resync-period=60s",
			"--worker-count=10",
			"--shutdown-timeout=45s",
			"--metrics-port=9090",
			"--health-port=9091",
		}
		
		err := fs.Parse(args)
		assert.NoError(t, err)
		
		// Verify values were parsed correctly
		assert.Equal(t, "/path/to/kcp/config", opts.KCPKubeconfig)
		assert.Equal(t, "root:production", opts.Workspace)
		assert.Equal(t, map[string]string{
			"cluster1": "/path/to/cluster1",
			"cluster2": "/path/to/cluster2",
		}, opts.ClusterKubeconfigs)
		assert.Equal(t, 60*time.Second, opts.ResyncPeriod)
		assert.Equal(t, 10, opts.WorkerCount)
		assert.Equal(t, 45*time.Second, opts.ShutdownTimeout)
		assert.Equal(t, 9090, opts.MetricsPort)
		assert.Equal(t, 9091, opts.HealthPort)
	})
}

func TestOptions_Validate(t *testing.T) {
	tests := map[string]struct {
		setupOpts   func(*Options)
		expectError bool
		errorText   string
	}{
		"valid options": {
			setupOpts: func(opts *Options) {
				opts.KCPKubeconfig = "/path/to/kcp/config"
				opts.Workspace = "root:test"
				opts.ClusterKubeconfigs = map[string]string{
					"cluster1": "/path/to/cluster1",
				}
			},
			expectError: false,
		},
		"missing kcp kubeconfig": {
			setupOpts: func(opts *Options) {
				opts.Workspace = "root:test"
				opts.ClusterKubeconfigs = map[string]string{
					"cluster1": "/path/to/cluster1",
				}
			},
			expectError: true,
			errorText:   "--kcp-kubeconfig is required",
		},
		"missing workspace": {
			setupOpts: func(opts *Options) {
				opts.KCPKubeconfig = "/path/to/kcp/config"
				opts.ClusterKubeconfigs = map[string]string{
					"cluster1": "/path/to/cluster1",
				}
			},
			expectError: true,
			errorText:   "--workspace is required",
		},
		"missing cluster kubeconfigs": {
			setupOpts: func(opts *Options) {
				opts.KCPKubeconfig = "/path/to/kcp/config"
				opts.Workspace = "root:test"
				// ClusterKubeconfigs is empty
			},
			expectError: true,
			errorText:   "at least one --cluster-kubeconfigs entry is required",
		},
		"invalid worker count": {
			setupOpts: func(opts *Options) {
				opts.KCPKubeconfig = "/path/to/kcp/config"
				opts.Workspace = "root:test"
				opts.ClusterKubeconfigs = map[string]string{
					"cluster1": "/path/to/cluster1",
				}
				opts.WorkerCount = 0
			},
			expectError: true,
			errorText:   "--worker-count must be positive",
		},
		"negative worker count": {
			setupOpts: func(opts *Options) {
				opts.KCPKubeconfig = "/path/to/kcp/config"
				opts.Workspace = "root:test"
				opts.ClusterKubeconfigs = map[string]string{
					"cluster1": "/path/to/cluster1",
				}
				opts.WorkerCount = -5
			},
			expectError: true,
			errorText:   "--worker-count must be positive",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := NewOptions()
			tc.setupOpts(opts)
			
			err := opts.Validate()
			
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorText)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOptions_Integration(t *testing.T) {
	t.Run("full flag parsing and validation workflow", func(t *testing.T) {
		opts := NewOptions()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		
		opts.AddFlags(fs)
		
		// Parse valid arguments
		args := []string{
			"--kcp-kubeconfig=/tmp/kcp-config",
			"--workspace=root:integration-test",
			"--cluster-kubeconfigs=test-cluster=/tmp/test-cluster-config",
			"--resync-period=45s",
			"--worker-count=3",
			"--shutdown-timeout=60s",
			"--metrics-port=8090",
			"--health-port=8091",
		}
		
		err := fs.Parse(args)
		assert.NoError(t, err)
		
		// Validate the parsed options
		err = opts.Validate()
		assert.NoError(t, err)
		
		// Verify all values are set correctly
		assert.Equal(t, "/tmp/kcp-config", opts.KCPKubeconfig)
		assert.Equal(t, "root:integration-test", opts.Workspace)
		assert.Equal(t, map[string]string{
			"test-cluster": "/tmp/test-cluster-config",
		}, opts.ClusterKubeconfigs)
		assert.Equal(t, 45*time.Second, opts.ResyncPeriod)
		assert.Equal(t, 3, opts.WorkerCount)
		assert.Equal(t, 60*time.Second, opts.ShutdownTimeout)
		assert.Equal(t, 8090, opts.MetricsPort)
		assert.Equal(t, 8091, opts.HealthPort)
	})
	
	t.Run("minimal valid configuration", func(t *testing.T) {
		opts := NewOptions()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		
		opts.AddFlags(fs)
		
		// Parse minimal required arguments
		args := []string{
			"--kcp-kubeconfig=/tmp/kcp-config",
			"--workspace=root:minimal",
			"--cluster-kubeconfigs=minimal-cluster=/tmp/minimal-config",
		}
		
		err := fs.Parse(args)
		assert.NoError(t, err)
		
		// Should validate successfully with defaults
		err = opts.Validate()
		assert.NoError(t, err)
		
		// Verify required fields are set and defaults are preserved
		assert.Equal(t, "/tmp/kcp-config", opts.KCPKubeconfig)
		assert.Equal(t, "root:minimal", opts.Workspace)
		assert.Equal(t, map[string]string{
			"minimal-cluster": "/tmp/minimal-config",
		}, opts.ClusterKubeconfigs)
		
		// Defaults should be preserved
		assert.Equal(t, 30*time.Second, opts.ResyncPeriod)
		assert.Equal(t, 5, opts.WorkerCount)
		assert.Equal(t, 30*time.Second, opts.ShutdownTimeout)
		assert.Equal(t, 8080, opts.MetricsPort)
		assert.Equal(t, 8081, opts.HealthPort)
	})
}
