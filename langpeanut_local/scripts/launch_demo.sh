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

# Build latest binary if needed
if [ ! -f "./langPeanut" ]; then
  echo "📦 Compiling langPeanut static binary..."
  go build -o langPeanut ./cmd/langPeanut
fi

echo "🚀 Booting live web server..."
./langPeanut demo --port "${PORT}" --open
