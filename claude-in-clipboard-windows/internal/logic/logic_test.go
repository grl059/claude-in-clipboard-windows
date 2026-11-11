package logic

import (
	"strings"
	"testing"
)

func TestStripFences(t *testing.T) {
	cases := map[string]string{
		"```tsx\nconst a = 1\n```":      "const a = 1",
		"const a = 1\n":                 "const a = 1",
		"```\r\nx\r\n```\r\n":           "x",
		"```":                           "",
		"plain\ntext":                   "plain\ntext",
		"  \n```go\nfunc main(){}\n```": "func main(){}",
	}
	for in, want := range cases {
		if got := StripFences(in); got != want {
			t.Errorf("StripFences(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickProject(t *testing.T) {
	api, app := `C:\p\api`, `C:\p\my-app`
	if got := PickProject("● App.tsx - my-app - Visual Studio Code", []string{api, app}); got != app {
		t.Errorf("hyphen title: got %q", got)
	}
	if got := PickProject("● App.tsx — my-app", []string{api, app}); got != app {
		t.Errorf("em dash title: got %q", got)
	}
	// no exact match -> newest
	if got := PickProject("App.tsx - my-app-api - Visual Studio Code", []string{api, app}); got != api {
		t.Errorf("no match: got %q", got)
	}
	// Windows folder names are case-insensitive
	if got := PickProject("a.go - MY-APP", []string{api, app}); got != app {
		t.Errorf("case: got %q", got)
	}
	if got := PickProject("", nil); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestFolderName(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Users\me\proj`: "proj", `C:\Users\me\proj\`: "proj", "/home/x/y": "y", "solo": "solo",
	} {
		if got := FolderName(in); got != want {
			t.Errorf("FolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFileURIToPath(t *testing.T) {
	good := map[string]string{
		"file:///c%3A/Users/me/my-app":      `c:\Users\me\my-app`,
		"file:///C:/Users/me/My%20App":      `C:\Users\me\My App`,
		"file://server/share/proj":          `\\server\share\proj`,
		"file://localhost/d%3A/code/x":      `d:\code\x`,
		"file:///c%3A/Users/%E3%83%86/proj": `c:\Users\テ\proj`,
	}
	for in, want := range good {
		if got, ok := FileURIToPath(in); !ok || got != want {
			t.Errorf("FileURIToPath(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"vscode-remote://wsl%2Bubuntu/home/x", "vscode-vfs://github/a/b", "", "not a uri", "file:///"} {
		if got, ok := FileURIToPath(bad); ok {
			t.Errorf("FileURIToPath(%q) should be rejected, got %q", bad, got)
		}
	}
}

func TestGhostPreview(t *testing.T) {
	if GhostPreview("", 2000, 300) != "No text on clipboard" || GhostPreview("  \n", 2000, 300) != "No text on clipboard" {
		t.Error("empty clipboard text")
	}
	if GhostPreview("a\nb", 2000, 300) != "a\nb" {
		t.Error("passthrough")
	}
	if got := GhostPreview("1\n2\n3\n4", 2, 300); got != "1\n2\n… 2 more lines" {
		t.Errorf("line cap: %q", got)
	}
	if got := GhostPreview("abcdef", 2000, 3); got != "abc" {
		t.Errorf("col cap: %q", got)
	}
	if got := GhostPreview("тест", 2000, 2); got != "те" { // cut on runes, not bytes
		t.Errorf("rune cap: %q", got)
	}
}

func TestExpandTabs(t *testing.T) {
	if got := ExpandTabs("\ta", 4); got != "    a" {
		t.Errorf("leading tab: %q", got)
	}
	if got := ExpandTabs("ab\tc\n\td", 4); got != "ab  c\n    d" {
		t.Errorf("mid tab / newline reset: %q", got)
	}
	if got := ExpandTabs("none", 4); got != "none" {
		t.Errorf("no tabs: %q", got)
	}
}

func TestAccountLabel(t *testing.T) {
	if got := AccountLabel(map[string]any{"loggedIn": true, "email": "a@b.c", "subscriptionType": "max"}); got != "Claude: a@b.c · Max" {
		t.Errorf("logged in: %q", got)
	}
	if got := AccountLabel(map[string]any{"loggedIn": true}); got != "Claude: logged in" {
		t.Errorf("no email: %q", got)
	}
	if !strings.HasPrefix(AccountLabel(map[string]any{"loggedIn": false}), "Claude: not logged in") {
		t.Error("logged out")
	}
	if !strings.HasPrefix(AccountLabel(nil), "Claude: unknown") {
		t.Error("nil")
	}
}

func TestTapDetector(t *testing.T) {
	var d TapDetector
	if d.Key(true, 0) || !d.Key(false, 0.1) { // quick tap
		t.Error("quick tap should be a tap")
	}
	if d.Key(true, 1) || d.Key(false, 1.6) { // held too long
		t.Error("long hold is not a tap")
	}
	d.Key(true, 2)
	d.OtherKey()
	if d.Key(false, 2.1) { // chord with another key
		t.Error("chord is not a tap")
	}
	if d.Key(false, 3) { // release with no press seen
		t.Error("stray release is not a tap")
	}
}

func TestNormalizeNewlines(t *testing.T) {
	if got := NormalizeNewlines("a\r\nb\rc\n"); got != "a\nb\nc\n" {
		t.Errorf("got %q", got)
	}
}

// The prompt is written to a file for claude, but keep it quote-free in case it is ever passed as an argument.
func TestSystemPromptIsPlain(t *testing.T) {
	if strings.ContainsAny(SystemPrompt, "\"\n\r&|<>^%") {
		t.Error("system prompt contains characters that are awkward in Windows command lines")
	}
}

func TestNotepadFile(t *testing.T) {
	good := map[string]string{
		"notes.txt - Notepad":      "notes.txt",
		"*notes.txt - Notepad":     "notes.txt",
		"my - file.py - Notepad":   "my - file.py",
		"todo.md - Блокнот":        "todo.md",
		"report (1).txt - Notepad": "report (1).txt",
		"notes.txt":                "notes.txt",
	}
	for in, want := range good {
		if got, ok := NotepadFile(in); !ok || got != want {
			t.Errorf("NotepadFile(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"Untitled - Notepad", "*Untitled - Notepad", "", " - Notepad"} {
		if got, ok := NotepadFile(in); ok {
			t.Errorf("NotepadFile(%q) should be rejected, got %q", in, got)
		}
	}
}

// buildLnk crafts a minimal .lnk: header, optional ID list, LinkInfo with a local base path.
func buildLnk(withIDList bool, unicode bool, path string) []byte {
	put16 := func(b []byte, v int) []byte { return append(b, byte(v), byte(v>>8)) }
	put32 := func(b []byte, v int) []byte { return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	hdr := make([]byte, 0x4C)
	copy(hdr, []byte{0x4C, 0, 0, 0})
	flags := 0x2
	if withIDList {
		flags |= 0x1
	}
	hdr[0x14] = byte(flags)
	b := hdr
	if withIDList {
		b = put16(b, 4)
		b = append(b, 1, 2, 3, 4) // dummy ID list bytes
	}
	// LinkInfo
	headerSize := 0x1C
	if unicode {
		headerSize = 0x24
	}
	var base []byte
	if unicode {
		for _, r := range path {
			base = put16(base, int(r))
		}
		base = put16(base, 0)
	} else {
		base = append([]byte(path), 0)
	}
	pathOff := headerSize
	size := headerSize + len(base) + 1
	li := []byte{}
	li = put32(li, size)
	li = put32(li, headerSize)
	li = put32(li, 1) // VolumeIDAndLocalBasePath
	li = put32(li, 0) // VolumeIDOffset
	if unicode {
		li = put32(li, 0) // ANSI base path absent
	} else {
		li = put32(li, pathOff)
	}
	li = put32(li, 0)      // network link
	li = put32(li, size-1) // suffix offset -> trailing NUL (empty string)
	if unicode {
		li = put32(li, pathOff) // unicode base path
		li = put32(li, size-1)  // unicode suffix (points at a zero byte; one byte only, so it reads as empty)
	}
	li = append(li, base...)
	li = append(li, 0)
	return append(b, li...)
}

func TestParseLnkTarget(t *testing.T) {
	for _, c := range []struct {
		idList, uni bool
		path        string
	}{
		{false, false, `C:\Users\me\Desktop\notes.txt`},
		{true, false, `D:\work\a b\x.py`},
		{true, true, `C:\Users\me\Документы\тэмдэглэл.txt`},
	} {
		got, ok := ParseLnkTarget(buildLnk(c.idList, c.uni, c.path))
		if !ok || got != c.path {
			t.Errorf("ParseLnkTarget(idList=%v unicode=%v) = %q,%v want %q", c.idList, c.uni, got, ok, c.path)
		}
	}
	for _, bad := range [][]byte{nil, []byte("short"), make([]byte, 100)} {
		if got, ok := ParseLnkTarget(bad); ok {
			t.Errorf("garbage accepted: %q", got)
		}
	}
	// truncated input must not panic
	full := buildLnk(true, true, `C:\a\b.txt`)
	for n := 0; n < len(full); n++ {
		ParseLnkTarget(full[:n])
	}
}

func TestDirOf(t *testing.T) {
	for in, want := range map[string]string{`C:\a\b.txt`: `C:\a`, "/x/y": "/x", "plain": ""} {
		if got := DirOf(in); got != want {
			t.Errorf("DirOf(%q) = %q want %q", in, got, want)
		}
	}
}

func TestNotepadPromptIsPlain(t *testing.T) {
	if strings.ContainsAny(NotepadSystemPrompt, "\"\n\r&|<>^%") {
		t.Error("notepad system prompt contains awkward characters")
	}
}
