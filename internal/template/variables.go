// Package template renders application configuration templates from a color
// palette. Templates use {name} placeholders with optional format modifiers
// ({name.rgb}, {name.strip}, {name.rgba:0.5}, {name.yaru}).
package template

import (
	"fmt"
	"sort"

	"aether/internal/color"
)

// ColorRoles maps the 16-slot ANSI palette onto semantic roles. Additional
// roles (accent, cursor, selection) may be overridden per-app or through
// extended colors.
type ColorRoles struct {
	Background          string
	Foreground          string
	Black               string
	Red                 string
	Green               string
	Yellow              string
	Blue                string
	Magenta             string
	Cyan                string
	White               string
	BrightBlack         string
	BrightRed           string
	BrightGreen         string
	BrightYellow        string
	BrightBlue          string
	BrightMagenta       string
	BrightCyan          string
	BrightWhite         string
	Accent              string
	Cursor              string
	SelectionForeground string
	SelectionBackground string
}

// rolesMap converts the roles struct into the base variable map.
func rolesMap(r ColorRoles, lightMode bool) map[string]string {
	themeType := "dark"
	if lightMode {
		themeType = "light"
	}
	return map[string]string{
		"theme_type":           themeType,
		"background":           r.Background,
		"foreground":           r.Foreground,
		"bg":                   r.Background,
		"fg":                   r.Foreground,
		"black":                r.Black,
		"red":                  r.Red,
		"green":                r.Green,
		"yellow":               r.Yellow,
		"blue":                 r.Blue,
		"magenta":              r.Magenta,
		"cyan":                 r.Cyan,
		"white":                r.White,
		"bright_black":         r.BrightBlack,
		"bright_red":           r.BrightRed,
		"bright_green":         r.BrightGreen,
		"bright_yellow":        r.BrightYellow,
		"bright_blue":          r.BrightBlue,
		"bright_magenta":       r.BrightMagenta,
		"bright_cyan":          r.BrightCyan,
		"bright_white":         r.BrightWhite,
		"accent":               r.Accent,
		"cursor":               r.Cursor,
		"selection_foreground": r.SelectionForeground,
		"selection_background": r.SelectionBackground,
		"selection":            r.SelectionBackground,
	}
}

// BuildVariables builds the complete template variable map: base roles,
// derived shades (dark_bg, lighter_bg, muted, orange, brown, ...) and the
// ANSI color0-15 slots. Extended colors may override any base variable.
func BuildVariables(roles ColorRoles, lightMode bool, extendedColors map[string]string) map[string]string {
	vars := rolesMap(roles, lightMode)

	// ANSI slot names.
	ansi := [16]string{
		roles.Black, roles.Red, roles.Green, roles.Yellow,
		roles.Blue, roles.Magenta, roles.Cyan, roles.White,
		roles.BrightBlack, roles.BrightRed, roles.BrightGreen, roles.BrightYellow,
		roles.BrightBlue, roles.BrightMagenta, roles.BrightCyan, roles.BrightWhite,
	}
	for i, c := range ansi {
		vars[fmt.Sprintf("color%d", i)] = c
	}

	// Derived shades.
	vars["dark_bg"] = color.DarkenHex(roles.Background, 25)
	vars["darker_bg"] = color.DarkenHex(roles.Background, 45)
	vars["lighter_bg"] = color.LightenHex(roles.Background, 12)
	vars["dark_fg"] = color.DarkenHex(roles.Foreground, 25)
	vars["light_fg"] = color.LightenHex(roles.Foreground, 15)
	vars["bright_fg"] = color.LightenHex(roles.Foreground, 25)
	vars["muted"] = color.BlendHex(roles.Foreground, roles.Background, 45)
	vars["selection"] = roles.SelectionBackground
	vars["orange"] = color.HueRotateHex(roles.Yellow, -25)
	vars["brown"] = color.DarkenHex(roles.Yellow, 35)

	// User overrides win over derived defaults.
	for k, v := range extendedColors {
		if v != "" {
			vars[k] = v
		}
	}
	return vars
}

// RecomputeDerived recomputes the derived variables in an existing map after
// base roles were overridden (e.g. per-app role overrides). The supplied map
// is modified in place and also returned.
func RecomputeDerived(vars map[string]string) map[string]string {
	bg, okBg := vars["background"]
	fg, okFg := vars["foreground"]
	yellow, okYellow := vars["yellow"]
	if okBg && color.IsHexColor(bg) {
		vars["dark_bg"] = color.DarkenHex(bg, 25)
		vars["darker_bg"] = color.DarkenHex(bg, 45)
		vars["lighter_bg"] = color.LightenHex(bg, 12)
	}
	if okFg && color.IsHexColor(fg) {
		vars["dark_fg"] = color.DarkenHex(fg, 25)
		vars["light_fg"] = color.LightenHex(fg, 15)
		vars["bright_fg"] = color.LightenHex(fg, 25)
	}
	if okFg && okBg && color.IsHexColor(fg) && color.IsHexColor(bg) {
		vars["muted"] = color.BlendHex(fg, bg, 45)
	}
	if okYellow && color.IsHexColor(yellow) {
		vars["orange"] = color.HueRotateHex(yellow, -25)
		vars["brown"] = color.DarkenHex(yellow, 35)
	}
	return vars
}

// SortedKeys returns the variable names in sorted order (helper for output).
func SortedKeys(vars map[string]string) []string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
