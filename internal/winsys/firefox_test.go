package winsys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindFirefoxProfileFromProfilesINI(t *testing.T) {
	appData := t.TempDir()
	root := filepath.Join(appData, "Mozilla", "Firefox")
	profile := filepath.Join(root, "Profiles", "abc.default-release")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	ini := `[General]
StartWithLastProfile=1

[Profile1]
Name=other
IsRelative=1
Path=Profiles/other.default

[Profile0]
Name=default
IsRelative=1
Path=Profiles/abc.default-release
Default=1
`
	if err := os.WriteFile(filepath.Join(root, "profiles.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findFirefoxProfile(appData)
	if err != nil {
		t.Fatalf("findFirefoxProfile: %v", err)
	}
	if got != profile {
		t.Errorf("profile = %q, want %q", got, profile)
	}
}

func TestFindFirefoxProfilePrefersInstallsINI(t *testing.T) {
	appData := t.TempDir()
	root := filepath.Join(appData, "Mozilla", "Firefox")
	active := filepath.Join(root, "Profiles", "active.default-release")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	installs := `[Install308046B0AF4A39CB]
Default=Profiles/active.default-release
Locked=1
`
	if err := os.WriteFile(filepath.Join(root, "installs.ini"), []byte(installs), 0o644); err != nil {
		t.Fatal(err)
	}
	profiles := `[Profile0]
Name=other
IsRelative=1
Path=Profiles/other.default
Default=1
`
	if err := os.WriteFile(filepath.Join(root, "profiles.ini"), []byte(profiles), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findFirefoxProfile(appData)
	if err != nil {
		t.Fatalf("findFirefoxProfile: %v", err)
	}
	if got != active {
		t.Errorf("profile = %q, want installs.ini profile %q", got, active)
	}
}

func TestFindFirefoxProfileMissing(t *testing.T) {
	if _, err := findFirefoxProfile(t.TempDir()); err == nil {
		t.Error("expected an error when no Firefox profile exists")
	}
}

func TestRenderFirefoxCSSUsesPalette(t *testing.T) {
	css := RenderFirefoxCSS(map[string]string{
		"background": "#101014",
		"foreground": "#f2eef7",
		"accent":     "#c09ef5",
		"dark_bg":    "#0a0a0e",
	})
	for _, want := range []string{
		"--aether-bg: #101014;",
		"--aether-accent: #c09ef5;",
		".tabbrowser-tab[selected] .tab-background",
		"#urlbar-background",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS missing %q", want)
		}
	}
	if !strings.HasPrefix(css, firefoxCSSMarker) {
		t.Error("CSS should start with the Aether marker")
	}

	fallback := RenderFirefoxCSS(nil)
	if !strings.Contains(fallback, "--aether-accent: #c09ef5;") {
		t.Error("default stylesheet should keep the lavender fallback accent")
	}
}

func TestFirefoxUserChromeRoundTrip(t *testing.T) {
	profile := t.TempDir()
	chromeDir := filepath.Join(profile, "chrome")
	if err := os.MkdirAll(chromeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(chromeDir, "userChrome.css")
	original := []byte("/* my existing chrome */\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{
		"background": "#101014",
		"foreground": "#f2eef7",
		"accent":     "#c09ef5",
	}
	if err := writeFirefoxUserChrome(profile, vars); err != nil {
		t.Fatalf("writeFirefoxUserChrome: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), firefoxCSSMarker) || !strings.Contains(string(data), "#101014") {
		t.Error("userChrome.css was not generated from the palette")
	}
	if _, err := os.Stat(target + firefoxBackupExt); err != nil {
		t.Errorf("expected a backup of the existing stylesheet: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(profile, "user.js")); err != nil || !strings.Contains(string(data), firefoxPrefLine) {
		t.Errorf("expected generated user.js enabling userChrome.css: %v", err)
	}

	if err := restoreFirefoxUserChrome(profile); err != nil {
		t.Fatalf("restoreFirefoxUserChrome: %v", err)
	}
	data, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Errorf("restored CSS = %q, want %q", data, original)
	}
	if _, err := os.Stat(filepath.Join(profile, "user.js")); !os.IsNotExist(err) {
		t.Error("generated user.js should have been removed")
	}
}
