/*
Package interfaces defines the core abstractions for cross-workspace placement
in KCP. It provides pluggable interfaces for workspace discovery, policy
evaluation, and placement scheduling.

This package follows the strategy pattern to allow different implementations
of placement algorithms, policy evaluators, and workspace discovery mechanisms.
Each interface is designed to be independently implementable and testable,
enabling flexible composition of placement strategies.

The main interfaces provided are:

  - PlacementEngine: Orchestrates the entire placement decision process
  - WorkspaceDiscovery: Handles workspace traversal and cluster discovery
  - PolicyEvaluator: Evaluates placement policies and constraints
  - Scheduler: Implements scheduling algorithms for cluster selection

Common types like PlacementDecision, ClusterTarget, and PlacementPolicy
provide the data structures used throughout the placement system.
*/
package interfaces