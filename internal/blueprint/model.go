// Package blueprint models saved themes ("blueprints") — the serializable
// palette state plus metadata — and provides importers for Base16 and
// colors.toml formats.
package blueprint

import (
	"aether/internal/icontheme"
)

// Blueprint is a saved theme. Name is the human-facing title and also the
// file slug on disk; Timestamp records the save time.
type Blueprint struct {
	Name         string                       `json:"name"`
	Timestamp    int64                        `json:"timestamp"`
	Palette      PaletteData                  `json:"palette"`
	Adjustments  map[string]float64           `json:"adjustments,omitempty"`
	AppOverrides map[string]map[string]string `json:"appOverrides,omitempty"`
	IconTheme    *icontheme.Selection         `json:"iconTheme,omitempty"`
}

// PaletteData holds the palette part of a blueprint.
type PaletteData struct {
	Colors           []string            `json:"colors"`
	Wallpaper        string              `json:"wallpaper,omitempty"`
	WallpaperURL     string              `json:"wallpaperURL,omitempty"`
	WallpaperBlur    bool                `json:"wallpaperBlur,omitempty"`
	LightMode        bool                `json:"lightMode,omitempty"`
	Mode             string              `json:"mode,omitempty"` // "light"|"dark"|""
	LockedColors     []int               `json:"lockedColors,omitempty"`
	ExtendedColors   map[string]string   `json:"extendedColors,omitempty"`
	NativeColors     map[string]string   `json:"nativeColors,omitempty"`
	AdditionalImages []string            `json:"additionalImages,omitempty"`
	IconTheme        *icontheme.Selection `json:"iconTheme,omitempty"`
}

// IconThemeSelection resolves the effective icon theme selection: the
// top-level field wins, falling back to the palette's embedded selection.
// A missing selection yields the automatic default.
func (b *Blueprint) IconThemeSelection() (icontheme.Selection, error) {
	sel := b.IconTheme
	if sel == nil {
		sel = b.Palette.IconTheme
	}
	if sel == nil || sel.Mode == "" {
		return icontheme.Automatic(), nil
	}
	return icontheme.NormalizeSelection(*sel)
}

// SetIconThemeSelection stores a normalized selection at the top level.
func (b *Blueprint) SetIconThemeSelection(sel icontheme.Selection) error {
	normalized, err := icontheme.NormalizeSelection(sel)
	if err != nil {
		return err
	}
	b.IconTheme = &normalized
	return nil
}
