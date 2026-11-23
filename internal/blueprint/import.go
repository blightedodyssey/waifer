package blueprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/wallpaper"
)

// ImportJSON imports a blueprint from a local JSON file.
func ImportJSON(filePath string) (*Blueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var bp Blueprint
	if err := json.Unmarshal(data, &bp); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if bp.Name == "" {
		bp.Name = strings.TrimSuffix(filepath.Base(filePath), ".json")
	}
	bp.Timestamp = time.Now().UnixMilli()
	if err := validateBlueprint(&bp); err != nil {
		return nil, fmt.Errorf("validate blueprint: %w", err)
	}

	return &bp, nil
}

// ImportFromURL downloads and imports a blueprint from a URL.
func ImportFromURL(url string) (*Blueprint, error) {
	path, err := wallpaper.DownloadToCache(url, wallpaper.MaxDocumentBytes)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read download: %w", err)
	}

	var bp Blueprint
	if err := json.Unmarshal(data, &bp); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if bp.Name == "" {
		bp.Name = extractNameFromURL(url)
	}
	bp.Timestamp = time.Now().UnixMilli()
	if err := validateBlueprint(&bp); err != nil {
		return nil, fmt.Errorf("validate blueprint: %w", err)
	}

	return &bp, nil
}

// ImportBase16 parses a Base16 YAML file into a blueprint.
// Base16 YAML format: base00-base0F keys with hex values.
//
// Base16 colour mapping to ANSI 16-colour palette:
//
//	base00 → 0  (black / background)
//	base03 → 8  (bright black / comments)
//	base05 → 7  (white / foreground)
//	base07 → 15 (bright white)
//	base08 → 1  (red)          + 9  (bright red)
//	base0B → 2  (green)        + 10 (bright green)
//	base0A → 3  (yellow)       + 11 (bright yellow)
//	base0D → 4  (blue)         + 12 (bright blue)
//	base0E → 5  (magenta)      + 13 (bright magenta)
//	base0C → 6  (cyan)         + 14 (bright cyan)
func ImportBase16(filePath string) (*Blueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	base := make(map[int]string, 16)
	var scheme, author string

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, ":")) {
			continue
		}

		if strings.HasPrefix(line, "scheme:") {
			scheme = parseYAMLValue(strings.TrimPrefix(line, "scheme:"))
		}
		if strings.HasPrefix(line, "author:") {
			author = parseYAMLValue(strings.TrimPrefix(line, "author:"))
		}

		// Parse base00-base0F
		for i := 0; i < 16; i++ {
			key := fmt.Sprintf("base%02X:", i)
			if strings.HasPrefix(line, key) {
				base[i] = normalizeImportedHex(strings.TrimPrefix(line, key))
			}
		}
	}

	// Validate we got enough base colours
	if len(base) < 8 {
		return nil, fmt.Errorf("not enough colors found (got %d)", len(base))
	}

	// Map base16 to ANSI 16-colour palette
	colors := base16ANSIColors(base)

	name := scheme
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	_ = author

	bp := &Blueprint{
		Name: name,
		Palette: PaletteData{
			Colors: colors[:],
		},
		Timestamp: time.Now().UnixMilli(),
	}
	if err := validateBlueprint(bp); err != nil {
		return nil, fmt.Errorf("validate Base16 scheme: %w", err)
	}

	return bp, nil
}

// base16ANSIColors maps the canonical base00-base0F slots onto the ANSI
// 16-colour palette shared by Base16 and Base24 imports.
func base16ANSIColors(base map[int]string) [16]string {
	return [16]string{
		base[0x00], // 0  black / background
		base[0x08], // 1  red
		base[0x0B], // 2  green
		base[0x0A], // 3  yellow
		base[0x0D], // 4  blue
		base[0x0E], // 5  magenta
		base[0x0C], // 6  cyan
		base[0x05], // 7  white / foreground
		base[0x03], // 8  bright black
		base[0x08], // 9  bright red
		base[0x0B], // 10 bright green
		base[0x0A], // 11 bright yellow
		base[0x0D], // 12 bright blue
		base[0x0E], // 13 bright magenta
		base[0x0C], // 14 bright cyan
		base[0x07], // 15 bright white
	}
}

// normalizeImportedHex strips YAML/JSON quoting and inline comments and adds a
// leading '#' when the value is a bare hex triplet/sixth. Empty values are
// preserved so optional extended slots stay distinguishable from bad hex.
func normalizeImportedHex(raw string) string {
	val := parseYAMLValue(raw)
	// Strip inline comments: "090300" # ---- → 090300
	if idx := strings.Index(val, "#"); idx > 0 {
		before := val[:idx]
		if strings.TrimSpace(before) != "" {
			val = strings.TrimSpace(before)
		}
	}
	val = strings.Trim(val, `"' `)
	if val != "" && !strings.HasPrefix(val, "#") {
		val = "#" + val
	}
	return val
}

