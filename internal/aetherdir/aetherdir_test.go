package aetherdir

import (
	"encoding/json"
	"fmt"
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
		want := filepath.Join(tmp, "Waifer")
		if got != want {
			t.Errorf("Root() = %q, want %q", got, want)
		}
	} else {
		want := filepath.Join(tmp, ".local", "share", "aether")
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

func fullTestVars() map[string]string {
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#f5c2e7",
		"cursor":     "#cba6f7",
	}
	for i := 0; i < 16; i++ {
		vars[fmt.Sprintf("color%d", i)] = fmt.Sprintf("#%06x", i)
	}
	return vars
}

func TestMigrateFromLegacy(t *testing.T) {
	tmp := setupTestEnv(t)
	legacy := filepath.Join(tmp, legacyAppDir)
	waifer := filepath.Join(tmp, waiferAppDir)
	files := map[string]string{
		filepath.Join("current", "colors.toml"):          "background = '#000000'\n",
		filepath.Join("current", "presets", "rose.json"): `{"name":"Rose Pine"}`,
		filepath.Join("blueprints", "bp.json"):           `{"name":"Blueprint"}`,
	}
	for rel, content := range files {
		path := filepath.Join(legacy, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir legacy %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write legacy %s: %v", rel, err)
		}
	}

	if err := MigrateFromLegacy(); err != nil {
		t.Fatalf("MigrateFromLegacy error: %v", err)
	}
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(waifer, rel))
		if err != nil {
			t.Errorf("read migrated %s: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("migrated %s = %q, want %q", rel, got, want)
		}
		if _, err := os.Stat(filepath.Join(legacy, rel)); err != nil {
			t.Errorf("legacy %s should be preserved: %v", rel, err)
		}
	}

	// Second call is a no-op and must not overwrite anything in Waifer.
	sentinel := filepath.Join(waifer, "custom.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if err := MigrateFromLegacy(); err != nil {
		t.Fatalf("second MigrateFromLegacy error: %v", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Errorf("sentinel changed: data=%q err=%v", data, err)
	}
}

func TestMigrateFromLegacy_NoLegacyTree(t *testing.T) {
	setupTestEnv(t)
	if err := MigrateFromLegacy(); err != nil {
		t.Fatalf("MigrateFromLegacy with no legacy tree: %v", err)
	}
}

