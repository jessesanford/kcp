#!/usr/bin/env bash
set -e

echo "🔧 Setting up shell configuration for Node.js/Claude CLI..."

# Add NVM initialization to shell profiles
SHELL_CONFIGS=(
    "/home/vscode/.bashrc"
    "/home/vscode/.zshrc"
)

NVM_INIT='
# NVM initialization for devcontainer
if [ -f "/usr/local/share/nvm/nvm.sh" ]; then
    source /usr/local/share/nvm/nvm.sh
    # Auto-use default Node.js version if available
    if [ -f "/usr/local/share/nvm/versions/node" ] && [ "$(ls -A /usr/local/share/nvm/versions/node 2>/dev/null)" ]; then
        nvm use default &>/dev/null || true
    fi
fi
'

for config in "${SHELL_CONFIGS[@]}"; do
    if [ -f "$config" ]; then
        # Check if NVM initialization is already present
        if ! grep -q "NVM initialization for devcontainer" "$config"; then
            echo "📝 Adding NVM initialization to $config"
            echo "$NVM_INIT" >> "$config"
        else
            echo "✅ NVM initialization already present in $config"
        fi
    else
        echo "📝 Creating $config with NVM initialization"
        echo "$NVM_INIT" > "$config"
    fi
done

echo "✅ Shell configuration completed!"
