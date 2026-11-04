//go:build windows && (amd64 || arm64)

package main

import "unsafe"

// Compile-time checks that our Go structs match the Win32 layouts on 64-bit Windows.
// Each pair of array types only compiles when the size is exactly the expected one.
var (
	_ [976 - unsafe.Sizeof(notifyIconData{})]byte // NOTIFYICONDATAW
	_ [unsafe.Sizeof(notifyIconData{}) - 976]byte
	_ [80 - unsafe.Sizeof(wndClassEx{})]byte // WNDCLASSEXW
	_ [unsafe.Sizeof(wndClassEx{}) - 80]byte
	_ [48 - unsafe.Sizeof(msg{})]byte // MSG
	_ [unsafe.Sizeof(msg{}) - 48]byte
	_ [72 - unsafe.Sizeof(paintStruct{})]byte // PAINTSTRUCT
	_ [unsafe.Sizeof(paintStruct{}) - 72]byte
	_ [40 - unsafe.Sizeof(monitorInfo{})]byte // MONITORINFO
	_ [unsafe.Sizeof(monitorInfo{}) - 40]byte
	_ [24 - unsafe.Sizeof(kbdLLHook{})]byte // KBDLLHOOKSTRUCT
	_ [unsafe.Sizeof(kbdLLHook{}) - 24]byte
	_ [32 - unsafe.Sizeof(iconInfo{})]byte // ICONINFO
	_ [unsafe.Sizeof(iconInfo{}) - 32]byte
	_ [40 - unsafe.Sizeof(bitmapInfoHeader{})]byte // BITMAPINFOHEADER
	_ [unsafe.Sizeof(bitmapInfoHeader{}) - 40]byte
)
