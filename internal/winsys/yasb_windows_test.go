//go:build windows

package winsys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyYasbThemeReplacesColorsDirectly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	yasbDir := filepath.Join(tmpDir, ".config", "yasb")
	os.MkdirAll(yasbDir, 0o755)

	stylesPath := filepath.Join(yasbDir, "styles.css")
	original := `@import "aether-colors.css";
:root {
    --bg-color: #1e1e2e;
    --text-color: #cdd6f4;
    --accent-color: #cba6f7;
    --border-color: #f5c2e7;
}

* {
    font-family: "JetBrainsMono Nerd Font", monospace;
    font-size: 13px;
    font-weight: bold;
    color: var(--text-color);
}

.yasb-bar {
    background-color: var(--bg-color);
    border-bottom: 2px solid var(--border-color);
}

.widget {
    padding: 0px 10px;
    margin: 0px 2px;
}

.shortcuts-menu {
    color: var(--accent-color);
    padding: 0px 8px;
}

.shortcuts-menu:hover {
    color: #9ccfd8;
    background-color: rgba(255, 255, 255, 0.05);
}
`
	os.WriteFile(stylesPath, []byte(original), 0o644)

	vars := map[string]string{
		"base00": "#111111",
		"base05": "#222222",
		"base0D": "#333333",
		"base0E": "#444444",
	}

	if err := ApplyYasbTheme(vars); err != nil {
		t.Fatalf("ApplyYasbTheme: %v", err)
	}

	data, err := os.ReadFile(stylesPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// 1. @import must be gone.
	if strings.Contains(content, `@import "aether-colors.css"`) {
		t.Error("stale @import was not removed")
	}

	// 2. var(...) must be replaced with literal hex.
	for _, bad := range []string{"var(--bg-color)", "var(--text-color)", "var(--accent-color)", "var(--border-color)"} {
		if strings.Contains(content, bad) {
			t.Errorf("found unexpanded CSS var: %q", bad)
		}
	}

	// 3. Correct colours applied.
	if !strings.Contains(content, "background-color: #111111") {
		t.Error("expected bg colour (#111111) to be present")
	}
	if !strings.Contains(content, "color: #222222") {
		t.Error("expected text colour (#222222) to be present")
	}
	if !strings.Contains(content, "color: #333333") {
		t.Error("expected accent colour (#333333) to be present")
	}
	if !strings.Contains(content, "border-bottom: 2px solid #444444") {
		t.Error("expected border colour (#444444) to be present")
	}

	// 4. Hover overlay replaced with semi-transparent text colour.
	if strings.Contains(content, "rgba(255, 255, 255, 0.05)") {
		t.Error("expected rgba(255,255,255,0.05) to be replaced")
	}
	if !strings.Contains(content, "rgba(34, 34, 34, 0.05)") {
		t.Error("expected semi-transparent text colour hover overlay")
	}

	// 5. Hardcoded accent colours removed / replaced.
	for _, bad := range []string{"#cba6f7", "#f5c2e7", "#9ccfd8", "#89b4fa"} {
		if strings.Contains(content, bad) {
			t.Errorf("found stale accent colour %s", bad)
		}
	}

	// 6. Geometry preserved.
	for _, keep := range []string{"font-size: 13px", "padding: 0px 10px", "margin: 0px 2px", "padding: 0px 8px", "font-weight: bold"} {
		if !strings.Contains(content, keep) {
			t.Errorf("expected geometry preserved: %s", keep)
		}
	}

	// 7. Backup created.
	backupPath := stylesPath + ".aether.bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected .aether.bak backup to be created")
	}
}

func TestApplyYasbThemeCreatesDefaultsIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	yasbDir := filepath.Join(tmpDir, ".config", "yasb")
	os.MkdirAll(yasbDir, 0o755)

	vars := map[string]string{
		"base00": "#555555",
		"base05": "#666666",
		"base0D": "#777777",
		"base0E": "#888888",
	}

	if err := ApplyYasbTheme(vars); err != nil {
		t.Fatalf("ApplyYasbTheme: %v", err)
	}

	stylesPath := filepath.Join(yasbDir, "styles.css")
	data, err := os.ReadFile(stylesPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Ensure the generated file has the proper QSS structure.
	if !strings.Contains(content, ".yasb-bar") {
		t.Error("expected .yasb-bar selector in generated styles.css")
	}
	if !strings.Contains(content, "font-size: 13px") {
		t.Error("expected font-size geometry preserved")
	}
	// Colours applied.
	if !strings.Contains(content, "background-color: #555555") {
		t.Error("expected bg colour applied")
	}
}
