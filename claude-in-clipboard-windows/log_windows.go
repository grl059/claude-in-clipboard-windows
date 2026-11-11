//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

var (
	logFile       *os.File
	pSetStdHandle = kernel32.NewProc("SetStdHandle")
)

// initLog writes diagnostics to %APPDATA%\ClaudeClip\log.txt. A tray app has no console, so without this
// a crash is invisible. Go's own fatal error output (stderr) goes there too.
func initLog() {
	p := filepath.Join(dataDir(), "log.txt")
	if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
		os.Remove(p)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	logFile = f
	pSetStdHandle.Call(uintptr(^uint32(11)), f.Fd()) // STD_ERROR_HANDLE (-12)
	os.Stderr = f
}

func logf(format string, args ...any) {
	if logFile != nil {
		fmt.Fprintf(logFile, time.Now().Format("2006-01-02 15:04:05.000")+" "+format+"\n", args...)
	}
}

// guard is deferred at the top of every callback from Windows: a panic is logged instead of killing the app.
func guard(where string) {
	if r := recover(); r != nil {
		logf("PANIC in %s: %v\n%s", where, r, debug.Stack())
	}
}
