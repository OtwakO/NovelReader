#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

ACTION="${1:-all}"

case "$ACTION" in
  backend|build-backend)
    echo "==> Building Go backend..."
    cd "$ROOT/backend"
    go build -o "$ROOT/backend/server" ./cmd/server/
    echo "    Done: backend/server"
    ;;
  frontend|build-frontend)
    echo "==> Building Svelte frontend..."
    cd "$ROOT/frontend"
    npm install --silent
    npm run build
    echo "    Done: frontend/dist/"
    ;;
  server|run)
    shift 2>/dev/null || true
    echo "==> Starting server on port ${PORT:-8888}..."
    cd "$ROOT/backend"
    DEBUG=1 exec go run ./cmd/server/ "$@"
    ;;
  build)
    "$0" build-backend
    "$0" build-frontend
    ;;
  *)
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  build-backend    Build Go binary"
    echo "  build-frontend   Build Svelte frontend"
    echo "  build            Build both"
    echo "  server           Run server (go run, for development)"
    echo "  run              Alias for server"
    echo ""
    echo "Environment:"
    echo "  PORT             Server port (default: 8888)"
    echo "  DATABASE_PATH    SQLite path (default: ./data/novelreader.db)"
    echo "  DEBUG=0          Disable debug-level logging (on by default in dev mode)"
    echo "  PUBLIC_URL       Optional canonical browser origin for proxies that rewrite Host"
    echo ""
    echo "Examples:"
    echo "  $0 server              # Start dev server"
    echo "  PORT=9999 $0 server    # Start on port 9999"
    ;;
esac
