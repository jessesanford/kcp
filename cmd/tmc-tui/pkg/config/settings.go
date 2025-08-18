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

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"k8s.io/klog/v2"
)

// Config holds all TUI configuration settings.
type Config struct {
	// RefreshRate controls how often data is refreshed from metrics endpoints.
	RefreshRate time.Duration `json:"refresh_rate"`

	// MetricsEndpoint is the URL to the TMC metrics endpoint.
	MetricsEndpoint string `json:"metrics_endpoint"`

	// Theme controls the color scheme and appearance.
	Theme ThemeConfig `json:"theme"`

	// Features controls which TUI features are enabled.
	Features FeatureConfig `json:"features"`

	// Connection settings for accessing TMC components.
	Connection ConnectionConfig `json:"connection"`
}

// ThemeConfig controls the visual appearance of the TUI.
type ThemeConfig struct {
	// ColorScheme can be "dark", "light", or "auto".
	ColorScheme string `json:"color_scheme"`

	// ShowBorders controls whether UI elements have borders.
	ShowBorders bool `json:"show_borders"`

	// CompactMode reduces spacing for smaller terminals.
	CompactMode bool `json:"compact_mode"`
}

// FeatureConfig controls which dashboard features are enabled.
type FeatureConfig struct {
	// ShowClusterView enables the cluster status view.
	ShowClusterView bool `json:"show_cluster_view"`

	// ShowSyncerView enables the syncer status view.
	ShowSyncerView bool `json:"show_syncer_view"`

	// ShowMetricsView enables the detailed metrics view.
	ShowMetricsView bool `json:"show_metrics_view"`

	// EnableKeyboardShortcuts shows keyboard shortcuts in footer.
	EnableKeyboardShortcuts bool `json:"enable_keyboard_shortcuts"`

	// AutoRefresh enables automatic data refresh.
	AutoRefresh bool `json:"auto_refresh"`
}

// ConnectionConfig holds connection settings for TMC components.
type ConnectionConfig struct {
	// KubeconfigPath specifies the path to kubeconfig file.
	KubeconfigPath string `json:"kubeconfig_path"`

	// Timeout for API calls.
	Timeout time.Duration `json:"timeout"`

	// RetryAttempts for failed connections.
	RetryAttempts int `json:"retry_attempts"`

	// InsecureSkipTLSVerify skips TLS verification (for development only).
	InsecureSkipTLSVerify bool `json:"insecure_skip_tls_verify"`
}

// DefaultConfig returns a default configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		RefreshRate:     5 * time.Second,
		MetricsEndpoint: "http://localhost:8080/metrics",
		Theme: ThemeConfig{
			ColorScheme: "dark",
			ShowBorders: true,
			CompactMode: false,
		},
		Features: FeatureConfig{
			ShowClusterView:         true,
			ShowSyncerView:          true,
			ShowMetricsView:         true,
			EnableKeyboardShortcuts: true,
			AutoRefresh:             true,
		},
		Connection: ConnectionConfig{
			KubeconfigPath:        "", // Will use default kubeconfig discovery
			Timeout:               30 * time.Second,
			RetryAttempts:         3,
			InsecureSkipTLSVerify: false,
		},
	}
}

// LoadConfig loads configuration from the specified file path.
// If path is empty or file doesn't exist, returns default configuration.
func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		klog.V(2).Info("No config file specified, using defaults")
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(2).Infof("Config file %s does not exist, using defaults", configPath)
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	klog.V(2).Infof("Loaded config from %s", configPath)
	return cfg, nil
}

// SaveConfig saves the configuration to the specified file path.
func (c *Config) SaveConfig(configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Validate checks if the configuration is valid and sets defaults for invalid values.
func (c *Config) Validate() {
	// Validate refresh rate
	if c.RefreshRate < time.Second {
		klog.Warningf("Refresh rate %v too low, setting to 1s", c.RefreshRate)
		c.RefreshRate = time.Second
	}
	if c.RefreshRate > 60*time.Second {
		klog.Warningf("Refresh rate %v too high, setting to 60s", c.RefreshRate)
		c.RefreshRate = 60 * time.Second
	}

	// Validate theme color scheme
	validSchemes := map[string]bool{"dark": true, "light": true, "auto": true}
	if !validSchemes[c.Theme.ColorScheme] {
		klog.Warningf("Invalid color scheme %s, setting to dark", c.Theme.ColorScheme)
		c.Theme.ColorScheme = "dark"
	}

	// Validate connection timeout
	if c.Connection.Timeout < time.Second {
		klog.Warningf("Connection timeout %v too low, setting to 10s", c.Connection.Timeout)
		c.Connection.Timeout = 10 * time.Second
	}

	// Validate retry attempts
	if c.Connection.RetryAttempts < 0 {
		c.Connection.RetryAttempts = 0
	}
	if c.Connection.RetryAttempts > 10 {
		klog.Warningf("Retry attempts %d too high, setting to 10", c.Connection.RetryAttempts)
		c.Connection.RetryAttempts = 10
	}

	klog.V(3).Info("Configuration validated successfully")
}