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

const (
	hookTimeout     = 10 * time.Second
	hookScriptPS1   = "theme-set.ps1"
	hookScriptBatch = "theme-set.bat"
)

// RunHook checks for %LOCALAPPDATA%\Aether\hooks\theme-set.ps1 (or .bat) and
// executes it silently in the background with a 10-second timeout.  The hook
// runs after all theme state has been written so external tools can react.
func RunHook() error {
	hooksDir := HooksDir()

	// Prefer PowerShell, fall back to batch.
	var scriptPath, shell string
	psPath := filepath.Join(hooksDir, hookScriptPS1)
	if _, err := os.Stat(psPath); err == nil {
		scriptPath = psPath
		shell = "powershell.exe"
	} else {
		batPath := filepath.Join(hooksDir, hookScriptBatch)
		if _, err := os.Stat(batPath); err == nil {
			scriptPath = batPath
			shell = "cmd.exe"
		} else {
			return nil // No hook present — not an error.
		}
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
			return fmt.Errorf("hook timed out after %v", hookTimeout)
		}
		return fmt.Errorf("hook execution failed: %w", err)
	}
	return nil
}
