package winsys

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Registry value types used by the theme transaction.
const (
	registrySZ    uint32 = 1
	registryDWORD uint32 = 4
)

const (
	dwmKey             = `Software\Microsoft\Windows\DWM`
	explorerAccentKey  = `Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`
	personalizeKey     = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	chromePolicyKey    = `Software\Policies\Google\Chrome`
	chromiumPolicyKey  = `Software\Policies\Chromium`
	colorPrevalenceKey = "ColorPrevalence"
)

// registryValue is a single value in the atomic theme transaction. Keeping the
// value model platform-neutral makes the palette-to-registry mapping testable
// without Windows.
type registryValue struct {
	Path   string
	Name   string
	Type   uint32
	DWord  uint32
	String string
}

func dwordValue(path, name string, value uint32) registryValue {
	return registryValue{Path: path, Name: name, Type: registryDWORD, DWord: value}
}

func stringValue(path, name, value string) registryValue {
	return registryValue{Path: path, Name: name, Type: registrySZ, String: value}
}

// parseHexColor turns "#rrggbb" into its components.
func parseHexColor(hex string) (r, g, b uint8, err error) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid color %q", hex)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid color %q: %w", hex, err)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}

// normalizeHex returns a lower-case #rrggbb color, or "" when the input is not
// a six-digit hex color.
func normalizeHex(hex string) string {
	r, g, b, err := parseHexColor(hex)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// blendHex mixes fg toward bg. weight is the proportion of bg (0..1).
func blendHex(fg, bg string, weight float64) string {
	fr, fgG, fb, ferr := parseHexColor(fg)
	br, bgG, bb, berr := parseHexColor(bg)
	if ferr != nil || berr != nil {
		return normalizeHex(fg)
	}
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	mix := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a)*(1-weight) + float64(b)*weight))
	}
	return fmt.Sprintf("#%02x%02x%02x", mix(fr, br), mix(fgG, bgG), mix(fb, bb))
}

// abgrColor converts #rrggbb to the 0xAABBGGRR layout used by Explorer and the
// DWM AccentColor values.
func abgrColor(hex string) (uint32, error) {
	r, g, b, err := parseHexColor(hex)
	if err != nil {
		return 0, err
	}
	return uint32(0xFF)<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r), nil
}

// argbColor converts #rrggbb to the 0xAARRGGBB layout used by DWM
// ColorizationColor.
func argbColor(hex string, alpha uint8) (uint32, error) {
	r, g, b, err := parseHexColor(hex)
	if err != nil {
		return 0, err
	}
	return uint32(alpha)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b), nil
}

// accentThemeValues maps an accent and background color to the Windows accent
// keys. Windows stores DWM colors as 0xAARRGGBB and accent colors as
// 0xAABBGGRR, so the channel order differs between the two.
func accentThemeValues(vars map[string]string) ([]registryValue, error) {
	accent := normalizeHex(vars["accent"])
	if accent == "" {
		return nil, fmt.Errorf("invalid accent color %q", vars["accent"])
	}
	argb, err := argbColor(accent, 0xC4)
	if err != nil {
		return nil, err
	}
	abgr, err := abgrColor(accent)
	if err != nil {
		return nil, err
	}

	// Inactive title bars keep the accent hue but recede toward the theme
	// background, so unfocused windows do not look disabled.
	background := normalizeHex(vars["background"])
	if background == "" {
		background = "#000000"
	}
	inactive, err := abgrColor(blendHex(accent, background, 0.35))
	if err != nil {
		return nil, err
	}

	return []registryValue{
		dwordValue(dwmKey, colorPrevalenceKey, 1),
		dwordValue(dwmKey, "ColorizationColor", argb),
		dwordValue(dwmKey, "ColorizationAfterglow", argb),
		dwordValue(dwmKey, "AccentColor", abgr),
		dwordValue(dwmKey, "AccentColorInactive", inactive),
		dwordValue(dwmKey, "ColorizationColorBalance", 78),
		dwordValue(dwmKey, "ColorizationBlurBalance", 49),
		dwordValue(explorerAccentKey, "AccentColorMenu", abgr),
		dwordValue(explorerAccentKey, "StartColorMenu", abgr),
	}, nil
}

// systemThemeValues returns every Windows shell value for a theme. The
// returned slice is written in one registry transaction: personalization,
// accent/title-bar colors, and Chrome's accent policy all commit together (or
// not at all).
func systemThemeValues(vars map[string]string, lightMode bool) ([]registryValue, error) {
	appsLight := uint32(1)
	systemLight := uint32(1)
	if !lightMode {
		appsLight = 0
		systemLight = 0
	}

	values := []registryValue{
		dwordValue(personalizeKey, colorPrevalenceKey, 1),
		dwordValue(personalizeKey, "AppsUseLightTheme", appsLight),
		dwordValue(personalizeKey, "SystemUsesLightTheme", systemLight),
		dwordValue(personalizeKey, "EnableTransparency", 1),
	}

	accentValues, err := accentThemeValues(vars)
	if err != nil {
		if strings.TrimSpace(vars["accent"]) != "" {
			return nil, err
		}
	} else {
		values = append(values, accentValues...)
	}

	values = append(values, chromeThemeValues(vars)...)
	return values, nil
}

// chromeThemeValues configures Chrome's policy-backed theme color. Chrome reads
// these per-profile policies from HKCU without elevation, and dynamically
// refreshes them when the policy store changes. Chromium uses the same policy
// under its own vendor key.
func chromeThemeValues(vars map[string]string) []registryValue {
	accent := normalizeHex(vars["accent"])
	if accent == "" {
		return nil
	}
	return []registryValue{
		stringValue(chromePolicyKey, "BrowserThemeColor", accent),
		stringValue(chromiumPolicyKey, "BrowserThemeColor", accent),
	}
}
