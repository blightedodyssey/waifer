package browsers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWebUICSS(t *testing.T) {
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"dark_bg":    "#181825",
	}
	css := RenderWebUICSS(vars)

	for _, want := range []string{
		"--waifer-bg: #1e1e2e;",
		"--waifer-surface: rgba(24, 24, 37, 0.88);",
		"--waifer-fg: #cdd6f4;",
		"--waifer-accent: #89b4fa;",
		"bard-sidenav-container",
		".input-area-container",
		"!important",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("RenderWebUICSS missing %q:\n%s", want, css)
		}
	}
}

func TestRenderWebUICSSFallsBackWithoutDarkBg(t *testing.T) {
	css := RenderWebUICSS(map[string]string{
		"background": "#f38b83",
		"foreground": "#fdf6f5",
		"accent":     "#f38b83",
	})

	if !strings.Contains(css, "--waifer-bg: #f38b83;") {
		t.Errorf("missing background var:\n%s", css)
	}
	// Surface falls back to background blended 15% toward black, still a
	// valid rgba() value rather than an empty variable.
	if !strings.Contains(css, "--waifer-surface: rgba(") {
		t.Errorf("surface var missing rgba() fallback:\n%s", css)
	}
}

func TestWriteWebUIExtension(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tempDir)

	// A missing extension directory must be reported (non-fatal) without
	// creating anything on disk.
	if err := writeWebUIExtension(map[string]string{"background": "#000000"}); err == nil {
		t.Fatal("expected an error when the extension directory is missing")
	} else if !strings.Contains(err.Error(), "chrome-webui-ext") {
		t.Errorf("error should name the extension directory, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "Waifer", "chrome-webui-ext")); !os.IsNotExist(err) {
		t.Fatal("extension directory was created despite being missing")
	}

	extDir := filepath.Join(tempDir, "Waifer", "chrome-webui-ext")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"Waifer WebUI Theme Sync","version":"1.0.0"}`
	if err := os.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"dark_bg":    "#181825",
	}
	if err := writeWebUIExtension(vars); err != nil {
		t.Fatalf("writeWebUIExtension: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(extDir, webUICSSFile))
	if err != nil {
		t.Fatalf("read waifer-webui.css: %v", err)
	}
	if !strings.Contains(string(data), "--waifer-bg: #1e1e2e;") {
		t.Errorf("waifer-webui.css missing background var:\n%s", data)
	}

	// The manifest is user-managed: it must be left byte-identical.
	gotManifest, err := os.ReadFile(filepath.Join(extDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(gotManifest) != manifest {
		t.Errorf("manifest.json was modified:\n%s", gotManifest)
	}

	// Atomic write leaves no .tmp / backup leftovers.
	entries, err := os.ReadDir(extDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("extension dir should contain only manifest.json and waifer-webui.css, got %v", names)
	}
}
