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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTMCControllerOptions(t *testing.T) {
	opts := NewTMCControllerOptions()
	
	// Check default values
	assert.Equal(t, "", opts.KubeConfig)
	assert.Empty(t, opts.Workspaces)
	assert.Equal(t, 10*time.Minute, opts.ResyncPeriod)
	assert.True(t, opts.LeaderElection)
	assert.Equal(t, "kcp-system", opts.LeaderElectionNamespace)
	assert.Equal(t, "tmc-controller", opts.LeaderElectionID)
	assert.Equal(t, 30*time.Second, opts.ClusterHealthCheckInterval)
	assert.Equal(t, 10, opts.MaxConcurrentReconciles)
	assert.Equal(t, 2, opts.LogLevel)
	assert.Nil(t, opts.Config)
}

func TestTMCControllerOptions_AddFlags(t *testing.T) {
	opts := NewTMCControllerOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	
	opts.AddFlags(fs)
	
	// Test that flags were added
	flag := fs.Lookup("kubeconfig")
	require.NotNil(t, flag)
	assert.Equal(t, "", flag.DefValue)
	
	flag = fs.Lookup("workspaces")
	require.NotNil(t, flag)
	
	flag = fs.Lookup("resync-period")
	require.NotNil(t, flag)
	assert.Equal(t, "10m0s", flag.DefValue)
	
	flag = fs.Lookup("leader-election")
	require.NotNil(t, flag)
	assert.Equal(t, "true", flag.DefValue)
	
	flag = fs.Lookup("cluster-health-check-interval")
	require.NotNil(t, flag)
	assert.Equal(t, "30s", flag.DefValue)
	
	flag = fs.Lookup("max-concurrent-reconciles")
	require.NotNil(t, flag)
	assert.Equal(t, "10", flag.DefValue)
	
	flag = fs.Lookup("log-level")
	require.NotNil(t, flag)
	assert.Equal(t, "2", flag.DefValue)
}

func TestTMCControllerOptions_Validate(t *testing.T) {
	tests := map[string]struct {
		opts        func() *TMCControllerOptions
		wantError   bool
		errorString string
	}{
		"valid default options": {
			opts: func() *TMCControllerOptions {
				return NewTMCControllerOptions()
			},
			wantError: false,
		},
		"negative resync period": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.ResyncPeriod = -1 * time.Second
				return opts
			},
			wantError:   true,
			errorString: "resync-period must be positive",
		},
		"zero resync period": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.ResyncPeriod = 0
				return opts
			},
			wantError:   true,
			errorString: "resync-period must be positive",
		},
		"negative health check interval": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.ClusterHealthCheckInterval = -1 * time.Second
				return opts
			},
			wantError:   true,
			errorString: "cluster-health-check-interval must be positive",
		},
		"zero max concurrent reconciles": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.MaxConcurrentReconciles = 0
				return opts
			},
			wantError:   true,
			errorString: "max-concurrent-reconciles must be positive",
		},
		"negative max concurrent reconciles": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.MaxConcurrentReconciles = -1
				return opts
			},
			wantError:   true,
			errorString: "max-concurrent-reconciles must be positive",
		},
		"log level too low": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.LogLevel = -1
				return opts
			},
			wantError:   true,
			errorString: "log-level must be between 0 and 10",
		},
		"log level too high": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.LogLevel = 11
				return opts
			},
			wantError:   true,
			errorString: "log-level must be between 0 and 10",
		},
		"leader election enabled but no namespace": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.LeaderElection = true
				opts.LeaderElectionNamespace = ""
				return opts
			},
			wantError:   true,
			errorString: "leader-election-namespace is required when leader election is enabled",
		},
		"leader election enabled but no ID": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.LeaderElection = true
				opts.LeaderElectionID = ""
				return opts
			},
			wantError:   true,
			errorString: "leader-election-id is required when leader election is enabled",
		},
		"leader election disabled with empty namespace and ID": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.LeaderElection = false
				opts.LeaderElectionNamespace = ""
				opts.LeaderElectionID = ""
				return opts
			},
			wantError: false,
		},
		"invalid workspace path": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.Workspaces = []string{"invalid:workspace:path:with:too:many:colons"}
				return opts
			},
			wantError:   true,
			errorString: "invalid workspace path",
		},
		"valid workspace paths": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.Workspaces = []string{"root:org:workspace1", "root:org:workspace2"}
				return opts
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := tc.opts()
			err := opts.Validate()

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTMCControllerOptions_Complete(t *testing.T) {
	tests := map[string]struct {
		opts        func() *TMCControllerOptions
		wantError   bool
		errorString string
	}{
		"complete with empty kubeconfig": {
			opts: func() *TMCControllerOptions {
				return NewTMCControllerOptions()
			},
			wantError:   true, // Will fail because no in-cluster config in test
			errorString: "failed to get in-cluster config",
		},
		"complete with invalid kubeconfig path": {
			opts: func() *TMCControllerOptions {
				opts := NewTMCControllerOptions()
				opts.KubeConfig = "/nonexistent/kubeconfig"
				return opts
			},
			wantError:   true,
			errorString: "failed to build config from kubeconfig",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := tc.opts()
			err := opts.Complete()

			if tc.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorString)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, opts.Config)
				assert.Equal(t, float32(100), opts.Config.QPS)
				assert.Equal(t, 200, opts.Config.Burst)
			}
		})
	}
}