func TestEnsureMigratesLegacyTree(t *testing.T) {
	tmp := setupTestEnv(t)
	legacyFile := filepath.Join(tmp, legacyAppDir, "current", "colors.toml")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0755); err != nil {
		t.Fatalf("mkdir legacy current: %v", err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy-colors"), 0644); err != nil {
		t.Fatalf("write legacy colors: %v", err)
	}

	if err := Ensure(); err != nil {
		t.Fatalf("Ensure error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, waiferAppDir, "current", "colors.toml"))
	if err != nil {
		t.Fatalf("Ensure did not migrate legacy colors: %v", err)
	}
	if string(got) != "legacy-colors" {
		t.Errorf("migrated colors = %q, want %q", got, "legacy-colors")
	}
}

func TestWriteThemeCSS(t *testing.T) {
	setupTestEnv(t)
	if err := WriteThemeCSS(fullTestVars()); err != nil {
		t.Fatalf("WriteThemeCSS error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(CurrentDir(), "theme.css"))
	if err != nil {
		t.Fatalf("read theme.css: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"--bg: #1e1e2e;", "--fg: #cdd6f4;", "--accent: #f5c2e7;",
		"--cursor: #cba6f7;", "--color0: #000000;", "--color15: #00000f;",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("theme.css missing %q:\n%s", want, content)
		}
	}

	vars := fullTestVars()
	delete(vars, "cursor")
	if err := WriteThemeCSS(vars); err != nil {
		t.Fatalf("WriteThemeCSS without cursor: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(CurrentDir(), "theme.css"))
	if err != nil {
		t.Fatalf("read theme.css: %v", err)
	}
	if strings.Contains(string(data), "--cursor:") {
		t.Errorf("theme.css should omit missing cursor:\n%s", data)
	}
}

func TestWriteThemeJSON(t *testing.T) {
	setupTestEnv(t)
	if err := WriteThemeJSON(fullTestVars()); err != nil {
		t.Fatalf("WriteThemeJSON error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(CurrentDir(), "theme.json"))
	if err != nil {
		t.Fatalf("read theme.json: %v", err)
	}
	var payload struct {
		Background string            `json:"background"`
		Foreground string            `json:"foreground"`
		Accent     string            `json:"accent"`
		Cursor     string            `json:"cursor"`
		Colors     map[string]string `json:"colors"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid theme.json: %v\n%s", err, data)
	}
	if payload.Background != "#1e1e2e" || payload.Foreground != "#cdd6f4" ||
		payload.Accent != "#f5c2e7" || payload.Cursor != "#cba6f7" {
		t.Errorf("unexpected semantic colors: %+v", payload)
	}
	if len(payload.Colors) != 16 || payload.Colors["color0"] != "#000000" ||
		payload.Colors["color15"] != "#00000f" {
		t.Errorf("unexpected colors object: %+v", payload.Colors)
	}
}

func TestWriteColorsINI(t *testing.T) {
	setupTestEnv(t)
	if err := WriteColorsINI(fullTestVars()); err != nil {
		t.Fatalf("WriteColorsINI error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(CurrentDir(), "colors.ini"))
	if err != nil {
		t.Fatalf("read colors.ini: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"[Colors]", "background=#1e1e2e", "foreground=#cdd6f4", "accent=#f5c2e7",
		"cursor=#cba6f7", "color0=#000000", "color15=#00000f",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("colors.ini missing %q:\n%s", want, content)
		}
	}
}

func TestWriteColorsPS1(t *testing.T) {
	setupTestEnv(t)
	if err := WriteColorsPS1(fullTestVars()); err != nil {
		t.Fatalf("WriteColorsPS1 error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(CurrentDir(), "colors.ps1"))
	if err != nil {
		t.Fatalf("read colors.ps1: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"$WaiferColors = @{", "background = '#1e1e2e'", "cursor = '#cba6f7'",
		"color0 = '#000000'", "color15 = '#00000f'",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("colors.ps1 missing %q:\n%s", want, content)
		}
	}
}

func TestWriteLiveExports(t *testing.T) {
	setupTestEnv(t)
	if err := WriteLiveExports(fullTestVars()); err != nil {
		t.Fatalf("WriteLiveExports error: %v", err)
	}
	for _, name := range []string{
		"theme.css", "theme.json", "colors.ini", "colors.ps1", "colors.toml", "theme.md",
	} {
		if _, err := os.Stat(filepath.Join(CurrentDir(), name)); err != nil {
			t.Errorf("missing live export %s: %v", name, err)
		}
	}
}

func TestWriteLiveExports_ContinuesAfterFailure(t *testing.T) {
	setupTestEnv(t)
	if err := Ensure(); err != nil {
		t.Fatalf("Ensure error: %v", err)
	}
	// A directory at theme.css makes that single export fail.
	if err := os.Mkdir(filepath.Join(CurrentDir(), "theme.css"), 0755); err != nil {
		t.Fatalf("mkdir theme.css: %v", err)
	}

	if err := WriteLiveExports(fullTestVars()); err == nil {
		t.Fatal("WriteLiveExports should return the first export error")
	}
	for _, name := range []string{"theme.json", "colors.ini", "colors.ps1", "colors.toml", "theme.md"} {
		if _, err := os.Stat(filepath.Join(CurrentDir(), name)); err != nil {
			t.Errorf("export %s was skipped after theme.css failure: %v", name, err)
		}
	}
}
