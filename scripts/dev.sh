#!/bin/bash

PROJECTDIR=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
cd "$PROJECTDIR"

cleanup() {
    pids=$(lsof -ti tcp:9245 2>/dev/null)
    if [ -n "$pids" ]; then
        echo "Cleaning up port 9245..."
        echo "$pids" | xargs kill -9 2>/dev/null || true
    fi
}
trap cleanup EXIT

wails3 dev "$@"
