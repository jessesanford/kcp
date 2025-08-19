#!/usr/bin/env bash
# Bulletproof Node.js environment initialization script
# This ensures NVM/Node.js/Claude are available regardless of shell startup method

export NVM_DIR="/usr/local/share/nvm"

# Function to initialize NVM and Node.js
init_node_env() {
    if [ -f "$NVM_DIR/nvm.sh" ]; then
        source "$NVM_DIR/nvm.sh"
        
        # Install Node.js LTS if not available
        if [ "$(nvm current 2>/dev/null)" = "none" ] || [ "$(nvm current 2>/dev/null)" = "system" ]; then
            nvm install --lts >/dev/null 2>&1 || true
            nvm use --lts >/dev/null 2>&1 || true
            nvm alias default lts/* >/dev/null 2>&1 || true
        fi
        
        # Use default version
        nvm use default >/dev/null 2>&1 || true
        
        # Install Claude CLI if not available
        if ! command -v claude >/dev/null 2>&1; then
            npm install -g @anthropic-ai/claude-code >/dev/null 2>&1 || true
        fi
    fi
}

# Initialize if we're being sourced
if [ "${BASH_SOURCE[0]}" != "${0}" ] || [ "${ZSH_EVAL_CONTEXT}" = "file" ]; then
    init_node_env
fi
