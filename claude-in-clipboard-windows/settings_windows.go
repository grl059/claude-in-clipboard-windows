//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser = 0x80000001
	runKey          = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue        = "ClaudeClip"
)

// settings live in %APPDATA%\ClaudeClip\settings.json (the macOS app uses UserDefaults).
type settings struct {
	Model   string `json:"model"`   // "opus" or "sonnet": aliases Claude Code resolves to the newest version
	Trigger string `json:"trigger"` // "rctrl", "rshift" or "caps"
}

type triggerKey struct {
	id    string
	label string
	vk    uint32
}

var triggerKeys = []triggerKey{
	{"rctrl", "Right Ctrl", vkRControl},
	{"rshift", "Right Shift", vkRShift},
	{"caps", "Caps Lock", vkCapital},
}

func dataDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "ClaudeClip")
	os.MkdirAll(dir, 0o755)
	return dir
}

func loadSettings() settings {
	s := settings{Model: "opus", Trigger: "rctrl"}
	if b, err := os.ReadFile(filepath.Join(dataDir(), "settings.json")); err == nil {
		json.Unmarshal(b, &s)
	}
	if s.Model != "opus" && s.Model != "sonnet" {
		s.Model = "opus"
	}
	if currentTrigger(s).id != s.Trigger {
		s.Trigger = "rctrl"
	}
	return s
}

func saveSettings(s settings) {
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(filepath.Join(dataDir(), "settings.json"), b, 0o644)
	}
}

func currentTrigger(s settings) triggerKey {
	for _, t := range triggerKeys {
		if t.id == s.Trigger {
			return t
		}
	}
	return triggerKeys[0]
}

// MARK: registry helpers

func regDword(subkey, name string) (uint32, bool) {
	var data, n uint32 = 0, 4
	r, _, _ := pRegGetValueW.Call(hkeyCurrentUser, ptr(unsafe.Pointer(u16(subkey))), ptr(unsafe.Pointer(u16(name))),
		0x10 /* RRF_RT_REG_DWORD */, 0, ptr(unsafe.Pointer(&data)), ptr(unsafe.Pointer(&n)))
	return data, r == 0
}

func autostartEnabled() bool {
	var n uint32
	r, _, _ := pRegGetValueW.Call(hkeyCurrentUser, ptr(unsafe.Pointer(u16(runKey))), ptr(unsafe.Pointer(u16(runValue))),
		0x2 /* RRF_RT_REG_SZ */, 0, 0, ptr(unsafe.Pointer(&n)))
	return r == 0
}

func setAutostart(on bool) {
	var hk uintptr
	r, _, _ := pRegCreateKeyExW.Call(hkeyCurrentUser, ptr(unsafe.Pointer(u16(runKey))), 0, 0, 0,
		0x0002 /* KEY_SET_VALUE */, 0, ptr(unsafe.Pointer(&hk)), 0)
	if r != 0 {
		return
	}
	defer pRegCloseKey.Call(hk)
	if !on {
		pRegDeleteValueW.Call(hk, ptr(unsafe.Pointer(u16(runValue))))
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	val, _ := syscall.UTF16FromString(`"` + exe + `"`)
	pRegSetValueExW.Call(hk, ptr(unsafe.Pointer(u16(runValue))), 0, 1, /* REG_SZ */
		ptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2))
}
