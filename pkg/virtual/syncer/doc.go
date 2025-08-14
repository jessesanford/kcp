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

// Package syncer provides a virtual workspace that implements the API endpoint
// for syncers to connect to KCP. It handles authentication, authorization, and
// bidirectional transformation of resources between KCP and syncer formats.
//
// The virtual workspace provides:
//   - Certificate-based authentication for syncers
//   - Workspace isolation and multi-tenancy
//   - Resource transformation and filtering
//   - API discovery with permission-based filtering
//   - High-performance resource streaming for syncing
//
// This virtual workspace is accessed by syncers at paths like:
//   /services/syncer/<syncer-id>/clusters/<workspace>/...
package syncer