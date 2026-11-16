package omarchy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Theme describes one installed Omarchy theme folder.
type Theme struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Wallpaper string `json:"wallpaper,omitempty"`
	LightMode bool   `json:"lightMode"`
	Current   bool   `json:"current"`
}

// ThemeExists reports whether a theme folder with that name exists.
func ThemeExists(name string) bool {
	info, err := os.Stat(filepath.Join(UserThemesDir(), name))
	return err == nil && info.IsDir()
}

// slugInvalid matches anything that is not a letter/digit.
var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyThemeName converts a user-typed name into a safe theme slug.
// Returns "" when nothing usable remains.
func SlugifyThemeName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugInvalid.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return ""
	}
	return s
}

// LoadAllThemes returns every theme folder that contains a colors.toml,
// sorted by name.
func LoadAllThemes() ([]Theme, error) {
	root := UserThemesDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Theme{}, nil
		}
		return nil, fmt.Errorf("read Omarchy themes dir: %w", err)
	}

	current := currentThemeName()

	themes := make([]Theme, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(filepath.Join(dir, "colors.toml"))
		if err != nil {
			continue // not a theme folder
		}
		colors, bg, fg, err := ParseColorsToml(string(data))
		if err != nil {
			continue
		}
		wallpaper := themeWallpaper(dir)
		theme := Theme{
			Name:      entry.Name(),
			Path:      dir,
			Wallpaper: wallpaper,
			LightMode: isLightPalette(colors, bg),
			Current:   entry.Name() == current,
		}
		_ = bg
		_ = fg
		themes = append(themes, theme)
	}
	sort.Slice(themes, func(i, j int) bool { return themes[i].Name < themes[j].Name })
	return themes, nil
}

// currentThemeName reads Omarchy's active theme marker.
func currentThemeName() string {
	data, err := os.ReadFile(filepath.Join(CurrentStateDir(), "theme.name"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// themeWallpaper finds the wallpaper stored with a theme folder.
func themeWallpaper(dir string) string {
	backgrounds := filepath.Join(dir, "backgrounds")
	entries, err := os.ReadDir(backgrounds)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp":
			return filepath.Join(backgrounds, entry.Name())
		}
	}
	return ""
}

// isLightPalette guesses the mode from the background color.
func isLightPalette(colors [16]string, bg string) bool {
	hex := bg
	if hex == "" {
		hex = colors[0]
	}
	if hex == "" {
		return false
	}
	r, g, b, ok := hexChannels(hex)
	if !ok {
		return false
	}
	// Perceived luminance > 0.5 means a light background.
	return (0.299*float64(r)+0.587*float64(g)+0.114*float64(b))/255 > 0.5
}

// hexChannels decodes #rrggbb into 8-bit channels.
func hexChannels(hex string) (r, g, b byte, ok bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	var v uint32
	for i := 0; i < 6; i++ {
		c := h[i]
		var d uint32
		switch {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint32(c-'A') + 10
		default:
			return 0, 0, 0, false
		}
		v = v<<4 | d
	}
	return byte(v >> 16), byte(v >> 8), byte(v), true
}

// ValidateThemeDirName reports whether name is a safe theme folder name.
func ValidateThemeDirName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\")
}
