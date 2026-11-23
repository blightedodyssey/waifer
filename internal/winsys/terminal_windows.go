//go:build windows

package winsys

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// terminalSchemeName is the color scheme waifer owns inside settings.json.
	terminalSchemeName = "Waifer"
	// terminalThemeName is the UI theme object waifer owns inside settings.json.
	// A theme object carries window chrome (frame colors) and the tab row, which
	// is the "horizontal colored banner" above the panes.
	terminalThemeName = "Waifer"
	terminalSettings  = "settings.json"
	terminalOpacity   = 85
	terminalPadding   = "12"
	terminalBackupExt = ".aether-backup"
)

// Sentinel errors so callers and tests can distinguish "we refused to write"
// from io/parse failures. Refusing is always preferable to writing a file that
// makes Windows Terminal report "All profiles were hidden in your settings".
var (
	ErrTerminalProfilesMissing = errors.New("windows terminal profiles key is missing or unexpected type; file left unchanged")
	ErrTerminalProfilesEmpty   = errors.New("windows terminal profiles list is empty; refusing to write because Windows Terminal would hide every profile")
	ErrTerminalAllHidden       = errors.New("every windows terminal profile is hidden; refusing to write because Windows Terminal would report no visible profiles")
)

// terminalMu serialises every read-modify-write against settings.json. The
// writer pipeline and the native sync both funnel through
// ApplyWindowsTerminal, so a single lock is enough to prevent interleaved
// merges (the original cause of truncated settings files).
var (
	terminalMu sync.Mutex
	// terminalLastOut remembers the last payload written per path so repeated
	// applies with an unchanged palette are a no-op instead of a file rewrite
	// (no notification churn, no risk of racing the Terminal's own save).
	terminalLastOut = map[string]string{}
)

// terminalSettingsCandidates returns every Windows Terminal settings file that
// may exist, Store install first, then Preview/Canary and the unpackaged
// build.
func terminalSettingsCandidates() []string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	packages := filepath.Join(local, "Packages")
	return []string{
		filepath.Join(packages, "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(packages, "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(packages, "Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(local, "Microsoft", "Windows Terminal", terminalSettings),
	}
}

func findTerminalSettings() string {
	for _, p := range terminalSettingsCandidates() {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// ApplyWindowsTerminal merges the active waifer palette into every installed
// Windows Terminal settings.json. It is the ONLY writer of that file: it never
// replaces the document, it mutates the keys waifer owns and preserves every
// other key, profile, action and keybinding.
func ApplyWindowsTerminal(vars map[string]string, themeDir string) error {
	terminalMu.Lock()
	defer terminalMu.Unlock()

	var existing []string
	for _, p := range terminalSettingsCandidates() {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return fmt.Errorf("Windows Terminal settings.json not found (is Windows Terminal installed?)")
	}

	scheme := terminalScheme(themeDir, vars)
	theme := terminalTheme(vars)

	var errs []error
	applied := 0
	for _, settingsPath := range existing {
		if err := applyTerminalSettingsFile(settingsPath, scheme, theme); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", settingsPath, err))
			continue
		}
		applied++
	}
	// A refusal on one install (e.g. an empty profiles list) must not hide a
	// successful merge on another, but it must still surface to the caller.
	if applied == 0 {
		return errors.Join(errs...)
	}
	return errors.Join(errs...)
}

func applyTerminalSettingsFile(settingsPath string, scheme, theme map[string]any) error {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("read Windows Terminal settings: %w", err)
	}

	// JSONC-aware parse: Windows Terminal writes comments and trailing commas,
	// and its own saves must never break the merge.
	root, err := parseJSONC(raw)
	if err != nil {
		return fmt.Errorf("parse Windows Terminal settings: %w", err)
	}
	if root == nil {
		return fmt.Errorf("parse Windows Terminal settings: document is not a JSON object")
	}

	profiles, ok := root["profiles"].(map[string]any)
	if !ok || profiles == nil {
		return ErrTerminalProfilesMissing
	}

	// --- Profile Safety Lock -------------------------------------------------
	// Validate BEFORE touching the document. Abort on any shape that would make
	// Windows Terminal load zero visible profiles.
	if err := validateVisibleProfiles(profiles); err != nil {
		return err
	}

	// --- waifer-managed keys only -------------------------------------------
	upsertScheme(root, scheme)
	upsertTheme(root, theme)

	root["theme"] = terminalThemeName
	root["launchMode"] = "focus"
	root["alwaysShowTabs"] = false
	root["showTabsInTitlebar"] = true

	applyTerminalDefaults(profiles)
	stripProfileSchemeOverrides(profiles)

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Windows Terminal settings: %w", err)
	}
	encoded = append(encoded, '\n')

	if terminalLastOut[settingsPath] == string(encoded) {
		return nil // already in the desired state; nothing to write
	}

	if err := backupOnce(settingsPath); err != nil {
		return fmt.Errorf("back up Windows Terminal settings: %w", err)
	}
	if err := writeFileAtomic(settingsPath, encoded); err != nil {
		return fmt.Errorf("write Windows Terminal settings: %w", err)
	}
	terminalLastOut[settingsPath] = string(encoded)
	return nil
}

