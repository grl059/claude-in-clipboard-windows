//go:build windows

// ClaudeClip for Windows: tray app. Tap the trigger key (Right Ctrl by default), type a prompt blind,
// press Enter. Claude Code reads your current VS Code project (or the file open in Notepad) and puts the result on the clipboard.
// Double-tap clears the clipboard; holding the key previews the clipboard.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"claudeclip/internal/logic"
)

const (
	wmTray    = wmApp + 0 // tray icon mouse events
	wmDone    = wmApp + 1 // a claude run finished
	wmTap     = wmApp + 2 // the trigger key was tapped
	wmStarted = wmApp + 3 // a job found its project

	timerGhostShow  = 1
	timerGhostWatch = 2
	timerRevert     = 3
)

var (
	mainHwnd           uintptr
	taskbarCreatedMsg  uintptr
	hook               uintptr
	debugPrompt        bool
	cfg                settings
	tap                logic.TapDetector
	triggerDown        bool
	ghostPending       bool
	lastTap, tapAt     float64
	task               *job
	taskID             int
	infoText           = "Ready"
	startedAt          = time.Now()
	hookCallback       uintptr
	singletonMutexName = u16(`Local\ClaudeClipSingleton`)
)

func init() { runtime.LockOSThread() } // window and hook callbacks must stay on one thread

func nowSec() float64 { return time.Since(startedAt).Seconds() }

