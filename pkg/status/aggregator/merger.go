/*
Copyright The KCP Authors.

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

// Package aggregator implements status field-level merging strategies.
package aggregator

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/status/interfaces"
)

// Merger implements the StatusMerger interface for field-level status merging
type Merger struct {
	// mu protects access to custom mergers
	mu sync.RWMutex

	// customMergers maps field paths to custom merge functions
	customMergers map[string]interfaces.FieldMerger
}

// NewMerger creates a new status field merger
func NewMerger() *Merger {
	return &Merger{
		customMergers: make(map[string]interfaces.FieldMerger),
	}
}

// MergeFields merges status fields from multiple sources
func (m *Merger) MergeFields(ctx context.Context, statuses []*interfaces.StatusUpdate, config interfaces.MergeConfig) (*unstructured.Unstructured, error) {
	if len(statuses) == 0 {
		return nil, fmt.Errorf("no statuses to merge")
	}

	if len(statuses) == 1 {
		return statuses[0].Status.DeepCopy(), nil
	}

	// Use first status as base
	result := statuses[0].Status.DeepCopy()

	// Merge each subsequent status
	for i := 1; i < len(statuses); i++ {
		if err := m.mergeInto(ctx, result, statuses[i], config); err != nil {
			return nil, fmt.Errorf("failed to merge status from %s: %w", statuses[i].Source, err)
		}
	}

	return result, nil
}

// RegisterFieldMerger registers a custom merger for specific fields
func (m *Merger) RegisterFieldMerger(fieldPath string, merger interfaces.FieldMerger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customMergers[fieldPath] = merger
}

// UnregisterFieldMerger removes a custom field merger
func (m *Merger) UnregisterFieldMerger(fieldPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.customMergers, fieldPath)
}

// mergeInto merges source status into target
func (m *Merger) mergeInto(ctx context.Context, target *unstructured.Unstructured, source *interfaces.StatusUpdate, config interfaces.MergeConfig) error {
	if source.Status == nil {
		return nil
	}

	// Recursively merge status objects
	return m.mergeObjects(ctx, target.Object, source.Status.Object, "status", config)
}

// mergeObjects recursively merges object fields
func (m *Merger) mergeObjects(ctx context.Context, target, source map[string]interface{}, path string, config interfaces.MergeConfig) error {
	for key, sourceValue := range source {
		fieldPath := path + "." + key
		targetValue, exists := target[key]

		if !exists {
			// Field doesn't exist in target, add it
			target[key] = sourceValue
			continue
		}

		// Check for custom merger first
		if merger := m.getCustomMerger(fieldPath); merger != nil {
			merged, err := merger.MergeField(ctx, []interface{}{targetValue, sourceValue})
			if err != nil {
				return fmt.Errorf("custom merger failed for %s: %w", fieldPath, err)
			}
			target[key] = merged
			continue
		}

		// Get merge strategy for this field
		strategy := config.DefaultStrategy
		if fieldStrategy, exists := config.FieldStrategies[fieldPath]; exists {
			strategy = fieldStrategy
		}

		// Apply merge strategy
		merged, err := m.mergeValues(targetValue, sourceValue, strategy)
		if err != nil {
			return m.handleMergeError(fieldPath, err, config.ConflictBehavior)
		}

		target[key] = merged
	}

	return nil
}

// mergeValues merges two values using the specified strategy
func (m *Merger) mergeValues(target, source interface{}, strategy interfaces.FieldMergeStrategy) (interface{}, error) {
	switch strategy {
	case interfaces.FieldMergeLatest:
		return source, nil

	case interfaces.FieldMergeConcat:
		return m.mergeConcat(target, source)

	case interfaces.FieldMergeSum:
		return m.mergeSum(target, source)

	case interfaces.FieldMergeMax:
		return m.mergeMax(target, source)

	case interfaces.FieldMergeMin:
		return m.mergeMin(target, source)

	case interfaces.FieldMergeArray:
		return m.mergeArray(target, source)

	default:
		return nil, fmt.Errorf("unknown merge strategy: %s", strategy)
	}
}

// mergeConcat concatenates string values
func (m *Merger) mergeConcat(target, source interface{}) (interface{}, error) {
	targetStr := fmt.Sprintf("%v", target)
	sourceStr := fmt.Sprintf("%v", source)
	return targetStr + "," + sourceStr, nil
}

// mergeSum sums numeric values
func (m *Merger) mergeSum(target, source interface{}) (interface{}, error) {
	targetFloat, err := toFloat64(target)
	if err != nil {
		return nil, err
	}
	sourceFloat, err := toFloat64(source)
	if err != nil {
		return nil, err
	}
	return targetFloat + sourceFloat, nil
}

// mergeMax returns the maximum value
func (m *Merger) mergeMax(target, source interface{}) (interface{}, error) {
	targetFloat, err := toFloat64(target)
	if err != nil {
		return nil, err
	}
	sourceFloat, err := toFloat64(source)
	if err != nil {
		return nil, err
	}
	if targetFloat > sourceFloat {
		return target, nil
	}
	return source, nil
}

// mergeMin returns the minimum value
func (m *Merger) mergeMin(target, source interface{}) (interface{}, error) {
	targetFloat, err := toFloat64(target)
	if err != nil {
		return nil, err
	}
	sourceFloat, err := toFloat64(source)
	if err != nil {
		return nil, err
	}
	if targetFloat < sourceFloat {
		return target, nil
	}
	return source, nil
}

// mergeArray merges array values
func (m *Merger) mergeArray(target, source interface{}) (interface{}, error) {
	targetSlice, ok := target.([]interface{})
	if !ok {
		return nil, fmt.Errorf("target is not an array")
	}
	sourceSlice, ok := source.([]interface{})
	if !ok {
		return nil, fmt.Errorf("source is not an array")
	}

	// Simple concatenation - could be enhanced with deduplication
	result := make([]interface{}, len(targetSlice)+len(sourceSlice))
	copy(result, targetSlice)
	copy(result[len(targetSlice):], sourceSlice)

	return result, nil
}

// getCustomMerger returns custom merger for field path
func (m *Merger) getCustomMerger(fieldPath string) interfaces.FieldMerger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check exact match first
	if merger, exists := m.customMergers[fieldPath]; exists {
		return merger
	}

	// Check for prefix matches (for nested fields)
	for path, merger := range m.customMergers {
		if strings.HasPrefix(fieldPath, path+".") {
			return merger
		}
	}

	return nil
}

// handleMergeError handles merge errors based on conflict behavior
func (m *Merger) handleMergeError(fieldPath string, err error, behavior interfaces.ConflictBehavior) error {
	switch behavior {
	case interfaces.ConflictBehaviorIgnore:
		return nil

	case interfaces.ConflictBehaviorError:
		return err

	case interfaces.ConflictBehaviorLog:
		klog.V(2).ErrorS(err, "Merge conflict detected", "fieldPath", fieldPath)
		return nil

	default:
		return err
	}
}

// toFloat64 converts various numeric types to float64
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		// Try using reflection for other numeric types
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return rv.Float(), nil
		default:
			return 0, fmt.Errorf("cannot convert %T to float64", v)
		}
	}
}