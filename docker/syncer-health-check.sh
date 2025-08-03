#!/bin/sh

# Health check script for KCP workload syncer with TMC components
# This script validates that the syncer is running and healthy

set -e

# Configuration
SYNCER_HOST=${SYNCER_HOST:-"localhost"}
METRICS_PORT=${METRICS_PORT:-"8080"}
TIMEOUT=${TIMEOUT:-"10"}

log_info() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

log_warn() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [WARN] $1" >&2
}

log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $1" >&2
}

# Check if syncer process is running
check_syncer_process() {
    log_info "Checking workload syncer process..."
    
    if pgrep -f "workload-syncer" >/dev/null 2>&1; then
        log_info "Workload syncer process is running"
        return 0
    else
        log_error "Workload syncer process not found"
        return 1
    fi
}

# Check syncer metrics endpoint
check_syncer_metrics() {
    log_info "Checking syncer metrics endpoint..."
    
    if ! nc -z "$SYNCER_HOST" "$METRICS_PORT" 2>/dev/null; then
        log_error "Syncer metrics port $METRICS_PORT is not accessible"
        return 1
    fi
    
    if command -v curl >/dev/null 2>&1; then
        if curl -f -s --connect-timeout "$TIMEOUT" "http://$SYNCER_HOST:$METRICS_PORT/metrics" >/dev/null; then
            log_info "Syncer metrics endpoint is healthy"
            return 0
        else
            log_warn "Syncer metrics endpoint not responding"
            return 1
        fi
    else
        log_info "curl not available, assuming syncer is healthy (port is open)"
        return 0
    fi
}

# Check TMC components in syncer metrics
check_tmc_metrics() {
    log_info "Checking TMC component metrics..."
    
    if command -v curl >/dev/null 2>&1; then
        local metrics_output
        if metrics_output=$(curl -f -s --connect-timeout "$TIMEOUT" "http://$SYNCER_HOST:$METRICS_PORT/metrics" 2>/dev/null); then
            # Check for TMC-specific metrics
            if echo "$metrics_output" | grep -q "tmc_"; then
                log_info "TMC metrics are being reported"
                return 0
            else
                log_warn "TMC metrics not found in output"
                return 1
            fi
        else
            log_warn "Failed to retrieve metrics for TMC check"
            return 1
        fi
    else
        log_info "curl not available, skipping TMC metrics check"
        return 0
    fi
}

# Check syncer readiness
check_syncer_readiness() {
    log_info "Checking syncer readiness..."
    
    # Look for log indicators of successful startup
    if [ -f "/var/log/syncer/syncer.log" ]; then
        if tail -n 100 "/var/log/syncer/syncer.log" 2>/dev/null | grep -q "syncer started\|controller started\|watching"; then
            log_info "Syncer appears to be ready (found startup indicators)"
            return 0
        else
            log_warn "Syncer readiness unclear from logs"
            return 1
        fi
    else
        log_info "Syncer log file not found, assuming ready"
        return 0
    fi
}

# Main health check function
main() {
    log_info "Starting workload syncer health check..."
    
    local failures=0
    
    # Essential checks
    if ! check_syncer_process; then
        failures=$((failures + 1))
    fi
    
    if ! check_syncer_metrics; then
        failures=$((failures + 1))
    fi
    
    # Optional checks (warnings only)
    check_tmc_metrics || log_warn "TMC metrics check failed (non-critical)"
    check_syncer_readiness || log_warn "Syncer readiness check failed (non-critical)"
    
    # Determine overall health
    if [ "$failures" -eq 0 ]; then
        log_info "✅ Workload syncer is healthy"
        exit 0
    else
        log_error "❌ Workload syncer health check failed ($failures critical failures)"
        exit 1
    fi
}

# Run main function
main "$@"