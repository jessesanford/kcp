# CRD Installation Improvements for TMC Virtual Cluster Demo

## Issue
The original script was failing to install CRDs properly, causing errors like:
```
failed to retrieve CRD pods: customresourcedefinition.apiextensions.k8s.io "pods" not found
failed to retrieve CRD services: customresourcedefinition.apiextensions.k8s.io "services" not found
```

## Root Cause
1. **Incorrect CRD names**: Script was waiting for `crd/pods.core` but using wrong wait conditions
2. **Incomplete CRD installation**: Only installing a few CRDs instead of all available ones
3. **Insufficient verification**: No proper checking that CRDs were established and functional
4. **Poor error handling**: Limited troubleshooting information when failures occurred

## Solutions Implemented

### 1. Enhanced CRD Installation (`install_core_crds` function)
- **Install ALL available CRDs**: Now installs all CRDs from both `contrib/crds/core/` and `contrib/crds/apps/`
- **Correct CRD naming**: Uses proper CRD names like `pods.core`, `services.core`, `deployments.apps`
- **Better error handling**: Individual file error handling with warnings instead of failing completely
- **Extended timeout**: Increased from 30 to 60 seconds for CRD establishment
- **Improved progress indication**: Better feedback during installation process

### 2. CRD Verification (`verify_crds_functional` function)
- **Comprehensive testing**: Tests both CRD existence and API endpoint functionality
- **Multi-stage verification**: 
  1. Check CRD exists with `kubectl get crd`
  2. Check CRD is established with `kubectl wait`
  3. Test API endpoints with `kubectl get` commands
- **Clear error reporting**: Specific error messages for different failure modes

### 3. Troubleshooting Support (`troubleshoot_crd_issues` function)
- **File availability check**: Lists available CRD files to verify they exist
- **Current CRD status**: Shows what CRDs are actually installed
- **API server health**: Tests if KCP API server is responding
- **Context information**: Shows current kubeconfig and workspace context
- **Log analysis**: Displays recent error messages from KCP logs

### 4. Success Reporting (`list_installed_crds` function)
- **Installation verification**: Lists all successfully installed CRDs
- **Visual confirmation**: Provides clear ✓ indicators for successfully installed CRDs

## Key Changes in Script Flow

### Before:
```bash
# Apply core CRDs
kubectl apply -f contrib/crds/core/_pods.yaml
kubectl apply -f contrib/crds/core/_services.yaml 
kubectl apply -f contrib/crds/core/_namespaces.yaml
kubectl apply -f contrib/crds/apps/apps_deployments.yaml

# Wait with incorrect names and limited timeout
kubectl wait --for condition=established --timeout=5s crd/pods.core
```

### After:
```bash
# Install ALL available CRDs with proper error handling
for core_crd in contrib/crds/core/_*.yaml; do
    kubectl apply -f "$core_crd" || echo "Warning: Failed to apply..."
done

for apps_crd in contrib/crds/apps/apps_*.yaml; do
    kubectl apply -f "$apps_crd" || echo "Warning: Failed to apply..."
done

# Comprehensive verification with proper CRD names
verify_crds_functional "workspace-name"
# Includes troubleshooting on failure
```

## Expected CRDs Installed

### Core Resources (`contrib/crds/core/`):
- `endpoints.core`
- `namespaces.core` 
- `nodes.core`
- `persistentvolumeclaims.core`
- `persistentvolumes.core`
- `pods.core`
- `podtemplates.core`
- `replicationcontrollers.core`
- `services.core`

### Apps Resources (`contrib/crds/apps/`):
- `controllerrevisions.apps`
- `daemonsets.apps`
- `deployments.apps`
- `replicasets.apps`
- `statefulsets.apps`

### KCP Resources (`config/crds/`):
- `synctargets.workload.kcp.io`
- `clusterworkloadplacements.workload.kcp.io`
- Various KCP-specific CRDs

## Benefits
1. **More reliable CRD installation**: Handles all available CRDs instead of hardcoded subset
2. **Better error diagnosis**: Clear troubleshooting information when issues occur
3. **Improved user experience**: Better progress feedback and success confirmation
4. **Future-proof**: Automatically includes new CRDs added to the directories
5. **Robust verification**: Multi-level checks ensure CRDs are truly functional

## Testing
The script now:
- Validates syntax with `bash -n`
- Installs all available CRDs automatically
- Provides comprehensive error information if CRD installation fails
- Verifies API endpoints are working before proceeding with workload deployment