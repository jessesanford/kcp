# Syncer Architecture Overview

This document describes the legacy syncer found on branch `main-pre-tmc-removal` and the high level design for a more modular implementation.

## Legacy Layout

The legacy syncer implementation lives under `cmd/syncer` and `pkg/syncer`. It contains a variety of controllers and helper packages:

- **apiimporter.go** – imports API resources into a workload cluster.
- **namespace**, **status**, **endpoints** – controllers for specific resource types.
- **synctarget** – utilities for referencing `SyncTarget` objects.
- **tunneler** – support for network tunnels used by the syncer.

`StartSyncer` in `pkg/syncer/syncer.go` orchestrates initialization of informers, controllers and the optional DNS processor.

## Modular Design

New interfaces in `pkg/syncer/module` define pluggable components:

- `ResourceSyncer` – starts resource synchronization.
- `ConflictResolver` – resolves conflicts during sync.
- `ErrorHandler` – handles generic errors.
- `MetricsCollector` – collects metrics for monitoring.
- `Logger` – abstracts logging.

`Modules` aggregates implementations and provides defaults. `StartSyncer` now delegates to `startSyncerWithModules`, allowing callers to provide alternative implementations.

This approach enables future extensions without changing the core syncer logic.
