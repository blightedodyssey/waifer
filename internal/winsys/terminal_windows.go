//go:build windows

package winsys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	terminalSchemeName = "Aether"
	terminalSettings   = "settings.json"
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
	candidates := []string{
		filepath.Join(packages, "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(packages, "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(packages, "Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe", "LocalState", terminalSettings),
		filepath.Join(local, "Microsoft", "Windows Terminal", terminalSettings),
	}
	return candidates
}

func findTerminalSettings() string {
	for _, p := range terminalSettingsCandidates() {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// ApplyWindowsTerminal registers (or refreshes) the Aether color scheme in
// Windows Terminal and makes it the default for every profile. The original
// settings file is backed up once as settings.json.aether-backup.
func ApplyWindowsTerminal(vars map[string]string, themeDir string) error {
	settingsPath := findTerminalSettings()
	if settingsPath == "" {
		return fmt.Errorf("Windows Terminal settings.json not found (is Windows Terminal installed?)")
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("read Windows Terminal settings: %w", err)
	}
	root, err := parseJSONC(raw)
	if err != nil {
		return fmt.Errorf("parse Windows Terminal settings: %w", err)
	}

	scheme := terminalScheme(themeDir, vars)
	upsertScheme(root, scheme)

	switch profiles := root["profiles"].(type) {
	case map[string]any:
		defaults, ok := profiles["defaults"].(map[string]any)
		if !ok {
			defaults = map[string]any{}
			profiles["defaults"] = defaults
		}
		defaults["colorScheme"] = terminalSchemeName
	case []any:
		// Very old Windows Terminal schemas store profiles as an array.
		for _, entry := range profiles {
			if profile, ok := entry.(map[string]any); ok {
				profile["colorScheme"] = terminalSchemeName
			}
		}
	default:
		root["profiles"] = map[string]any{
			"defaults": map[string]any{"colorScheme": terminalSchemeName},
		}
	}

	encoded, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("encode Windows Terminal settings: %w", err)
	}
	encoded = append(encoded, '\n')

	backup := settingsPath + ".aether-backup"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := copyFile(settingsPath, backup); err != nil {
			return fmt.Errorf("back up Windows Terminal settings: %w", err)
		}
	}
	if err := writeFileAtomic(settingsPath, encoded); err != nil {
		return fmt.Errorf("write Windows Terminal settings: %w", err)
	}
	return nil
}

// terminalScheme reads the rendered template from the theme directory, falling
// back to building the scheme straight from the palette variables.
func terminalScheme(themeDir string, vars map[string]string) map[string]any {
	if themeDir != "" {
		if data, err := os.ReadFile(filepath.Join(themeDir, "windows-terminal.json")); err == nil {
			var scheme map[string]any
			if json.Unmarshal(data, &scheme) == nil && len(scheme) > 0 {
				scheme["name"] = terminalSchemeName
				return scheme
			}
		}
	}
	return map[string]any{
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

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".aether-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
