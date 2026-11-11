//go:build windows

package main

// The invisible prompt box: a fully transparent, click-through window that takes keyboard focus.
// Type blind, Enter sends, Esc (or clicking elsewhere) cancels.

import (
	"strings"
	"unicode/utf16"
	"unsafe"
)

type promptWindow struct {
	hwnd    uintptr
	buf     []uint16
	visible bool
}

var prompt promptWindow

const (
	promptW = 400
	promptH = 24
)

func createPrompt() {
	class := u16("ClaudeClipPrompt")
	registerClass(class, promptProc)
	h, _, _ := pCreateWindowExW.Call(wsExTopmost|wsExToolWindow|wsExLayered|wsExTranspar,
		ptr(unsafe.Pointer(class)), ptr(unsafe.Pointer(u16("ClaudeClip prompt"))), wsPopup,
		0, 0, promptW, promptH, 0, 0, getModuleHandle(), 0)
	prompt.hwnd = h
	var alpha uintptr // invisible by design; --show-prompt (or CLAUDECLIP_DEBUG=1) makes it visible to debug
	if debugPrompt {
		alpha = 255
	}
	pSetLayeredWindowAttributes.Call(h, 0, alpha, lwaAlpha)
}

func showPrompt() {
	prompt.buf = nil
	area := workAreaAt(cursorPos())
	x := int(area.left+area.right)/2 - promptW/2
	y := int(area.top+area.bottom)/2 - promptH/2
	pSetWindowPos.Call(prompt.hwnd, hwndTopmost, uintptr(int32(x)), uintptr(int32(y)), promptW, promptH, swpShowWindow)
	ok := forceForeground(prompt.hwnd)
	logf("show prompt: got foreground=%v", ok)
	if !ok {
		pShowWindow.Call(prompt.hwnd, swHide)
		flash("exclamationmark.triangle", "Windows wouldn't give the prompt keyboard focus")
		return
	}
	prompt.visible = true
	setIcon("keyboard")
	pInvalidateRect.Call(prompt.hwnd, 0, 1)
}

func hidePrompt() {
	if !prompt.visible {
		return
	}
	prompt.visible = false
	prompt.buf = nil
	pShowWindow.Call(prompt.hwnd, swHide) // focus returns to the app that was in front
	setIcon(idleIcon())
}

// forceForeground makes hwnd the foreground window. Windows only lets the foreground app do that,
// so briefly share input state with it.
func forceForeground(hwnd uintptr) bool {
	fg, _, _ := pGetForegroundWindow.Call()
	var fgThread uintptr
	if fg != 0 {
		fgThread, _, _ = pGetWindowThreadProcessId.Call(fg, 0)
	}
	me, _, _ := pGetCurrentThreadId.Call()
	attached := false
	if fgThread != 0 && fgThread != me {
		r, _, _ := pAttachThreadInput.Call(me, fgThread, 1)
		attached = r != 0
	}
	pAllowSetForegroundWindow.Call(0xFFFFFFFF) // ASFW_ANY
	pSetForegroundWindow.Call(hwnd)
	pBringWindowToTop.Call(hwnd)
	pSetFocus.Call(hwnd)
	if attached {
		pAttachThreadInput.Call(me, fgThread, 0)
	}
	now, _, _ := pGetForegroundWindow.Call()
	return now == hwnd
}

func promptProc(hwnd, m, wp, lp uintptr) uintptr {
	defer guard("promptProc")
	switch uint32(m) {
	case wmChar:
		onPromptChar(uint16(wp))
		return 0
	case wmActivate:
		if loword(wp) == waInactive && prompt.visible { // clicked elsewhere: don't trap keystrokes
			hidePrompt()
		}
		return defWindowProc(hwnd, m, wp, lp) // default handling gives the window keyboard focus
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		paintPrompt(hwnd)
		return 0
	}
	return defWindowProc(hwnd, m, wp, lp)
}

func onPromptChar(c uint16) {
	switch {
	case c == 0x0D || c == 0x0A: // Enter
		text := strings.TrimSpace(string(utf16.Decode(prompt.buf)))
		hidePrompt()
		if text != "" {
			runPrompt(text)
		}
		return
	case c == 0x1B: // Esc
		hidePrompt()
		return
	case c == 0x08: // Backspace (drop a whole surrogate pair)
		if n := len(prompt.buf); n >= 2 && prompt.buf[n-1]&0xFC00 == 0xDC00 {
			prompt.buf = prompt.buf[:n-2]
		} else if n >= 1 {
			prompt.buf = prompt.buf[:n-1]
		}
	case c == 0x7F: // Ctrl+Backspace: delete the last word
		n := len(prompt.buf)
		for n > 0 && prompt.buf[n-1] == ' ' {
			n--
		}
		for n > 0 && prompt.buf[n-1] != ' ' {
			n--
		}
		prompt.buf = prompt.buf[:n]
	case c == 0x16: // Ctrl+V
		pasted := strings.Join(strings.Fields(clipboardText()), " ")
		prompt.buf = append(prompt.buf, utf16.Encode([]rune(pasted))...)
	case c < 0x20:
		return // other control characters
	default:
		prompt.buf = append(prompt.buf, c)
	}
	pInvalidateRect.Call(prompt.hwnd, 0, 1)
}

// paintPrompt only matters in debug mode, where the window is opaque and shows what you typed.
func paintPrompt(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, ptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, ptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	white, _, _ := pGetStockObject.Call(0 /* WHITE_BRUSH */)
	rc := rect{0, 0, promptW, promptH}
	pFillRect.Call(hdc, ptr(unsafe.Pointer(&rc)), white)
	if len(prompt.buf) > 0 {
		font, _, _ := pGetStockObject.Call(17 /* DEFAULT_GUI_FONT */)
		old, _, _ := pSelectObject.Call(hdc, font)
		pTextOutW.Call(hdc, 4, 4, ptr(unsafe.Pointer(&prompt.buf[0])), uintptr(len(prompt.buf)))
		pSelectObject.Call(hdc, old)
	}
}
