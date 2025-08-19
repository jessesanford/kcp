# KCP Test Harness - Port Conflict Fix

## Issue Fixed
Fixed the KCP test harness port conflict issue where port 6443 was already in use by existing KCP instances.

## Files Created

### 1. `tmc-test-improved.sh` - Interactive Test Harness
- **Purpose**: Full-featured interactive test harness for TMC development
- **Features**:
  - Detects existing KCP processes and offers options (kill, reuse, or exit)
  - Automatically finds free ports if 6443 is busy
  - Extended timeouts for API server startup
  - Robust error handling and connection testing
  - Creates test resources with validation disabled for compatibility
  - Runs continuously until manually stopped

### 2. `cleanup-kcp.sh` - Cleanup Script
- **Purpose**: Clean up KCP processes and temporary directories
- **Features**:
  - Kills all running KCP processes
  - Verifies port 6443 is released
  - Cleans old test directories (keeps last 2)
  - Shows remaining test directories for debugging

### 3. `tmc-quick-test.sh` - Automated Test
- **Purpose**: Non-interactive quick test for CI/automated testing
- **Features**:
  - Automatically handles port conflicts
  - Minimal output for automation
  - Creates test namespace and ConfigMap
  - Runs for 10 seconds then exits automatically
  - Reports success/failure status clearly

## Usage

### Quick Test (Recommended)
```bash
./tmc-quick-test.sh
```

### Full Interactive Test
```bash
./tmc-test-improved.sh
```

### Cleanup
```bash
./cleanup-kcp.sh
```

## Test Results

✅ **SUCCESSFUL TEST RUN**: 
- KCP starts successfully with TMC feature flags
- Port conflict resolution works automatically
- Namespace and ConfigMap creation succeeds
- API server becomes available and responsive
- Test resources are created and verified

## Key Improvements Made

1. **Port Conflict Resolution**: Automatically detects port 6443 usage and finds free ports
2. **Extended Timeouts**: Increased wait times for KCP initialization (60s) and API server readiness (30s)
3. **Better Error Handling**: Graceful handling of connection failures and timeouts
4. **Resource Validation**: Uses `--validate=false` to avoid API validation issues during startup
5. **Interactive Options**: Allows reusing existing KCP instances instead of always killing them
6. **Cleanup Automation**: Proper cleanup scripts to prevent process accumulation

## Feature Flags Tested
- `TMCFeature=true`
- `TMCAPIs=true` 
- `TMCControllers=true`
- `TMCPlacement=true`

The test harness now successfully starts KCP with all TMC feature flags enabled and creates test resources without port conflicts.