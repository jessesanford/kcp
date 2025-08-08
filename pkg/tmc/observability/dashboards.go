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

package observability

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/features"
)

//go:embed dashboards/*.json dashboards/*.yml
var dashboardFiles embed.FS

// DashboardManager manages Grafana dashboard provisioning for TMC.
type DashboardManager struct {
	grafanaURL string
	apiKey     string
	client     *http.Client
	enabled    bool
}

// Dashboard represents a Grafana dashboard configuration.
type Dashboard struct {
	Name        string          `json:"name"`
	Content     json.RawMessage `json:"content"`
	Type        string          `json:"type"`
	Description string          `json:"description"`
}

// NewDashboardManager creates a new dashboard manager for TMC observability.
func NewDashboardManager(grafanaURL, apiKey string) *DashboardManager {
	return &DashboardManager{
		grafanaURL: grafanaURL,
		apiKey:     apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		enabled: features.DefaultFeatureGate.Enabled(TMCDashboards),
	}
}

// LoadEmbeddedDashboards loads all embedded dashboard configurations.
func (dm *DashboardManager) LoadEmbeddedDashboards() ([]Dashboard, error) {
	if !dm.enabled {
		klog.V(2).InfoS("TMC dashboards feature is disabled")
		return nil, nil
	}

	entries, err := dashboardFiles.ReadDir("dashboards")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded dashboards directory: %w", err)
	}

	var dashboards []Dashboard
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		content, err := dashboardFiles.ReadFile("dashboards/" + entry.Name())
		if err != nil {
			continue
		}

		dashboard := Dashboard{
			Name:        strings.TrimSuffix(entry.Name(), ".json"),
			Content:     json.RawMessage(content),
			Type:        "grafana",
			Description: fmt.Sprintf("TMC %s dashboard", strings.TrimSuffix(entry.Name(), ".json")),
		}

		dashboards = append(dashboards, dashboard)
	}

	return dashboards, nil
}

// ProvisionDashboards provisions all TMC dashboards to Grafana.
func (dm *DashboardManager) ProvisionDashboards(ctx context.Context) error {
	if !dm.enabled {
		klog.InfoS("TMC dashboards feature is disabled")
		return nil
	}

	dashboards, err := dm.LoadEmbeddedDashboards()
	if err != nil {
		return err
	}

	for _, dashboard := range dashboards {
		if err := dm.provisionSingleDashboard(ctx, dashboard); err != nil {
			klog.ErrorS(err, "Failed to provision dashboard", "dashboard", dashboard.Name)
			continue
		}
	}

	return nil
}

// provisionSingleDashboard provisions a single dashboard to Grafana.
func (dm *DashboardManager) provisionSingleDashboard(ctx context.Context, dashboard Dashboard) error {
	url := fmt.Sprintf("%s/api/dashboards/db", dm.grafanaURL)
	
	payload := map[string]interface{}{
		"dashboard": json.RawMessage(dashboard.Content),
		"overwrite": true,
		"message":   fmt.Sprintf("Provisioned TMC %s dashboard", dashboard.Name),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", dm.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := dm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana API returned status %d", resp.StatusCode)
	}

	return nil
}

// IsEnabled returns whether the dashboard manager is enabled.
func (dm *DashboardManager) IsEnabled() bool {
	return dm.enabled
}