// ImportBase24 parses a Base24 YAML file into a blueprint. Base24 extends
// Base16 with base10-base17; those extra slots are preserved as extended
// colours while base00-base0F map onto ANSI exactly as in ImportBase16.
func ImportBase24(filePath string) (*Blueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	base := make(map[int]string, 24)
	extended := make(map[string]string, 8)
	var scheme, author string

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, ":")) {
			continue
		}
		if strings.HasPrefix(line, "scheme:") {
			scheme = parseYAMLValue(strings.TrimPrefix(line, "scheme:"))
		}
		if strings.HasPrefix(line, "author:") {
			author = parseYAMLValue(strings.TrimPrefix(line, "author:"))
		}

		for i := 0; i < 24; i++ {
			key := fmt.Sprintf("base%02X:", i)
			// EqualFold accepts both base0A and base0a spellings.
			if len(line) < len(key) || !strings.EqualFold(line[:len(key)], key) {
				continue
			}
			val := normalizeImportedHex(line[len(key):])
			base[i] = val
			if i >= 0x10 {
				extended[fmt.Sprintf("base%02X", i)] = val
			}
		}
	}
	_ = author

	// Only base00-base0F count towards the minimum: extended slots alone are
	// not a usable ANSI palette.
	baseCount := 0
	for i := 0; i < 16; i++ {
		if _, ok := base[i]; ok {
			baseCount++
		}
	}
	if baseCount < 8 {
		return nil, fmt.Errorf("not enough colors found (got %d)", baseCount)
	}

	name := scheme
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	colors := base16ANSIColors(base)
	bp := &Blueprint{
		Name: name,
		Palette: PaletteData{
			Colors:         colors[:],
			ExtendedColors: extended,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	if err := validateBlueprint(bp); err != nil {
		return nil, fmt.Errorf("validate Base24 scheme: %w", err)
	}
	return bp, nil
}

// pywalDocument covers both the nested Wallust/pywal colour file shape and the
// flat legacy shape; flat color0-color15 keys are read separately.
type pywalDocument struct {
	Colors     map[string]string `json:"colors"`
	Special    map[string]string `json:"special"`
	Background string            `json:"background"`
	Foreground string            `json:"foreground"`
	Cursor     string            `json:"cursor"`
	Accent     string            `json:"accent"`
	Mode       string            `json:"mode"`
	Wallust    struct {
		Mode string `json:"mode"`
	} `json:"wallust"`
	Pywal struct {
		Mode string `json:"mode"`
	} `json:"pywal"`
}

// ImportPywal parses a pywal/Wallust colors.json into a blueprint. The nested
// {"colors": ..., "special": ...} and flat {color0: ..., background: ...}
// layouts are both accepted.
func ImportPywal(filePath string) (*Blueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var doc pywalDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse colors.json: %w", err)
	}
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, fmt.Errorf("parse colors.json: %w", err)
	}

	lookupColor := func(key string) string {
		if value, ok := doc.Colors[key]; ok {
			return value
		}
		if raw, ok := flat[key]; ok {
			var value string
			if json.Unmarshal(raw, &value) == nil {
				return value
			}
		}
		return ""
	}

	colors := make([]string, 16)
	found := 0
	for i := range colors {
		colors[i] = normalizeImportedHex(lookupColor(fmt.Sprintf("color%d", i)))
		if colors[i] != "" {
			found++
		}
	}
	if found < 16 {
		return nil, fmt.Errorf("not enough colors found (got %d of 16)", found)
	}

	extended := make(map[string]string, 4)
	for _, item := range []struct{ key, value string }{
		{"background", doc.Special["background"]},
		{"foreground", doc.Special["foreground"]},
		{"cursor", doc.Special["cursor"]},
		{"accent", doc.Special["accent"]},
	} {
		if item.value == "" {
			switch item.key {
			case "background":
				item.value = doc.Background
			case "foreground":
				item.value = doc.Foreground
			case "cursor":
				item.value = doc.Cursor
			case "accent":
				item.value = doc.Accent
			}
		}
		if value := normalizeImportedHex(item.value); value != "" {
			extended[item.key] = value
		}
	}

	mode := doc.Wallust.Mode
	if mode == "" {
		mode = doc.Pywal.Mode
	}
	if mode == "" {
		mode = doc.Mode
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "light" && mode != "dark" {
		mode = ""
	}

	bp := &Blueprint{
		Name: strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
		Palette: PaletteData{
			Colors:         colors,
			ExtendedColors: extended,
			Mode:           mode,
			LightMode:      mode == "light",
		},
		Timestamp: time.Now().UnixMilli(),
	}
	if err := validateBlueprint(bp); err != nil {
		return nil, fmt.Errorf("validate pywal palette: %w", err)
	}
	return bp, nil
}

