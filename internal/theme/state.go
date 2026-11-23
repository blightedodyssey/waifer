package theme

import (
	"aether/internal/color"
	"aether/internal/icontheme"
	"aether/internal/template"
)

// ThemeState holds all mutable state for the current theme.
type ThemeState struct {
	Palette       [16]string `json:"palette"`
	BasePalette   [16]string `json:"basePalette"`
	WallpaperPath string     `json:"wallpaperPath"`
	WallpaperBlur bool       `json:"wallpaperBlur"`
	// OriginalWallpaperPath is the unblurred source image when
	// WallpaperPath is a derived variant (e.g. the heavy-blur JPEG). Both
	// are copied into the theme's backgrounds so the desktop cycler can
	// switch between them. Empty when WallpaperPath is the source itself.
	OriginalWallpaperPath string                       `json:"originalWallpaperPath"`
	LightMode             bool                         `json:"lightMode"`
	LockedColors          map[int]bool                 `json:"lockedColors"`
	Adjustments           color.Adjustments            `json:"adjustments"`
	ColorRoles            template.ColorRoles          `json:"colorRoles"`
	ExtendedColors        map[string]string            `json:"extendedColors"`
	NativeColors          map[string]string            `json:"nativeColors"`
	ExtractionMode        string                       `json:"extractionMode"`
	AdditionalImages      []string                     `json:"additionalImages"`
	AppOverrides          map[string]map[string]string `json:"appOverrides"`
	IconTheme             icontheme.Selection          `json:"iconTheme"`
}

// DefaultPalette is the Waifer default 16-color palette: deep slate-violet
// dark theme with violet/gold accents, high contrast out of the box.
var DefaultPalette = [16]string{
	"#0f0e17", // 0: Background/Black
	"#ff5c8a", // 1: Red
	"#5ee6a0", // 2: Green
	"#e5a93b", // 3: Yellow
	"#7f5af0", // 4: Blue
	"#c792ea", // 5: Magenta
	"#4fd6be", // 6: Cyan
	"#f4f4f8", // 7: White
	"#161522", // 8: Bright Black
	"#ff7ba3", // 9: Bright Red
	"#7df0b3", // 10: Bright Green
	"#f0c25c", // 11: Bright Yellow
	"#9a7ff5", // 12: Bright Blue
	"#d8a8f0", // 13: Bright Magenta
	"#6fe3d0", // 14: Bright Cyan
	"#ffffff", // 15: Bright White
}

// NewThemeState returns a ThemeState initialised with default values.
func NewThemeState() *ThemeState {
	s := &ThemeState{
		Palette:          DefaultPalette,
		BasePalette:      DefaultPalette,
		LockedColors:     make(map[int]bool),
		Adjustments:      color.DefaultAdjustments(),
		ExtendedColors:   make(map[string]string),
		NativeColors:     make(map[string]string),
		ExtractionMode:   "auto",
		AdditionalImages: []string{},
		AppOverrides:     make(map[string]map[string]string),
		IconTheme:        icontheme.Automatic(),
	}
	s.ColorRoles = s.buildColorRoles()
	return s
}

// SetPalette replaces both the display palette and the base palette, then rebuilds color roles.
func (s *ThemeState) SetPalette(colors [16]string) {
	s.Palette = colors
	s.BasePalette = colors
	s.ColorRoles = s.buildColorRoles()
}

// SetAdjustedPalette replaces only the display palette (keeps base unchanged).
func (s *ThemeState) SetAdjustedPalette(colors [16]string) {
	s.Palette = colors
	s.ColorRoles = s.buildColorRoles()
}

// SetColor sets a single palette colour at the given index (0-15) and
// rebuilds color roles.
func (s *ThemeState) SetColor(index int, hex string) {
	if index < 0 || index > 15 {
		return
	}
	s.Palette[index] = hex
	s.ColorRoles = s.buildColorRoles()
}

