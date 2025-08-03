#!/bin/bash

echo "🧹 Cleaning up TMC tutorial..."

# Delete kind clusters
for cluster in tmc-kcp tmc-east tmc-west; do
    if kind get clusters 2>/dev/null | grep -q "^${cluster}$"; then
        echo "Deleting cluster: ${cluster}"
        kind delete cluster --name "${cluster}" || true
    fi
done

echo "✅ TMC tutorial cleanup complete!"
