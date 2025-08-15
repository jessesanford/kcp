/*
Copyright 2023 The KCP Authors.

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

package contracts

// DiscoveryProviderName is the name used to identify discovery providers
const DiscoveryProviderName = "kcp-discovery"

// DefaultCacheTTLSeconds is the default cache TTL in seconds
const DefaultCacheTTLSeconds = int64(300) // 5 minutes

// DefaultCacheCleanupIntervalSeconds is the default cache cleanup interval
const DefaultCacheCleanupIntervalSeconds = int64(60) // 1 minute

// MaxCachedWorkspaces is the maximum number of workspaces to cache
const MaxCachedWorkspaces = 1000

// DefaultWatchChannelBuffer is the default buffer size for watch channels
const DefaultWatchChannelBuffer = 100