package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func detectOpenCode() (string, bool) {
	configDir := filepath.Join(homeDir(), ".config", "opencode")
	if _, err := os.Stat(configDir); err == nil {
		return configDir, true
	}
	return "", false
}

func localAppData() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return v
	}
	return filepath.Join(homeDir(), "AppData", "Local")
}

func applyOpenCode(vars map[string]string) ([]string, []string) {
	configDir := filepath.Join(homeDir(), ".config", "opencode")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, nil
	}

	var applied []string
	var warnings []string

	mode := "dark"
	if vars["theme_type"] == "light" || vars["mode"] == "light" {
		mode = "light"
	}

	// ----------------------------------------------------------
	// A. OpenCode Desktop theme & GUI plugin
	// ----------------------------------------------------------
	// Write to both legacy path and the official OpenCode plugins dir.
	legacyPluginDir := filepath.Join(localAppData(), "Aether", "opencode-plugin")
	officialPluginDir := filepath.Join(configDir, "plugins", "aether-theme")
	pluginDirs := []string{legacyPluginDir, officialPluginDir}

	for _, pluginDir := range pluginDirs {
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			warnings = append(warnings, fmt.Sprintf("opencode plugin dir %s: %v", pluginDir, err))
			continue
		}

		// Desktop theme JSON conforming to OpenCode Desktop schema.
		desktopThemePath := filepath.Join(pluginDir, "desktop-theme.json")
		desktopTheme := map[string]interface{}{
			"name":    "Aether",
			"id":      "aether",
			"version": "1.0.0",
			"dark": map[string]interface{}{
				"neutral":  firstHex(vars["base01"], "#181825"),
				"ink":      firstHex(vars["base04"], "#585b70"),
				"primary":  firstHex(vars["base0D"], "#89b4fa"),
				"success":  firstHex(vars["base0B"], "#a6e3a1"),
				"warning":  firstHex(vars["base0A"], "#f9e2af"),
				"error":    firstHex(vars["base08"], "#f38ba8"),
				"info":     firstHex(vars["base0C"], "#94e2d5"),
				"tokenOverrides": map[string]interface{}{
					"comment":         firstHex(vars["base03"], "#45475a"),
					"operator":        firstHex(vars["base05"], "#cdd6f4"),
					"punctuation":     firstHex(vars["base04"], "#585b70"),
					"string":          firstHex(vars["base0B"], "#a6e3a1"),
					"number":          firstHex(vars["base09"], "#fab387"),
					"keyword":         firstHex(vars["base08"], "#f38ba8"),
					"function":        firstHex(vars["base0D"], "#89b4fa"),
					"parameter":       firstHex(vars["base0E"], "#cba6f7"),
					"type":            firstHex(vars["base0F"], "#f2cdcd"),
					"variable":        firstHex(vars["base05"], "#cdd6f4"),
					"constant":        firstHex(vars["base06"], "#f5e0dc"),
					"property":        firstHex(vars["base0E"], "#cba6f7"),
					"namespace":       firstHex(vars["base07"], "#b4befe"),
					"regular":         firstHex(vars["base05"], "#cdd6f4"),
					"regularEmphasis": firstHex(vars["base06"], "#f5e0dc"),
				},
			},
			"light": map[string]interface{}{
				"neutral":  firstHex(vars["base01"], "#181825"),
				"ink":      firstHex(vars["base04"], "#585b70"),
				"primary":  firstHex(vars["base0D"], "#89b4fa"),
				"success":  firstHex(vars["base0B"], "#a6e3a1"),
				"warning":  firstHex(vars["base0A"], "#f9e2af"),
				"error":    firstHex(vars["base08"], "#f38ba8"),
				"info":     firstHex(vars["base0C"], "#94e2d5"),
				"tokenOverrides": map[string]interface{}{
					"comment":         firstHex(vars["base03"], "#45475a"),
					"operator":        firstHex(vars["base05"], "#cdd6f4"),
					"punctuation":     firstHex(vars["base04"], "#585b70"),
					"string":          firstHex(vars["base0B"], "#a6e3a1"),
					"number":          firstHex(vars["base09"], "#fab387"),
					"keyword":         firstHex(vars["base08"], "#f38ba8"),
					"function":        firstHex(vars["base0D"], "#89b4fa"),
					"parameter":       firstHex(vars["base0E"], "#cba6f7"),
					"type":            firstHex(vars["base0F"], "#f2cdcd"),
					"variable":        firstHex(vars["base05"], "#cdd6f4"),
					"constant":        firstHex(vars["base06"], "#f5e0dc"),
					"property":        firstHex(vars["base0E"], "#cba6f7"),
					"namespace":       firstHex(vars["base07"], "#b4befe"),
					"regular":         firstHex(vars["base05"], "#cdd6f4"),
					"regularEmphasis": firstHex(vars["base06"], "#f5e0dc"),
				},
			},
		}
		desktopThemeData, _ := jsonMarshal(desktopTheme)
		if err := writeFileAtomic(desktopThemePath, desktopThemeData); err != nil {
			warnings = append(warnings, fmt.Sprintf("opencode desktop theme %s: %v", pluginDir, err))
		} else {
			applied = append(applied, fmt.Sprintf("opencode-desktop-theme-%s", filepath.Base(pluginDir)))
		}

		// Self-contained GUI plugin — index.ts for OpenCode Desktop.
		pluginIndexPath := filepath.Join(pluginDir, "index.ts")
		pluginCodeTS := `import { gui } from "@opencode/plugin/gui";
import themeData from "./desktop-theme.json";

export const activate = (ctx: any) => {
  ctx.add(gui.Theme, themeData);
  console.log("[Aether] OpenCode Desktop theme registered.");
};
`
		if err := writeFileAtomic(pluginIndexPath, []byte(pluginCodeTS)); err != nil {
			warnings = append(warnings, fmt.Sprintf("opencode plugin index.ts %s: %v", pluginDir, err))
		} else {
			applied = append(applied, fmt.Sprintf("opencode-desktop-plugin-%s", filepath.Base(pluginDir)))
		}
	}

	// ----------------------------------------------------------
	// B. Register the official plugin in opencode.json / opencode.jsonc
	// ----------------------------------------------------------
	pluginURI := "file://" + filepath.ToSlash(officialPluginDir)
	for _, filename := range []string{"opencode.json", "opencode.jsonc"} {
		altPath := filepath.Join(configDir, filename)
		if _, err := os.Stat(altPath); os.IsNotExist(err) {
			continue
		}
		err := func(name string) error {
			// Decide whether to use plain JSON or JSONC parser.
			useJSONC := strings.HasSuffix(strings.ToLower(name), ".jsonc")
			if useJSONC {
				return safeMergeJSONC(altPath, func(data map[string]interface{}) error {
					return ensurePluginInList(data, pluginURI)
				})
			}
			return safeMergeJSON(altPath, func(data map[string]interface{}) error {
				return ensurePluginInList(data, pluginURI)
			})
		}(filename)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("opencode %s plugin registration: %v", filename, err))
		} else {
			applied = append(applied, fmt.Sprintf("opencode-%s-plugin-registered", filename))
		}
	}

	// ----------------------------------------------------------
	// C. Write legacy TUI theme JSON
	// ----------------------------------------------------------
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
	mode = "dark"
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

// ensurePluginInList inserts pluginURI into data["plugins"] as a string list if
// it is not already present. It normalises the field so it can be either an
// array of strings or a comma-separated string.
func ensurePluginInList(data map[string]interface{}, pluginURI string) error {
	// Make sure plugins field is a []string.
	var plugins []string
	if raw, ok := data["plugins"]; ok {
		switch v := raw.(type) {
		case []string:
			plugins = v
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					plugins = append(plugins, s)
				}
			}
		case string:
			for _, s := range strings.Split(v, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					plugins = append(plugins, s)
				}
			}
		default:
			// Unexpected type — overwrite with a clean list rather than fail.
			plugins = nil
		}
	}
	// De-duplicate while checking for existing entry.
	found := false
	for _, p := range plugins {
		if p == pluginURI {
			found = true
			break
		}
	}
	if !found {
		plugins = append(plugins, pluginURI)
	}
	data["plugins"] = plugins
	return nil
}
