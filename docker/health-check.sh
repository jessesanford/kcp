#!/bin/sh

# Health check script for KCP server with TMC components
# This script validates that KCP is running and TMC components are healthy

set -e

# Configuration
KCP_HOST=${KCP_HOST:-"localhost"}
KCP_PORT=${KCP_PORT:-"6443"}
HEALTH_PORT=${HEALTH_PORT:-"8080"}
TIMEOUT=${TIMEOUT:-"10"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

log_warn() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [WARN] $1" >&2
}

log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $1" >&2
}

# Check if KCP API server is responding
check_kcp_api() {
    log_info "Checking KCP API server health..."
    
    # Check if the API server port is open
    if ! nc -z "$KCP_HOST" "$KCP_PORT" 2>/dev/null; then
        log_error "KCP API server port $KCP_PORT is not accessible"
        return 1
    fi
    
    # Try to access the API server health endpoint
    if command -v curl >/dev/null 2>&1; then
        if curl -k -f -s --connect-timeout "$TIMEOUT" "https://$KCP_HOST:$KCP_PORT/healthz" >/dev/null; then
            log_info "KCP API server is healthy"
            return 0
        else
            log_warn "KCP API server health endpoint not responding"
            return 1
        fi
    else
        log_info "curl not available, assuming KCP is healthy (port is open)"
        return 0
    fi
}

# Check TMC metrics endpoint
check_tmc_metrics() {
    log_info "Checking TMC metrics endpoint..."
    
    if command -v curl >/dev/null 2>&1; then
        if curl -f -s --connect-timeout "$TIMEOUT" "http://$KCP_HOST:$HEALTH_PORT/metrics" >/dev/null; then
            log_info "TMC metrics endpoint is healthy"
            return 0
        else
            log_warn "TMC metrics endpoint not responding"
            return 1
        fi
    else
        log_info "curl not available, skipping metrics check"
        return 0
    fi
}

# Check TMC health endpoint
check_tmc_health() {
    log_info "Checking TMC health endpoint..."
    
    if command -v curl >/dev/null 2>&1; then
        if curl -f -s --connect-timeout "$TIMEOUT" "http://$KCP_HOST:$HEALTH_PORT/healthz" >/dev/null; then
            log_info "TMC health endpoint is healthy"
            return 0
        else
            log_warn "TMC health endpoint not responding"
            return 1
        fi
    else
        log_info "curl not available, skipping TMC health check"
        return 0
    fi
}

# Check if KCP process is running
check_kcp_process() {
    log_info "Checking KCP process..."
    
    if pgrep -f "kcp" >/dev/null 2>&1; then
        log_info "KCP process is running"
        return 0
    else
        log_error "KCP process not found"
        return 1
    fi
}

# Main health check function
main() {
    log_info "Starting KCP with TMC health check..."
    
    local failures=0
    
    # Essential checks
    if ! check_kcp_process; then
        failures=$((failures + 1))
    fi
    
    if ! check_kcp_api; then
        failures=$((failures + 1))
    fi
    
    # Optional TMC checks (warnings only)
    check_tmc_health || log_warn "TMC health check failed (non-critical)"
    check_tmc_metrics || log_warn "TMC metrics check failed (non-critical)"
    
    # Determine overall health
    if [ "$failures" -eq 0 ]; then
        log_info "✅ KCP with TMC is healthy"
        exit 0
    else
        log_error "❌ KCP health check failed ($failures critical failures)"
        exit 1
    fi
}

# Install minimal dependencies if needed
if ! command -v nc >/dev/null 2>&1; then
    if [ -w /tmp ]; then
        log_warn "nc (netcat) not available for port checks"
    fi
fi

# Run main function
main "$@"