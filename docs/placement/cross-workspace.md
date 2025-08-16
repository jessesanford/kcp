# Cross-Workspace Placement Guide

## Overview

Cross-workspace placement enables deploying and managing workloads across multiple KCP workspaces using intelligent placement policies, canary deployments, and automated rollback mechanisms. This feature provides a comprehensive solution for multi-workspace application deployment with advanced traffic management and policy enforcement.

## Architecture

The cross-workspace placement system consists of several key components:

- **Placement Controller**: Manages workload placement across workspaces
- **Policy Engine**: Evaluates CEL-based placement policies
- **Workspace Discovery**: Automatically discovers available target workspaces  
- **Canary Controller**: Handles progressive deployment and traffic splitting
- **Rollback Engine**: Provides automatic and manual rollback capabilities

## Core Concepts

### Placement Policies

Placement policies define where and how workloads should be deployed across workspaces. They support CEL expressions for complex decision making:

```yaml
apiVersion: placement.tmc.io/v1alpha1
kind: PlacementPolicy
metadata:
  name: multi-region-policy
spec:
  placement:
    strategy: "spread"
    targets:
      - workspace: "production-us-west"
        weight: 40
      - workspace: "production-us-east"  
        weight: 60
  constraints:
    - expression: 'request.object.metadata.labels["tier"] == "premium"'
      message: "Only premium tier workloads allowed"
    - expression: 'size(request.object.spec.template.spec.containers) <= 5'
      message: "Maximum 5 containers per workload"
```

### Workspace Discovery

The system automatically discovers available target workspaces based on:

- Workspace labels and annotations
- Resource availability
- Access permissions
- Health status

### Canary Deployments

Progressive deployment across workspaces with traffic splitting:

```yaml
apiVersion: deployment.tmc.io/v1alpha1
kind: CanaryDeployment
metadata:
  name: web-service-canary
spec:
  sourceWorkspace: "staging"
  targetWorkspaces:
    - name: "production-us-west"
      traffic: 20
    - name: "production-us-east"
      traffic: 20
  strategy:
    increments: [10, 25, 50, 100]
    intervalDuration: "5m"
    rollbackOnFailure: true
  healthCheck:
    maxErrorRate: 5.0
    maxResponseTime: "500ms"
    minSuccessRate: 95.0
```

## Getting Started

### Prerequisites

- KCP cluster with TMC feature flags enabled
- Appropriate RBAC permissions for cross-workspace access
- Target workspaces configured and accessible

### Basic Usage

1. **Create a Placement Policy**

```bash
kubectl apply -f - <<EOF
apiVersion: placement.tmc.io/v1alpha1
kind: PlacementPolicy
metadata:
  name: simple-placement
spec:
  placement:
    strategy: "round-robin"
    targets:
      - workspace: "dev-workspace"
      - workspace: "staging-workspace"
EOF
```

2. **Deploy a Workload with Placement**

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-app
  annotations:
    placement.tmc.io/policy: "simple-placement"
spec:
  replicas: 3
  selector:
    matchLabels:
      app: sample-app
  template:
    metadata:
      labels:
        app: sample-app
    spec:
      containers:
      - name: app
        image: nginx:1.21
        ports:
        - containerPort: 80
EOF
```

3. **Monitor Placement Status**

```bash
kubectl get placementpolicy simple-placement -o yaml
kubectl get deployment sample-app -o jsonpath='{.status.placement}'
```

## Advanced Features

### Policy Precedence

When multiple policies apply to a workload, they are evaluated in priority order:

- Workspace-level policies (priority 300)
- Organization-level policies (priority 200)  
- Global policies (priority 100)

### Constraint Enforcement

Policies can enforce various constraints:

- **Required Labels**: Ensure specific labels are present
- **Resource Limits**: Enforce CPU/memory quotas
- **Security Policies**: Apply security contexts and policies
- **Network Policies**: Control network access between workspaces

### Canary Strategies

Multiple canary deployment strategies are supported:

- **Linear**: Equal increments (10% → 20% → 30% ...)
- **Exponential**: Exponential growth (5% → 10% → 20% → 40% ...)
- **Custom**: User-defined increment schedule

### Rollback Mechanisms

Automatic rollback triggers:

- Error rate exceeds threshold
- Response time degrades beyond limits
- Health check failures
- Custom metric violations

Manual rollback options:

```bash
kubectl patch canarydeployment web-service-canary --type='merge' -p='{"spec":{"rollback":{"trigger":"manual","reason":"operator decision"}}}'
```

## Troubleshooting

### Common Issues

**Placement Policy Not Applied**

Check policy validation and workspace permissions:

```bash
kubectl describe placementpolicy my-policy
kubectl auth can-i create deployments --as=system:serviceaccount:kcp-system:placement-controller -n target-workspace
```

**Canary Deployment Stuck**

Verify traffic splitting configuration and health checks:

```bash
kubectl get canarydeployment my-canary -o yaml
kubectl logs deployment/canary-controller -n kcp-system
```

**Cross-Workspace Access Denied**

Ensure proper RBAC configuration:

```bash
kubectl get clusterrolebinding | grep placement
kubectl describe workspace target-workspace
```

### Performance Tuning

For large-scale deployments:

- Use placement policy caching
- Configure appropriate controller parallelism
- Monitor workspace discovery performance
- Optimize policy evaluation expressions

## Security Considerations

- Cross-workspace placement respects RBAC boundaries
- Policies cannot override workspace-level security constraints
- All placement decisions are audited and logged
- Sensitive data is encrypted in transit between workspaces

## Best Practices

1. **Start Simple**: Begin with basic placement policies and gradually add complexity
2. **Test Policies**: Validate policies in non-production environments first
3. **Monitor Metrics**: Set up monitoring for placement operations and canary deployments
4. **Plan Rollbacks**: Always have rollback procedures documented
5. **Resource Planning**: Consider resource usage across target workspaces
6. **Security First**: Apply appropriate security policies at the workspace level

For more examples and advanced configurations, see the [examples directory](../../examples/placement/).