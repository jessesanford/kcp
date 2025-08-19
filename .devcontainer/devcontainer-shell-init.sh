#!/usr/bin/env bash
# This script gets sourced by shell RC files to ensure Node.js/npm/claude are available
# It's part of the codebase so it survives devcontainer rebuilds

export NVM_DIR="/usr/local/share/nvm"
SETUP_COMPLETE_FILE="/tmp/.node-setup-complete"

# Only run Node.js installation once per container if not already installed
if [ ! -f "$SETUP_COMPLETE_FILE" ]; then
    if [ -f "$NVM_DIR/nvm.sh" ]; then
        source "$NVM_DIR/nvm.sh" >/dev/null 2>&1
        
        # Install Node.js LTS if no versions exist
        if [ ! -d "$NVM_DIR/versions/node" ] || [ -z "$(ls -A "$NVM_DIR/versions/node" 2>/dev/null)" ]; then
            echo "🔧 Installing Node.js LTS..."
            nvm install --lts >/dev/null 2>&1
            nvm use --lts >/dev/null 2>&1  
            nvm alias default 'lts/*' >/dev/null 2>&1
        fi
        
        # Install Claude CLI if not available
        if ! command -v claude >/dev/null 2>&1; then
            npm install -g @anthropic-ai/claude-code >/dev/null 2>&1
        fi
        
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