// ImportAuto detects the theme format from the file extension (and YAML key
// presence) and dispatches to the matching importer.
func ImportAuto(filePath string) (*Blueprint, error) {
	switch ext := strings.ToLower(filepath.Ext(filePath)); ext {
	case ".json":
		bp, pyErr := ImportPywal(filePath)
		if pyErr == nil {
			return bp, nil
		}
		bp, jsonErr := ImportJSON(filePath)
		if jsonErr == nil {
			return bp, nil
		}
		return nil, fmt.Errorf("import as pywal colors: %v; import as blueprint JSON: %w", pyErr, jsonErr)
	case ".yaml", ".yml":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		if hasBase24Keys(string(data)) {
			return ImportBase24(filePath)
		}
		return ImportBase16(filePath)
	case ".toml":
		return ImportColorsToml(filePath)
	default:
		return nil, fmt.Errorf("unsupported theme import format %q", ext)
	}
}

// hasBase24Keys reports whether a YAML document declares any base10-base17 slot.
func hasBase24Keys(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		for i := 0x10; i <= 0x17; i++ {
			key := fmt.Sprintf("base%02X:", i)
			if len(line) >= len(key) && strings.EqualFold(line[:len(key)], key) {
				return true
			}
		}
	}
	return false
}

// ImportColorsTomlFromURL downloads a colors.toml file into the web-import
// cache and parses it into a blueprint.
func ImportColorsTomlFromURL(url string) (*Blueprint, error) {
	path, err := wallpaper.DownloadToCache(url, wallpaper.MaxDocumentBytes)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	bp, err := ImportColorsToml(path)
	if err != nil {
		return nil, err
	}
	if name := extractNameFromURL(url); name != "" && name != "imported_blueprint" {
		bp.Name = strings.TrimSuffix(name, ".toml")
	}
	return bp, nil
}

// ImportCurrentColorsToml loads the active Omarchy palette when available,
// falling back to Aether's standalone output.
func ImportCurrentColorsToml() (*Blueprint, error) {
	var nativeErr error
	if omarchy.IsInstalled() {
		bp, err := ImportColorsToml(omarchy.CurrentThemeColorsPath())
		if err == nil {
			return bp, nil
		}
		nativeErr = err
	}

	bp, err := ImportColorsToml(filepath.Join(platform.ThemeDir(), "colors.toml"))
	if err == nil {
		return bp, nil
	}
	if nativeErr != nil {
		return nil, fmt.Errorf("load active Omarchy palette: %v; load standalone palette: %w", nativeErr, err)
	}
	return nil, err
}

// ImportColorsToml parses a colors.toml file into a blueprint.
func ImportColorsToml(filePath string) (*Blueprint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	colors, bg, fg, parsedMode, extended := omarchy.ParseColorsTomlFull(string(data))

	// Fill empty slots: use bg for dark colors (0,8), fg for light colors (7,15)
	if bg == "" {
		bg = "#1a1b26"
	}
	if fg == "" {
		fg = "#a9b1d6"
	}
	for i := range colors {
		if colors[i] == "" {
			if i == 0 || i == 8 {
				colors[i] = bg
			} else if i == 7 || i == 15 {
				colors[i] = fg
			} else {
				// Still empty — leave as-is, will be caught below
			}
		}
	}

	filled := 0
	for _, c := range colors {
		if c != "" {
			filled++
		}
	}
	if filled < 2 {
		return nil, fmt.Errorf("not enough colors found (got %d of 16)", filled)
	}

	name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	// Preserve the theme's explicit accent/cursor/selection so the import
	// stays faithful instead of re-deriving them from the palette.
	ext := map[string]string{}
	for k, v := range extended {
		ext[k] = v
	}
	bp := &Blueprint{
		Name: name,
		Palette: PaletteData{
			Colors:         colors[:],
			ExtendedColors: ext,
			NativeColors:   omarchy.ParseNativeColors(string(data)),
			LightMode:      parsedMode == "light",
			Mode:           parsedMode,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	if err := validateBlueprint(bp); err != nil {
		return nil, fmt.Errorf("validate colors.toml: %w", err)
	}

	return bp, nil
}

// parseYAMLValue extracts a clean value from a YAML line fragment.
func parseYAMLValue(raw string) string {
	v := strings.TrimSpace(raw)
	// Strip inline comments (# preceded by space)
	if idx := strings.Index(v, " #"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	v = strings.Trim(v, `"' `)
	return v
}

func extractNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			return name
		}
	}
	return "imported_blueprint"
}

// SaveImported saves an imported blueprint to the blueprints directory.
func SaveImported(bp *Blueprint) (string, error) {
	dir := platform.BlueprintDir()
	_ = platform.EnsureDir(dir)

	safeName := strings.ReplaceAll(bp.Name, "/", "-")
	safeName = strings.ReplaceAll(safeName, " ", "-")
	filename := fmt.Sprintf("%s_imported_%d.json", safeName, time.Now().Unix())
	path := filepath.Join(dir, filename)

	if err := platform.WriteJSON(path, bp); err != nil {
		return "", err
	}
	return path, nil
}
