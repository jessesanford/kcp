#!/usr/bin/env bash
# This script gets sourced by shell RC files to ensure Node.js/npm/claude are available
# It's part of the codebase so it survives devcontainer rebuilds

export NVM_DIR="/usr/local/share/nvm"
SETUP_COMPLETE_FILE="/tmp/.node-setup-complete"

# Only run full setup once per container
if [ ! -f "$SETUP_COMPLETE_FILE" ]; then
    echo "🔧 Setting up Node.js environment..."
    
    # Run our setup script if it exists
    if [ -f "/workspaces/kcp/.devcontainer/setup-shell-environment.sh" ]; then
        /workspaces/kcp/.devcontainer/setup-shell-environment.sh >/dev/null 2>&1
        touch "$SETUP_COMPLETE_FILE"
        echo "✅ Node.js environment ready!"
    fi
fi

# Always ensure NVM and Node.js are loaded
if [ -f "$NVM_DIR/nvm.sh" ]; then
    source "$NVM_DIR/nvm.sh" >/dev/null 2>&1
    
    # Activate default Node.js version
    if command -v nvm >/dev/null 2>&1; then
        nvm use default >/dev/null 2>&1 || true
    fi
fi
