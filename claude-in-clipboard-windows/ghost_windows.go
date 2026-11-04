//go:build windows

package main

// The hold-key clipboard "ghost": a translucent, scrollable, never-focused overlay under the pointer.

import (
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"claudeclip/internal/logic"
)

const (
	ghostMaxLines = 2000
	ghostMaxCols  = 300
	ghostAlpha    = 225 // 0-255: lower = fainter
)

type ghostWindow struct {
	hwnd             uintptr
	font             uintptr
	lines            []string
	lineH, charW     int
	contentW         int
	contentH         int
	viewW, viewH     int
	pad              int
	winW, winH       int
	scrollX, scrollY float64
	visible          bool
}

var gh ghostWindow

func createGhost() {
	class := u16("ClaudeClipGhost")
	registerClass(class, ghostProc)
	h, _, _ := pCreateWindowExW.Call(wsExTopmost|wsExToolWindow|wsExNoActivate|wsExLayered,
		ptr(unsafe.Pointer(class)), ptr(unsafe.Pointer(u16("ClaudeClip clipboard preview"))), wsPopup,
		0, 0, 10, 10, 0, 0, getModuleHandle(), 0)
	gh.hwnd = h
	pSetLayeredWindowAttributes.Call(h, 0, ghostAlpha, lwaAlpha)
	// 10pt Consolas, scaled to the system DPI.
	height := -mulDiv(10, systemDPI(), 72)
	f, _, _ := pCreateFontW.Call(uintptr(int32(height)), 0, 0, 0, 400, 0, 0, 0, 1 /* DEFAULT_CHARSET */, 0, 0,
		5 /* CLEARTYPE_QUALITY */, 0x31 /* FIXED_PITCH | FF_MODERN */, ptr(unsafe.Pointer(u16("Consolas"))))
	gh.font = f
}

func scheduleGhost() {
	killTimer(mainHwnd, timerGhostShow)
	ghostPending = true
	setTimer(mainHwnd, timerGhostShow, uint32(logic.HoldTime*1000))
}

func showGhost() {
	var text string
	if clipboardConcealed() { // password managers tag copied secrets
		text = "Hidden: password"
	} else {
		text = logic.GhostPreview(clipboardText(), ghostMaxLines, ghostMaxCols)
	}
	gh.lines = strings.Split(logic.ExpandTabs(text, 4), "\n")

	// Measure with the preview font.
	hdc, _, _ := pGetDC.Call(0)
	old, _, _ := pSelectObject.Call(hdc, gh.font)
	var tm textMetric
	pGetTextMetricsW.Call(hdc, ptr(unsafe.Pointer(&tm)))
	gh.lineH, gh.charW = int(tm.height), int(tm.aveCharWidth)
	if gh.lineH < 1 {
		gh.lineH = 14
	}
	longest := ""
	for _, l := range gh.lines {
		if len(l) > len(longest) { // byte length is a fine proxy for the widest line of mostly-ASCII code
			longest = l
		}
	}
	gh.contentW = 0
	if w := utf16.Encode([]rune(longest)); len(w) > 0 {
		var sz size
		pGetTextExtentPoint32W.Call(hdc, ptr(unsafe.Pointer(&w[0])), uintptr(len(w)), ptr(unsafe.Pointer(&sz)))
		gh.contentW = int(sz.cx)
	}
	pSelectObject.Call(hdc, old)
	pReleaseDC.Call(0, hdc)
	gh.contentH = len(gh.lines) * gh.lineH

	// Centered on the pointer (kept on screen) so scrolling lands on the preview, not the app behind it.
	gh.pad = mulDiv(16, systemDPI(), 96)
	mouse := cursorPos()
	area := workAreaAt(mouse)
	maxW := int(area.right-area.left)*9/10 - gh.pad*2
	maxH := int(area.bottom-area.top)*9/10 - gh.pad*2
	gh.viewW, gh.viewH = min(max(gh.contentW, 1), maxW), min(max(gh.contentH, 1), maxH)
	gh.winW, gh.winH = gh.viewW+gh.pad*2, gh.viewH+gh.pad*2
	x := min(max(int(mouse.x)-gh.winW/2, int(area.left)), int(area.right)-gh.winW)
	y := min(max(int(mouse.y)-gh.winH/2, int(area.top)), int(area.bottom)-gh.winH)
	gh.scrollX, gh.scrollY = 0, 0 // start at the top

	pSetWindowPos.Call(gh.hwnd, hwndTopmost, uintptr(int32(x)), uintptr(int32(y)), uintptr(gh.winW), uintptr(gh.winH),
		swpNoActivate|swpShowWindow)
	r := mulDiv(12, systemDPI(), 96) * 2
	rgn, _, _ := pCreateRoundRectRgn.Call(0, 0, uintptr(gh.winW+1), uintptr(gh.winH+1), uintptr(r), uintptr(r))
	pSetWindowRgn.Call(gh.hwnd, rgn, 1) // the window owns the region now
	pInvalidateRect.Call(gh.hwnd, 0, 0)
	gh.visible = true
	setTimer(mainHwnd, timerGhostWatch, 150) // key-up can be missed (e.g. UAC prompt), so poll the real key state
}

