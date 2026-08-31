#!/usr/bin/env bash
set -e

# ==============================================================================
# launch_demo.sh — 1-Click Launch of Interactive Web App Localization Demo
# ==============================================================================

PORT=${1:-3000}

echo "🥜 ==========================================================="
echo "   langPeanut: Live Interactive Web Demo Launcher"
echo "   Starting at http://localhost:${PORT}"
echo "============================================================="

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/go.mod" ]; then
    PROJECT_DIR="$SCRIPT_DIR"
elif [ -f "$SCRIPT_DIR/../go.mod" ]; then
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    PROJECT_DIR="$(pwd)"
fi
cd "$PROJECT_DIR"

# Build latest binary if needed
if [ ! -f "./bin/langPeanut" ] && [ ! -f "./langPeanut" ]; then
  echo "📦 Compiling langPeanut static binary..."
  mkdir -p bin
  go build -o bin/langPeanut ./cmd/langPeanut
  ln -sf bin/langPeanut langPeanut
fi

BIN="./bin/langPeanut"
if [ ! -f "$BIN" ]; then
  BIN="./langPeanut"
fi

echo "🚀 Booting live web server..."
"$BIN" web --port "${PORT}" --open
