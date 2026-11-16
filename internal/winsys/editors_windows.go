package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// zedThemeName is the theme name written into Zed's settings.json.
const zedThemeName = "Aether"

// zedRoots returns the user's Zed themes and settings locations. Zed stores
// per-user data under %APPDATA%\Zed on Windows.
func zedRoots() (themesDir, settingsPath string) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "Zed", "themes"), filepath.Join(appData, "Zed", "settings.json")
}

// ApplyZed installs the rendered aether.zed.json template into Zed's themes
// directory and selects it in settings.json.
func ApplyZed(themeDir string) error {
	src := filepath.Join(themeDir, "aether.zed.json")
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // template not generated (excluded)
		}
		return fmt.Errorf("read %s: %w", src, err)
	}

	themesDir, settingsPath := zedRoots()
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return fmt.Errorf("create Zed themes dir: %w", err)
	}
	dest := filepath.Join(themesDir, "aether.json")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("zed: %w", err)
	}
	return upsertZedSetting(settingsPath, zedThemeName)
}

// ClearZed removes the Aether theme file and resets the theme selection.
func ClearZed() error {
	themesDir, settingsPath := zedRoots()
	_ = os.Remove(filepath.Join(themesDir, "aether.json"))
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // Zed not configured
	}
	content := removeJSONStringField(string(raw), "theme")
	if content == string(raw) {
		return nil
	}
	return writeFileAtomic(settingsPath, content)
}

// removeJSONStringField deletes a top-level `"key": "value"` line from JSONC
// content. Content without the field is returned unchanged.
func removeJSONStringField(content, key string) string {
	needle := "\"" + key + "\""
	idx := indexOfStr(content, needle)
	if idx < 0 {
		return content
	}
	// Extend to the end of the line so the comma/brace layout stays sane.
	lineStart := idx
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := indexOfStr(content[idx:], "\n")
	if lineEnd < 0 {
		lineEnd = len(content) - idx
	} else {
		lineEnd += idx + 1
	}
	// Drop a trailing comma on the previous line when removing the last one.
	prefix := content[:lineStart]
	suffix := content[lineEnd:]
	if strings.HasSuffix(strings.TrimRight(prefix, " \t"), ",") && strings.HasPrefix(strings.TrimSpace(suffix), "}") {
		prefix = strings.TrimRight(prefix, " \t\r\n")
		prefix = strings.TrimSuffix(prefix, ",")
		prefix += "\n"
	}
	return prefix + suffix
}

// upsertZedSetting points Zed's theme settings at the installed theme. Zed
// stores appearance.theme_mode and appearance.theme; both are updated for
// light and dark.
func upsertZedSetting(path, theme string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(raw)

	content = replaceJSONStringField(content, "theme", theme)
	content = replaceJSONStringField(content, "theme_mode", "system")

	if !strings.Contains(content, `"theme"`) {
		trimmed := strings.TrimRight(strings.TrimSpace(content), "}")
		if trimmed == "" {
			content = "{\n  \"theme\": \"" + theme + "\",\n  \"theme_mode\": \"system\"\n}"
		} else {
			content = trimmed + ",\n  \"theme\": \"" + theme + "\",\n  \"theme_mode\": \"system\"\n}"
		}
	}
	return writeFileAtomic(path, content)
}

// replaceJSONStringField swaps the value of a top-level string field in JSONC
// content when present, leaving formatting intact.
func replaceJSONStringField(content, key, value string) string {
	needle := "\"" + key + "\""
	idx := indexOfStr(content, needle)
	if idx < 0 {
		return content
	}
	rest := content[idx+len(needle):]
	colon := indexOfStr(rest, ":")
	if colon < 0 {
		return content
	}
	rest = rest[colon+1:]

	// Find the current quoted value.
	start := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '"' {
			start = i
			break
		}
		if rest[i] == '\n' || rest[i] == '}' {
			return content // value is not a string; leave untouched
		}
	}
	if start < 0 {
		return content
	}
	end := -1
	for i := start + 1; i < len(rest); i++ {
		if rest[i] == '"' && rest[i-1] != '\\' {
			end = i
			break
		}
	}
	if end < 0 {
		return content
	}
	updated := content[:idx+len(needle)] + content[idx+len(needle):idx+len(needle)+colon+1] +
		" \"" + value + "\"" + rest[end+1:]
	return updated
}
