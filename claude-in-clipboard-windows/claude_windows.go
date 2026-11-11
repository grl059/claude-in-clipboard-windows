//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"claudeclip/internal/logic"
)

const createNoWindow = 0x08000000

// findClaude locates the Claude Code CLI: PATH first (claude.exe or an npm claude.cmd shim),
// then the places the installers use.
func findClaude() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	var candidates []string
	if h, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(h, ".local", "bin", "claude.exe"))
	}
	if a := os.Getenv("APPDATA"); a != "" {
		candidates = append(candidates, filepath.Join(a, "npm", "claude.cmd"))
	}
	if l := os.Getenv("LOCALAPPDATA"); l != "" {
		candidates = append(candidates, filepath.Join(l, "Microsoft", "WinGet", "Links", "claude.exe"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("claude not found. Install Claude Code and make sure `claude` works in a terminal")
}

// systemPromptFile writes the prompt where claude can read it (a file avoids Windows quoting problems).
func systemPromptFile(t target) (string, error) {
	name, text := "system-prompt.txt", logic.SystemPrompt
	if t.kind == "notepad" {
		name, text = "system-prompt-notepad.txt", logic.NotepadSystemPrompt
	}
	p := filepath.Join(dataDir(), name)
	if old, err := os.ReadFile(p); err == nil && string(old) == text {
		return p, nil
	}
	return p, os.WriteFile(p, []byte(text), 0o644)
}

type result struct {
	id       int
	out, err string
	code     int
}

var (
	resMu   sync.Mutex
	results []result
)

// job is one prompt being handled in the background: it finds the VS Code project or Notepad file, runs `claude -p`
// and queues the result for the UI thread. Doing this off the UI thread matters because the keyboard
// hook shares that thread: a slow file read there would stall typing system-wide.
type job struct {
	id        int
	mu        sync.Mutex
	cmd       *exec.Cmd
	t         target
	cancelled bool
}

// kill stops claude and anything it spawned (a new prompt replaces a running one).
func (j *job) kill() {
	j.mu.Lock()
	j.cancelled = true
	c := j.cmd
	j.mu.Unlock()
	killTree(c)
}

func (j *job) target() target {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.t
}

func finish(r result) {
	resMu.Lock()
	results = append(results, r)
	resMu.Unlock()
	postMessage(mainHwnd, wmDone, 0, 0)
}

// startJob runs prompt against the project VS Code has open, or the file open in Notepad.
// Read-only tools: Claude can look at the code but can't edit files or run commands.
// User settings only: a cloned repo's .claude/settings.json hooks would otherwise run shell commands.
func startJob(id int, prompt, model string) *job {
	j := &job{id: id}
	go func() {
		fail := func(msg string) { finish(result{id: id, err: msg, code: -2}) }
		t := resolveTarget()
		if !t.found() {
			fail("No VS Code project found (and no Notepad window open)")
			return
		}
		j.mu.Lock()
		j.t = t
		j.mu.Unlock()
		postMessage(mainHwnd, wmStarted, 0, 0)

		exe, err := findClaude()
		if err != nil {
			fail(err.Error())
			return
		}
		spf, err := systemPromptFile(t)
		if err != nil {
			fail(err.Error())
			return
		}
		if t.file != "" { // let Claude know which file is open (its folder is the working directory)
			prompt = "The file open in Notepad is " + t.file + " (in the current directory).\n" + prompt
		}
		logf("starting %s for %s %q in %s (model %s)", exe, t.app(), t.name, t.dir, model)
		cmd := exec.Command(exe, "-p", "--strict-mcp-config", "--no-session-persistence",
			"--setting-sources", "user", "--model", model,
			"--tools", "Read,Grep,Glob", "--append-system-prompt-file", spf)
		cmd.Dir = t.dir
		cmd.Stdin = strings.NewReader(prompt) // prompt via stdin, never via a command line
		var so, se bytes.Buffer
		cmd.Stdout, cmd.Stderr = &so, &se
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		if err := cmd.Start(); err != nil {
			fail(err.Error())
			return
		}
		j.mu.Lock()
		j.cmd = cmd
		cancelled := j.cancelled
		j.mu.Unlock()
		if cancelled { // replaced while starting up
			killTree(cmd)
		}
		werr := cmd.Wait()
		code := 0
		if werr != nil {
			code = -1
			var ee *exec.ExitError
			if errors.As(werr, &ee) {
				code = ee.ExitCode()
			}
		}
		finish(result{id: id, out: so.String(), err: se.String(), code: code})
	}()
	return j
}

func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	k := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	go k.Run()
}

func takeResults() []result {
	resMu.Lock()
	defer resMu.Unlock()
	r := results
	results = nil
	return r
}

var (
	acctMu    sync.Mutex
	acctLabel = "Claude: …"
)

func accountText() string {
	acctMu.Lock()
	defer acctMu.Unlock()
	return acctLabel
}

// refreshAccount asks `claude auth status --json` who is logged in; the menu shows the last known answer.
func refreshAccount() {
	go func() {
		var d map[string]any
		if exe, err := findClaude(); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "auth", "status", "--json")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
			if out, err := cmd.Output(); err == nil {
				if json.Unmarshal(out, &d) != nil {
					d = nil
				}
			}
		}
		acctMu.Lock()
		acctLabel = logic.AccountLabel(d)
		acctMu.Unlock()
	}()
}
