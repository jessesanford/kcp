/*
Copyright 2025 The KCP Authors.

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

// Package workqueue provides TMC-specific workqueue utilities with enhanced
// retry logic, rate limiting, and workspace-aware processing for KCP environments.
//
// This package implements workqueue enhancements that:
// - Provide intelligent retry strategies for TMC operations
// - Support workspace-aware rate limiting and prioritization
// - Implement circuit breakers for failing operations
// - Offer metrics and observability for queue performance
// - Support batch processing and priority handling
// - Integrate with TMC event system for comprehensive tracking
package workqueue