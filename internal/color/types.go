// Package color implements color parsing, conversion (RGB/HSL/OKLab/OKLCH),
// adjustment pipelines and WCAG accessibility helpers.
package color

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGB holds 8-bit sRGB channels (floats for interpolation math).
type RGB struct {
	R, G, B float64
}

// RGBA holds 8-bit sRGB channels plus 0-1 alpha.
type RGBA struct {
	R, G, B int
	A       float64
}

// HSL holds hue (0-360), saturation (0-1) and lightness (0-1).
type HSL struct {
	H float64
	S float64
	L float64
}

// OKLab holds the OKLab perceptual components.
type OKLab struct {
	L, A, B float64
}

// OKLCH holds lightness (0-1), chroma and hue (degrees).
type OKLCH struct {
	L, C, H float64
}

// IsHexColor reports whether s is a valid #rgb / #rrggbb color.
func IsHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return false
	}
	h := s[1:]
	if len(h) != 3 && len(h) != 6 {
		return false
	}
	for _, c := range h {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// normalizeHex ensures a leading # and lowercase digits.
func normalizeHex(hex string) string {
	hex = strings.TrimSpace(hex)
	if !strings.HasPrefix(hex, "#") {
		hex = "#" + hex
	}
	return strings.ToLower(hex)
}

// expandHex expands 3-digit hex to 6 digits.
func expandHex(hex string) string {
	hex = normalizeHex(hex)
	h := hex[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	return "#" + h
}

// HexToRGB parses #rgb/#rrggbb into 8-bit channels.
func HexToRGB(hex string) RGB {
	h := expandHex(hex)
	r, _ := strconv.ParseUint(h[1:3], 16, 32)
	g, _ := strconv.ParseUint(h[3:5], 16, 32)
	b, _ := strconv.ParseUint(h[5:7], 16, 32)
	return RGB{R: float64(r), G: float64(g), B: float64(b)}
}

// HexToRGBA renders hex as an rgba() string with the given 0-1 alpha,
// e.g. "rgba(210, 15, 79, 1)".
func HexToRGBA(hex string, alpha float64) string {
	c := HexToRGB(hex)
	return fmt.Sprintf("rgba(%.0f, %.0f, %.0f, %s)", c.R, c.G, c.B,
		strconv.FormatFloat(alpha, 'g', -1, 64))
}

// RGBToHex formats 8-bit channels as #rrggbb.
func RGBToHex(r, g, b float64) string {
	return fmt.Sprintf("#%02x%02x%02x",
		clampByte(int(math.Round(r))),
		clampByte(int(math.Round(g))),
		clampByte(int(math.Round(b))))
}

// HexToRGBString renders hex as comma-separated channels, e.g. "210, 15, 79".
func HexToRGBString(hex string) string {
	c := HexToRGB(hex)
	return fmt.Sprintf("%.0f, %.0f, %.0f", c.R, c.G, c.B)
}

// HexToRGBSpaceString renders hex as space-separated channels, e.g. "210 15 79".
func HexToRGBSpaceString(hex string) string {
	c := HexToRGB(hex)
	return fmt.Sprintf("%.0f %.0f %.0f", c.R, c.G, c.B)
}

// clampByte constrains a value to 0-255.
func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
