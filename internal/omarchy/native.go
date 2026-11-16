// Package omarchy integrates with the Omarchy desktop CLI: detecting the
// installation, listing and activating themes, and parsing the theme TOML
// formats.
package omarchy

import (
	"fmt"
	"strings"
)

// nativeColorKeys are the Omarchy-native color tokens Aether understands.
var nativeColorKeys = map[string]bool{
	"background": true, "foreground": true, "accent": true,
	"selection": true, "cursor": true,
	"dark_bg": true, "darker_bg": true, "lighter_bg": true,
	"dark_fg": true, "light_fg": true, "bright_fg": true,
	"muted": true, "orange": true, "brown": true,
}

// ValidateNativeColors reports which native color keys are recognized.
func ValidateNativeColors(colors map[string]string) error {
	for key := range colors {
		if !nativeColorKeys[key] {
			return fmt.Errorf("unknown native color key: %s", key)
		}
	}
	return nil
}

// ParseNativeColors normalizes a native color map (lowercase keys, hex
// values only).
func ParseNativeColors(colors map[string]string) map[string]string {
	out := make(map[string]string, len(colors))
	for k, v := range colors {
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		if strings.HasPrefix(v, "#") {
			v = strings.ToLower(v)
		}
		out[k] = v
	}
	return out
}

// omarchyOverrideApps lists applications Omarchy's native theme loader
// manages, so Aether skips emitting files for them on such systems.
var omarchyOverrideApps = map[string]bool{
	"kitty": true, "hyprland": true, "waybar": true, "foot": true,
	"alacritty": true, "ghostty": true, "zed": true, "neovim": true,
	"walker": true, "wofi": true, "mako": true, "warp": true,
	"btop": true, "chromium": true, "hyprlock": true, "swayosd": true,
}

// SupportsThemeOverride reports whether Omarchy's native loader handles the
// application (Aether then leaves it to Omarchy).
func SupportsThemeOverride(app string) bool {
	return omarchyOverrideApps[app]
}
