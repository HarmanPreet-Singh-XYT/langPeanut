#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "🥜 ==========================================================="
echo "   Testing langPeanut Across All 4 Real Framework Apps"
echo "==========================================================="

echo ""
echo "1. Building langPeanut binary..."
go build -o "$DIR/langPeanut" "$DIR/cmd/langPeanut"

echo ""
echo "2. [React / Next.js] Translating examples/nextjs-app..."
"$DIR/langPeanut" translate --dir "$DIR/examples/nextjs-app" --locales fr,es,de,ja --style gen_z

echo ""
echo "3. [Flutter / Dart] Translating examples/flutter-app..."
"$DIR/langPeanut" translate --dir "$DIR/examples/flutter-app" --locales fr,es,de,ja

echo ""
echo "4. [iOS / SwiftUI] Translating examples/swiftui-app..."
"$DIR/langPeanut" translate --dir "$DIR/examples/swiftui-app" --locales fr,es,de,ja

echo ""
echo "5. [Android / Kotlin] Translating examples/android-app..."
"$DIR/langPeanut" translate --dir "$DIR/examples/android-app" --locales fr,es,de,ja

echo ""
echo "🎉 All 4 Framework Example Projects Successfully Localized with 4-Tier Critic Verification!"
