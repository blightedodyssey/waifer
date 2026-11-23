//go:build windows

package winsys

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTerminalTestEnv points LOCALAPPDATA at a temp dir and creates the
// standard Windows Terminal settings path so the merge is exercised without
// touching a real installation.
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

func testPaletteVars() map[string]string {
	return map[string]string{
		"background":           "#07040c",
		"foreground":           "#dacbb2",
		"cursor":               "#ff9529",
		"selection_background": "#3f212f",
		"accent":               "#ac1c3b",
		"dark_bg":              "#050309",
		"black":                "#07040c",
		"red":                  "#5f2b29",
		"green":                "#cb4d05",
		"yellow":               "#ff9529",
		"blue":                 "#3f212f",
		"magenta":              "#ac1c3b",
		"cyan":                 "#bc7100",
		"white":                "#dacbb2",
		"bright_black":         "#1f1e20",
		"bright_red":           "#a5241f",
		"bright_green":         "#d86600",
		"bright_yellow":        "#fe8400",
		"bright_blue":          "#622441",
		"bright_magenta":       "#fe0046",
		"bright_cyan":          "#ae8000",
		"bright_white":         "#ffdddd",
	}
}

// readSettings parses the settings file, stripping JSONC if the merger kept any.
func readSettings(t *testing.T, path string) (map[string]any, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var root map[string]any
	cleaned := removeTrailingCommas(stripJSONComments(raw))
	if err := json.Unmarshal(cleaned, &root); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(raw))
	}
	return root, string(raw)
}

const realisticSettings = `{
    // Windows Terminal settings with user customisations
    "$schema": "https://aka.ms/terminal-profiles-schema",
    "defaultProfile": "{abc-123}",
    "copyOnSelect": true,
    "profiles": {
        "defaults": {
            "font": {"face": "Cascadia Code", "size": 11}
        },
        "list": [
            {
                "guid": "{abc-123}",
                "name": "PowerShell",
                "hidden": false,
                "colorScheme": "One Half Dark",
            },
            {
                "guid": "{def-456}",
                "name": "Ubuntu",
                "source": "CanonicalGroupLimited.Ubuntu",
                "hidden": false,
            },
        ]
    },
    "keybindings": [
        {"command": "copy", "keys": "ctrl+c"},
        {"command": "paste", "keys": "ctrl+v"},
    ],
    "schemes": [
        {"name": "One Half Dark", "background": "#282C34", "foreground": "#DCDFE4"}
    ],
    "themes": [
        {"name": "user-theme", "tab": {"background": "#123456"}}
    ],
}`

func TestApplyWindowsTerminalPreservesUnrelatedKeys(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)
	if err := os.WriteFile(settingsPath, []byte(realisticSettings), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("ApplyWindowsTerminal: %v", err)
	}

	root, _ := readSettings(t, settingsPath)

	// Untouched scalars survive verbatim.
	if root["defaultProfile"] != "{abc-123}" {
		t.Errorf("defaultProfile = %v, want it preserved", root["defaultProfile"])
	}
	if root["copyOnSelect"] != true {
		t.Errorf("copyOnSelect = %v, want true", root["copyOnSelect"])
	}
	if root["$schema"] != "https://aka.ms/terminal-profiles-schema" {
		t.Errorf("$schema was dropped")
	}

	// Keybindings survive byte-for-byte (as decoded values).
	keys, ok := root["keybindings"].([]any)
	if !ok || len(keys) != 2 {
		t.Fatalf("keybindings = %v, want 2 entries", root["keybindings"])
	}
	first := keys[0].(map[string]any)
	if first["command"] != "copy" || first["keys"] != "ctrl+c" {
		t.Errorf("keybindings[0] = %v, want copy/ctrl+c", first)
	}

	// Profiles survive, including the one with source and the custom font.
	profiles := root["profiles"].(map[string]any)
	list := profiles["list"].([]any)
	if len(list) != 2 {
		t.Fatalf("profiles.list length = %d, want 2", len(list))
	}
	if got := list[1].(map[string]any)["name"]; got != "Ubuntu" {
		t.Errorf("profiles.list[1].name = %v, want Ubuntu", got)
	}
	defaults := profiles["defaults"].(map[string]any)
	font := defaults["font"].(map[string]any)
	if font["face"] != "Cascadia Code" {
		t.Errorf("profiles.defaults.font.face was lost: %v", font)
	}

	// Foreign schemes and themes survive.
	schemes := root["schemes"].([]any)
	if len(schemes) != 2 {
		t.Fatalf("schemes length = %d, want 2 (One Half Dark + Waifer)", len(schemes))
	}
	themes := root["themes"].([]any)
	if len(themes) != 2 {
		t.Fatalf("themes length = %d, want 2 (user-theme + Waifer)", len(themes))
	}
}

