#!/usr/bin/env bash
set -e

echo "🔧 Installing kubectl for current architecture..."

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    aarch64|arm64)
        KUBECTL_ARCH="arm64"
        ;;
    x86_64|amd64)
        KUBECTL_ARCH="amd64"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "📦 Detected architecture: $ARCH -> kubectl $KUBECTL_ARCH"

# Get latest stable version
KUBECTL_VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt)
echo "📥 Downloading kubectl $KUBECTL_VERSION for $KUBECTL_ARCH..."

# Download kubectl binary
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${KUBECTL_ARCH}/kubectl"

# Make it executable
chmod +x kubectl

# Move to system PATH
sudo mv kubectl /usr/local/bin/

# Verify installation
echo "✅ kubectl installed successfully:"
kubectl version --client --output=yaml

echo "🎉 kubectl installation completed!"
