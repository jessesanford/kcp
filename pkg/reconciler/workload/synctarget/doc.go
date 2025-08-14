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

// Package synctarget provides a controller for managing SyncTarget resources
// that represent physical clusters where workloads can be synchronized.
//
// The SyncTarget controller is responsible for:
// - Managing the lifecycle of SyncTarget resources
// - Deploying and configuring syncer components to physical clusters
// - Updating SyncTarget status based on cluster connectivity and health
// - Handling authentication and authorization for syncer deployments
// - Monitoring resource capacity and allocation on target clusters
//
// This controller integrates with the TMC (Transparent Multi-Cluster) system
// and follows KCP's workspace isolation and multi-tenancy patterns.
package synctarget