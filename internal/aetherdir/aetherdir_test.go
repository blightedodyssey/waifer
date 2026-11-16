package aetherdir

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupTestEnv(t *testing.T) string {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)
	if runtime.GOOS != "windows" {
		t.Setenv("HOME", tmp)
	}
	return tmp
}

func TestRoot(t *testing.T) {
	tmp := setupTestEnv(t)
	got := Root()
	if runtime.GOOS == "windows" {
		want := filepath.Join(tmp, "Aether")
		if got != want {
			t.Errorf("Root() = %q, want %q", got, want)
		}
	}
}

func TestEnsure(t *testing.T) {
	setupTestEnv(t)
	if err := Ensure(); err != nil {
		t.Fatalf("Ensure() error: %v", err)
	}
	for _, d := range []string{CurrentDir(), TemplatesDir(), HooksDir()} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("missing directory %s: %v", d, err)
		}
	}
}

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("AETHER_TEST_VAR", "hello")
	if os.Getenv("AETHER_TEST_VAR") != "hello" {
		t.Fatalf("os.Getenv did not see test env var")
	}
	got := ExpandEnvVars("prefix %AETHER_TEST_VAR% suffix")
	want := "prefix hello suffix"
	if got != want {
		t.Errorf("ExpandEnvVars() = %q, want %q", got, want)
	}
}

func TestWriteColorsToml(t *testing.T) {
	setupTestEnv(t)
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"cursor":     "#cba6f7",
		"accent":     "#f5c2e7",
		"color0":     "#1e1e2e",
		"color1":     "#f38ba8",
		"color15":    "#b4befe",
	}
	if err := WriteColorsToml(vars); err != nil {
		t.Fatalf("WriteColorsToml error: %v", err)
	}

	path := filepath.Join(CurrentDir(), "colors.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read colors.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `background = "#1e1e2e"`) {
		t.Error("missing background color")
	}
	if !strings.Contains(content, `color0 = "#1e1e2e"`) {
		t.Error("missing color0")
	}
	if !strings.Contains(content, `color15 = "#b4befe"`) {
		t.Error("missing color15")
	}
}

func TestWriteThemeMeta(t *testing.T) {
	setupTestEnv(t)
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
	}
	if err := WriteThemeMeta(vars, "Rose Pine"); err != nil {
		t.Fatalf("WriteThemeMeta error: %v", err)
	}

	path := filepath.Join(CurrentDir(), "theme.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read theme.md: %v", err)
	}
	if !strings.Contains(string(data), "# Rose Pine") {
		t.Error("missing theme name")
	}
}

func TestWriteWallpaper(t *testing.T) {
	setupTestEnv(t)

	// Create a fake wallpaper
	src := filepath.Join(t.TempDir(), "test.jpg")
	if err := os.WriteFile(src, []byte("fake-image-data"), 0644); err != nil {
		t.Fatalf("create fake wallpaper: %v", err)
	}

	if err := WriteWallpaper(src); err != nil {
		t.Fatalf("WriteWallpaper error: %v", err)
	}

	// Verify copied wallpaper
	wpPath := filepath.Join(CurrentDir(), "wallpaper.jpg")
	if _, err := os.Stat(wpPath); err != nil {
		t.Errorf("wallpaper.jpg not found: %v", err)
	}

	// Verify pointer file
	ptrPath := filepath.Join(CurrentDir(), "wallpaper.path")
	ptrData, err := os.ReadFile(ptrPath)
	if err != nil {
		t.Fatalf("read wallpaper.path: %v", err)
	}
	if string(ptrData) != src {
		t.Errorf("wallpaper.path = %q, want %q", string(ptrData), src)
	}

	// Test clearing wallpaper
	if err := WriteWallpaper(""); err != nil {
		t.Fatalf("WriteWallpaper(\"\") error: %v", err)
	}
	if _, err := os.Stat(wpPath); err == nil {
		t.Error("wallpaper.jpg should be removed")
	}
	if _, err := os.Stat(ptrPath); err == nil {
		t.Error("wallpaper.path should be removed")
	}
}

func TestSeedStarterTemplates(t *testing.T) {
	setupTestEnv(t)
	if err := SeedStarterTemplates(); err != nil {
		t.Fatalf("SeedStarterTemplates error: %v", err)
	}

	// Check that starters were created
	for _, name := range []string{
		"alacritty-aether.toml.tpl",
		"windows-terminal-aether.json.tpl",
		"vscode-settings-aether.json.tpl",
		"yasb-aether-colors.css.tpl",
		"glazewm-aether-colors.yaml.tpl",
		"opencode-aether-theme.json.tpl",
		"hermes-aether-dashboard.yaml.tpl",
		"pi-aether-theme.json.tpl",
		"jan-aether-theme.css.tpl",
		"aider-aether-colors.yml.tpl",
	} {
		path := filepath.Join(TemplatesDir(), name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing starter template %s: %v", name, err)
		}
	}

	// Verify idempotency — seeding again should not error and should not overwrite
	if err := SeedStarterTemplates(); err != nil {
		t.Fatalf("second SeedStarterTemplates error: %v", err)
	}
}

func TestSeedStarterTemplates_SkipsWhenNotEmpty(t *testing.T) {
	setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	// Create an existing file
	existing := filepath.Join(TemplatesDir(), "my-custom.tpl")
	if err := os.WriteFile(existing, []byte("# target: %TEST%\n"), 0644); err != nil {
		t.Fatalf("write existing template: %v", err)
	}

	if err := SeedStarterTemplates(); err != nil {
		t.Fatalf("SeedStarterTemplates error: %v", err)
	}

	// Starters should NOT have been created because dir is not empty
	for _, name := range []string{"alacritty-aether.toml.tpl", "windows-terminal-aether.json.tpl"} {
		path := filepath.Join(TemplatesDir(), name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("starter %s should not exist when dir is not empty", name)
		}
	}

	// Existing file should still be there
	if _, err := os.Stat(existing); err != nil {
		t.Errorf("existing template was removed: %v", err)
	}
}