func main() {
	if _, _, err := pCreateMutexW.Call(0, 0, ptr(unsafe.Pointer(singletonMutexName))); err == syscall.Errno(183) {
		return // already running
	}
	pSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	for _, a := range os.Args[1:] {
		if a == "--show-prompt" {
			debugPrompt = true
		}
	}
	if os.Getenv("CLAUDECLIP_DEBUG") == "1" {
		debugPrompt = true
	}
	initLog()
	cfg = loadSettings()
	infoText = readyText()
	logf("start: model=%s trigger=%s debugPrompt=%v args=%v", cfg.Model, cfg.Trigger, debugPrompt, os.Args[1:])

	class := u16("ClaudeClipMain")
	registerClass(class, mainProc)
	mainHwnd, _, _ = pCreateWindowExW.Call(wsExToolWindow, ptr(unsafe.Pointer(class)),
		ptr(unsafe.Pointer(u16("ClaudeClip"))), 0, 0, 0, 0, 0, 0, 0, getModuleHandle(), 0)
	createGhost()
	createPrompt()
	taskbarCreatedMsg, _, _ = pRegisterWindowMessageW.Call(ptr(unsafe.Pointer(u16("TaskbarCreated"))))
	lastLightMode = taskbarIsLight()
	trayNotify(nimAdd)

	// Watching the trigger key system-wide needs a low-level keyboard hook (no special permission).
	hookCallback = syscall.NewCallback(keyboardProc)
	hook, _, _ = pSetWindowsHookExW.Call(whKeyboardLL, hookCallback, getModuleHandle(), 0)
	logf("keyboard hook installed: %v", hook != 0)
	if hook == 0 {
		fail("Could not install the keyboard hook")
	}
	refreshAccount()

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(ptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(ptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(ptr(unsafe.Pointer(&m)))
	}
}

// MARK: keyboard

func keyboardProc(nCode, wp, lp uintptr) uintptr {
	defer guard("keyboardProc")
	if int32(nCode) == 0 { // HC_ACTION
		k := (*kbdLLHook)(fromPtr(lp))
		down := wp == wmKeyDown || wp == wmSysKeyDown
		up := wp == wmKeyUp || wp == wmSysKeyUp
		if (down || up) && handleKey(k.vkCode, down) {
			return 1 // swallow (only ever the trigger key itself, and only for Caps Lock)
		}
	}
	r, _, _ := pCallNextHookEx.Call(hook, nCode, wp, lp)
	return r
}

// handleKey runs inside the hook, so it only does cheap work; taps are posted to the message loop.
// It returns true when the key event must not reach other apps.
func handleKey(vk uint32, down bool) bool {
	trig := currentTrigger(cfg)
	if vk != trig.vk { // Right Ctrl + C etc. is a shortcut, not a tap or a hold
		if down {
			tap.OtherKey()
			hideGhost()
		}
		return false
	}
	swallow := trig.vk == vkCapital // Caps Lock must not also toggle caps
	if down {
		if triggerDown && (swallow || keyPhysicallyDown(trig.vk)) {
			return swallow // auto-repeat while held
		}
		triggerDown = true
		scheduleGhost()
		tap.Key(true, nowSec())
		return swallow
	}
	triggerDown = false
	hideGhost()
	t := nowSec()
	if tap.Key(false, t) {
		tapAt = t
		postMessage(mainHwnd, wmTap, 0, 0)
	}
	return swallow
}

func keyPhysicallyDown(vk uint32) bool {
	r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
	return uint16(r)&0x8000 != 0
}

// MARK: main window

func mainProc(hwnd, m, wp, lp uintptr) uintptr {
	defer guard("mainProc")
	if m == taskbarCreatedMsg && m != 0 { // Explorer restarted: put the icon back
		trayNotify(nimAdd)
		return 0
	}
	switch uint32(m) {
	case wmTray:
		if e := loword(lp); e == wmLButtonUp || e == wmRButtonUp {
			showMenu()
		}
		return 0
	case wmTap:
		onTap(tapAt)
		return 0
	case wmDone:
		handleDone()
		return 0
	case wmStarted:
		if task != nil && task.target().found() {
			infoText = fmt.Sprintf("Working in %s with %s…", task.target().name, modelName())
			trayNotify(nimModify)
		}
		return 0
	case wmCommand:
		onCommand(int(loword(wp)))
		return 0
	case wmSettingChg:
		if l := taskbarIsLight(); l != lastLightMode { // taskbar theme changed: redraw the glyph in the right colour
			lastLightMode = l
			trayNotify(nimModify)
		}
		return 0
	case wmTimer:
		switch wp {
		case timerGhostShow:
			killTimer(hwnd, timerGhostShow)
			ghostPending = false
			if triggerDown {
				showGhost()
			}
		case timerGhostWatch:
			trig := currentTrigger(cfg)
			if gh.visible && trig.vk != vkCapital && !keyPhysicallyDown(trig.vk) {
				triggerDown = false
				hideGhost()
			}
		case timerRevert:
			killTimer(hwnd, timerRevert)
			setIcon(idleIcon())
		}
		return 0
	case wmDestroy:
		logf("quit (window destroyed)")
		if hook != 0 {
			pUnhookWindowsHookEx.Call(hook)
		}
		if task != nil {
			task.kill()
		}
		trayNotify(nimDelete)
		pPostQuitMessage.Call(0)
		return 0
	}
	return defWindowProc(hwnd, m, wp, lp)
}

// MARK: actions

func onTap(t float64) {
	logf("tap (prompt visible=%v, %.0f ms since last tap)", prompt.visible, (t-lastTap)*1000)
	if t-lastTap < logic.DoubleTapWindow { // double tap: first tap opened the prompt, so close it and clear
		lastTap = 0
		hidePrompt()
		clearClipboard()
		messageBeep(0) // MB_OK
		flash("trash", "Clipboard cleared")
		return
	}
	lastTap = t
	if prompt.visible {
		hidePrompt()
	} else {
		showPrompt()
	}
}

func modelName() string { return strings.ToUpper(cfg.Model[:1]) + cfg.Model[1:] }

func runPrompt(text string) {
	logf("run prompt (%d chars)", len([]rune(text)))
	if task != nil {
		task.kill() // a new prompt replaces a running one
	}
	taskID++
	task = startJob(taskID, text, cfg.Model)
	killTimer(mainHwnd, timerRevert)
	infoText = "Working with " + modelName() + "…"
	setIcon("hourglass")
}

func handleDone() {
	for _, r := range takeResults() {
		logf("claude finished: id=%d current=%d code=%d stdout=%d bytes stderr=%q", r.id, taskID, r.code, len(r.out), r.err)
		if r.id != taskID { // replaced by a newer prompt
			continue
		}
		task = nil
		code := logic.StripFences(r.out)
		if r.code != 0 || code == "" {
			msg := r.err
			if strings.TrimSpace(msg) == "" {
				msg = r.out
			}
			if strings.TrimSpace(msg) == "" {
				msg = fmt.Sprintf("claude exited with code %d", r.code)
			}
			fail(msg)
			continue
		}
		if err := setClipboardText(code); err != nil {
			fail(err.Error())
			continue
		}
		messageBeep(0x40) // MB_ICONASTERISK
		flash("checkmark.circle", fmt.Sprintf("Copied %d lines", strings.Count(code, "\n")+1))
	}
}
