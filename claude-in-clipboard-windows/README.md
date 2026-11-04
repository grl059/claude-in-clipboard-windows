# ClaudeClip for Windows

Tap **Right Ctrl**, type a prompt (invisibly), press **Enter**. Claude Code reads the project open in VS Code and puts the code on your clipboard. Double-tap **Right Ctrl** to clear the clipboard.

It lives in the system tray: no taskbar button, no window. A Windows port of [claude-in-clipboard](https://github.com/roaziy/claude-in-clipboard) for macOS.

```
Right Ctrl  →  type "add a dark mode toggle to settings"  →  Enter  →  Ctrl+V
```

## Why not Fn?

The Fn key is handled inside the keyboard's firmware, and Windows never sees it, so no app can listen for it. ClaudeClip uses a key Windows can see instead. Pick it from the tray menu → **Trigger key**:

| Key | Notes |
| --- | --- |
| **Right Ctrl** (default) | Tapping it alone does nothing in other apps. Missing on some laptops. |
| **Right Shift** | Same idea, present on nearly every keyboard. |
| **Caps Lock** | ClaudeClip swallows the key, so it stops toggling caps while ClaudeClip is running. |

## Requirements

- Windows 10 or 11 (x64 or ARM64)
- [Claude Code](https://code.claude.com/docs/en/setup) installed and logged in (`claude` works in a terminal)
- VS Code (the regular one, not Insiders)

## Install

**From a release:** download `ClaudeClip.exe` (or `ClaudeClip-arm64.exe` on Windows on ARM) from [Releases](../../releases) and put it anywhere, for example `C:\Tools`. Double-click it. The exe isn't code-signed, so SmartScreen may say "Windows protected your PC": click **More info → Run anyway**.

**From source** (needs [Go](https://go.dev/dl/) 1.22+, `winget install GoLang.Go`):

```powershell
git clone <this repo>
cd claude-in-clipboard-windows
.\build.ps1
.\build\ClaudeClip.exe
```

On macOS or Linux, `./build.sh` cross-compiles the same exe (`GOARCH=arm64 ./build.sh` for ARM).

## Setup

Nothing is required: no accessibility or folder permissions on Windows. Two optional things:

1. **Show the icon:** Windows hides new tray icons in the **^** overflow. Drag ClaudeClip out of it to keep it visible.
2. **Start with Windows:** tray menu → **Start with Windows**.

## Usage

| Do | Result |
| --- | --- |
| Tap the trigger key | Opens the invisible prompt |
| Type, then Enter | Sends it to Claude |
| Esc, tap the key again, or click elsewhere | Cancels |
| Double-tap the key | Clears the clipboard |
| Hold the key (over 0.2 s) | Shows a preview of the clipboard under your pointer (nothing is pasted). Scroll it with the mouse wheel or trackpad (Shift+wheel scrolls sideways, except when the trigger is Right Shift). Release to hide it. Passwords copied from a password manager show as "Hidden" |

While the prompt is open, Backspace, Ctrl+Backspace (delete word) and Ctrl+V (paste, as one line) work. You can't see what you type; run `ClaudeClip.exe --show-prompt` to show the box while debugging.

The tray icon shows what's happening (hover for the status text):

| Icon | Meaning |
| --- | --- |
| ✨ | Ready |
| ⌨️ | Prompt open, typing |
| ⌛ | Claude is working (usually 5–20 s) |
| ✓ + sound | Code is on the clipboard |
| ⚠️ + sound | Failed. Click the icon to read the error |

Click the tray icon for the menu: status, the project and Claude account it will use, the model (**Opus (latest)** by default or **Sonnet (latest)**, both always the newest version), the trigger key, and Quit. Settings are saved in `%APPDATA%\ClaudeClip\settings.json`.

If an answer spans several files, they come as one block with a `// path/to/file` comment before each file.

## How it finds your project

1. Looks through your open windows for VS Code (`Code.exe`) and reads the title of the frontmost one, for example `index.tsx - my-app - Visual Studio Code`. This works even when Chrome or another app is in front.
2. Matches `my-app` against the folders VS Code has opened (`%APPDATA%\Code\User\workspaceStorage`) to get the full path.
3. If nothing matches, uses the project you used most recently.

## Privacy and safety

- Runs `claude -p` locally with your own Claude Code login. No other servers.
- Claude only gets read-only tools (`Read`, `Grep`, `Glob`). It can't edit files or run commands.
- The project's `.claude/settings.json` (including hooks) is ignored (`--setting-sources user`), so a cloned repo can't run commands on your PC.
- Chats aren't saved (`--no-session-persistence`).
- MCP servers are skipped (`--strict-mcp-config`), which roughly halves startup time.
- Your prompt is passed on stdin, never through a command line. The system prompt is passed as a file (`%APPDATA%\ClaudeClip\system-prompt.txt`).
- The keyboard hook only looks at the trigger key; typed prompt text goes into ClaudeClip's own window and nowhere else.

## Limitations

- VS Code only, not Cursor or VS Code Insiders. Remote and WSL workspaces aren't found (only local folders).
- Two open projects with the same folder name: it picks the newer one.
- Every prompt is a fresh chat, so no follow-ups.
- The trigger key doesn't work while an app running **as administrator** has focus (Windows blocks hooks from seeing it). Run ClaudeClip as administrator too if you need that.
- Windows decides which app may take keyboard focus. If it refuses, the icon shows a warning instead of opening the prompt.
- You can't see what you type.

## Development

| File | What |
| --- | --- |
| `internal/logic/` | Pure logic (tap detection, project matching, text helpers) with unit tests: `go test ./...` runs on any OS |
| `main_windows.go` | Entry point, keyboard hook, tap and run flow |
| `prompt_windows.go`, `ghost_windows.go` | The invisible prompt window and the hold-to-preview overlay |
| `tray_windows.go`, `icons_windows.go` | Tray icon, menu, and the icons (drawn in code) |
| `vscode_windows.go`, `claude_windows.go` | Finding the VS Code project, running Claude Code |
| `clipboard_windows.go`, `settings_windows.go`, `win32_windows.go` | Clipboard, settings and autostart, raw Win32 bindings |
| `build.ps1` / `build.sh` | Vet, test, then build `build\ClaudeClip.exe` |
| `.github/workflows/release.yml` | Builds and publishes a release on every push to `main` |

There are no third-party dependencies: only the Go standard library and Win32 calls.

Version numbers follow the commit type: `fix:` bumps the patch (`0.0.1 → 0.0.2`), `feat:` bumps the minor (`0.1.0 → 0.2.0`).

---

Not affiliated with Anthropic.
