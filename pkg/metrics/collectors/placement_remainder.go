	c.policyEvalTime.WithLabelValues(workspace, policyType).Observe(duration.Seconds())
}

// RecordPlacementConflict records a placement conflict.
func (c *PlacementCollector) RecordPlacementConflict(workspace, resource, conflictType string) {
	if !c.enabled {
		return
	}

	c.placementConflicts.WithLabelValues(workspace, resource, conflictType).Inc()
}

// RecordPlacementUpdate records a placement update.
func (c *PlacementCollector) RecordPlacementUpdate(workspace, cluster, operation string) {
	if !c.enabled {
		return
	}

	c.placementUpdates.WithLabelValues(workspace, cluster, operation).Inc()
}

// SetActiveWorkloads sets the current number of active workloads.
func (c *PlacementCollector) SetActiveWorkloads(workspace, cluster, resource string, count float64) {
	if !c.enabled {
		return
	}

	c.activeWorkloads.WithLabelValues(workspace, cluster, resource).Set(count)
}

// RecordResourceRequest records resource requirements for a placement.
func (c *PlacementCollector) RecordResourceRequest(workspace, cluster, resourceType string, amount float64) {
	if !c.enabled {
		return
	}

	c.resourceRequests.WithLabelValues(workspace, cluster, resourceType).Observe(amount)
}

// GetPlacementCollector returns a shared instance of the placement collector.
var (
	placementCollectorInstance *PlacementCollector
	placementCollectorOnce     sync.Once
)

// GetPlacementCollector returns the global placement collector instance.
func GetPlacementCollector() *PlacementCollector {
	placementCollectorOnce.Do(func() {
		placementCollectorInstance = NewPlacementCollector()
		// Register with global registry
		if err := metrics.GetRegistry().RegisterCollector(placementCollectorInstance); err != nil {
			klog.Errorf("Failed to register placement collector: %v", err)
		}
	})
	return placementCollectorInstance
}