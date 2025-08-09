<!--

Thanks for creating a pull request!
If this is your first time, please make sure to review CONTRIBUTING.MD.

-->

## Summary

This PR introduces the foundational API types for TMC's HorizontalPodAutoscalerPolicy, providing comprehensive auto-scaling capabilities across multiple clusters. The implementation includes:

- **Core HPA API Type**: Complete `HorizontalPodAutoscalerPolicy` with cross-cluster awareness
- **Multiple Scaling Strategies**: Support for Distributed, Centralized, and Hybrid scaling approaches
- **Rich Metric Support**: Comprehensive metric specifications (Resource, Pods, Object, External, ContainerResource)
- **Advanced Scaling Behaviors**: Configurable scale-up/down policies and stabilization windows
- **Cluster-Aware Design**: Integration with TMC's placement engine and workspace isolation
- **Full CRD Generation**: Complete kubebuilder annotations for automated CRD generation

The API design follows KCP patterns and provides the foundation for TMC's auto-scaling controller implementation.

## What Type of PR Is This?

/kind feature
/kind api-change

## Related Issue(s)

Part of TMC Reimplementation Plan 2 - Phase 5G (Auto-scaling)

## Release Notes

```
Add foundational HorizontalPodAutoscalerPolicy API for TMC auto-scaling across multiple clusters. Supports distributed, centralized, and hybrid scaling strategies with comprehensive metrics and placement integration.
```

## Architecture & Design

- **API Package Structure**: Following KCP conventions with proper registration and documentation
- **Cross-Cluster Object References**: Custom types for referencing workloads across clusters
- **Metric Framework**: Extensible metric system supporting all Kubernetes HPA metric types
- **Scaling Policies**: Fine-grained control over scaling behavior and cluster preferences
- **Condition Framework**: Standard Kubernetes conditions for observability

## Testing

- ✅ Basic API type validation tests
- ✅ Constant value verification tests  
- ✅ Metric specification structure tests
- ✅ Scaling policy enum validation

## Dependencies

None - this is a foundational API-only PR that can be merged independently.

## Size Metrics

- **Implementation Lines**: 568 lines (18% under 700-line target)
- **Test Coverage**: Basic validation tests included
- **Generated Code**: CRD generation will be handled in subsequent PRs

🤖 Generated with [Claude Code](https://claude.ai/code)