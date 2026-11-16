//go:build windows

package winsys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildThemeContent_Basic(t *testing.T) {
	vars := map[string]string{
		"accent": "#cba6f7",
		"name":   "Catppuccin Mocha",
	}
	wallpaper := `C:\Users\Odyssey\Pictures\Wallpapers\mountains.jpg`
	content := BuildThemeContent(vars, wallpaper, false)

	if !strings.Contains(content, "[Theme]") {
		t.Error("missing [Theme] section")
	}
	if !strings.Contains(content, "DisplayName=Aether - Catppuccin Mocha") {
		t.Error("missing or incorrect DisplayName")
	}
	if !strings.Contains(content, wallpaper) {
		t.Error("missing wallpaper path")
	}
	if !strings.Contains(content, "ColorizationColor=0XFFCBA6F7") {
		t.Errorf("missing or incorrect ColorizationColor: got content\n%s", content)
	}
	if !strings.Contains(content, "SystemMode=Dark") {
		t.Error("expected SystemMode=Dark")
	}
	if !strings.Contains(content, "AppMode=Dark") {
		t.Error("expected AppMode=Dark")
	}
	if !strings.Contains(content, "TileWallpaper=0") {
		t.Error("missing TileWallpaper=0")
	}
	if !strings.Contains(content, "WallpaperStyle=10") {
		t.Error("missing WallpaperStyle=10")
	}
}

func TestBuildThemeContent_LightMode(t *testing.T) {
	vars := map[string]string{
		"accent": "#89b4fa",
		"name":   "Latte",
	}
	content := BuildThemeContent(vars, "", true)

	if !strings.Contains(content, "DisplayName=Aether - Latte") {
		t.Error("missing or incorrect DisplayName")
	}
	if !strings.Contains(content, "SystemMode=Light") {
		t.Error("expected SystemMode=Light")
	}
	if !strings.Contains(content, "AppMode=Light") {
		t.Error("expected AppMode=Light")
	}
	if !strings.Contains(content, "ColorizationColor=0XFF89B4FA") {
		t.Errorf("missing or incorrect ColorizationColor: got content\n%s", content)
	}
}

func TestBuildThemeContent_FallbackName(t *testing.T) {
	vars := map[string]string{
		"accent": "#cba6f7",
	}
	content := BuildThemeContent(vars, "", false)

	if !strings.Contains(content, "DisplayName=Aether - Live Palette") {
		t.Errorf("expected fallback name, got content\n%s", content)
	}
}

func TestBuildThemeContent_NilVars(t *testing.T) {
	content := BuildThemeContent(nil, "", false)
	if !strings.Contains(content, "[Theme]") {
		t.Error("nil vars should still produce valid INI")
	}
	if !strings.Contains(content, "DisplayName=Aether - Live Palette") {
		t.Error("nil vars should use fallback name")
	}
}

func TestBuildThemeContent_NoWallpaper(t *testing.T) {
	vars := map[string]string{"accent": "#cba6f7"}
	content := BuildThemeContent(vars, "", false)
	if strings.Contains(content, "\nWallpaper=") {
		t.Error("should not write Wallpaper key when path is empty")
	}
}

func TestBuildThemeContent_ColorizationColorFormat(t *testing.T) {
	vars := map[string]string{"accent": "#1e1e2e"}
	content := BuildThemeContent(vars, "", false)
	// The format must be 0XFF<RRGGBB> — uppercase X, special format.
	if !strings.Contains(content, "ColorizationColor=0XFF1E1E2E") {
		t.Errorf("ColorizationColor wrong format:\n%s", content)
	}
}

func TestBuildThemeContent_NoAccent(t *testing.T) {
	vars := map[string]string{"background": "#000000"}
	content := BuildThemeContent(vars, "", false)
	if strings.Contains(content, "ColorizationColor=") {
		t.Error("should not write ColorizationColor when accent is empty")
	}
}

func TestWriteThemeFile_FileWritten(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)

	vars := map[string]string{
		"accent": "#cba6f7",
		"name":   "Test Theme",
	}
	wallpaper := `C:\test\wallpaper.jpg`

	err := WriteThemeFile(vars, wallpaper, false)
	if err != nil {
		t.Fatalf("WriteThemeFile: %v", err)
	}

	themePath := filepath.Join(localAppData, "Microsoft", "Windows", "Themes", "Aether.theme")
	data, err := os.ReadFile(themePath)
	if err != nil {
		t.Fatalf("read Aether.theme: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "DisplayName=Aether - Test Theme") {
		t.Errorf("unexpected content:\n%s", content)
	}
	if !strings.Contains(content, "Wallpaper="+wallpaper) {
		t.Errorf("missing wallpaper in content:\n%s", content)
	}
}
