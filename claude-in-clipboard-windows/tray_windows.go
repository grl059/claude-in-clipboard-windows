//go:build windows

package main

import (
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"claudeclip/internal/logic"
)

const (
	trayUID = 1

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2
	nifMsg    = 0x1
	nifIcon   = 0x2
	nifTip    = 0x4

	idModelBase   = 100 // 100 + index into models
	idTriggerBase = 110 // 110 + index into triggerKeys
	idAutostart   = 120
	idQuit        = 199
)

var models = []struct{ alias, label string }{
	{"opus", "Opus (latest)"}, {"sonnet", "Sonnet (latest)"},
}

var (
	currentIcon   = "sparkles"
	lastLightMode bool
)

func trayNotify(op uintptr) {
	nid := notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             mainHwnd,
		uID:              trayUID,
		uFlags:           nifMsg | nifIcon | nifTip,
		uCallbackMessage: wmTray,
		hIcon:            trayIcon(currentIcon),
	}
	tip := "ClaudeClip: " + infoText
	if u, err := syscall.UTF16FromString(tip); err == nil {
		if len(u) > len(nid.szTip) {
			u = append(u[:len(nid.szTip)-1], 0)
		}
		copy(nid.szTip[:], u)
	}
	pShellNotifyIconW.Call(op, ptr(unsafe.Pointer(&nid)))
}

func setIcon(name string) {
	currentIcon = name
	trayNotify(nimModify)
}

func setInfo(text string) {
	infoText = text
	trayNotify(nimModify)
}

func idleIcon() string {
	if task != nil {
		return "hourglass"
	}
	return "sparkles"
}

// flash shows an icon and message briefly, then goes back to the idle icon.
func flash(icon, text string) {
	infoText = text
	setIcon(icon)
	killTimer(mainHwnd, timerRevert)
	setTimer(mainHwnd, timerRevert, 2500)
}

func fail(msg string) {
	logf("fail: %s", msg)
	task = nil
	messageBeep(0x10) // MB_ICONHAND
	killTimer(mainHwnd, timerRevert)
	msg = strings.Join(strings.Fields(msg), " ")
	if r := []rune(msg); len(r) > 150 {
		msg = string(r[:150])
	}
	infoText = "Error: " + msg
	setIcon("exclamationmark.triangle")
}

func appendMenu(menu uintptr, flags uintptr, id uintptr, text string) {
	p := u16(text)
	pAppendMenuW.Call(menu, flags, id, ptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}

// showMenu pops up the tray menu at the pointer (the equivalent of clicking the menu bar icon on macOS).
func showMenu() {
	t := targetNow(250 * time.Millisecond)
	refreshAccount() // async: the menu shows the last known account until this returns

	menu, _, _ := pCreatePopupMenu.Call()
	defer pDestroyMenu.Call(menu)
	const off = mfString | mfGrayed | mfDisabled
	appendMenu(menu, off, 0, infoText)
	appendMenu(menu, mfSeparator, 0, "")
	switch {
	case !t.found():
		appendMenu(menu, off, 0, "No VS Code project or Notepad window found")
	case t.kind == "notepad" && t.file == "":
		appendMenu(menu, off, 0, "Notepad: "+t.name)
		appendMenu(menu, off, 0, "    (file location unknown: using a scratch folder)")
	default:
		appendMenu(menu, off, 0, t.app()+": "+t.name)
		appendMenu(menu, off, 0, "    "+logic.ShortPath(t.dir))
	}
	appendMenu(menu, off, 0, accountText())
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, off, 0, "Model")
	for i, m := range models {
		fl := uintptr(mfString)
		if cfg.Model == m.alias {
			fl |= mfChecked
		}
		appendMenu(menu, fl, uintptr(idModelBase+i), m.label)
	}
	appendMenu(menu, mfSeparator, 0, "")

	sub, _, _ := pCreatePopupMenu.Call() // destroyed together with its parent menu
	for i, t := range triggerKeys {
		fl := uintptr(mfString)
		if cfg.Trigger == t.id {
			fl |= mfChecked
		}
		appendMenu(sub, fl, uintptr(idTriggerBase+i), t.label)
	}
	appendMenu(menu, mfString|mfPopup, sub, "Trigger key")
	auto := uintptr(mfString)
	if autostartEnabled() {
		auto |= mfChecked
	}
	appendMenu(menu, auto, idAutostart, "Start with Windows")
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, idQuit, "Quit ClaudeClip")

	pt := cursorPos()
	pSetForegroundWindow.Call(mainHwnd) // required, or the menu won't close when you click elsewhere
	pTrackPopupMenu.Call(menu, tpmRightButton|tpmBottomAlign|tpmLeftAlign, uintptr(int32(pt.x)), uintptr(int32(pt.y)), 0, mainHwnd, 0)
	postMessage(mainHwnd, wmNull, 0, 0)
}

func onCommand(id int) {
	switch {
	case id >= idModelBase && id < idModelBase+len(models):
		cfg.Model = models[id-idModelBase].alias
		saveSettings(cfg)
	case id >= idTriggerBase && id < idTriggerBase+len(triggerKeys):
		cfg.Trigger = triggerKeys[id-idTriggerBase].id
		saveSettings(cfg)
		triggerDown = false
		tap.OtherKey()
		hideGhost()
		if task == nil && !prompt.visible {
			setInfo(readyText())
		}
	case id == idAutostart:
		setAutostart(!autostartEnabled())
	case id == idQuit:
		pDestroyWindow.Call(mainHwnd)
	}
}

func readyText() string { return "Ready · tap " + currentTrigger(cfg).label }
