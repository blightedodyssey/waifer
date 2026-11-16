// Package theme holds the theme state machine and the Writer that renders
// templates and applies results to the system.
package theme

import (
	"aether/internal/color"
	"aether/internal/icontheme"
	"aether/internal/template"
)

// ThemeState is the complete editable theme state shared between the GUI,
// CLI and IPC surfaces.
type ThemeState struct {
	// Palette is the current (possibly adjusted) 16-color ANSI palette.
	Palette [16]string
	// BasePalette is the pristine extraction result; adjustments re-apply
	// from here so the pipeline stays idempotent.
	BasePalette [16]string

	WallpaperPath string
	WallpaperBlur bool
	LightMode     bool
	ExtractionMode string

	ExtendedColors   map[string]string
	NativeColors     map[string]string
	AppOverrides     map[string]map[string]string
	AdditionalImages []string
	Adjustments      color.Adjustments
	IconTheme        icontheme.Selection

	// ColorRoles is the semantic role mapping derived from Palette.
	ColorRoles template.ColorRoles
}

// DefaultPalette is the built-in seed palette (Catppuccin Macchiato).
var DefaultPalette = [16]string{
	"#24273a", // black / background
	"#ed8796", // red
	"#a6da95", // green
	"#eed49f", // yellow
	"#8aadf4", // blue
	"#f5bde6", // magenta
	"#8bd5ca", // cyan
	"#cad3f5", // white / foreground
	"#5b6078", // bright black
	"#f5a2a7", // bright red
	"#b7e0ae", // bright green
	"#f2ddaa", // bright yellow
	"#a5c2f6", // bright blue
	"#f7c8e8", // bright magenta
	"#a8dfd6", // bright cyan
	"#e2e7f2", // bright white
}

// NewThemeState returns a state seeded with DefaultPalette and initialized
// override maps.
func NewThemeState() *ThemeState {
	return &ThemeState{
		Palette:        DefaultPalette,
		BasePalette:    DefaultPalette,
		ExtractionMode: "normal",
		ExtendedColors: make(map[string]string),
		NativeColors:   make(map[string]string),
		AppOverrides:   make(map[string]map[string]string),
		IconTheme:      icontheme.Automatic(),
	}
}

// SetPalette stores a fresh palette as both the current and base palette.
func (s *ThemeState) SetPalette(p [16]string) {
	s.Palette = p
	s.BasePalette = p
}

// SetAdjustedPalette updates only the live palette, keeping the pristine
// base palette untouched so adjustment sliders re-apply from the source.
func (s *ThemeState) SetAdjustedPalette(p [16]string) {
	s.Palette = p
}

// SetColor replaces one palette slot in both the live and base palette.
func (s *ThemeState) SetColor(index int, value string) {
	if index < 0 || index >= 16 {
		return
	}
	s.Palette[index] = value
	s.BasePalette[index] = value
}

// StateSnapshot is the JSON-friendly view of ThemeState returned to the
// frontend by GetInitialState.
type StateSnapshot struct {
	Palette          []string                     `json:"palette"`
	BasePalette      []string                     `json:"basePalette,omitempty"`
	WallpaperPath    string                       `json:"wallpaper"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	ExtractionMode   string                       `json:"mode"`
	ExtendedColors   map[string]string            `json:"extendedColors,omitempty"`
	NativeColors     map[string]string            `json:"nativeColors,omitempty"`
	AppOverrides     map[string]map[string]string `json:"appOverrides,omitempty"`
	AdditionalImages []string                     `json:"additionalImages,omitempty"`
	Adjustments      color.Adjustments            `json:"adjustments,omitempty"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// Snapshot copies the state into a serializable snapshot.
func (s *ThemeState) Snapshot() StateSnapshot {
	snap := StateSnapshot{
		Palette:          s.Palette[:],
		BasePalette:      s.BasePalette[:],
		WallpaperPath:    s.WallpaperPath,
		WallpaperBlur:    s.WallpaperBlur,
		LightMode:        s.LightMode,
		ExtractionMode:   s.ExtractionMode,
		ExtendedColors:   make(map[string]string, len(s.ExtendedColors)),
		NativeColors:     make(map[string]string, len(s.NativeColors)),
		AppOverrides:     make(map[string]map[string]string, len(s.AppOverrides)),
		AdditionalImages: append([]string(nil), s.AdditionalImages...),
		Adjustments:      s.Adjustments,
		IconTheme:        s.IconTheme,
	}
	for k, v := range s.ExtendedColors {
		snap.ExtendedColors[k] = v
	}
	for k, v := range s.NativeColors {
		snap.NativeColors[k] = v
	}
	for k, v := range s.AppOverrides {
		inner := make(map[string]string, len(v))
		for ik, iv := range v {
			inner[ik] = iv
		}
		snap.AppOverrides[k] = inner
	}
	return snap
}
