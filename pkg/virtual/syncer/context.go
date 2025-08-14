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

package syncer

import (
	"context"
)

type syncerIdentityKey struct{}

// SyncerIdentity holds information about the syncer making a request.
type SyncerIdentity struct {
	SyncerID  string
	Workspace string
}

// withSyncerIdentity adds syncer identity information to the request context.
func withSyncerIdentity(ctx context.Context, syncerID, workspace string) context.Context {
	identity := &SyncerIdentity{
		SyncerID:  syncerID,
		Workspace: workspace,
	}
	return context.WithValue(ctx, syncerIdentityKey{}, identity)
}

// extractSyncerIdentity retrieves syncer identity from the request context.
func extractSyncerIdentity(ctx context.Context) (syncerID, workspace string, ok bool) {
	identity, ok := ctx.Value(syncerIdentityKey{}).(*SyncerIdentity)
	if !ok || identity == nil {
		return "", "", false
	}
	return identity.SyncerID, identity.Workspace, true
}