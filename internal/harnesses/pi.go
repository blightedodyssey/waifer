package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
)

func detectPi() (string, bool) {
	dir := filepath.Join(homeDir(), ".pi", "agent")
	if _, err := os.Stat(dir); err == nil {
		return dir, true
	}
	return "", false
}

func applyPi(vars map[string]string) ([]string, []string) {
	piDir := filepath.Join(homeDir(), ".pi", "agent")
	if _, err := os.Stat(piDir); os.IsNotExist(err) {
		return nil, nil
	}

	var applied []string
	var warnings []string

	bg := firstHex(vars["background"], "#1e1e2e")
	fg := firstHex(vars["foreground"], "#cdd6f4")
	accent := firstHex(vars["accent"], "#89b4fa")
	surface := firstHex(vars["dark_bg"], "#181825")
	muted := firstHex(vars["bright_black"], "#45475a")

	// 1. Write theme JSON
	themeDir := filepath.Join(piDir, "themes")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		warnings = append(warnings, fmt.Sprintf("pi themes dir: %v", err))
		return applied, warnings
	}

	theme := map[string]interface{}{
		"name": "aether",
		"type": "dark",
		"colors": map[string]interface{}{
			"accent":          accent,
			"border":          bg,
			"text":            fg,
			"muted":           muted,
			"success":         firstHex(vars["base0B"], "#a6e3a1"),
			"warning":         firstHex(vars["base0A"], "#f9e2af"),
			"error":           firstHex(vars["base08"], "#f38ba8"),
			"selectedBg":      surface,
			"userMessageBg":   surface,
			"toolPendingBg":   bg,
		},
	}

	themeData, _ := jsonMarshal(theme)
	themePath := filepath.Join(themeDir, "aether.json")
	if err := writeFileAtomic(themePath, themeData); err != nil {
		warnings = append(warnings, fmt.Sprintf("pi theme file: %v", err))
		return applied, warnings
	}
	applied = append(applied, "pi-theme")

	// 2. Safely update settings.json "theme" key only
	settingsPath := filepath.Join(piDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		err := safeMergeJSON(settingsPath, func(data map[string]interface{}) error {
			data["theme"] = "aether"
			return nil
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("pi settings.json: %v", err))
		} else {
			applied = append(applied, "pi-config")
		}
	}

	return applied, warnings
}
