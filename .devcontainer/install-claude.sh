#!/usr/bin/env bash
set -e

echo "🚀 Installing Claude Code CLI..."

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Ensure NVM is sourced
if [ -f "/usr/local/share/nvm/nvm.sh" ]; then
    echo "📦 Sourcing NVM..."
    source /usr/local/share/nvm/nvm.sh
else
    echo "❌ NVM not found at /usr/local/share/nvm/nvm.sh"
    exit 1
fi

# Check if Node.js is already installed and working
if command_exists node && command_exists npm; then
    echo "✅ Node.js and npm are already available"
    NODE_VERSION=$(node --version)
    NPM_VERSION=$(npm --version)
    echo "   Node.js: $NODE_VERSION"
    echo "   npm: $NPM_VERSION"
else
    echo "📥 Node.js not found, installing latest LTS version..."
    
    # Install latest LTS Node.js
    nvm install --lts
    nvm use --lts
    nvm alias default lts/*
    
    # Verify installation
    if command_exists node && command_exists npm; then
        NODE_VERSION=$(node --version)
        NPM_VERSION=$(npm --version)
        echo "✅ Node.js installed successfully"
        echo "   Node.js: $NODE_VERSION"
        echo "   npm: $NPM_VERSION"
    else
        echo "❌ Failed to install Node.js"
        exit 1
    fi
fi

# Check if Claude CLI is already installed
if command_exists claude; then
    CLAUDE_VERSION=$(claude --version)
    echo "✅ Claude CLI already installed: $CLAUDE_VERSION"
else
    echo "📥 Installing Claude CLI..."
    
    # Install Claude CLI
    npm install -g @anthropic-ai/claude-code
    
    # Verify installation
    if command_exists claude; then
        CLAUDE_VERSION=$(claude --version)
        echo "✅ Claude CLI installed successfully: $CLAUDE_VERSION"
    else
        echo "❌ Failed to install Claude CLI"
        exit 1
    fi
fi

echo "🎉 Claude Code CLI setup completed!"
