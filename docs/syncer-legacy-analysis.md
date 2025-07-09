# Legacy Syncer Structure

The `main-pre-tmc-removal` branch includes a full implementation of a syncer used for transparent multi-cluster scheduling. Important locations include:

- `cmd/syncer` – CLI entry point and option parsing.
- `pkg/syncer` – core library with controllers for namespace syncing, status updates and more.
- `pkg/tunneler` – helpers for establishing a reverse tunnel from the SyncTarget back to KCP.
- `test/e2e/syncer` – end-to-end tests for syncer functionality.

`StartSyncer` sets up Kubernetes informers, waits for a `SyncTarget` to appear, then starts multiple controllers such as API importers and status synchronizers.
