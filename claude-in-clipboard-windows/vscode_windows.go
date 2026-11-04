//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"claudeclip/internal/logic"
)

// recentVSCodeFolders lists folders VS Code has opened, newest first (from its per-workspace storage).
func recentVSCodeFolders() []string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil
	}
	root := filepath.Join(appData, "Code", "User", "workspaceStorage")
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	type entry struct {
		mod  time.Time
		path string
	}
	var found []entry
	for _, d := range dirs {
		dir := filepath.Join(root, d.Name())
		data, err := os.ReadFile(filepath.Join(dir, "workspace.json"))
		if err != nil {
			continue
		}
		var ws struct {
			Folder string `json:"folder"`
		}
		if json.Unmarshal(data, &ws) != nil || ws.Folder == "" {
			continue
		}
		p, ok := logic.FileURIToPath(ws.Folder)
		if !ok {
			continue
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			continue
		}
		var mod time.Time
		if st, err := os.Stat(filepath.Join(dir, "state.vscdb")); err == nil {
			mod = st.ModTime()
		}
		found = append(found, entry{mod, p})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].mod.After(found[j].mod) })
	out := make([]string, len(found))
	for i, e := range found {
		out[i] = e.path
	}
	return out
}

var (
	projMu    sync.Mutex
	projCache string
)

// projectNow returns the project ClaudeClip would use right now. The lookup touches the disk, which can
// hang on an offline network drive, so it runs in the background; if it takes longer than timeout the
// last known answer is returned instead.
func projectNow(timeout time.Duration) string {
	ch := make(chan string, 1)
	go func() {
		d := logic.PickProject(vscodeWindowTitle(), recentVSCodeFolders())
		projMu.Lock()
		projCache = d
		projMu.Unlock()
		ch <- d
	}()
	select {
	case d := <-ch:
		return d
	case <-time.After(timeout):
		projMu.Lock()
		defer projMu.Unlock()
		return projCache
	}
}

var (
	enumMu       sync.Mutex
	enumResult   string
	enumPidIsVSC = map[uint32]bool{}
	enumCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if !isVisible(hwnd) {
			return 1
		}
		var buf [512]uint16
		n, _, _ := pGetWindowTextW.Call(hwnd, ptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return 1
		}
		var cls [64]uint16
		pGetClassNameW.Call(hwnd, ptr(unsafe.Pointer(&cls[0])), uintptr(len(cls)))
		if syscall.UTF16ToString(cls[:]) != "Chrome_WidgetWin_1" { // Electron's top-level window class
			return 1
		}
		var pid uint32
		pGetWindowThreadProcessId.Call(hwnd, ptr(unsafe.Pointer(&pid)))
		isVSC, seen := enumPidIsVSC[pid]
		if !seen {
			isVSC = strings.EqualFold(processImageName(pid), "Code.exe")
			enumPidIsVSC[pid] = isVSC
		}
		if !isVSC {
			return 1
		}
		enumResult = syscall.UTF16ToString(buf[:])
		return 0 // EnumWindows walks top to bottom in z-order: the first hit is the most recently used VS Code window
	})
)

func processImageName(pid uint32) string {
	h, _, _ := pOpenProcess.Call(0x1000 /* PROCESS_QUERY_LIMITED_INFORMATION */, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer pCloseHandle.Call(h)
	var buf [1024]uint16
	n := uint32(len(buf))
	if r, _, _ := pQueryFullProcessImageNam.Call(h, 0, ptr(unsafe.Pointer(&buf[0])), ptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return filepath.Base(syscall.UTF16ToString(buf[:n]))
}

// vscodeWindowTitle is the title of the frontmost VS Code window, even when another app has focus.
func vscodeWindowTitle() string {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumResult = ""
	enumPidIsVSC = map[uint32]bool{}
	pEnumWindows.Call(enumCallback, 0)
	return enumResult
}
