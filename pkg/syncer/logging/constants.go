package logging

const (
	SyncTargetKeyPrefix = "syncTarget."
	SyncTargetWorkspace = SyncTargetKeyPrefix + "workspace"
	SyncTargetNamespace = SyncTargetKeyPrefix + "namespace"
	SyncTargetName      = SyncTargetKeyPrefix + "name"
	SyncTargetKey       = SyncTargetKeyPrefix + "key"

	DownstreamKeyPrefix = "downstream."
	DownstreamNamespace = DownstreamKeyPrefix + "namespace"
	DownstreamName      = DownstreamKeyPrefix + "name"
)
