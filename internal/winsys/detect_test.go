package winsys

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setupDetectTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	appData := filepath.Join(home, "AppData", "Roaming")
	localAppData := filepath.Join(home, "AppData", "Local")
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	for _, dir := range []string{appData, localAppData, configHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestDetectReturnsFullKeySet(t *testing.T) {
	setupDetectTestEnv(t)
	got := Detect()
	for _, name := range detectTargetNames {
		if _, ok := got[name]; !ok {
			t.Errorf("Detect() is missing key %q", name)
		}
	}
	// Windhawk keys are deliberately not asserted: the runner may have real
	// mods installed and detection must never deny a real installation.
	if runtime.GOOS == "windows" {
		if !got["system"] || !got["windows-terminal"] {
			t.Error("vanilla Windows and Windows Terminal must always be detected")
		}
	} else if got["system"] || got["windows-terminal"] {
		t.Error("Windows-only targets must be false on other platforms")
	}
}

func TestDetectFilesystemTargets(t *testing.T) {
	home := setupDetectTestEnv(t)
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")
	writeDetectFile(t, filepath.Join(home, ".config", "yasb", "styles.css"))
	writeDetectFile(t, filepath.Join(home, ".config", "glazewm", "config.yaml"))
	writeDetectFile(t, filepath.Join(appData, "discord", "settings.json"))
	writeDetectFile(t, filepath.Join(localAppData, "Vencord", "settings.json"))

	got := Detect()
	for _, app := range []string{"yasb", "glazewm", "discord", "vencord"} {
		if runtime.GOOS == "windows" && !got[app] {
			t.Errorf("Detect()[%q] = false, want true for staged config", app)
		}
		if runtime.GOOS != "windows" && got[app] {
			t.Errorf("Detect()[%q] = true on non-Windows", app)
		}
	}
}

func writeDetectFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}
