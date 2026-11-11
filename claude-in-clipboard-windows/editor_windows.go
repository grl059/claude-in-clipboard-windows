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

// target is what a prompt will run against: a VS Code project folder or the file open in Notepad.
type target struct {
	kind string // "vscode" or "notepad"; empty when nothing was found
	name string // folder name (VS Code) or file name (Notepad)
	dir  string // working directory for claude
	file string // Notepad: the open file's name when its folder is known
}

func (t target) app() string {
	if t.kind == "notepad" {
		return "Notepad"
	}
	return "VS Code"
}

func (t target) found() bool { return t.dir != "" }

func scratchDir() string {
	d := filepath.Join(dataDir(), "scratch")
	os.MkdirAll(d, 0o755)
	return d
}

// recentFilePath finds the folder of a file Notepad has open. Notepad's title only has the file name, but
// opening a file through Explorer or a dialog leaves a shortcut for it in the Recent folder.
func recentFilePath(name string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" || strings.ContainsAny(name, `\/:*?"<>|`) {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(appData, "Microsoft", "Windows", "Recent", name+".lnk"))
	if err != nil {
		return ""
	}
	p, ok := logic.ParseLnkTarget(data)
	if !ok || !strings.EqualFold(filepath.Base(p), name) {
		return ""
	}
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return ""
	}
	return p
}

func notepadTarget(title string) target {
	t := target{kind: "notepad", name: "Untitled", dir: scratchDir()}
	name, ok := logic.NotepadFile(title)
	if !ok {
		return t // nothing saved yet: a scratch folder keeps Claude away from your other files
	}
	t.name = name
	if p := recentFilePath(name); p != "" {
		t.dir, t.file = logic.DirOf(p), name
	}
	return t
}

// resolveTarget looks at the frontmost editor (VS Code or Notepad) and decides where claude runs.
// It reads the disk, so call it off the UI thread.
func resolveTarget() target {
	kind, title := frontEditor()
	if kind == "notepad" {
		return notepadTarget(title)
	}
	dir := logic.PickProject(title, recentVSCodeFolders()) // title is empty when no VS Code window is open: newest project
	if dir == "" {
		return target{}
	}
	return target{kind: "vscode", name: logic.FolderName(dir), dir: dir}
}

var (
	projMu    sync.Mutex
	projCache target
)

// targetNow returns the target ClaudeClip would use right now. The lookup touches the disk, which can
// hang on an offline network drive, so it runs in the background; if it takes longer than timeout the
// last known answer is returned instead.
func targetNow(timeout time.Duration) target {
	ch := make(chan target, 1)
	go func() {
		t := resolveTarget()
		projMu.Lock()
		projCache = t
		projMu.Unlock()
		ch <- t
	}()
	select {
	case t := <-ch:
		return t
	case <-time.After(timeout):
		projMu.Lock()
		defer projMu.Unlock()
		return projCache
	}
}

var (
	enumMu       sync.Mutex
	enumKind     string
	enumTitle    string
	enumPidApp   = map[uint32]string{}
	enumCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if !isVisible(hwnd) {
			return 1
		}
		var buf [512]uint16
		n, _, _ := pGetWindowTextW.Call(hwnd, ptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return 1
		}
		var pid uint32
		pGetWindowThreadProcessId.Call(hwnd, ptr(unsafe.Pointer(&pid)))
		kind, seen := enumPidApp[pid]
		if !seen {
			switch strings.ToLower(processImageName(pid)) {
			case "code.exe":
				kind = "vscode"
			case "notepad.exe":
				kind = "notepad"
			}
			enumPidApp[pid] = kind
		}
		if kind == "" {
			return 1
		}
		if kind == "vscode" { // Electron's top-level window class; skips helper windows
			var cls [64]uint16
			pGetClassNameW.Call(hwnd, ptr(unsafe.Pointer(&cls[0])), uintptr(len(cls)))
			if syscall.UTF16ToString(cls[:]) != "Chrome_WidgetWin_1" {
				return 1
			}
		}
		enumKind, enumTitle = kind, syscall.UTF16ToString(buf[:])
		return 0 // EnumWindows walks top to bottom in z-order: the first hit is the editor you used last
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

// frontEditor returns the app ("vscode" or "notepad") and title of the frontmost VS Code or Notepad
// window, even when another app has focus. kind is empty when neither is open.
func frontEditor() (kind, title string) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumKind, enumTitle = "", ""
	enumPidApp = map[uint32]string{}
	pEnumWindows.Call(enumCallback, 0)
	return enumKind, enumTitle
}
