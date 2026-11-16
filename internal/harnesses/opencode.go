package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
)

func detectOpenCode() (string, bool) {
	configDir := filepath.Join(homeDir(), ".config", "opencode")
	if _, err := os.Stat(configDir); err == nil {
		return configDir, true
	}
	return "", false
}

func applyOpenCode(vars map[string]string) ([]string, []string) {
	configDir := filepath.Join(homeDir(), ".config", "opencode")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, nil
	}

	var applied []string
	var warnings []string

	// 1. Write theme JSON
	themeDir := filepath.Join(configDir, "themes")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		warnings = append(warnings, fmt.Sprintf("opencode themes dir: %v", err))
		return applied, warnings
	}

	theme := map[string]interface{}{
		"name": "aether",
		"type": "dark",
		"colors": map[string]interface{}{
			"base00": firstHex(vars["base00"], "#1e1e2e"),
			"base01": firstHex(vars["base01"], "#181825"),
			"base02": firstHex(vars["base02"], "#313244"),
			"base03": firstHex(vars["base03"], "#45475a"),
			"base04": firstHex(vars["base04"], "#585b70"),
			"base05": firstHex(vars["base05"], "#cdd6f4"),
			"base06": firstHex(vars["base06"], "#f5e0dc"),
			"base07": firstHex(vars["base07"], "#b4befe"),
			"base08": firstHex(vars["base08"], "#f38ba8"),
			"base09": firstHex(vars["base09"], "#fab387"),
			"base0A": firstHex(vars["base0A"], "#f9e2af"),
			"base0B": firstHex(vars["base0B"], "#a6e3a1"),
			"base0C": firstHex(vars["base0C"], "#94e2d5"),
			"base0D": firstHex(vars["base0D"], "#89b4fa"),
			"base0E": firstHex(vars["base0E"], "#cba6f7"),
			"base0F": firstHex(vars["base0F"], "#f2cdcd"),
			"background": firstHex(vars["background"], "#1e1e2e"),
			"foreground": firstHex(vars["foreground"], "#cdd6f4"),
			"accent":     firstHex(vars["accent"], "#89b4fa"),
			"cursor":     firstHex(vars["cursor"], "#cdd6f4"),
		},
	}

	themeData, _ := jsonMarshal(theme)
	themePath := filepath.Join(themeDir, "aether.json")
	if err := writeFileAtomic(themePath, themeData); err != nil {
		warnings = append(warnings, fmt.Sprintf("opencode theme file: %v", err))
	} else {
		applied = append(applied, "opencode-theme")
	}

	// 2. Safely update cli.json "theme" key only
	mode := "dark"
	if vars["theme_type"] == "light" || vars["mode"] == "light" {
		mode = "light"
	}
	cliPath := filepath.Join(configDir, "cli.json")
	if _, err := os.Stat(cliPath); err == nil {
		err := safeMergeJSON(cliPath, func(data map[string]interface{}) error {
			// Only touch theme-related keys; preserve mcpServers, providers, oauthAccount, etc.
			themeBlock, ok := data["theme"].(map[string]interface{})
			if !ok || themeBlock == nil {
				themeBlock = make(map[string]interface{})
			}
			themeBlock["name"] = "aether"
			themeBlock["mode"] = mode
			data["theme"] = themeBlock
			return nil
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("opencode cli.json: %v", err))
		} else {
			applied = append(applied, "opencode-config")
		}
	}

	// 3. Also update opencode.json / opencode.jsonc if present and top-level "theme" key exists
	for _, filename := range []string{"opencode.json", "opencode.jsonc"} {
		altPath := filepath.Join(configDir, filename)
		if _, err := os.Stat(altPath); err == nil {
			err := safeMergeJSON(altPath, func(data map[string]interface{}) error {
				if themeBlock, ok := data["theme"].(map[string]interface{}); ok && themeBlock != nil {
					themeBlock["name"] = "aether"
					themeBlock["mode"] = mode
					data["theme"] = themeBlock
				}
				return nil
			})
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("opencode %s: %v", filename, err))
			} else {
				applied = append(applied, fmt.Sprintf("opencode-%s", filename))
			}
		}
	}

	return applied, warnings
}
