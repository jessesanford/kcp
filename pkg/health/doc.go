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

// Package health provides health monitoring capabilities for TMC components.
//
// This package implements a comprehensive health checking system that allows
// TMC components to report their health status, supports periodic monitoring,
// aggregation of multiple health checkers, and provides various reporting formats.
//
// The package includes:
// - Core health checking interfaces and types
// - Base implementations for common health checking patterns
// - Periodic health checking with retry logic and failure thresholds
// - System-wide health aggregation
// - Multiple monitoring strategies for different TMC components
// - Health probes for liveness and readiness checks
// - JSON and status-based reporting mechanisms
//
// Usage:
//   // Create a simple health checker
//   checker := health.NewFuncHealthChecker("my-component", func(ctx context.Context) health.HealthStatus {
//       // Perform your health check logic
//       return health.HealthStatus{
//           Healthy: true,
//           Message: "Component is operational",
//           Timestamp: time.Now(),
//       }
//   })
//
//   // Use with periodic monitoring
//   config := health.DefaultHealthConfiguration()
//   periodic := health.NewPeriodicHealthChecker(checker, config)
//   go periodic.Start(context.Background())
//
//   // Add to system-wide aggregation
//   aggregator := health.NewDefaultAggregator()
//   aggregator.AddChecker(checker)
//   systemHealth := aggregator.CheckAll(context.Background())
package health