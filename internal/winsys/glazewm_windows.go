//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyGlazeWMTheme updates active border colors in
// %USERPROFILE%\.glzr\glazewm\config.yaml, preserving all user keybindings,
// gaps, and rules. If the file does not exist, nothing is written.
func ApplyGlazeWMTheme(vars map[string]string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}
	configPath := filepath.Join(home, ".glzr", "glazewm", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read glazewm config.yaml: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	accent := normalizeHex(vars["accent"])
	if accent == "" {
		accent = "#cba6f7"
	}
	fg := normalizeHex(vars["foreground"])
	if fg == "" {
		fg = "#cdd6f4"
	}

	updated := false
	var result []string
	inGeneral := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track whether we're inside the general section.
		if strings.HasPrefix(trimmed, "general:") {
			inGeneral = true
		} else if inGeneral && !strings.HasPrefix(trimmed, "#") && trimmed != "" && strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") {
			// A top-level key that is not a comment and not sub-property ends general.
			inGeneral = false
		}

		if inGeneral {
			keyVal := parseKeyValue(line)
			if keyVal != nil {
				key := keyVal.key
				val := keyVal.value
				if key == "focused_border_color" && val != accent {
					result = append(result, fmt.Sprintf("%s: %s", indent(line), accent))
					updated = true
					continue
				}
				if key == "other_windows_border_color" && val != fg {
					result = append(result, fmt.Sprintf("%s: %s", indent(line), fg))
					updated = true
					continue
				}
			}
		}
		result = append(result, line)
	}

	if updated {
		if err := os.WriteFile(configPath, []byte(strings.Join(result, "\n")), 0o644); err != nil {
			return fmt.Errorf("write glazewm config.yaml: %w", err)
		}
	}
	return nil
}

type keyValue struct {
	key   string
	value string
}

// parseKeyValue splits "key: value" on the first colon, trimming spaces.
func parseKeyValue(line string) *keyValue {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return nil
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return nil
	}
	return &keyValue{key: key, value: value}
}

// indent returns the leading whitespace of line.
func indent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}
