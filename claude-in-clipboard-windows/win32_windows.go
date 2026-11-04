//go:build windows

package main

// Thin, dependency-free Win32 bindings (standard library only).

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassExW           = user32.NewProc("RegisterClassExW")
	pCreateWindowExW            = user32.NewProc("CreateWindowExW")
	pDefWindowProcW             = user32.NewProc("DefWindowProcW")
	pDestroyWindow              = user32.NewProc("DestroyWindow")
	pGetMessageW                = user32.NewProc("GetMessageW")
	pTranslateMessage           = user32.NewProc("TranslateMessage")
	pDispatchMessageW           = user32.NewProc("DispatchMessageW")
	pPostMessageW               = user32.NewProc("PostMessageW")
	pPostQuitMessage            = user32.NewProc("PostQuitMessage")
	pShowWindow                 = user32.NewProc("ShowWindow")
	pSetWindowPos               = user32.NewProc("SetWindowPos")
	pIsWindowVisible            = user32.NewProc("IsWindowVisible")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pSetWindowRgn               = user32.NewProc("SetWindowRgn")
	pInvalidateRect             = user32.NewProc("InvalidateRect")
	pBeginPaint                 = user32.NewProc("BeginPaint")
	pEndPaint                   = user32.NewProc("EndPaint")
	pGetClientRect              = user32.NewProc("GetClientRect")
	pFillRect                   = user32.NewProc("FillRect")
	pSetTimer                   = user32.NewProc("SetTimer")
	pKillTimer                  = user32.NewProc("KillTimer")
	pGetCursorPos               = user32.NewProc("GetCursorPos")
	pMonitorFromPoint           = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
	pLoadCursorW                = user32.NewProc("LoadCursorW")
	pSetWindowsHookExW          = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx        = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx             = user32.NewProc("CallNextHookEx")
	pGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	pGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	pSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop           = user32.NewProc("BringWindowToTop")
	pSetFocus                   = user32.NewProc("SetFocus")
	pAllowSetForegroundWindow   = user32.NewProc("AllowSetForegroundWindow")
	pGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	pAttachThreadInput          = user32.NewProc("AttachThreadInput")
	pEnumWindows                = user32.NewProc("EnumWindows")
	pGetWindowTextW             = user32.NewProc("GetWindowTextW")
	pGetClassNameW              = user32.NewProc("GetClassNameW")
	pCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	pAppendMenuW                = user32.NewProc("AppendMenuW")
	pTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                = user32.NewProc("DestroyMenu")
	pDestroyIcon                = user32.NewProc("DestroyIcon")
	pCreateIconIndirect         = user32.NewProc("CreateIconIndirect")
	pGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	pGetDC                      = user32.NewProc("GetDC")
	pReleaseDC                  = user32.NewProc("ReleaseDC")
	pMessageBeep                = user32.NewProc("MessageBeep")
	pRegisterWindowMessageW     = user32.NewProc("RegisterWindowMessageW")
	pSetProcessDpiAwarenessCtx  = user32.NewProc("SetProcessDpiAwarenessContext")
	pOpenClipboard              = user32.NewProc("OpenClipboard")
	pCloseClipboard             = user32.NewProc("CloseClipboard")
	pEmptyClipboard             = user32.NewProc("EmptyClipboard")
	pGetClipboardData           = user32.NewProc("GetClipboardData")
	pSetClipboardData           = user32.NewProc("SetClipboardData")
	pIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	pRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")

	pGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	pGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	pGetCurrentProcessId      = kernel32.NewProc("GetCurrentProcessId")
	pCreateMutexW             = kernel32.NewProc("CreateMutexW")
	pOpenProcess              = kernel32.NewProc("OpenProcess")
	pCloseHandle              = kernel32.NewProc("CloseHandle")
	pQueryFullProcessImageNam = kernel32.NewProc("QueryFullProcessImageNameW")
	pGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	pGlobalLock               = kernel32.NewProc("GlobalLock")
	pGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	pGlobalFree               = kernel32.NewProc("GlobalFree")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	pCreateBitmap           = gdi32.NewProc("CreateBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreateRoundRectRgn     = gdi32.NewProc("CreateRoundRectRgn")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pTextOutW               = gdi32.NewProc("TextOutW")
	pGetTextMetricsW        = gdi32.NewProc("GetTextMetricsW")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	pGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	pIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	pSelectClipRgn          = gdi32.NewProc("SelectClipRgn")
	pGetStockObject         = gdi32.NewProc("GetStockObject")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	pRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW  = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey     = advapi32.NewProc("RegCloseKey")
	pRegGetValueW    = advapi32.NewProc("RegGetValueW")
)

