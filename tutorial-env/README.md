# TMC Hello World Tutorial Environment

This directory contains a complete TMC (Transparent Multi-Cluster) tutorial environment.

## Quick Start

1. **Run the demo**: `./scripts/tmc-demo.sh`
2. **Validate setup**: `./scripts/validate-tmc.sh`
3. **Full setup**: `../scripts/setup-tmc-tutorial.sh` (requires kind/Docker)

## What's Included

### Example Applications
- `examples/hello-world.yaml` - Multi-cluster Hello World application
- `examples/placement.yaml` - Cross-cluster placement configuration
- `examples/tmc-config.yaml` - TMC system configuration

### Demo Scripts
- `scripts/tmc-demo.sh` - Interactive TMC features demonstration
- `scripts/validate-tmc.sh` - Validation and testing script

### Configuration
- `cluster-config.yaml` - Mock multi-cluster configuration

## TMC Features Demonstrated

✅ **Multi-Cluster Workload Placement**
- Intelligent workload distribution across clusters
- Placement policies and constraints
- Cluster selection strategies

✅ **Cross-Cluster Resource Aggregation**
- Unified views of distributed resources
- Health aggregation across clusters
- Resource status consolidation

✅ **Virtual Workspace Management**
- Cross-cluster resource projections
- Resource transformations
- Namespace-level abstractions

✅ **Automated Health Monitoring**
- Component health tracking
- Cluster connectivity monitoring
- Health status aggregation

✅ **Intelligent Recovery Strategies**
- Automated error detection and recovery
- Multiple recovery strategies per error type
- Recovery execution tracking

✅ **Comprehensive Observability**
- Prometheus metrics integration
- Structured logging
- Performance monitoring

## Documentation

For detailed information about TMC components:

- [TMC Error Handling](../docs/content/developers/tmc/error-handling.md)
- [TMC Health Monitoring](../docs/content/developers/tmc/health-monitoring.md)
- [TMC Metrics & Observability](../docs/content/developers/tmc/metrics-observability.md)
- [TMC Recovery Manager](../docs/content/developers/tmc/recovery-manager.md)
- [TMC Virtual Workspace Manager](../docs/content/developers/tmc/virtual-workspace-manager.md)

## Tutorial Flow

1. **Setup** - Environment preparation and cluster creation
2. **Deploy** - Application deployment across clusters
3. **Observe** - TMC features in action
4. **Test** - Recovery and scaling scenarios
5. **Monitor** - Health and metrics observation

## Next Steps

After completing this tutorial:

1. Explore the TMC source code in `pkg/reconciler/workload/tmc/`
2. Read the architectural documentation
3. Try modifying the placement policies
4. Experiment with different recovery scenarios
5. Create your own multi-cluster applications

## Support

For questions or issues:
- Check the troubleshooting section in the main tutorial
- Review the TMC component documentation
- Examine the example configurations

Happy clustering! 🚀