func backupOnce(settingsPath string) error {
	backup := settingsPath + terminalBackupExt
	if _, err := os.Stat(backup); err == nil {
		return nil
	}
	return copyFile(settingsPath, backup)
}

// validateVisibleProfiles enforces the safety invariant on profiles.list.
// Windows Terminal accepts profiles.list as an array of objects, or (newer
// schema) as an object keyed by profile GUID; both are inspected.
func validateVisibleProfiles(profiles map[string]any) error {
	entries := profileEntries(profiles)
	if entries == nil {
		return ErrTerminalProfilesEmpty
	}
	if len(entries) == 0 {
		return ErrTerminalProfilesEmpty
	}
	for _, prof := range entries {
		if prof == nil {
			continue
		}
		if hidden, _ := prof["hidden"].(bool); !hidden {
			return nil
		}
	}
	return ErrTerminalAllHidden
}

// profileEntries returns the profile objects in profiles.list, or nil when the
// list is absent/unusable.
func profileEntries(profiles map[string]any) []map[string]any {
	switch list := profiles["list"].(type) {
	case []any:
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if prof, ok := item.(map[string]any); ok {
				out = append(out, prof)
			}
		}
		return out
	case map[string]any:
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if prof, ok := item.(map[string]any); ok {
				out = append(out, prof)
			}
		}
		return out
	default:
		return nil
	}
}

func applyTerminalDefaults(profiles map[string]any) {
	defaults, ok := profiles["defaults"].(map[string]any)
	if !ok || defaults == nil {
		defaults = map[string]any{}
		profiles["defaults"] = defaults
	}
	defaults["colorScheme"] = terminalSchemeName
	defaults["useAcrylic"] = true
	defaults["opacity"] = terminalOpacity
	defaults["padding"] = terminalPadding
	// A visible scrollbar breaks the flush-glass look; hide it by default but
	// leave any explicit user choice untouched.
	if _, set := defaults["scrollbarState"]; !set {
		defaults["scrollbarState"] = "hidden"
	}
}

// stripProfileSchemeOverrides removes per-profile colorScheme overrides so a
// live waifer switch reaches every shell immediately. A profile can opt out by
// setting "colorSchemeLocked": true.
func stripProfileSchemeOverrides(profiles map[string]any) {
	for _, prof := range profileEntries(profiles) {
		if prof == nil {
			continue
		}
		if locked, _ := prof["colorSchemeLocked"].(bool); locked {
			continue
		}
		delete(prof, "colorScheme")
	}
}