// StateSnapshot is a JSON-safe representation of ThemeState for sending to
// the Wails frontend.
type StateSnapshot struct {
	Palette          [16]string                   `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	LockedColors     map[int]bool                 `json:"lockedColors"`
	ColorRoles       template.ColorRoles          `json:"colorRoles"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	ExtractionMode   string                       `json:"extractionMode"`
	AdditionalImages []string                     `json:"additionalImages"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// Snapshot returns a copy of the current state suitable for Wails binding.
func (s *ThemeState) Snapshot() StateSnapshot {
	locked := make(map[int]bool, len(s.LockedColors))
	for k, v := range s.LockedColors {
		locked[k] = v
	}

	ext := make(map[string]string, len(s.ExtendedColors))
	for k, v := range s.ExtendedColors {
		ext[k] = v
	}
	native := make(map[string]string, len(s.NativeColors))
	for k, v := range s.NativeColors {
		native[k] = v
	}

	overrides := make(map[string]map[string]string, len(s.AppOverrides))
	for app, colors := range s.AppOverrides {
		m := make(map[string]string, len(colors))
		for k, v := range colors {
			m[k] = v
		}
		overrides[app] = m
	}

	images := make([]string, len(s.AdditionalImages))
	copy(images, s.AdditionalImages)

	return StateSnapshot{
		Palette:          s.Palette,
		WallpaperPath:    s.WallpaperPath,
		WallpaperBlur:    s.WallpaperBlur,
		LightMode:        s.LightMode,
		LockedColors:     locked,
		ColorRoles:       s.ColorRoles,
		ExtendedColors:   ext,
		NativeColors:     native,
		ExtractionMode:   s.ExtractionMode,
		AdditionalImages: images,
		AppOverrides:     overrides,
		IconTheme:        s.IconTheme,
	}
}

// semanticNames maps palette index to semantic color role name.
var semanticNames = [16]string{
	"black", "red", "green", "yellow",
	"blue", "magenta", "cyan", "white",
	"bright_black", "bright_red", "bright_green", "bright_yellow",
	"bright_blue", "bright_magenta", "bright_cyan", "bright_white",
}

// buildColorRoles constructs a ColorRoles from the current palette and
// extended color overrides.
func (s *ThemeState) buildColorRoles() template.ColorRoles {
	roles := template.ColorRoles{
		Background:    s.Palette[0],
		Foreground:    s.Palette[7],
		Black:         s.Palette[0],
		Red:           s.Palette[1],
		Green:         s.Palette[2],
		Yellow:        s.Palette[3],
		Blue:          s.Palette[4],
		Magenta:       s.Palette[5],
		Cyan:          s.Palette[6],
		White:         s.Palette[7],
		BrightBlack:   s.Palette[8],
		BrightRed:     s.Palette[9],
		BrightGreen:   s.Palette[10],
		BrightYellow:  s.Palette[11],
		BrightBlue:    s.Palette[12],
		BrightMagenta: s.Palette[13],
		BrightCyan:    s.Palette[14],
		BrightWhite:   s.Palette[15],
	}

	// Extended colors: use overrides or auto-derive from palette
	if v, ok := s.ExtendedColors["accent"]; ok && v != "" {
		roles.Accent = v
	} else {
		roles.Accent = s.Palette[4] // blue
	}

	if v, ok := s.ExtendedColors["cursor"]; ok && v != "" {
		roles.Cursor = v
	} else {
		roles.Cursor = s.Palette[7] // foreground/white
	}

	if v, ok := s.ExtendedColors["selection_foreground"]; ok && v != "" {
		roles.SelectionForeground = v
	} else {
		roles.SelectionForeground = s.Palette[0] // background
	}

	if v, ok := s.ExtendedColors["selection_background"]; ok && v != "" {
		roles.SelectionBackground = v
	} else {
		roles.SelectionBackground = s.Palette[7] // foreground/white
	}

	return roles
}
