//go:build windows

package winsys

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTerminalTestEnv sets LOCALAPPDATA to a temp directory and creates the
// standard Windows Terminal settings path so applyTerminalSettingsFile is
// exercised without touching a real installation.
func setupTerminalTestEnv(t *testing.T) (localAppData, settingsPath string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	localState := filepath.Join(dir, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState")
	if err := os.MkdirAll(localState, 0o755); err != nil {
		t.Fatalf("mkdir local state: %v", err)
	}
	return dir, filepath.Join(localState, "settings.json")
}

func TestApplyWindowsTerminalPreservesProfilesList(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)

	// Realistic settings.json with JSONC comments, trailing commas, valid profiles.list.
	initial := `{
    // This is a Windows Terminal file
    "defaultProfile": "{abc-123}",
    "profiles": {
        "defaults": {
            "font": {"face": "Cascadia Code"}
        },
        "list": [
            {
                "guid": "{abc-123}",
                "name": "PowerShell",
                "hidden": false
            }
        ]
    },
    "actions": [
        {"command": "copy", "keys": "ctrl+c"}
    ],
}`

	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	vars := map[string]string{
		"background":           "#1e1e2e",
		"foreground":           "#cdd6f4",
		"cursor":               "#f5e0dc",
		"selection_background": "#585b70",
		"black":                "#45475a",
		"red":                  "#f38ba8",
		"green":                "#a6e3a1",
		"yellow":               "#f9e2af",
		"blue":                 "#89b4fa",
		"magenta":              "#f5c2e7",
		"cyan":                 "#94e2d5",
		"white":                "#bac2de",
		"bright_black":         "#585b70",
		"bright_red":           "#f38ba8",
		"bright_green":         "#a6e3a1",
		"bright_yellow":        "#f9e2af",
		"bright_blue":          "#89b4fa",
		"bright_magenta":       "#f5c2e7",
		"bright_cyan":          "#94e2d5",
		"bright_white":         "#a6adc8",
	}

	themeDir := filepath.Join(filepath.Dir(settingsPath), "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir themeDir: %v", err)
	}

	if err := ApplyWindowsTerminal(vars, themeDir); err != nil {
		t.Fatalf("ApplyWindowsTerminal: %v", err)
	}

	// Read back and validate.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	// Must still be valid JSON (comments stripped).
	var root map[string]any
	cleaned := stripJSONComments(data)
	cleaned = removeTrailingCommas(cleaned)
	if err := json.Unmarshal(cleaned, &root); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(data))
	}

	// Verify profiles.list is intact and non-empty.
	profiles, ok := root["profiles"].(map[string]any)
	if !ok {
		t.Fatal("profiles key is missing or not an object")
	}
	list, ok := profiles["list"].([]any)
	if !ok || len(list) == 0 {
		t.Fatal("profiles.list is missing or empty — would cause terminal error")
	}

	// Verify original list contents preserved.
	first := list[0].(map[string]any)
	if first["name"] != "PowerShell" {
		t.Errorf("list[0].name = %q, want %q", first["name"], "PowerShell")
	}

	// Verify defaults.colorScheme was set.
	defaults, ok := profiles["defaults"].(map[string]any)
	if !ok {
		t.Fatal("profiles.defaults is missing")
	}
	if cs, _ := defaults["colorScheme"].(string); cs != "Aether" {
		t.Errorf("defaults.colorScheme = %q, want %q", cs, "Aether")
	}

	// Verify actions key is preserved.
	actions, ok := root["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatal("actions key was lost")
	}
}

func TestApplyWindowsTerminalAbortsOnEmptyProfilesList(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)

	// profiles.list is empty — this is the regression case.
	initial := `{
    "profiles": {
        "list": []
    }
}`

	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	vars := map[string]string{"background": "#000", "foreground": "#fff"}

	err := ApplyWindowsTerminal(vars, filepath.Dir(settingsPath))
	if err == nil {
		t.Fatal("expected an error for empty profiles.list, got nil")
	}
	if !strings.Contains(err.Error(), "profiles.list is empty or missing") {
		t.Errorf("error message = %q, want substring %q", err.Error(), "profiles.list is empty or missing")
	}

	// File must remain unchanged.
	data, _ := os.ReadFile(settingsPath)
	if string(data) != initial {
		t.Errorf("settings.json was modified on failure:\nbefore: %s\nafter:  %s", initial, string(data))
	}
}

func TestApplyWindowsTerminalAbortsOnMissingProfiles(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)

	initial := `{"schemes": []}`

	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	vars := map[string]string{"background": "#000", "foreground": "#fff"}

	err := ApplyWindowsTerminal(vars, filepath.Dir(settingsPath))
	if err == nil {
		t.Fatal("expected an error for missing profiles, got nil")
	}
	if !strings.Contains(err.Error(), "profiles key is missing or unexpected type") {
		t.Errorf("error message = %q, want substring %q", err.Error(), "profiles key is missing or unexpected type")
	}

	data, _ := os.ReadFile(settingsPath)
	if string(data) != initial {
		t.Errorf("settings.json was modified on failure")
	}
}