// Window messages and styles.
const (
	wmDestroy      = 0x0002
	wmActivate     = 0x0006
	wmPaint        = 0x000F
	wmEraseBkgnd   = 0x0014
	wmSettingChg   = 0x001A
	wmChar         = 0x0102
	wmTimer        = 0x0113
	wmCommand      = 0x0111
	wmMouseWheel   = 0x020A
	wmMouseHWheel  = 0x020E
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmNull         = 0x0000
	wmKeyDown      = 0x0100
	wmKeyUp        = 0x0101
	wmSysKeyDown   = 0x0104
	wmSysKeyUp     = 0x0105
	wmApp          = 0x8000
	waInactive     = 0
	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008
	wsExNoActivate = 0x08000000
	wsExLayered    = 0x00080000
	wsExTranspar   = 0x00000020
	swHide         = 0
	swShow         = 5
	swShowNoActiv  = 4
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoActivate  = 0x0010
	swpShowWindow  = 0x0040
	hwndTopmost    = ^uintptr(0) // (HWND)-1
	lwaAlpha       = 0x00000002
	whKeyboardLL   = 13
	idcArrow       = 32512
	spiNone        = 0
)

// Menu flags.
const (
	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfDisabled  = 0x0002
	mfChecked   = 0x0008
	mfPopup     = 0x0010
	mfSeparator = 0x0800

	tpmRightButton  = 0x0002
	tpmBottomAlign  = 0x0020
	tpmLeftAlign    = 0x0000
	smCxSmIcon      = 49
	cfUnicodeText   = 13
	gmemMoveable    = 0x0002
	transparentMode = 1
)

// Virtual keys.
const (
	vkCapital  = 0x14
	vkRControl = 0xA3
	vkRShift   = 0xA1
)

type point struct{ x, y int32 }
type rect struct{ left, top, right, bottom int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

type kbdLLHook struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type size struct{ cx, cy int32 }

// textMetric mirrors the leading fields of TEXTMETRICW; the tail is padding so the OS can write the full struct.
type textMetric struct {
	height, ascent, descent, internalLeading, externalLeading int32
	aveCharWidth, maxCharWidth                                int32
	rest                                                      [12]int32
}

// notifyIconData mirrors NOTIFYICONDATAW (Vista+ layout).
type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeout         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func ptr(p unsafe.Pointer) uintptr { return uintptr(p) }

// fromPtr converts an OS-owned address (e.g. a hook's lParam) to a pointer without tripping vet's unsafeptr check.
func fromPtr(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

func loword(v uintptr) uint16 { return uint16(v & 0xFFFF) }
func hiword(v uintptr) uint16 { return uint16((v >> 16) & 0xFFFF) }

func boolInt(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

func defWindowProc(hwnd, m, wp, lp uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(hwnd, m, wp, lp)
	return r
}

func postMessage(hwnd uintptr, m uint32, wp, lp uintptr) {
	pPostMessageW.Call(hwnd, uintptr(m), wp, lp)
}

func setTimer(hwnd uintptr, id uintptr, ms uint32) { pSetTimer.Call(hwnd, id, uintptr(ms), 0) }
func killTimer(hwnd uintptr, id uintptr)           { pKillTimer.Call(hwnd, id) }

func isVisible(hwnd uintptr) bool {
	r, _, _ := pIsWindowVisible.Call(hwnd)
	return r != 0
}

func cursorPos() point {
	var p point
	pGetCursorPos.Call(ptr(unsafe.Pointer(&p)))
	return p
}

// workAreaAt returns the usable area of the monitor containing p.
func workAreaAt(p point) rect {
	// MonitorFromPoint takes the POINT by value; on 64-bit it travels packed in one register.
	packed := uintptr(uint32(p.x)) | uintptr(uint32(p.y))<<32
	hmon, _, _ := pMonitorFromPoint.Call(packed, 2 /* MONITOR_DEFAULTTONEAREST */)
	mi := monitorInfo{cbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if hmon != 0 {
		if r, _, _ := pGetMonitorInfoW.Call(hmon, ptr(unsafe.Pointer(&mi))); r != 0 {
			return mi.rcWork
		}
	}
	return rect{0, 0, 1920, 1080}
}

func messageBeep(kind uintptr) { pMessageBeep.Call(kind) }

func getModuleHandle() uintptr {
	h, _, _ := pGetModuleHandleW.Call(0)
	return h
}

func systemDPI() int {
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 96
	}
	defer pReleaseDC.Call(0, hdc)
	dpi, _, _ := pGetDeviceCaps.Call(hdc, 90 /* LOGPIXELSY */)
	if dpi == 0 {
		return 96
	}
	return int(dpi)
}

func mulDiv(a, b, c int) int { return (a*b + c/2) / c }