func hideGhost() {
	if !ghostPending && !gh.visible {
		return
	}
	killTimer(mainHwnd, timerGhostShow)
	killTimer(mainHwnd, timerGhostWatch)
	ghostPending = false
	if gh.visible {
		gh.visible = false
		pShowWindow.Call(gh.hwnd, swHide)
	}
}

func ghostScrollBy(dx, dy float64) {
	maxX := float64(max(gh.contentW-gh.viewW, 0))
	maxY := float64(max(gh.contentH-gh.viewH, 0))
	gh.scrollX = min(max(gh.scrollX+dx, 0), maxX)
	gh.scrollY = min(max(gh.scrollY+dy, 0), maxY)
	pInvalidateRect.Call(gh.hwnd, 0, 0)
}

func ghostProc(hwnd, m, wp, lp uintptr) uintptr {
	switch uint32(m) {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		paintGhost(hwnd)
		return 0
	case 0x0021: // WM_MOUSEACTIVATE: never take focus from the app being edited
		return 3 // MA_NOACTIVATE
	case wmMouseWheel:
		notches := float64(int16(hiword(wp))) / 120
		if loword(wp)&0x0004 != 0 && currentTrigger(cfg).vk != vkRShift { // MK_SHIFT: sideways
			ghostScrollBy(-notches*3*float64(gh.charW), 0)
		} else {
			ghostScrollBy(0, -notches*3*float64(gh.lineH))
		}
		return 0
	case wmMouseHWheel:
		ghostScrollBy(float64(int16(hiword(wp)))/120*3*float64(gh.charW), 0)
		return 0
	}
	return defWindowProc(hwnd, m, wp, lp)
}

func paintGhost(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, ptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(hwnd, ptr(unsafe.Pointer(&ps)))
	if hdc == 0 || gh.winW == 0 {
		return
	}
	// Draw into a memory bitmap first so scrolling doesn't flicker.
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(gh.winW), uintptr(gh.winH))
	oldBmp, _, _ := pSelectObject.Call(mem, bmp)
	rc := rect{0, 0, int32(gh.winW), int32(gh.winH)}
	black, _, _ := pGetStockObject.Call(4 /* BLACK_BRUSH */)
	pFillRect.Call(mem, ptr(unsafe.Pointer(&rc)), black)

	oldFont, _, _ := pSelectObject.Call(mem, gh.font)
	pSetBkMode.Call(mem, transparentMode)
	pSetTextColor.Call(mem, 0x00FFFFFF)
	pIntersectClipRect.Call(mem, uintptr(gh.pad), uintptr(gh.pad), uintptr(gh.pad+gh.viewW), uintptr(gh.pad+gh.viewH))
	sy, sx := int(gh.scrollY), int(gh.scrollX)
	first := sy / gh.lineH
	last := min(len(gh.lines), (sy+gh.viewH)/gh.lineH+1)
	for i := first; i < last; i++ {
		if gh.lines[i] == "" {
			continue
		}
		w := utf16.Encode([]rune(gh.lines[i]))
		pTextOutW.Call(mem, uintptr(int32(gh.pad-sx)), uintptr(int32(gh.pad+i*gh.lineH-sy)),
			ptr(unsafe.Pointer(&w[0])), uintptr(len(w)))
	}
	pSelectClipRgn.Call(mem, 0)
	pSelectObject.Call(mem, oldFont)

	// Thin overlay scroll indicators, like the macOS overlay scrollers.
	grey, _, _ := pCreateSolidBrush.Call(0x00909090)
	const thick = 4
	if gh.contentH > gh.viewH {
		th := max(gh.viewH*gh.viewH/gh.contentH, 20)
		ty := gh.pad + int(gh.scrollY/float64(gh.contentH-gh.viewH)*float64(gh.viewH-th))
		r := rect{int32(gh.winW - thick - 4), int32(ty), int32(gh.winW - 4), int32(ty + th)}
		pFillRect.Call(mem, ptr(unsafe.Pointer(&r)), grey)
	}
	if gh.contentW > gh.viewW {
		tw := max(gh.viewW*gh.viewW/gh.contentW, 20)
		tx := gh.pad + int(gh.scrollX/float64(gh.contentW-gh.viewW)*float64(gh.viewW-tw))
		r := rect{int32(tx), int32(gh.winH - thick - 4), int32(tx + tw), int32(gh.winH - 4)}
		pFillRect.Call(mem, ptr(unsafe.Pointer(&r)), grey)
	}
	pDeleteObject.Call(grey)

	pBitBlt.Call(hdc, 0, 0, uintptr(gh.winW), uintptr(gh.winH), mem, 0, 0, 0x00CC0020 /* SRCCOPY */)
	pSelectObject.Call(mem, oldBmp)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
}

func registerClass(name *uint16, proc func(hwnd, m, wp, lp uintptr) uintptr) {
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(proc),
		hInstance:     getModuleHandle(),
		hCursor:       cursor,
		lpszClassName: name,
	}
	pRegisterClassExW.Call(ptr(unsafe.Pointer(&wc)))
}
