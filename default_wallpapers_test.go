package main

import (
	"os"
	"path/filepath"
	"testing"

	"aether/internal/platform"
)

func TestInstallDefaultWallpapers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	if err := installDefaultWallpapers(); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(platform.WallpaperDir(), "Aether")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), 8; got != want {
		t.Fatalf("bundled wallpaper count = %d; want %d", got, want)
	}

	path := filepath.Join(dir, "aether-dark-2k.jpg")
	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installDefaultWallpapers(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "custom"; got != want {
		t.Errorf("existing wallpaper = %q; want %q", got, want)
	}
}

func TestInstallDefaultWallpapersUsesConfiguredFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	folder := t.TempDir()
	if err := NewApp().SaveSettings(map[string]interface{}{
		wallpaperFolderSetting: folder,
	}); err != nil {
		t.Fatal(err)
	}

	if err := installDefaultWallpapers(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(folder, "Aether"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), 8; got != want {
		t.Fatalf("bundled wallpaper count = %d; want %d", got, want)
	}
	if _, err := os.Stat(filepath.Join(home, "Wallpapers")); !os.IsNotExist(err) {
		t.Errorf("~/Wallpapers stat error = %v; want not exist", err)
	}
}

func TestInstallDefaultWallpapersSkipsMissingConfiguredFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	// Use a path entirely outside TempDir so the self-heal leak-detector
	// never matches it.
	missing := "C:\\__aether_test_missing_folder__\\wallpapers"
	if err := NewApp().SaveSettings(map[string]interface{}{
		wallpaperFolderSetting: missing,
	}); err != nil {
		t.Fatal(err)
	}

	if err := installDefaultWallpapers(); err == nil {
		t.Fatal("installDefaultWallpapers() error = nil; want missing folder error")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("configured folder stat error = %v; want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Wallpapers")); !os.IsNotExist(err) {
		t.Errorf("~/Wallpapers stat error = %v; want not exist", err)
	}
}
