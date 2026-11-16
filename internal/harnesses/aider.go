package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func detectAider() (string, bool) {
	// Aider uses ~/.aider.conf.yml in home directory
	confPath := filepath.Join(homeDir(), ".aider.conf.yml")
	if _, err := os.Stat(confPath); err == nil {
		return homeDir(), true
	}
	return "", false
}

func applyAider(vars map[string]string) ([]string, []string) {
	var applied []string
	var warnings []string

	confPath := filepath.Join(homeDir(), ".aider.conf.yml")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return applied, warnings
	}

	// Read existing config
	raw, err := os.ReadFile(confPath)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("aider read config: %v", err))
		return applied, warnings
	}

	// Colors to set
	bg := firstHex(vars["background"], "#1e1e2e")
	fg := firstHex(vars["foreground"], "#cdd6f4")
	accent := firstHex(vars["accent"], "#89b4fa")
	muted := firstHex(vars["bright_black"], "#45475a")

	themeColors := map[string]string{
		"user-input-color":           accent,
		"tool-output-color":           fg,
		"assistant-output-color":      fg,
		"completion-menu-color":       fg,
		"completion-menu-bg-color":    bg,
		"completion-menu-current-color": accent,
		"completion-menu-current-bg-color": muted,
	}

	// Parse existing YAML and update only theme color keys
	lines := strings.Split(string(raw), "\n")
	existingKeys := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		for key := range themeColors {
			if strings.HasPrefix(trimmed, key+":") {
				lines[i] = fmt.Sprintf("%s: %s", key, themeColors[key])
				existingKeys[key] = true
				break
			}
		}
	}

	// Append any missing keys at the end with a marker
	var missing []string
	for key, val := range themeColors {
		if !existingKeys[key] {
			missing = append(missing, fmt.Sprintf("%s: %s", key, val))
		}
	}

	if len(missing) > 0 {
		lines = append(lines, "")
		lines = append(lines, "# Aether theme overrides (safe merge)")
		lines = append(lines, missing...)
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}

	if err := backupAndWrite(confPath, []byte(out)); err != nil {
		warnings = append(warnings, fmt.Sprintf("aider write config: %v", err))
	} else {
		applied = append(applied, "aider")
	}

	return applied, warnings
}
