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

// Package auth provides authentication and authorization for KCP virtual workspaces.
//
// This package implements core authentication and authorization functionality for 
// KCP's virtual workspace architecture. It provides essential building blocks 
// for securing virtual workspace access with proper workspace isolation.
//
// Key Components:
//
// Provider Interface: Defines authentication capabilities including token 
// validation and subject extraction. BasicProvider provides token-based 
// authentication with caching.
//
// Evaluator Interface: Defines authorization evaluation capabilities. 
// BasicEvaluator provides permission checking with workspace isolation 
// and decision caching.
//
// Thread Safety: All implementations are safe for concurrent use across
// multiple goroutines.
//
// Workspace Isolation: Enforces KCP's workspace isolation by default.
// Subjects are restricted to their assigned logical clusters unless they
// have elevated privileges.
//
// Usage Example:
//
//	config := auth.DefaultConfig()
//	provider := auth.NewBasicProvider(config)
//	evaluator := auth.NewBasicEvaluator(config)
//	
//	tokenInfo, err := provider.ValidateToken(ctx, bearerToken)
//	if err != nil {
//		// Handle authentication failure
//	}
//	
//	permission := auth.Permission{
//		Verb:           "get",
//		Resource:       "pods", 
//		LogicalCluster: logicalcluster.Name("root:workspace"),
//	}
//	
//	result := evaluator.Authorize(ctx, &tokenInfo.Subject, permission)
//	if !result.Allowed {
//		// Handle authorization failure
//	}
package auth