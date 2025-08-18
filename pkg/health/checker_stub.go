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

package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BaseHealthChecker provides a basic implementation of HealthChecker
// that can be embedded by specific health checkers.
// NOTE: This is a minimal stub implementation for monitors package compatibility.
// The full implementation is provided in the core health package.
type BaseHealthChecker struct {
	name      string
	checkFunc func(ctx context.Context) HealthStatus
	lastCheck time.Time
	mutex     sync.RWMutex
}

// NewBaseHealthChecker creates a new base health checker with the given name and check function.
func NewBaseHealthChecker(name string, checkFunc func(ctx context.Context) HealthStatus) *BaseHealthChecker {
	return &BaseHealthChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Name returns the name of the health checker.
func (b *BaseHealthChecker) Name() string {
	return b.name
}

// Check performs the health check and updates the last check timestamp.
func (b *BaseHealthChecker) Check(ctx context.Context) HealthStatus {
	defer func() {
		b.mutex.Lock()
		b.lastCheck = time.Now()
		b.mutex.Unlock()
	}()
	
	if b.checkFunc == nil {
		return HealthStatus{
			Healthy:   false,
			Message:   fmt.Sprintf("No health check function defined for %s", b.name),
			Timestamp: time.Now(),
		}
	}
	
	return b.checkFunc(ctx)
}

// LastCheck returns the timestamp of the last health check.
func (b *BaseHealthChecker) LastCheck() time.Time {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.lastCheck
}