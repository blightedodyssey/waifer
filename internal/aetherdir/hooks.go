//go:build windows

package aetherdir

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const hookTimeout = 10 * time.Second

// hookCandidate is a post-apply script Waifer recognizes. Both the canonical
// on-apply.* names in CurrentDir()/HooksDir() and the legacy theme-set.* names
// are supported for existing user setups.
type hookCandidate struct {
	dir   string
	name  string
	shell string
}

// findHookScript returns the first existing Waifer hook script and its shell.
// PowerShell candidates are preferred over batch scripts.
func findHookScript() (string, string) {
	candidates := []hookCandidate{
		{CurrentDir(), "on-apply.ps1", "powershell.exe"},
		{HooksDir(), "on-apply.ps1", "powershell.exe"},
		{CurrentDir(), "on-apply.bat", "cmd.exe"},
		{HooksDir(), "on-apply.bat", "cmd.exe"},
		{HooksDir(), "theme-set.ps1", "powershell.exe"},
		{CurrentDir(), "theme-set.ps1", "powershell.exe"},
		{HooksDir(), "theme-set.bat", "cmd.exe"},
		{CurrentDir(), "theme-set.bat", "cmd.exe"},
	}
	for _, candidate := range candidates {
		path := filepath.Join(candidate.dir, candidate.name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, candidate.shell
		}
	}
	return "", ""
}

// RunHook executes a Waifer post-apply script silently with a 10-second timeout
// after all theme state has been written, so external tools can react.
// Hook failures are logged only and never fail the theme apply; the error return
// is retained for API compatibility and is always nil.
func RunHook() error {
	scriptPath, shell := findHookScript()
	if scriptPath == "" {
		return nil // No hook present - not an error.
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if shell == "powershell.exe" {
		cmd = exec.CommandContext(ctx, shell, "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	} else {
		cmd = exec.CommandContext(ctx, shell, "/c", scriptPath)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "waifer: hook %s timed out after %v\n", scriptPath, hookTimeout)
		} else {
			fmt.Fprintf(os.Stderr, "waifer: hook %s failed: %v\n", scriptPath, err)
		}
	}
	return nil
}