func TestApplyWindowsTerminalUpsertsWaiferSchemeAndTheme(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)
	if err := os.WriteFile(settingsPath, []byte(realisticSettings), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("ApplyWindowsTerminal: %v", err)
	}
	root, _ := readSettings(t, settingsPath)

	// Root chrome keys.
	if root["theme"] != "Waifer" {
		t.Errorf("theme = %v, want Waifer", root["theme"])
	}
	if root["launchMode"] != "focus" {
		t.Errorf("launchMode = %v, want focus", root["launchMode"])
	}
	if root["alwaysShowTabs"] != false {
		t.Errorf("alwaysShowTabs = %v, want false", root["alwaysShowTabs"])
	}
	if root["showTabsInTitlebar"] != true {
		t.Errorf("showTabsInTitlebar = %v, want true", root["showTabsInTitlebar"])
	}

	// profiles.defaults gets the glass canvas.
	defaults := root["profiles"].(map[string]any)["defaults"].(map[string]any)
	if defaults["colorScheme"] != "Waifer" {
		t.Errorf("defaults.colorScheme = %v, want Waifer", defaults["colorScheme"])
	}
	if defaults["useAcrylic"] != true {
		t.Errorf("defaults.useAcrylic = %v, want true", defaults["useAcrylic"])
	}
	if defaults["opacity"] != float64(85) {
		t.Errorf("defaults.opacity = %v, want 85", defaults["opacity"])
	}
	if defaults["padding"] != "12" {
		t.Errorf("defaults.padding = %v, want 12", defaults["padding"])
	}

	// Scheme: all 16 ANSI colors plus the four accent surfaces.
	scheme := findNamed(root["schemes"].([]any), "Waifer")
	if scheme == nil {
		t.Fatal("Waifer scheme missing")
	}
	for _, key := range []string{
		"background", "foreground", "cursorColor", "selectionBackground",
		"black", "red", "green", "yellow", "blue", "purple", "cyan", "white",
		"brightBlack", "brightRed", "brightGreen", "brightYellow",
		"brightBlue", "brightPurple", "brightCyan", "brightWhite",
	} {
		value, ok := scheme[key].(string)
		if !ok || !strings.HasPrefix(value, "#") || len(value) != 7 {
			t.Errorf("scheme[%q] = %v, want a #rrggbb color", key, scheme[key])
		}
	}
	if scheme["background"] != "#07040c" {
		t.Errorf("scheme.background = %v, want #07040c", scheme["background"])
	}

	// Theme: accent frame (alpha trailing), surface unfocused frame, banner.
	theme := findNamed(root["themes"].([]any), "Waifer")
	if theme == nil {
		t.Fatal("Waifer theme missing")
	}
	window := theme["window"].(map[string]any)
	if window["frame"] != "#ac1c3bFF" {
		t.Errorf("window.frame = %v, want #ac1c3bFF", window["frame"])
	}
	if window["unfocusedFrame"] != "#050309FF" {
		t.Errorf("window.unfocusedFrame = %v, want #050309FF", window["unfocusedFrame"])
	}
	tabRow := theme["tabRow"].(map[string]any)
	if tabRow["background"] != "#ac1c3bCC" {
		t.Errorf("tabRow.background = %v, want the accent banner #ac1c3bCC", tabRow["background"])
	}
	tab := theme["tab"].(map[string]any)
	if tab["background"] != "#07040cE6" {
		t.Errorf("tab.background = %v, want #07040cE6", tab["background"])
	}
	if tab["showCloseButton"] != "never" {
		t.Errorf("tab.showCloseButton = %v, want never", tab["showCloseButton"])
	}
}

