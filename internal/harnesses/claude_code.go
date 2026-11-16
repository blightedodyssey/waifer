package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
)

func detectClaudeCode() (string, bool) {
	configDir := filepath.Join(homeDir(), ".claude")
	// Claude Code uses ~/.claude.json at the home root, not a directory
	configFile := filepath.Join(homeDir(), ".claude.json")
	if _, err := os.Stat(configFile); err == nil {
		return homeDir(), true
	}
	// Also check for the .claude directory as a proxy
	if _, err := os.Stat(configDir); err == nil {
		return homeDir(), true
	}
	return "", false
}

func applyClaudeCode(vars map[string]string) ([]string, []string) {
	var applied []string
	var warnings []string

	mode := "dark-ansi"
	if vars["theme_type"] == "light" || vars["mode"] == "light" {
		mode = "light-ansi"
	}

	// Claude Code uses ~/.claude.json in the home directory.
	// We safely set only the "theme" key, preserving apiKey, mcpServers, etc.
	configPath := filepath.Join(homeDir(), ".claude.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Not installed, skip silently
		return applied, warnings
	}

	err := safeMergeJSON(configPath, func(data map[string]interface{}) error {
		data["theme"] = mode
		return nil
	})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("claude-code .claude.json: %v", err))
	} else {
		applied = append(applied, "claude-code")
	}

	return applied, warnings
}
