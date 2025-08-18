# Transparent Multi-Cluster (TMC) Documentation

Welcome to the comprehensive documentation for KCP's Transparent Multi-Cluster (TMC) functionality.

## Table of Contents

### User Guide
- [Overview and Architecture](user-guide/overview.md) - High-level concepts and architecture
- [Getting Started](user-guide/getting-started.md) - Installation and initial setup
- [Basic Usage](user-guide/basic-usage.md) - Common workflows and examples

### API Reference
- [ClusterRegistration API](api-reference/cluster-registration.md) - Cluster management API
- [WorkloadPlacement API](api-reference/workload-placement.md) - Workload placement API
- [Placement Policies](api-reference/placement-policies.md) - Policy configuration
- [Resource Management](api-reference/resource-management.md) - Resource lifecycle

### Operations
- [Performance Tuning](operations/performance-tuning.md) - Optimization guidelines
- [Monitoring](operations/monitoring.md) - Observability and metrics
- [Disaster Recovery](operations/disaster-recovery.md) - Backup and recovery

### Troubleshooting
- [Common Issues](troubleshooting/common-issues.md) - Frequent problems and solutions
- [Debugging Guide](troubleshooting/debugging.md) - Advanced troubleshooting
- [FAQ](troubleshooting/faq.md) - Frequently asked questions

## What is TMC?

Transparent Multi-Cluster (TMC) is KCP's solution for managing workloads across multiple Kubernetes clusters seamlessly. It provides:

- **Transparent Scheduling**: Workloads are automatically scheduled across available clusters
- **Dynamic Discovery**: Clusters can be added and removed dynamically
- **Policy-Based Placement**: Fine-grained control over where workloads run
- **High Availability**: Automatic failover and load balancing across clusters
- **Resource Optimization**: Intelligent placement based on resource availability

## Quick Start

1. [Install KCP with TMC](user-guide/getting-started.md#installation)
2. [Register your first cluster](user-guide/getting-started.md#cluster-registration)
3. [Deploy a workload](user-guide/basic-usage.md#deploying-workloads)

## Support

For questions and support:
- File issues on the [KCP GitHub repository](https://github.com/kcp-dev/kcp)
- Join the KCP community discussions
- Refer to the [KCP documentation](../README.md) for general KCP concepts