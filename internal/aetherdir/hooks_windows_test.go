//go:build windows

package aetherdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindHookScript(t *testing.T) {
	setupTestEnv(t)
	if err := Ensure(); err != nil {
		t.Fatalf("Ensure error: %v", err)
	}

	currentHook := filepath.Join(CurrentDir(), "on-apply.ps1")
	hooksHook := filepath.Join(HooksDir(), "theme-set.ps1")
	for _, path := range []string{currentHook, hooksHook} {
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatalf("write hook %s: %v", path, err)
		}
	}

	got, shell := findHookScript()
	if got != currentHook || shell != "powershell.exe" {
		t.Errorf("findHookScript() = (%q, %q), want (%q, powershell.exe)", got, shell, currentHook)
	}

	// CurrentDir on-apply.bat is preferred over the legacy HooksDir script.
	if err := os.Remove(currentHook); err != nil {
		t.Fatalf("remove hook: %v", err)
	}
	batHook := filepath.Join(CurrentDir(), "on-apply.bat")
	if err := os.WriteFile(batHook, []byte(""), 0644); err != nil {
		t.Fatalf("write batch hook: %v", err)
	}
	got, shell = findHookScript()
	if got != batHook || shell != "cmd.exe" {
		t.Errorf("findHookScript() = (%q, %q), want (%q, cmd.exe)", got, shell, batHook)
	}
}

func TestFindHookScript_None(t *testing.T) {
	setupTestEnv(t)
	if err := Ensure(); err != nil {
		t.Fatalf("Ensure error: %v", err)
	}
	if got, shell := findHookScript(); got != "" || shell != "" {
		t.Errorf("findHookScript() = (%q, %q), want empty", got, shell)
	}
}
