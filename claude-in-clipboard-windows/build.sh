#!/bin/sh
# Cross-compiles build/ClaudeClip.exe from macOS or Linux and runs the checks.
# Needs Go 1.22 or newer. Set GOARCH=arm64 for Windows on ARM.
set -e
cd "$(dirname "$0")"
go vet ./...
go test ./...
mkdir -p build
GOOS=windows GOARCH="${GOARCH:-amd64}" CGO_ENABLED=0 go build -trimpath -ldflags '-H=windowsgui -s -w' -o build/ClaudeClip.exe .
echo "Built build/ClaudeClip.exe"
