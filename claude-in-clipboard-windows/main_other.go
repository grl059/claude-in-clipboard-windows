//go:build !windows

// ClaudeClip only runs on Windows. This stub lets `go vet ./...` and `go test ./...` work on other systems.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "ClaudeClip for Windows only runs on Windows. Build with GOOS=windows (see build.ps1 / build.sh).")
	os.Exit(1)
}
