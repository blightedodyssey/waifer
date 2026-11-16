package theme

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/template"
	"aether/internal/wallpaper"
	"aether/internal/winsys"
)

// IsOmarchyInstalled reports whether the public Omarchy CLI is available.
func IsOmarchyInstalled() bool {
	return omarchy.IsInstalled()
}

// HandleLightModeMarker creates or removes the light.mode marker file in
// the theme directory. The presence of this file signals light mode to
// consumers.
func HandleLightModeMarker(themeDir string, lightMode bool) error {
	markerPath := filepath.Join(themeDir, "light.mode")

	if lightMode {
		// Create empty light.mode file (ignore if it already exists)
		f, err := os.OpenFile(markerPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		log.Println("Created light.mode marker file")
		return nil
	}

	// Remove light.mode file if it exists
	err := os.RemoveAll(markerPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err == nil {
		log.Println("Removed light.mode marker file")
	}
	return err
}

// IsImageFile returns true for still-image formats that can be decoded directly.
func IsImageFile(path string) bool {
	return wallpaper.IsImageFile(path)
}

// ClearTheme removes Aether's standalone override and restores the last native
// Omarchy theme that was active before an Aether-managed theme.
func ClearTheme() error {
	if err := RetireLegacyGTKStylesheets(); err != nil {
		return fmt.Errorf("retire legacy GTK stylesheets: %w", err)
	}

	// Delete Aether override CSS file in theme dir (cross-platform)
	overrideCss := filepath.Join(platform.ThemeDir(), "aether.override.css")
	if err := platform.DeleteFile(overrideCss); err != nil {
		log.Printf("Warning: could not delete theme override CSS: %v", err)
	}

	if err := winsys.Restore(); err != nil {
		log.Printf("Warning: could not restore Windows settings: %v", err)
	}

	if !IsOmarchyInstalled() {
		return nil
	}
	return omarchy.RevertTheme()
}

// MapColorsToRoles converts a 16-color palette into template ColorRoles.
// This is the same logic used by ThemeState.buildColorRoles but as a standalone
// function for headless CLI use.
func MapColorsToRoles(palette [16]string) template.ColorRoles {
	return template.ColorRoles{
		Background:          palette[0],
		Foreground:          palette[7],
		Black:               palette[0],
		Red:                 palette[1],
		Green:               palette[2],
		Yellow:              palette[3],
		Blue:                palette[4],
		Magenta:             palette[5],
		Cyan:                palette[6],
		White:               palette[7],
		BrightBlack:         palette[8],
		BrightRed:           palette[9],
		BrightGreen:         palette[10],
		BrightYellow:        palette[11],
		BrightBlue:          palette[12],
		BrightMagenta:       palette[13],
		BrightCyan:          palette[14],
		BrightWhite:         palette[15],
		Accent:              palette[4],
		Cursor:              palette[7],
		SelectionForeground: palette[0],
		SelectionBackground: palette[7],
	}
}

// ReadColorsToml reads an Aether/Omarchy colors.toml file and returns the
// color variables map. It supports both quoted string values ("#rrggbb")
// and bare hex values.
func ReadColorsToml(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read colors.toml: %w", err)
	}

	vars := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Strip quotes if present
		value = strings.Trim(value, `"'`)
		vars[key] = value
	}
	return vars, nil
}

// PaletteFromVars extracts a 16-color ANSI palette from a colors.toml-style
// variable map. It uses color0–color15 keys if present, otherwise falls back
// to semantic role names.
func PaletteFromVars(vars map[string]string) [16]string {
	var palette [16]string
	for i := 0; i < 16; i++ {
		key := fmt.Sprintf("color%d", i)
		if v := vars[key]; v != "" {
			palette[i] = v
			continue
		}
		// Fallback to semantic names
		switch i {
		case 0:
			palette[i] = vars["background"]
		case 1:
			palette[i] = vars["red"]
		case 2:
			palette[i] = vars["green"]
		case 3:
			palette[i] = vars["yellow"]
		case 4:
			palette[i] = vars["blue"]
		case 5:
			palette[i] = vars["magenta"]
		case 6:
			palette[i] = vars["cyan"]
		case 7:
			palette[i] = vars["foreground"]
		case 8:
			palette[i] = vars["bright_black"]
		case 9:
			palette[i] = vars["bright_red"]
		case 10:
			palette[i] = vars["bright_green"]
		case 11:
			palette[i] = vars["bright_yellow"]
		case 12:
			palette[i] = vars["bright_blue"]
		case 13:
			palette[i] = vars["bright_magenta"]
		case 14:
			palette[i] = vars["bright_cyan"]
		case 15:
			palette[i] = vars["bright_white"]
		}
		if palette[i] == "" {
			palette[i] = "#000000"
		}
	}
	return palette
}

// PaletteToVars converts a 16-color palette + extended colors into a
// colors.toml-style variable map.
func PaletteToVars(palette [16]string, extended map[string]string) map[string]string {
	vars := make(map[string]string, 24)
	vars["background"] = palette[0]
	vars["foreground"] = palette[7]
	for i := 0; i < 16; i++ {
		vars[fmt.Sprintf("color%d", i)] = palette[i]
	}
	semantic := []string{
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"bright_black", "bright_red", "bright_green", "bright_yellow",
		"bright_blue", "bright_magenta", "bright_cyan", "bright_white",
	}
	for i, name := range semantic {
		vars[name] = palette[i]
	}
	for k, v := range extended {
		vars[k] = v
	}
	return vars
}

// Hex to int helpers (used by template engine)
func mustParseHex(hex string) (r, g, b uint8) {
	s := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) == 6 {
		ri, _ := strconv.ParseUint(s[0:2], 16, 8)
		gi, _ := strconv.ParseUint(s[2:4], 16, 8)
		bi, _ := strconv.ParseUint(s[4:6], 16, 8)
		return uint8(ri), uint8(gi), uint8(bi)
	}
	return 0, 0, 0
}
