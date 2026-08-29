#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "🔄 Resetting all example projects to clean initial state..."

cd "$DIR"

# Restore tracked files in examples/
git checkout HEAD -- examples/

# Clean untracked generated locale files and checkpoints in examples/
rm -rf examples/nextjs-app/src/locales/
rm -rf examples/nextjs-app/public/locales/
rm -rf examples/nextjs-app/.langPeanut/
rm -rf examples/flutter-app/lib/l10n/
rm -rf examples/flutter-app/.langPeanut/
rm -rf examples/swiftui-app/Resources/
rm -rf examples/swiftui-app/.langPeanut/
rm -rf examples/android-app/app/src/main/res/values-*/
rm -rf examples/android-app/.langPeanut/

echo "✓ All example apps (Next.js, Flutter, SwiftUI, Android) reset to clean unlocalized state!"