// terminalScheme reads the rendered template from the theme directory, falling
// back to building the scheme straight from the palette variables. Every color
// is normalised to "#rrggbb" so Windows Terminal's parser never sees a bare
// hex string or a stray placeholder.
func terminalScheme(themeDir string, vars map[string]string) map[string]any {
	if themeDir != "" {
		if data, err := os.ReadFile(filepath.Join(themeDir, "windows-terminal.json")); err == nil {
			var scheme map[string]any
			if json.Unmarshal(data, &scheme) == nil && len(scheme) > 0 {
				scheme["name"] = terminalSchemeName
				normalizeSchemeColors(scheme)
				return scheme
			}
		}
	}
	scheme := map[string]any{
		"name":                terminalSchemeName,
		"background":          vars["background"],
		"foreground":          vars["foreground"],
		"cursorColor":         vars["cursor"],
		"selectionBackground": vars["selection_background"],
		"black":               vars["black"],
		"red":                 vars["red"],
		"green":               vars["green"],
		"yellow":              vars["yellow"],
		"blue":                vars["blue"],
		"purple":              vars["magenta"],
		"cyan":                vars["cyan"],
		"white":               vars["white"],
		"brightBlack":         vars["bright_black"],
		"brightRed":           vars["bright_red"],
		"brightGreen":         vars["bright_green"],
		"brightYellow":        vars["bright_yellow"],
		"brightBlue":          vars["bright_blue"],
		"brightPurple":        vars["bright_magenta"],
		"brightCyan":          vars["bright_cyan"],
		"brightWhite":         vars["bright_white"],
	}
	normalizeSchemeColors(scheme)
	return scheme
}

func normalizeSchemeColors(scheme map[string]any) {
	for key, value := range scheme {
		if key == "name" {
			continue
		}
		if s, ok := value.(string); ok {
			scheme[key] = opaqueColor(s)
		}
	}
}

// terminalTheme builds the UI theme object that gives Windows Terminal the
// Omarchy/Aether look: opaque accent frame, dimmer unfocused frame, a colored
// banner across the tab row, and pane-colored tabs with no close button.
func terminalTheme(vars map[string]string) map[string]any {
	background := opaqueColor(vars["background"])
	accent := opaqueColor(vars["accent"])
	surface := opaqueColor(vars["dark_bg"])
	if surface == "" {
		surface = opaqueColor(vars["background"])
	}

	return map[string]any{
		"name": terminalThemeName,
		"window": map[string]any{
			"applicationTheme": "dark",
			"frame":            withAlpha(accent, "FF"),
			"unfocusedFrame":   withAlpha(surface, "FF"),
		},
		// tabRow is the title bar (showTabsInTitlebar = true): this is the
		// horizontal accent banner that recolors on every waifer switch.
		"tabRow": map[string]any{
			"background":          withAlpha(accent, "CC"),
			"unfocusedBackground": withAlpha(accent, "99"),
		},
		// Tabs blend into the pane so a single shell looks frameless.
		"tab": map[string]any{
			"background":          withAlpha(background, "E6"),
			"unfocusedBackground": withAlpha(background, "80"),
			"showCloseButton":     "never",
		},
	}
}

// opaqueColor normalises any hex string to "#rrggbb". Invalid input yields an
// empty string so callers can fall back instead of writing a broken color.
func opaqueColor(value string) string {
	hex := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(hex) == 8 {
		// Drop an alpha channel: scheme colors must be opaque RGB(A).
		hex = hex[:6]
	}
	if len(hex) != 6 {
		return ""
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return "#" + hex
}

// withAlpha appends an RGBA alpha channel. Windows Terminal theme colors are
// documented as "#rrggbbaa" (alpha last).
func withAlpha(rgb, alpha string) any {
	if rgb == "" {
		if alpha == "FF" {
			return nil
		}
		return nil
	}
	return rgb + alpha
}

func upsertScheme(root map[string]any, scheme map[string]any) {
	schemes, _ := root["schemes"].([]any)
	kept := make([]any, 0, len(schemes)+1)
	for _, entry := range schemes {
		m, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if name, _ := m["name"].(string); strings.EqualFold(name, terminalSchemeName) {
			continue
		}
		kept = append(kept, entry)
	}
	root["schemes"] = append(kept, scheme)
}

func upsertTheme(root map[string]any, theme map[string]any) {
	themes, _ := root["themes"].([]any)
	kept := make([]any, 0, len(themes)+1)
	for _, entry := range themes {
		m, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		if name, _ := m["name"].(string); strings.EqualFold(name, terminalThemeName) {
			continue
		}
		kept = append(kept, entry)
	}
	root["themes"] = append(kept, theme)
}
