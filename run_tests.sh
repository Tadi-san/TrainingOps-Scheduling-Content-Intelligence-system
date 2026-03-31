#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

echo "Running backend tests..."
(
  cd "$BACKEND_DIR"
  export GOCACHE="$BACKEND_DIR/.gocache"
  export GOMODCACHE="$BACKEND_DIR/.gomodcache"
  mkdir -p "$GOCACHE" "$GOMODCACHE"
  go test ./...
)

if [ -d "$FRONTEND_DIR/node_modules" ]; then
  echo "Running frontend build..."
  (
    cd "$FRONTEND_DIR"
    npm run build
  )
else
  echo "Skipping frontend build because node_modules is not installed."
fi

echo "All available checks passed."

