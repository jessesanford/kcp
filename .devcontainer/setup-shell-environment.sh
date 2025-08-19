#!/usr/bin/env bash
set -e

echo "🔧 Setting up shell environment for Node.js/npm/claude..."

# Ensure NVM is available and Node.js is installed
if [ -f "/usr/local/share/nvm/nvm.sh" ]; then
    echo "📦 Loading NVM..."
    source /usr/local/share/nvm/nvm.sh
    
    # Install Node.js LTS if not already installed
    if [ "$(nvm current 2>/dev/null)" = "none" ] || [ "$(nvm current 2>/dev/null)" = "system" ]; then
        echo "📥 Installing Node.js LTS..."
        nvm install --lts
        nvm use --lts
        nvm alias default lts/*
    fi
    
    # Ensure default version is active
    nvm use default
    
    echo "✅ Node.js setup completed"
    echo "   Node.js: $(node --version)"
    echo "   npm: $(npm --version)"
else
    echo "❌ NVM not found - this is a problem with the devcontainer feature"
    exit 1
fi

# Install Claude CLI if not already installed
if ! command -v claude >/dev/null 2>&1; then
    echo "📥 Installing Claude CLI..."
    npm install -g @anthropic-ai/claude-code
else
    echo "✅ Claude CLI already installed: $(claude --version)"
fi

# NVM initialization script for shell RC files
NVM_INIT_SCRIPT='
# NVM and Node.js initialization for devcontainer
export NVM_DIR="/usr/local/share/nvm"
if [ -f "$NVM_DIR/nvm.sh" ]; then
    source "$NVM_DIR/nvm.sh"
    # Auto-load default Node.js version
    if [ "$(nvm current 2>/dev/null)" = "none" ] || [ "$(nvm current 2>/dev/null)" = "system" ]; then
        nvm use default >/dev/null 2>&1 || true
    fi
fi
'

# Add to bash
BASHRC="/home/vscode/.bashrc"
if [ -f "$BASHRC" ]; then
    if ! grep -q "NVM and Node.js initialization for devcontainer" "$BASHRC"; then
        echo "📝 Adding NVM initialization to .bashrc"
        echo "$NVM_INIT_SCRIPT" >> "$BASHRC"
    else
        echo "✅ NVM initialization already present in .bashrc"
    fi
else
    echo "📝 Creating .bashrc with NVM initialization"
    echo "$NVM_INIT_SCRIPT" > "$BASHRC"
fi

# Add to zsh
ZSHRC="/home/vscode/.zshrc"
if [ -f "$ZSHRC" ]; then
    if ! grep -q "NVM and Node.js initialization for devcontainer" "$ZSHRC"; then
        echo "📝 Adding NVM initialization to .zshrc"
        echo "$NVM_INIT_SCRIPT" >> "$ZSHRC"
    else
        echo "✅ NVM initialization already present in .zshrc"
    fi
else
    echo "📝 Creating .zshrc with NVM initialization"
    echo "$NVM_INIT_SCRIPT" > "$ZSHRC"
fi

# Set proper ownership
chown vscode:vscode /home/vscode/.bashrc /home/vscode/.zshrc 2>/dev/null || true

echo "🎉 Shell environment setup completed!"
echo ""
echo "Node.js, npm, and claude should now be available in all new shell sessions."
