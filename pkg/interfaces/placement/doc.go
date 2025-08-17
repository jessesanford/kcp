/*
Package placement defines interfaces for workload placement and scheduling
in the TMC system.

The placement package provides a pluggable architecture for implementing
custom placement strategies, schedulers, and constraint evaluators.

Core Interfaces:

- PlacementEngine: Main interface for placement computation
- Scheduler: Handles workload scheduling to locations
- PlacementStrategy: Implements specific placement algorithms
- ConstraintEvaluator: Evaluates placement constraints
- Scorer: Scores placement options

Strategy Types:

The package supports multiple placement strategies:
- Spread: Distributes replicas evenly across available locations
- Binpack: Consolidates workloads to minimize the number of locations used
- HighAvailability: Ensures redundancy across failure domains
- Singleton: Places workloads on exactly one location

Scheduling Framework:

The scheduler framework provides a plugin-based architecture for:
- Filtering unsuitable targets based on constraints
- Scoring targets based on various criteria
- Binding workloads to selected targets
- Managing scheduling queues and priorities

Constraint Evaluation:

The constraint evaluation system handles:
- Resource requirements and availability
- Taint and toleration matching
- Affinity and anti-affinity rules
- Topology spread constraints
- Custom policy constraints

Scoring System:

The scoring framework supports:
- Resource utilization scoring
- Locality preference scoring
- Load balancing scoring
- Affinity compliance scoring
- Custom scoring functions

Usage Example:

	import (
	    "github.com/kcp-dev/kcp/pkg/interfaces/placement"
	    "github.com/kcp-dev/kcp/pkg/interfaces/placement/strategy"
	)
	
	// Create placement engine
	engine := placement.NewEngine()
	
	// Compute placement
	decision, err := engine.ComputePlacement(ctx, workload, policy, targets)
	if err != nil {
	    return err
	}
	
	// Validate the placement decision
	err = engine.ValidatePlacement(ctx, decision)
	if err != nil {
	    return err
	}

For more information, see the TMC placement documentation.
*/
package placement