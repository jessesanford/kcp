#!/usr/bin/env bash
set -e

# Install KIND (Kubernetes IN Docker)
KIND_VERSION="v0.23.0" # Change as needed
curl -Lo /usr/local/bin/kind https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64
chmod +x /usr/local/bin/kind

echo "KIND installed!"
