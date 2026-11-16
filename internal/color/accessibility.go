package color

import (
	"fmt"
	"math"
)

// getRelativeLuminance computes the WCAG relative luminance of a hex color.
func getRelativeLuminance(hex string) float64 {
	c := HexToRGB(hex)
	return 0.2126*srgbToLinear(float64(c.R)/255) +
		0.7152*srgbToLinear(float64(c.G)/255) +
		0.0722*srgbToLinear(float64(c.B)/255)
}

// ContrastRatio computes the WCAG 2.x contrast ratio between two colors.
// Result range: 1.0 (identical) to 21.0 (black on white).
func ContrastRatio(hex1, hex2 string) float64 {
	if !IsHexColor(hex1) || !IsHexColor(hex2) {
		return 1
	}
	l1 := getRelativeLuminance(hex1)
	l2 := getRelativeLuminance(hex2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// WCAGGrade maps a contrast ratio to its WCAG grade label.
func WCAGGrade(ratio float64) string {
	switch {
	case ratio >= 7:
		return "AAA"
	case ratio >= 4.5:
		return "AA"
	case ratio >= 3:
		return "AA Large"
	default:
		return "Fail"
	}
}

// SuggestReadableForeground returns a foreground (same hue) with at least the
// target contrast against the given background.
func SuggestReadableForeground(background, preferred string, target float64) string {
	if ContrastRatio(background, preferred) >= target {
		return preferred
	}
	lch := HexToOKLCH(preferred)
	bg := HexToOKLCH(background)
	// Push lightness away from the background until the target is met.
	for step := 0; step < 64; step++ {
		if bg.L < 0.5 {
			lch.L = clampL(lch.L + 0.02)
		} else {
			lch.L = clampL(lch.L - 0.02)
		}
		candidate := OKLCHToHex(lch)
		if ContrastRatio(background, candidate) >= target {
			return candidate
		}
		if lch.L == 0 || lch.L == 1 {
			break
		}
	}
	if bg.L < 0.5 {
		return "#ffffff"
	}
	return "#000000"
}

// IsDarkColor reports whether a hex color is perceptually dark.
func IsDarkColor(hex string) bool {
	if !IsHexColor(hex) {
		return true
	}
	return HexToOKLCH(hex).L < 0.5
}

// GradingSuggestions returns human-readable hints for improving a pair's
// contrast, given the ratio.
func GradingSuggestions(ratio float64) []string {
	if ratio >= 4.5 {
		return []string{"contrast is sufficient for body text"}
	}
	var out []string
	out = append(out, fmtRatio(ratio))
	if ratio < 3 {
		out = append(out, "darken the background or lighten the foreground")
	} else {
		out = append(out, "small lightness nudge will reach AA")
	}
	return out
}

func fmtRatio(ratio float64) string {
	return "ratio " + trimFloat(ratio)
}

func trimFloat(v float64) string {
	rounded := math.Round(v*100) / 100
	s := fmt.Sprintf("%.2f", rounded)
	return s
}
