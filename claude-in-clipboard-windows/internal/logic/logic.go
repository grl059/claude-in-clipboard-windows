// Package logic holds the pure, platform-independent parts of ClaudeClip so they
// can be unit-tested on any OS (`go test ./...`).
package logic

import (
	"net/url"
	"os"
	"strings"
	"unicode/utf8"
)

// SystemPrompt is appended to Claude Code's own prompt. Keep it free of double quotes.
const SystemPrompt = "You run headless behind a clipboard hotkey. The user is coding in this project. " +
	"Read the codebase as needed (Read/Grep/Glob) so the code matches its existing style, imports and conventions. " +
	"Your final reply is copied straight to the clipboard and pasted into the editor, so reply with ONLY the code: " +
	"no explanation, no markdown fences, no preamble. If the answer spans several files, put a comment line with " +
	"the file path before each file's code. If the request is unclear, make the most reasonable assumption and still reply with code only."

// HoldTime: key released sooner = tap; held longer = clipboard ghost.
// Raise it if slow taps open the ghost instead of the prompt.
const HoldTime = 0.2

// DoubleTapWindow: two taps closer together than this clear the clipboard.
const DoubleTapWindow = 0.35

// GhostPreview is the text for the hold-key overlay (it scrolls); capped so a huge clipboard stays fast to lay out.
func GhostPreview(s string, maxLines, maxCols int) string {
	if strings.TrimSpace(s) == "" {
		return "No text on clipboard"
	}
	lines := strings.Split(s, "\n")
	n := len(lines)
	if n > maxLines {
		lines = lines[:maxLines]
	}
	for i, l := range lines {
		if utf8.RuneCountInString(l) > maxCols {
			lines[i] = string([]rune(l)[:maxCols])
		}
	}
	shown := strings.Join(lines, "\n")
	if n > maxLines {
		return shown + "\n… " + itoa(n-maxLines) + " more lines"
	}
	return shown
}

// NormalizeNewlines turns Windows \r\n (and lone \r) into \n.
func NormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ExpandTabs replaces tabs with spaces up to the next multiple of width, per line.
func ExpandTabs(s string, width int) string {
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			n := width - col%width
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case '\n':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// StripFences removes a leading and trailing ``` line if the model added them anyway.
func StripFences(s string) string {
	s = NormalizeNewlines(strings.TrimSpace(s))
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// FolderName is the last path component of a Windows or Unix style path.
func FolderName(p string) string {
	p = strings.TrimRight(p, `\/`)
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// PickProject: VS Code titles look like "● App.tsx - my-app - Visual Studio Code".
// Match a title part to a folder name, else the newest folder. Returns "" when recent is empty.
func PickProject(windowTitle string, recent []string) string {
	parts := map[string]bool{}
	for _, a := range strings.Split(windowTitle, " — ") {
		for _, b := range strings.Split(a, " - ") {
			parts[strings.ToLower(strings.TrimSpace(b))] = true
		}
	}
	for _, r := range recent {
		if parts[strings.ToLower(FolderName(r))] {
			return r
		}
	}
	if len(recent) > 0 {
		return recent[0]
	}
	return ""
}

// FileURIToPath converts the "folder" value VS Code stores in workspace.json,
// e.g. "file:///c%3A/Users/me/my-app", to a Windows path "c:\Users\me\my-app".
// Remote and virtual workspaces (vscode-remote://, vscode-vfs://) are skipped.
func FileURIToPath(s string) (string, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path // already percent-decoded
	if u.Host != "" && u.Host != "localhost" {
		return `\\` + u.Host + strings.ReplaceAll(p, "/", `\`), true // UNC share
	}
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // "/c:/Users" -> "c:/Users"
	}
	if strings.Trim(p, "/") == "" {
		return "", false
	}
	return strings.ReplaceAll(p, "/", `\`), true
}

// AccountLabel is the menu line built from `claude auth status --json`.
func AccountLabel(d map[string]any) string {
	if d == nil {
		return "Claude: unknown (is Claude Code installed?)"
	}
	if li, _ := d["loggedIn"].(bool); !li {
		return "Claude: not logged in (run claude auth login)"
	}
	plan := ""
	if st, ok := d["subscriptionType"].(string); ok && st != "" {
		plan = " · " + strings.ToUpper(st[:1]) + strings.ToLower(st[1:])
	}
	who := "logged in"
	if e, ok := d["email"].(string); ok && e != "" {
		who = e
	}
	return "Claude: " + who + plan
}

// TapDetector tells a quick press-and-release of the trigger key from a hold or a chord.
type TapDetector struct {
	downAt  float64
	pressed bool
}

// Key reports a trigger-key transition; it returns true when the key was pressed and
// released quickly with no other key in between.
func (t *TapDetector) Key(down bool, at float64) bool {
	if down {
		t.downAt, t.pressed = at, true
		return false
	}
	if !t.pressed {
		return false
	}
	t.pressed = false
	return at-t.downAt < HoldTime
}

// OtherKey: some other key was pressed, so this is a shortcut, not a tap or a hold.
func (t *TapDetector) OtherKey() { t.pressed = false }

// ShortPath replaces the home directory prefix with ~ for display.
func ShortPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(home)) {
		return "~" + p[len(home):]
	}
	return p
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
