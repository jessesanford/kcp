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

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcp-dev/kcp/cmd/tmc-controller/options"
)

// Helper function to create temporary kubeconfig file for testing
func createTempKubeconfig(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kubeconfig")
	
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	
	return configPath
}

// Sample kubeconfig content for testing
const sampleKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

func TestBuildKCPConfig(t *testing.T) {
	t.Run("successful config creation", func(t *testing.T) {
		// Create temporary kubeconfig file
		configPath := createTempKubeconfig(t, sampleKubeconfig)
		
		opts := &options.Options{
			KCPKubeconfig: configPath,
		}
		
		config, err := buildKCPConfig(opts)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		
		// Verify KCP-specific settings
		assert.Equal(t, float32(50.0), config.QPS)
		assert.Equal(t, 100, config.Burst)
		assert.Equal(t, "tmc-controller/v1alpha1", config.UserAgent)
		assert.Equal(t, "https://localhost:6443", config.Host)
	})
	
	t.Run("missing kubeconfig file", func(t *testing.T) {
		opts := &options.Options{
			KCPKubeconfig: "/nonexistent/path/config",
		}
		
		config, err := buildKCPConfig(opts)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to build KCP config")
	})
	
	t.Run("empty kubeconfig path", func(t *testing.T) {
		opts := &options.Options{
			KCPKubeconfig: "",
		}
		
		config, err := buildKCPConfig(opts)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "KCP kubeconfig is required")
	})
	
	t.Run("invalid kubeconfig content", func(t *testing.T) {
		// Create temporary file with invalid YAML
		configPath := createTempKubeconfig(t, "invalid: yaml: content: [")
		
		opts := &options.Options{
			KCPKubeconfig: configPath,
		}
		
		config, err := buildKCPConfig(opts)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to build KCP config")
	})
}

func TestBuildClusterConfigs(t *testing.T) {
	t.Run("successful multi-cluster config creation", func(t *testing.T) {
		// Create multiple temporary kubeconfig files
		cluster1Config := createTempKubeconfig(t, sampleKubeconfig)
		cluster2Config := createTempKubeconfig(t, `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster2:6443
  name: cluster2
contexts:
- context:
    cluster: cluster2
    user: cluster2-user
  name: cluster2-context
current-context: cluster2-context
users:
- name: cluster2-user
  user:
    token: cluster2-token
`)
		
		opts := &options.Options{
			ClusterKubeconfigs: map[string]string{
				"cluster1": cluster1Config,
				"cluster2": cluster2Config,
			},
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.NoError(t, err)
		assert.NotNil(t, configs)
		assert.Len(t, configs, 2)
		
		// Verify cluster1 config
		cluster1Cfg, exists := configs["cluster1"]
		assert.True(t, exists)
		assert.Equal(t, "https://localhost:6443", cluster1Cfg.Host)
		assert.Equal(t, float32(30.0), cluster1Cfg.QPS)
		assert.Equal(t, 60, cluster1Cfg.Burst)
		assert.Equal(t, "tmc-controller/v1alpha1 cluster=cluster1", cluster1Cfg.UserAgent)
		
		// Verify cluster2 config
		cluster2Cfg, exists := configs["cluster2"]
		assert.True(t, exists)
		assert.Equal(t, "https://cluster2:6443", cluster2Cfg.Host)
		assert.Equal(t, float32(30.0), cluster2Cfg.QPS)
		assert.Equal(t, 60, cluster2Cfg.Burst)
		assert.Equal(t, "tmc-controller/v1alpha1 cluster=cluster2", cluster2Cfg.UserAgent)
	})
	
	t.Run("empty cluster configs", func(t *testing.T) {
		opts := &options.Options{
			ClusterKubeconfigs: map[string]string{},
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "at least one cluster kubeconfig is required")
	})
	
	t.Run("nil cluster configs", func(t *testing.T) {
		opts := &options.Options{
			ClusterKubeconfigs: nil,
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "at least one cluster kubeconfig is required")
	})
	
	t.Run("missing cluster kubeconfig file", func(t *testing.T) {
		opts := &options.Options{
			ClusterKubeconfigs: map[string]string{
				"missing-cluster": "/nonexistent/path/config",
			},
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "failed to build cluster config for missing-cluster")
	})
	
	t.Run("invalid cluster kubeconfig content", func(t *testing.T) {
		// Create temporary file with invalid YAML
		invalidConfig := createTempKubeconfig(t, "invalid: yaml: content: [")
		
		opts := &options.Options{
			ClusterKubeconfigs: map[string]string{
				"invalid-cluster": invalidConfig,
			},
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "failed to build cluster config for invalid-cluster")
	})
	
	t.Run("partial failure in multi-cluster setup", func(t *testing.T) {
		// Create one valid and one invalid config
		validConfig := createTempKubeconfig(t, sampleKubeconfig)
		invalidConfig := "/nonexistent/path/config"
		
		opts := &options.Options{
			ClusterKubeconfigs: map[string]string{
				"valid-cluster":   validConfig,
				"invalid-cluster": invalidConfig,
			},
		}
		
		configs, err := buildClusterConfigs(opts)
		assert.Error(t, err)
		assert.Nil(t, configs)
		assert.Contains(t, err.Error(), "failed to build cluster config for invalid-cluster")
	})
}

func TestMain_Integration(t *testing.T) {
	t.Run("complete config building workflow", func(t *testing.T) {
		// Create temporary kubeconfig files
		kcpConfig := createTempKubeconfig(t, sampleKubeconfig)
		cluster1Config := createTempKubeconfig(t, sampleKubeconfig)
		
		opts := &options.Options{
			KCPKubeconfig: kcpConfig,
			Workspace:     "root:test",
			ClusterKubeconfigs: map[string]string{
				"test-cluster": cluster1Config,
			},
			ResyncPeriod:    30 * time.Second,
			WorkerCount:     5,
			ShutdownTimeout: 30 * time.Second,
			MetricsPort:     8080,
			HealthPort:      8081,
		}
		
		// Test building KCP config
		kcpRestConfig, err := buildKCPConfig(opts)
		assert.NoError(t, err)
		assert.NotNil(t, kcpRestConfig)
		
		// Test building cluster configs
		clusterConfigs, err := buildClusterConfigs(opts)
		assert.NoError(t, err)
		assert.NotNil(t, clusterConfigs)
		assert.Len(t, clusterConfigs, 1)
		
		// Verify that configs can be used together (integration test)
		assert.Equal(t, kcpRestConfig.Host, clusterConfigs["test-cluster"].Host)
		assert.NotEqual(t, kcpRestConfig.UserAgent, clusterConfigs["test-cluster"].UserAgent)
		
		// Verify different QPS settings
		assert.Equal(t, float32(50.0), kcpRestConfig.QPS)
		assert.Equal(t, float32(30.0), clusterConfigs["test-cluster"].QPS)
	})
}