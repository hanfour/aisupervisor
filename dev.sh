#!/bin/bash
# dev.sh — Start aisupervisor in development mode
# Usage: ./dev.sh
#
# This script starts Vite first, waits for it to be ready,
# then starts wails dev. This avoids the wails timeout issue
# when vite takes too long to start.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
WAILS_DIR="$SCRIPT_DIR/cmd/aisupervisor-gui"
WAILS_BIN="${GOPATH:-$HOME/go}/bin/wails"
VITE_PORT=41229

# Kill any existing processes
pkill -f "aisupervisor-gui" 2>/dev/null || true
pkill -f "vite.*$VITE_PORT" 2>/dev/null || true
sleep 1

echo "Starting Vite dev server on port $VITE_PORT..."
cd "$FRONTEND_DIR"
npm run dev &
VITE_PID=$!

# Wait for Vite to be ready
echo "Waiting for Vite to be ready..."
for i in $(seq 1 30); do
    if curl -s "http://localhost:$VITE_PORT" > /dev/null 2>&1; then
        echo "Vite is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Vite failed to start within 30 seconds"
        kill $VITE_PID 2>/dev/null
        exit 1
    fi
    sleep 1
done

echo "Starting Wails dev..."
cd "$WAILS_DIR"
"$WAILS_BIN" dev

# Cleanup on exit
kill $VITE_PID 2>/dev/null || true
