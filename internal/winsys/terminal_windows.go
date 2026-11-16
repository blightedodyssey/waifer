//go:build windows

package winsys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"aether/internal/platform"
)

// Packaged and unpackaged Windows Terminal data locations.
var terminalSettingsCandidates = []struct {
	pkg  string
	file string
}{
	{"Microsoft.WindowsTerminal_8wekyb3d8bbwe", "settings.json"},
	{"Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "settings.json"},
	{"Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe", "settings.json"},
}

// terminalSchemeName is the scheme Aether writes into Windows Terminal.
const terminalSchemeName = "Aether"

// findTerminalSettings returns every existing Windows Terminal settings.json
// path (packaged LocalState dirs plus the unpackaged install).
func findTerminalSettings() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}

	var out []string
	for _, candidate := range terminalSettingsCandidates {
		p := filepath.Join(localAppData, "Packages", candidate.pkg, "LocalState", candidate.file)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	unpackaged := filepath.Join(localAppData, "Microsoft", "Windows Terminal", "settings.json")
	if _, err := os.Stat(unpackaged); err == nil {
		out = append(out, unpackaged)
	}
	return out
}

// terminalScheme is the typed shape of a Windows Terminal color scheme.
type terminalScheme map[string]interface{}

// buildTerminalScheme converts a rendered windows-terminal.json template
// into the object stored in the schemes array.
func buildTerminalScheme(content string) (terminalScheme, error) {
	parsed, err := parseJSONC(content)
	if err != nil {
		return nil, err
	}
	if name, ok := parsed["name"].(string); ok {
		parsed["name"] = name
	} else {
		parsed["name"] = terminalSchemeName
	}
	return parsed, nil
}

// upsertScheme inserts or replaces the Aether scheme in the parsed settings
// document.
func upsertScheme(doc map[string]interface{}, scheme terminalScheme) {
	schemes, _ := doc["schemes"].([]interface{})
	replaced := false
	for i, entry := range schemes {
		if m, ok := entry.(map[string]interface{}); ok {
			if name, _ := m["name"].(string); name == terminalSchemeName {
				schemes[i] = scheme
				replaced = true
				break
			}
		}
	}
	if !replaced {
		schemes = append(schemes, scheme)
	}
	doc["schemes"] = schemes
}

// ApplyWindowsTerminal renders the terminal scheme into every detected
// Windows Terminal settings.json.
func ApplyWindowsTerminal(themeDir string) error {
	src := filepath.Join(themeDir, "windows-terminal.json")
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", src, err)
	}
	scheme, err := buildTerminalScheme(string(data))
	if err != nil {
		return fmt.Errorf("windows terminal scheme: %w", err)
	}

	targets := findTerminalSettings()
	if len(targets) == 0 {
		return nil // no Windows Terminal installed
	}
	var lastErr error
	for _, target := range targets {
		if err := applyTerminalSettings(target, scheme); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// applyTerminalSettings merges the scheme into one settings.json.
func applyTerminalSettings(path string, scheme terminalScheme) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	doc, err := parseJSONC(string(raw))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	upsertScheme(doc, scheme)

	// Point default profiles at the scheme.
	switch profiles := doc["profiles"].(type) {
	case map[string]interface{}:
		profiles["colorScheme"] = terminalSchemeName
	case []interface{}:
		// profiles is a plain list: add a defaults object alongside it.
		doc["profiles"] = map[string]interface{}{
			"defaults":    map[string]interface{}{"colorScheme": terminalSchemeName},
			"list":        profiles,
		}
	default:
		doc["profiles"] = map[string]interface{}{"colorScheme": terminalSchemeName}
	}

	out, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return platform.WriteText(path, string(out)+"\n")
}
