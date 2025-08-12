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

package priority

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityString(t *testing.T) {
	tests := map[string]struct {
		priority Priority
		expected string
	}{
		"critical priority": {
			priority: Critical,
			expected: "Critical",
		},
		"high priority": {
			priority: High,
			expected: "High",
		},
		"normal priority": {
			priority: Normal,
			expected: "Normal",
		},
		"low priority": {
			priority: Low,
			expected: "Low",
		},
		"background priority": {
			priority: Background,
			expected: "Background",
		},
		"custom priority": {
			priority: Priority(300),
			expected: "Priority(300)",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.priority.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPriorityIsValid(t *testing.T) {
	tests := map[string]struct {
		priority Priority
		valid    bool
	}{
		"critical is valid": {
			priority: Critical,
			valid:    true,
		},
		"normal is valid": {
			priority: Normal,
			valid:    true,
		},
		"background is valid": {
			priority: Background,
			valid:    true,
		},
		"too high is invalid": {
			priority: Priority(2000),
			valid:    false,
		},
		"too low is invalid": {
			priority: Priority(50),
			valid:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.priority.IsValid()
			assert.Equal(t, tc.valid, result)
		})
	}
}

func TestNewPriorityItem(t *testing.T) {
	key := "test-key"
	priority := High

	item := NewPriorityItem(key, priority)

	assert.NotNil(t, item)
	assert.Equal(t, key, item.Key)
	assert.Equal(t, priority, item.Priority)
	assert.Equal(t, 0, item.RetryCount)
	assert.True(t, time.Since(item.AddedAt) < time.Second)
}

func TestPriorityItemAge(t *testing.T) {
	item := NewPriorityItem("test", Normal)
	
	// Age should be very small initially
	age1 := item.Age()
	assert.True(t, age1 < time.Second)
	
	// Sleep and check that age increases
	time.Sleep(10 * time.Millisecond)
	age2 := item.Age()
	assert.True(t, age2 > age1)
	assert.True(t, age2 >= 10*time.Millisecond)
}

func TestPriorityItemEffectivePriority(t *testing.T) {
	t.Run("fresh item has same priority", func(t *testing.T) {
		item := NewPriorityItem("test", Normal)
		assert.Equal(t, Normal, item.EffectivePriority())
	})

	t.Run("retried item gets priority boost", func(t *testing.T) {
		item := NewPriorityItem("test", Normal)
		item.RetryCount = 3
		expected := Normal + Priority(3*25) // 25 points per retry
		assert.Equal(t, expected, item.EffectivePriority())
	})

	t.Run("priority capped at Critical", func(t *testing.T) {
		item := NewPriorityItem("test", Normal)
		item.RetryCount = 100 // Very high retry count
		item.AddedAt = time.Now().Add(-10 * time.Minute) // Very aged
		assert.Equal(t, Critical, item.EffectivePriority(), "Priority should be capped at Critical")
	})
}

func TestDefaultPriorityConfig(t *testing.T) {
	config := DefaultPriorityConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.Equal(t, 5*time.Minute, config.MaxDelay)
	assert.Equal(t, 30*time.Second, config.PriorityBoostInterval)
	assert.Equal(t, 2*time.Minute, config.StarvationThreshold)
}


// Test priority level ordering
func TestPriorityLevels(t *testing.T) {
	assert.True(t, Critical > High)
	assert.True(t, High > Normal)
	assert.True(t, Normal > Low)
	assert.True(t, Low > Background)
}

