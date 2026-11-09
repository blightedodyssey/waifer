#!/usr/bin/env bash
# Build the Windows port of Aether from any OS with Go + Node + Wails.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v wails >/dev/null 2>&1; then
    echo "wails CLI not found."
    echo "Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0"
    exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
    echo "npm is required to build the Svelte frontend."
    exit 1
fi

echo "==> Installing frontend dependencies"
npm --prefix frontend install --no-audit --no-fund

echo "==> Regenerating Wails bindings"
wails generate module

echo "==> Building windows/amd64"
wails build -platform windows/amd64 "$@"

echo
echo "Built build/bin/aether.exe"
echo "Copy it to a Windows machine and run it (WebView2 runtime required)."
