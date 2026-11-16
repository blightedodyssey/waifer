package omarchy

import (
	"fmt"
	"strings"
)

// ParseColorsToml extracts the 16 ANSI slots plus background/foreground from
// a colors.toml document. Missing slots stay empty.
func ParseColorsToml(content string) ([16]string, string, string, error) {
	var colors [16]string
	bg, fg := "", ""

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		value = strings.Trim(value, `"`)
		switch {
		case key == "background":
			bg = value
		case key == "foreground":
			fg = value
		default:
			// color0..color15, with and without the "color" prefix.
			var idx int
			if n, err := fmt.Sscanf(strings.ToLower(key), "color%d", &idx); err == nil && n == 1 && idx >= 0 && idx <= 15 {
				colors[idx] = value
			}
		}
	}
	return colors, bg, fg, nil
}

// ParseColorsTomlFull returns every key/value pair in the document.
func ParseColorsTomlFull(content string) (map[string]string, error) {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.Trim(strings.TrimSpace(line[eq+1:]), `"`)
		if key != "" {
			out[key] = value
		}
	}
	return out, nil
}

// ParseColorsKV parses a colors.toml into a flat key/value map (alias of
// ParseColorsTomlFull for the IPC surface).
func ParseColorsKV(content string) (map[string]string, error) {
	return ParseColorsTomlFull(content)
}

// ParseKittyConf extracts a key/value map from a Kitty-style config file
// (comments with #, `key value` lines).
func ParseKittyConf(content string) (map[string]string, error) {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		out[fields[0]] = strings.Join(fields[1:], " ")
	}
	return out, nil
}
