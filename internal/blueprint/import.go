package blueprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"aether/internal/icontheme"
	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/wallpaper"
)

// ImportJSON loads a blueprint JSON document from path.
func ImportJSON(path string) (*Blueprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read blueprint: %w", err)
	}
	var bp Blueprint
	if err := json.Unmarshal(data, &bp); err != nil {
		return nil, fmt.Errorf("parse blueprint: %w", err)
	}
	if bp.Name == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		bp.Name = base
	}
	return &bp, nil
}

// ImportFromURL downloads a blueprint JSON document and parses it.
func ImportFromURL(rawURL string) (*Blueprint, error) {
	local, err := wallpaper.DownloadToCache(rawURL, wallpaper.MaxDocumentBytes)
	if err != nil {
		return nil, fmt.Errorf("download blueprint: %w", err)
	}
	return ImportJSON(local)
}

// base16Slots maps Base16 keys to palette indices.
var base16Slots = []string{
	"base00", "base01", "base02", "base03", "base04", "base05", "base06", "base07",
	"base08", "base09", "base0A", "base0B", "base0C", "base0D", "base0E", "base0F",
}

// ImportBase16 parses a Base16 scheme YAML file into a blueprint.
func ImportBase16(path string) (*Blueprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read base16: %w", err)
	}
	scheme, err := parseSimpleYAML(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse base16: %w", err)
	}

	name := scheme["scheme"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	bp := &Blueprint{
		Name: name,
		Palette: PaletteData{
			Colors: make([]string, 16),
		},
	}
	for i, slot := range base16Slots {
		hex := scheme[slot]
		if hex == "" {
			return nil, fmt.Errorf("base16 scheme is missing %s", slot)
		}
		bp.Palette.Colors[i] = normalizeHexValue(hex)
	}

	if author := scheme["author"]; author != "" {
		bp.Palette.ExtendedColors = map[string]string{}
		_ = author
	}
	return bp, nil
}

// ImportColorsToml parses a colors.toml file into a blueprint.
func ImportColorsToml(path string) (*Blueprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read colors.toml: %w", err)
	}
	return importColorsTomlContent(string(data))
}

// ImportColorsTomlFromURL downloads a colors.toml document and parses it.
func ImportColorsTomlFromURL(rawURL string) (*Blueprint, error) {
	local, err := wallpaper.DownloadToCache(rawURL, wallpaper.MaxDocumentBytes)
	if err != nil {
		return nil, fmt.Errorf("download colors.toml: %w", err)
	}
	data, err := os.ReadFile(local)
	if err != nil {
		return nil, fmt.Errorf("read colors.toml: %w", err)
	}
	return importColorsTomlContent(string(data))
}

// ImportCurrentColorsToml loads the currently applied theme's colors.toml as
// a blueprint. Used for wallpaper-only aether:// installs so the palette
// stays untouched.
func ImportCurrentColorsToml() (*Blueprint, error) {
	path := filepath.Join(platform.ThemeDir(), "colors.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read current colors.toml: %w", err)
	}
	bp, err := importColorsTomlContent(string(data))
	if err != nil {
		return nil, err
	}
	bp.Name = "current"
	return bp, nil
}

// importColorsTomlContent turns colors.toml content into a blueprint.
func importColorsTomlContent(content string) (*Blueprint, error) {
	colors, bg, fg, err := omarchy.ParseColorsToml(content)
	if err != nil {
		return nil, err
	}
	name := "Imported theme"
	bp := &Blueprint{
		Name: name,
		Palette: PaletteData{
			Colors: colors[:],
		},
	}
	// Store derived roles as extended colors so a re-apply keeps them.
	extended := map[string]string{}
	if bg != "" {
		extended["background"] = bg
	}
	if fg != "" {
		extended["foreground"] = fg
	}
	if len(extended) > 0 {
		bp.Palette.ExtendedColors = extended
	}
	// Mode detection from a `mode = "light"` line.
	if strings.Contains(content, `mode = "light"`) {
		bp.Palette.LightMode = true
		bp.Palette.Mode = "light"
	} else if strings.Contains(content, `mode = "dark"`) {
		bp.Palette.Mode = "dark"
	}
	return bp, nil
}

// SaveImported persists an imported blueprint, assigning a unique name and
// timestamp. Returns the saved file path.
func SaveImported(bp *Blueprint) (string, error) {
	if bp.Name == "" {
		bp.Name = "Imported theme"
	}
	svc := NewService()
	if _, err := svc.FindByName(bp.Name); err == nil {
		// Name collision: suffix the slug.
		bp.Name = bp.Name + "-" + strconv.FormatInt(nowUnix(), 36)
	}
	if bp.Timestamp == 0 {
		bp.Timestamp = nowUnix()
	}
	slug := slugify(bp.Name)
	if slug == "" {
		slug = "imported"
	}
	path := filepath.Join(platform.BlueprintDir(), slug+".json")
	if err := platform.WriteJSON(path, bp); err != nil {
		return "", fmt.Errorf("save imported blueprint: %w", err)
	}
	return path, nil
}

// parseSimpleYAML parses the flat "key: value" subset of YAML that Base16
// scheme files use. Lists and nesting are not supported.
func parseSimpleYAML(content string) (map[string]string, error) {
	out := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(strings.TrimSpace(trimmed), "#") || strings.TrimSpace(trimmed) == "" {
			continue
		}
		if strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
			continue // nested content: skip
		}
		colon := strings.Index(trimmed, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])
		value = strings.Trim(value, `"'`)
		if key != "" {
			out[key] = value
		}
	}
	return out, nil
}

// normalizeHexValue coerces a Base16 hex value into #rrggbb lowercase.
func normalizeHexValue(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) > 6 {
		hex = hex[:6]
	}
	return "#" + strings.ToLower(hex)
}

var slugInvalidRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a name into a file-safe slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugInvalidRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// sortedKeys returns map keys in stable order (used by validators).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var _ = icontheme.Automatic // keep selection dependency explicit
