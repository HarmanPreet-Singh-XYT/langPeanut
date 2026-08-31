#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NEXTJS_DIR="$DIR/examples/nextjs-app"

echo "🥜 ==========================================================="
echo "   Testing langPeanut on Real Next.js / React Application"
echo "==========================================================="

echo ""
echo "1. Building latest langPeanut binary..."
go build -o "$DIR/langPeanut" "$DIR/cmd/langPeanut"

echo ""
echo "2. Running langPeanut Audit (Non-destructive scan)..."
"$DIR/langPeanut" audit --dir "$NEXTJS_DIR"

echo ""
echo "3. Running langPeanut Translate (with Gen-Z Slang Style)..."
"$DIR/langPeanut" translate --dir "$NEXTJS_DIR" --locales fr,es,de,ja --style gen_z

echo ""
echo "4. Checking Generated Locale Files:"
ls -lh "$NEXTJS_DIR/src/locales/"

echo ""
echo "5. Inspecting French (fr.json) Gen-Z Output:"
cat "$NEXTJS_DIR/src/locales/fr.json"

echo ""
echo "✓ Next.js Real Project Test Completed Successfully!"
