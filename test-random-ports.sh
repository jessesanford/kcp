#!/bin/bash

echo "Testing random port generation"
echo "==============================="
echo ""

# Source the port generation logic
FOLDER_SUFFIX=$(basename "/workspaces/tmc-pr-upstream")

echo "Testing 10 random port generations:"
for i in {1..10}; do
    WEST_NODE_PORT=$((30000 + RANDOM % 2768))
    EAST_NODE_PORT=$((30000 + RANDOM % 2768))
    while [ "$EAST_NODE_PORT" -eq "$WEST_NODE_PORT" ]; do
        EAST_NODE_PORT=$((30000 + RANDOM % 2768))
    done
    echo "  Run $i: West=${WEST_NODE_PORT}, East=${EAST_NODE_PORT}"
done

echo ""
echo "Testing that demo shows ports correctly:"
echo "" | timeout 1 ./tmc-multi-cluster-demo.sh 2>&1 | grep -E "NodePort assignments|Creating cluster.*port" | head -3