func TestApplyWindowsTerminalStripsProfileSchemeOverrides(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)

	initial := `{
    "profiles": {
        "list": [
            {"guid": "{1}", "name": "PowerShell", "hidden": false, "colorScheme": "Campbell"},
            {"guid": "{2}", "name": "Locked", "hidden": false, "colorScheme": "One Half Dark", "colorSchemeLocked": true}
        ]
    }
}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}
	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("ApplyWindowsTerminal: %v", err)
	}

	root, _ := readSettings(t, settingsPath)
	list := root["profiles"].(map[string]any)["list"].([]any)

	if _, has := list[0].(map[string]any)["colorScheme"]; has {
		t.Errorf("unlocked profile kept its colorScheme override")
	}
	if got := list[1].(map[string]any)["colorScheme"]; got != "One Half Dark" {
		t.Errorf("locked profile colorScheme = %v, want One Half Dark preserved", got)
	}
}

func TestApplyWindowsTerminalSafetyLockBlocksUnsafeDocuments(t *testing.T) {
	cases := map[string]struct {
		settings string
		wantErr  error
	}{
		"empty list": {
			settings: `{"profiles": {"list": []}}`,
			wantErr:  ErrTerminalProfilesEmpty,
		},
		"missing list": {
			settings: `{"profiles": {"defaults": {}}}`,
			wantErr:  ErrTerminalProfilesEmpty,
		},
		"all hidden": {
			settings: `{"profiles": {"list": [
                {"guid": "{1}", "name": "A", "hidden": true},
                {"guid": "{2}", "name": "B", "hidden": true}
            ]}}`,
			wantErr: ErrTerminalAllHidden,
		},
		"missing profiles": {
			settings: `{"schemes": []}`,
			wantErr:  ErrTerminalProfilesMissing,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, settingsPath := setupTerminalTestEnv(t)
			if err := os.WriteFile(settingsPath, []byte(tc.settings), 0o644); err != nil {
				t.Fatalf("write initial settings: %v", err)
			}

			err := ApplyWindowsTerminal(testPaletteVars(), "")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}

			after, readErr := os.ReadFile(settingsPath)
			if readErr != nil {
				t.Fatalf("read settings: %v", readErr)
			}
			if string(after) != tc.settings {
				t.Errorf("settings.json was modified despite a refused merge")
			}
		})
	}
}

func TestApplyWindowsTerminalIsIdempotent(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)
	if err := os.WriteFile(settingsPath, []byte(realisticSettings), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after first apply: %v", err)
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after second apply: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("second apply changed the file; merges should be stable")
	}
	infoAfter, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !infoAfter.ModTime().Equal(info.ModTime()) {
		t.Errorf("second apply rewrote the file; expected a no-op")
	}

	// A palette switch must still go through.
	switched := testPaletteVars()
	switched["background"] = "#0f0e17"
	switched["accent"] = "#7f5af0"
	if err := ApplyWindowsTerminal(switched, ""); err != nil {
		t.Fatalf("palette switch: %v", err)
	}
	third, _ := os.ReadFile(settingsPath)
	if string(third) == string(first) {
		t.Errorf("palette switch did not update settings.json")
	}
	root, _ := readSettings(t, settingsPath)
	scheme := findNamed(root["schemes"].([]any), "Waifer")
	if scheme["background"] != "#0f0e17" {
		t.Errorf("scheme.background = %v, want #0f0e17 after switch", scheme["background"])
	}
}

func TestApplyWindowsTerminalBacksUpOnceAndKeepsValidJSON(t *testing.T) {
	_, settingsPath := setupTerminalTestEnv(t)
	if err := os.WriteFile(settingsPath, []byte(realisticSettings), 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}
	if err := ApplyWindowsTerminal(testPaletteVars(), ""); err != nil {
		t.Fatalf("ApplyWindowsTerminal: %v", err)
	}

	backupPath := settingsPath + terminalBackupExt
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(backup) != realisticSettings {
		t.Errorf("backup does not hold the pre-merge document")
	}

	// A second apply must not overwrite the original backup.
	switched := testPaletteVars()
	switched["background"] = "#123456"
	if err := ApplyWindowsTerminal(switched, ""); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	backupAgain, _ := os.ReadFile(backupPath)
	if string(backupAgain) != realisticSettings {
		t.Errorf("backup was overwritten on the second apply")
	}
}

func findNamed(entries []any, name string) map[string]any {
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := m["name"].(string); strings.EqualFold(got, name) {
			return m
		}
	}
	return nil
}
