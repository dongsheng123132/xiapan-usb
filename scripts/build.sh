#!/usr/bin/env bash
# macOS / Linux counterpart of build.ps1. On macOS it also joins both Mac
# builds into one universal binary for 虾盘.app.
set -euo pipefail
cd "$(dirname "$0")/.."
export CGO_ENABLED=0
go test ./...
build() {
  local goos=$1 goarch=$2 dir=$3 name=$4 ldflags="-s -w"
  if [ "$goos" = windows ]; then ldflags="$ldflags -H windowsgui"; fi
  mkdir -p "dist/$dir"
  GOOS=$goos GOARCH=$goarch go build -buildvcs=false -trimpath "-ldflags=$ldflags" -o "dist/$dir/$name" .
  echo "$dir: $(wc -c <"dist/$dir/$name" | tr -d ' ') bytes"
}
build windows amd64 windows-x64 虾盘.exe
build darwin arm64 macos-arm64 xiapan
build darwin amd64 macos-x64 xiapan
build linux amd64 linux-x64 xiapan
if command -v lipo >/dev/null 2>&1; then
  mkdir -p dist/macos-universal
  lipo -create -output dist/macos-universal/xiapan dist/macos-arm64/xiapan dist/macos-x64/xiapan
  echo "macos-universal: $(wc -c <dist/macos-universal/xiapan | tr -d ' ') bytes ($(lipo -archs dist/macos-universal/xiapan))"
fi
