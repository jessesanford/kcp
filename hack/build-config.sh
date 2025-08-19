#!/usr/bin/env bash

# Copyright 2022 The KCP Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# build-config.sh provides centralized build configuration for KCP
# This script sets up build environment variables and configuration

set -euo pipefail

# Build configuration constants
DEFAULT_GO_VERSION="1.23"
DEFAULT_BUILD_TIMEOUT="10m"
DEFAULT_TEST_TIMEOUT="30m"

# Environment detection
detect_build_env() {
    if [[ -n "${CI:-}" ]]; then
        echo "ci"
    elif [[ -n "${CONTAINER:-}" ]]; then
        echo "container"
    else
        echo "local"
    fi
}

# Go version validation
validate_go_version() {
    local required_version="${1:-$DEFAULT_GO_VERSION}"
    local current_version
    
    if ! command -v go >/dev/null 2>&1; then
        echo "ERROR: Go is not installed or not in PATH" >&2
        return 1
    fi
    
    current_version=$(go version | sed -n 's/.*go\([0-9]\+\.[0-9]\+\).*/\1/p')
    
    if ! printf '%s\n%s\n' "$required_version" "$current_version" | sort -V -C; then
        echo "WARNING: Go version $current_version detected, but $required_version or higher is recommended" >&2
        return 1
    fi
    
    return 0
}

# Build flags optimization based on environment
get_build_flags() {
    local env="${1:-$(detect_build_env)}"
    local flags=""
    
    case "$env" in
        "ci")
            flags="-v -race"
            ;;
        "container")
            flags="-v"
            ;;
        "local")
            flags="-v"
            ;;
        *)
            flags="-v"
            ;;
    esac
    
    echo "$flags"
}

# Test flags optimization based on environment
get_test_flags() {
    local env="${1:-$(detect_build_env)}"
    local flags=""
    
    case "$env" in
        "ci")
            flags="-v -race -timeout=$DEFAULT_TEST_TIMEOUT"
            ;;
        "container")
            flags="-v -timeout=$DEFAULT_TEST_TIMEOUT"
            ;;
        "local")
            flags="-v -timeout=$DEFAULT_TEST_TIMEOUT"
            ;;
        *)
            flags="-v -timeout=$DEFAULT_TEST_TIMEOUT"
            ;;
    esac
    
    echo "$flags"
}

# Cache optimization
setup_build_cache() {
    local cache_dir="${KCP_BUILD_CACHE:-$HOME/.cache/kcp}"
    
    mkdir -p "$cache_dir"
    export GOCACHE="$cache_dir/go-build"
    export GOMODCACHE="$cache_dir/go-mod"
    
    echo "Build cache configured: $cache_dir"
}

# Main configuration function
configure_build() {
    local env
    env=$(detect_build_env)
    
    echo "Configuring build for environment: $env"
    
    # Validate Go version
    if ! validate_go_version; then
        echo "Consider upgrading Go for optimal build performance"
    fi
    
    # Setup build cache
    setup_build_cache
    
    # Export environment variables
    export KCP_BUILD_ENV="$env"
    export KCP_BUILD_FLAGS="$(get_build_flags "$env")"
    export KCP_TEST_FLAGS="$(get_test_flags "$env")"
    
    echo "Build configuration complete:"
    echo "  Environment: $env"
    echo "  Build flags: $KCP_BUILD_FLAGS"
    echo "  Test flags: $KCP_TEST_FLAGS"
    echo "  Go cache: $GOCACHE"
    echo "  Go mod cache: $GOMODCACHE"
}

# If script is executed directly (not sourced), run configuration
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    configure_build "$@"
fi