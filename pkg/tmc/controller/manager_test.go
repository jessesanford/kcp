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

package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			KCPConfig:      &rest.Config{Host: "https://kcp.example.com"},
			Workspace:      "root:test",
			ClusterConfigs: map[string]*rest.Config{},
			ResyncPeriod:   30 * time.Second,
			WorkerCount:    5,
		}

		err := validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("missing KCPConfig", func(t *testing.T) {
		config := &Config{
			Workspace:    "root:test",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "KCPConfig is required")
	})

	t.Run("empty workspace", func(t *testing.T) {
		config := &Config{
			KCPConfig:    &rest.Config{Host: "https://kcp.example.com"},
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Workspace is required")
	})
}